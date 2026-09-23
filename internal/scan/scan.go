package scan

import (
	"context"
	"errors"
	"fmt"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/teamleaderleo/gh-tidy-branches/internal/githubapi"
)

type API interface {
	GetRepository(context.Context, string) (githubapi.Repository, error)
	ListBranches(context.Context, string) ([]githubapi.Branch, error)
	GetBranch(context.Context, string, string) (githubapi.Branch, error)
	ListOpenPullRequests(context.Context, string) ([]githubapi.PullRequest, error)
	ListAllClosedPullRequests(context.Context, string) ([]githubapi.PullRequest, error)
	ListClosedPullRequestsForHead(context.Context, string, string) ([]githubapi.PullRequest, error)
	DeleteBranch(context.Context, string, string) error
}

// ForkAPI is the extra lookup Apply needs to revalidate a fork branch against its parent.
type ForkAPI interface {
	ListClosedPullRequestsForForkHead(context.Context, string, string, string) ([]githubapi.PullRequest, error)
}

type RestoreAPI interface {
	GetBranch(context.Context, string, string) (githubapi.Branch, error)
	CreateBranch(context.Context, string, string, string) error
}

type Candidate struct {
	Repository  string    `json:"repository"`
	Branch      string    `json:"branch"`
	PullRequest int       `json:"pull_request"`
	HeadSHA     string    `json:"head_sha"`
	MergedAt    time.Time `json:"merged_at"`
	// PullRequestRepository names the parent repository when the merged pull request lives
	// there rather than in Repository. Empty means Repository itself.
	PullRequestRepository string `json:"pull_request_repository,omitempty"`
}

type Result struct {
	Repository          string      `json:"repository"`
	DefaultBranch       string      `json:"default_branch"`
	BranchCount         int         `json:"branch_count"`
	OpenPRCount         int         `json:"open_pull_request_count"`
	ElapsedMilliseconds int64       `json:"elapsed_ms"`
	Candidates          []Candidate `json:"candidates"`
}

type ApplyStatus string

const (
	StatusDeleted ApplyStatus = "deleted"
	StatusSkipped ApplyStatus = "skipped"
	StatusFailed  ApplyStatus = "failed"
)

type ApplyResult struct {
	Candidate Candidate   `json:"candidate"`
	Status    ApplyStatus `json:"status"`
	Reason    string      `json:"reason,omitempty"`
}

type RestoreCandidate struct {
	Repository string `json:"repository"`
	Branch     string `json:"branch"`
	SHA        string `json:"sha"`
}

type RestoreStatus string

const (
	StatusRestored       RestoreStatus = "restored"
	StatusAlreadyPresent RestoreStatus = "already_present"
	StatusRestoreSkipped RestoreStatus = "skipped"
	StatusRestoreFailed  RestoreStatus = "failed"
)

type RestoreResult struct {
	Candidate RestoreCandidate `json:"candidate"`
	Status    RestoreStatus    `json:"status"`
	Reason    string           `json:"reason,omitempty"`
}

func Repository(ctx context.Context, api API, fullName string) (Result, error) {
	started := time.Now()
	repository, err := api.GetRepository(ctx, fullName)
	if err != nil {
		return Result{}, err
	}
	if repository.Archived {
		return Result{}, fmt.Errorf("%s is archived", fullName)
	}

	var branches []githubapi.Branch
	var openPRs []githubapi.PullRequest
	var closed []githubapi.PullRequest
	var errs [3]error
	var wg sync.WaitGroup
	wg.Add(3)
	go func() { defer wg.Done(); branches, errs[0] = api.ListBranches(ctx, fullName) }()
	go func() { defer wg.Done(); openPRs, errs[1] = api.ListOpenPullRequests(ctx, fullName) }()
	go func() { defer wg.Done(); closed, errs[2] = api.ListAllClosedPullRequests(ctx, fullName) }()
	wg.Wait()
	for _, scanErr := range errs {
		if scanErr != nil {
			return Result{}, scanErr
		}
	}

	var upstreamOpen, upstreamClosed []githubapi.PullRequest
	if repository.Parent != nil {
		var upstreamErrs [2]error
		wg.Add(2)
		go func() {
			defer wg.Done()
			upstreamOpen, upstreamErrs[0] = api.ListOpenPullRequests(ctx, repository.Parent.FullName)
		}()
		go func() {
			defer wg.Done()
			upstreamClosed, upstreamErrs[1] = api.ListAllClosedPullRequests(ctx, repository.Parent.FullName)
		}()
		wg.Wait()
		for _, scanErr := range upstreamErrs {
			if scanErr != nil {
				return Result{}, scanErr
			}
		}
	}

	result := EvaluateFork(repository, branches, openPRs, closed, upstreamOpen, upstreamClosed)
	result.ElapsedMilliseconds = time.Since(started).Milliseconds()
	return result, nil
}

func Evaluate(repository githubapi.Repository, branches []githubapi.Branch, openPRs, closedPRs []githubapi.PullRequest) Result {
	current := make(map[string]githubapi.Branch, len(branches))
	for _, branch := range branches {
		current[branch.Name] = branch
	}
	protectedByOpenPR := make(map[string]struct{})
	for _, pull := range openPRs {
		if pull.Head.Repo != nil && pull.Head.Repo.FullName == repository.FullName {
			protectedByOpenPR[pull.Head.Ref] = struct{}{}
		}
		protectedByOpenPR[pull.Base.Ref] = struct{}{}
	}
	latestClosed := make(map[string]githubapi.PullRequest)
	for _, pull := range closedPRs {
		if pull.Head.Repo == nil || pull.Head.Repo.FullName != repository.FullName {
			continue
		}
		existing, found := latestClosed[pull.Head.Ref]
		if !found || pull.Number > existing.Number {
			latestClosed[pull.Head.Ref] = pull
		}
	}
	candidates := make([]Candidate, 0)
	for branchName, pull := range latestClosed {
		if pull.MergedAt == nil || pull.Base.Ref != repository.DefaultBranch {
			continue
		}
		if branchName == repository.DefaultBranch {
			continue
		}
		if _, protected := protectedByOpenPR[branchName]; protected {
			continue
		}
		branch, exists := current[branchName]
		if !exists || branch.Protected || branch.SHA() == "" || branch.SHA() != pull.Head.SHA {
			continue
		}
		candidates = append(candidates, Candidate{Repository: repository.FullName, Branch: branchName, PullRequest: pull.Number, HeadSHA: pull.Head.SHA, MergedAt: *pull.MergedAt})
	}
	sortCandidates(candidates)
	return Result{Repository: repository.FullName, DefaultBranch: repository.DefaultBranch, BranchCount: len(branches), OpenPRCount: len(openPRs), Candidates: candidates}
}

// EvaluateFork extends Evaluate to a fork whose pull requests are opened against its parent.
//
// A branch becomes a candidate when its latest parent pull request from this fork merged into
// the parent's default branch at the branch's current SHA. An open parent pull request from this
// fork protects its head. A branch with closed pull requests in both repositories is skipped:
// numbers from two repositories cannot be ordered, so neither history can be trusted as latest.
func EvaluateFork(
	repository githubapi.Repository,
	branches []githubapi.Branch,
	openPRs, closedPRs, upstreamOpen, upstreamClosed []githubapi.PullRequest,
) Result {
	result := Evaluate(repository, branches, openPRs, closedPRs)
	if repository.Parent == nil {
		return result
	}
	upstream := *repository.Parent

	protected := openProtection(repository.FullName, openPRs)
	for _, pull := range upstreamOpen {
		if pull.Head.Repo != nil && pull.Head.Repo.FullName == repository.FullName {
			protected[pull.Head.Ref] = struct{}{}
		}
	}
	ownHistory := make(map[string]struct{})
	for _, pull := range closedPRs {
		if pull.Head.Repo != nil && pull.Head.Repo.FullName == repository.FullName {
			ownHistory[pull.Head.Ref] = struct{}{}
		}
	}
	latestUpstream := make(map[string]githubapi.PullRequest)
	for _, pull := range upstreamClosed {
		if pull.Head.Repo == nil || pull.Head.Repo.FullName != repository.FullName {
			continue
		}
		existing, found := latestUpstream[pull.Head.Ref]
		if !found || pull.Number > existing.Number {
			latestUpstream[pull.Head.Ref] = pull
		}
	}

	candidates := make([]Candidate, 0, len(result.Candidates))
	for _, candidate := range result.Candidates {
		_, open := protected[candidate.Branch]
		_, ambiguous := latestUpstream[candidate.Branch]
		if !open && !ambiguous {
			candidates = append(candidates, candidate)
		}
	}
	current := make(map[string]githubapi.Branch, len(branches))
	for _, branch := range branches {
		current[branch.Name] = branch
	}
	for branchName, pull := range latestUpstream {
		if pull.MergedAt == nil || pull.Base.Ref != upstream.DefaultBranch || branchName == repository.DefaultBranch {
			continue
		}
		if _, open := protected[branchName]; open {
			continue
		}
		if _, ambiguous := ownHistory[branchName]; ambiguous {
			continue
		}
		branch, exists := current[branchName]
		if !exists || branch.Protected || branch.SHA() == "" || branch.SHA() != pull.Head.SHA {
			continue
		}
		candidates = append(candidates, Candidate{
			Repository:            repository.FullName,
			Branch:                branchName,
			PullRequest:           pull.Number,
			HeadSHA:               pull.Head.SHA,
			MergedAt:              *pull.MergedAt,
			PullRequestRepository: upstream.FullName,
		})
	}
	sortCandidates(candidates)
	result.Candidates = candidates
	return result
}

func sortCandidates(candidates []Candidate) {
	sort.Slice(candidates, func(i, j int) bool {
		if candidates[i].MergedAt.Equal(candidates[j].MergedAt) {
			return candidates[i].Branch < candidates[j].Branch
		}
		return candidates[i].MergedAt.After(candidates[j].MergedAt)
	})
}

func latestClosedPullRequest(
	repository string,
	branchName string,
	pulls []githubapi.PullRequest,
) (githubapi.PullRequest, bool) {
	var latest githubapi.PullRequest
	found := false
	for _, pull := range pulls {
		if pull.Head.Ref != branchName || pull.Head.Repo == nil || pull.Head.Repo.FullName != repository {
			continue
		}
		if !found || pull.Number > latest.Number {
			latest = pull
			found = true
		}
	}
	return latest, found
}

func Apply(ctx context.Context, api API, repository string, candidates []Candidate, delay time.Duration) ([]ApplyResult, error) {
	results := make([]ApplyResult, 0, len(candidates))
	needsDelay := false
	for _, candidate := range candidates {
		if candidate.Repository != repository {
			results = append(results, ApplyResult{Candidate: candidate, Status: StatusSkipped, Reason: "candidate repository mismatch"})
			continue
		}
		if needsDelay && delay > 0 {
			if err := wait(ctx, delay); err != nil {
				return results, err
			}
		}

		repositoryState, err := api.GetRepository(ctx, repository)
		if err != nil {
			results = append(results, ApplyResult{Candidate: candidate, Status: StatusFailed, Reason: err.Error()})
			continue
		}
		if candidate.Branch == repositoryState.DefaultBranch {
			results = append(results, ApplyResult{Candidate: candidate, Status: StatusSkipped, Reason: "branch is now the default branch"})
			continue
		}

		openPRs, err := api.ListOpenPullRequests(ctx, repository)
		if err != nil {
			results = append(results, ApplyResult{Candidate: candidate, Status: StatusFailed, Reason: err.Error()})
			continue
		}
		if _, exists := openProtection(repository, openPRs)[candidate.Branch]; exists {
			results = append(results, ApplyResult{Candidate: candidate, Status: StatusSkipped, Reason: "branch is now used by an open pull request"})
			continue
		}

		closedPRs, err := api.ListClosedPullRequestsForHead(ctx, repository, candidate.Branch)
		if err != nil {
			results = append(results, ApplyResult{Candidate: candidate, Status: StatusFailed, Reason: err.Error()})
			continue
		}
		if repositoryState.Parent != nil || candidate.PullRequestRepository != "" {
			skip, reason, err := revalidateFork(ctx, api, repositoryState, candidate, closedPRs)
			if err != nil {
				results = append(results, ApplyResult{Candidate: candidate, Status: StatusFailed, Reason: err.Error()})
				continue
			}
			if skip {
				results = append(results, ApplyResult{Candidate: candidate, Status: StatusSkipped, Reason: reason})
				continue
			}
		}
		if candidate.PullRequestRepository == "" {
			latest, found := latestClosedPullRequest(repository, candidate.Branch, closedPRs)
			if !found || latest.Number != candidate.PullRequest || latest.MergedAt == nil || latest.Base.Ref != repositoryState.DefaultBranch || latest.Head.SHA != candidate.HeadSHA {
				results = append(results, ApplyResult{Candidate: candidate, Status: StatusSkipped, Reason: "branch ownership changed after scan"})
				continue
			}
		}

		branch, err := api.GetBranch(ctx, repository, candidate.Branch)
		if err != nil {
			var apiErr *githubapi.APIError
			if errors.As(err, &apiErr) && apiErr.StatusCode == 404 {
				results = append(results, ApplyResult{Candidate: candidate, Status: StatusSkipped, Reason: "branch no longer exists"})
				continue
			}
			results = append(results, ApplyResult{Candidate: candidate, Status: StatusFailed, Reason: err.Error()})
			continue
		}
		if branch.Protected {
			results = append(results, ApplyResult{Candidate: candidate, Status: StatusSkipped, Reason: "branch is now protected"})
			continue
		}
		if branch.SHA() != candidate.HeadSHA {
			results = append(results, ApplyResult{Candidate: candidate, Status: StatusSkipped, Reason: "branch advanced after scan"})
			continue
		}

		needsDelay = true
		if err := api.DeleteBranch(ctx, repository, candidate.Branch); err != nil {
			results = append(results, ApplyResult{Candidate: candidate, Status: StatusFailed, Reason: err.Error()})
			continue
		}
		results = append(results, ApplyResult{Candidate: candidate, Status: StatusDeleted})
	}
	return results, nil
}

// revalidateFork re-reads the parent's pull requests for one fork branch immediately before
// deletion. It returns skip=true with a reason when the branch is no longer safe to delete.
func revalidateFork(
	ctx context.Context,
	api API,
	repositoryState githubapi.Repository,
	candidate Candidate,
	ownClosed []githubapi.PullRequest,
) (bool, string, error) {
	if repositoryState.Parent == nil {
		return true, "repository is no longer a fork", nil
	}
	upstream := *repositoryState.Parent
	if candidate.PullRequestRepository != "" && candidate.PullRequestRepository != upstream.FullName {
		return true, "fork parent changed after scan", nil
	}
	forkAPI, ok := api.(ForkAPI)
	if !ok {
		return false, "", errors.New("cannot revalidate parent pull requests for a fork branch")
	}
	upstreamOpen, err := api.ListOpenPullRequests(ctx, upstream.FullName)
	if err != nil {
		return false, "", err
	}
	for _, pull := range upstreamOpen {
		if pull.Head.Repo != nil && pull.Head.Repo.FullName == repositoryState.FullName && pull.Head.Ref == candidate.Branch {
			return true, "branch is now used by an open parent pull request", nil
		}
	}
	owner, _, found := strings.Cut(repositoryState.FullName, "/")
	if !found {
		return false, "", fmt.Errorf("invalid repository name %q", repositoryState.FullName)
	}
	upstreamClosed, err := forkAPI.ListClosedPullRequestsForForkHead(ctx, upstream.FullName, owner, candidate.Branch)
	if err != nil {
		return false, "", err
	}
	latestUpstream, upstreamFound := latestClosedPullRequest(repositoryState.FullName, candidate.Branch, upstreamClosed)
	_, ownFound := latestClosedPullRequest(repositoryState.FullName, candidate.Branch, ownClosed)
	if candidate.PullRequestRepository == "" {
		if upstreamFound {
			return true, "branch ownership changed after scan", nil
		}
		return false, "", nil
	}
	if ownFound || !upstreamFound || latestUpstream.Number != candidate.PullRequest || latestUpstream.MergedAt == nil ||
		latestUpstream.Base.Ref != upstream.DefaultBranch || latestUpstream.Head.SHA != candidate.HeadSHA {
		return true, "branch ownership changed after scan", nil
	}
	return false, "", nil
}

func Restore(ctx context.Context, api RestoreAPI, candidates []RestoreCandidate, delay time.Duration) ([]RestoreResult, error) {
	results := make([]RestoreResult, 0, len(candidates))
	createAttempts := 0
	for _, candidate := range candidates {
		branch, err := api.GetBranch(ctx, candidate.Repository, candidate.Branch)
		if err == nil {
			if branch.SHA() == candidate.SHA {
				results = append(results, RestoreResult{Candidate: candidate, Status: StatusAlreadyPresent, Reason: "branch already exists at the recorded SHA"})
			} else {
				results = append(results, RestoreResult{Candidate: candidate, Status: StatusRestoreSkipped, Reason: "branch name now points to a different SHA"})
			}
			continue
		}
		var apiErr *githubapi.APIError
		if !errors.As(err, &apiErr) || apiErr.StatusCode != 404 {
			results = append(results, RestoreResult{Candidate: candidate, Status: StatusRestoreFailed, Reason: err.Error()})
			continue
		}
		if createAttempts > 0 && delay > 0 {
			if err := wait(ctx, delay); err != nil {
				return results, err
			}
		}
		createAttempts++
		if err := api.CreateBranch(ctx, candidate.Repository, candidate.Branch, candidate.SHA); err != nil {
			results = append(results, RestoreResult{Candidate: candidate, Status: StatusRestoreFailed, Reason: err.Error()})
			continue
		}
		results = append(results, RestoreResult{Candidate: candidate, Status: StatusRestored})
	}
	return results, nil
}

func wait(ctx context.Context, delay time.Duration) error {
	timer := time.NewTimer(delay)
	defer timer.Stop()
	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-timer.C:
		return nil
	}
}

func openProtection(repository string, pulls []githubapi.PullRequest) map[string]struct{} {
	protected := make(map[string]struct{})
	for _, pull := range pulls {
		if pull.Head.Repo != nil && pull.Head.Repo.FullName == repository {
			protected[pull.Head.Ref] = struct{}{}
		}
		protected[pull.Base.Ref] = struct{}{}
	}
	return protected
}

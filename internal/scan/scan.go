package scan

import (
	"context"
	"errors"
	"fmt"
	"sort"
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
	DeleteBranch(context.Context, string, string) error
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

	result := Evaluate(repository, branches, openPRs, closed)
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
	sort.Slice(candidates, func(i, j int) bool {
		if candidates[i].MergedAt.Equal(candidates[j].MergedAt) {
			return candidates[i].Branch < candidates[j].Branch
		}
		return candidates[i].MergedAt.After(candidates[j].MergedAt)
	})
	return Result{Repository: repository.FullName, DefaultBranch: repository.DefaultBranch, BranchCount: len(branches), OpenPRCount: len(openPRs), Candidates: candidates}
}

func Apply(ctx context.Context, api API, repository string, candidates []Candidate, delay time.Duration) ([]ApplyResult, error) {
	repositoryState, err := api.GetRepository(ctx, repository)
	if err != nil {
		return nil, err
	}
	openPRs, err := api.ListOpenPullRequests(ctx, repository)
	if err != nil {
		return nil, err
	}
	protected := openProtection(repository, openPRs)
	results := make([]ApplyResult, 0, len(candidates))
	deleteAttempts := 0
	for _, candidate := range candidates {
		if candidate.Repository != repository {
			results = append(results, ApplyResult{Candidate: candidate, Status: StatusSkipped, Reason: "candidate repository mismatch"})
			continue
		}
		if candidate.Branch == repositoryState.DefaultBranch {
			results = append(results, ApplyResult{Candidate: candidate, Status: StatusSkipped, Reason: "branch is now the default branch"})
			continue
		}
		if _, exists := protected[candidate.Branch]; exists {
			results = append(results, ApplyResult{Candidate: candidate, Status: StatusSkipped, Reason: "branch is now used by an open pull request"})
			continue
		}
		if deleteAttempts > 0 && delay > 0 {
			if err := wait(ctx, delay); err != nil {
				return results, err
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
		deleteAttempts++
		if err := api.DeleteBranch(ctx, repository, candidate.Branch); err != nil {
			results = append(results, ApplyResult{Candidate: candidate, Status: StatusFailed, Reason: err.Error()})
			continue
		}
		results = append(results, ApplyResult{Candidate: candidate, Status: StatusDeleted})
	}
	return results, nil
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

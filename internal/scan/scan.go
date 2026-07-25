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
	ListClosedPullRequests(context.Context, string, string) ([]githubapi.PullRequest, error)
	DeleteBranch(context.Context, string, string) error
}

type Candidate struct {
	Repository  string    `json:"repository"`
	Branch      string    `json:"branch"`
	PullRequest int       `json:"pull_request"`
	HeadSHA     string    `json:"head_sha"`
	MergedAt    time.Time `json:"merged_at"`
}

type Result struct {
	Repository    string      `json:"repository"`
	DefaultBranch string      `json:"default_branch"`
	BranchCount   int         `json:"branch_count"`
	OpenPRCount   int         `json:"open_pull_request_count"`
	Candidates    []Candidate `json:"candidates"`
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

func Repository(ctx context.Context, api API, fullName string) (Result, error) {
	repository, err := api.GetRepository(ctx, fullName)
	if err != nil {
		return Result{}, err
	}
	if repository.Archived {
		return Result{}, fmt.Errorf("%s is archived", fullName)
	}

	var (
		branches []githubapi.Branch
		openPRs  []githubapi.PullRequest
		closed   []githubapi.PullRequest
		errs     [3]error
		wg       sync.WaitGroup
	)

	wg.Add(3)
	go func() {
		defer wg.Done()
		branches, errs[0] = api.ListBranches(ctx, fullName)
	}()
	go func() {
		defer wg.Done()
		openPRs, errs[1] = api.ListOpenPullRequests(ctx, fullName)
	}()
	go func() {
		defer wg.Done()
		closed, errs[2] = api.ListClosedPullRequests(ctx, fullName, repository.DefaultBranch)
	}()
	wg.Wait()

	for _, scanErr := range errs {
		if scanErr != nil {
			return Result{}, scanErr
		}
	}

	return Evaluate(repository, branches, openPRs, closed), nil
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

	latestMerged := make(map[string]githubapi.PullRequest)
	for _, pull := range closedPRs {
		if pull.MergedAt == nil {
			continue
		}
		if pull.Base.Ref != repository.DefaultBranch {
			continue
		}
		if pull.Head.Repo == nil || pull.Head.Repo.FullName != repository.FullName {
			continue
		}
		existing, found := latestMerged[pull.Head.Ref]
		if !found || existing.MergedAt.Before(*pull.MergedAt) {
			latestMerged[pull.Head.Ref] = pull
		}
	}

	candidates := make([]Candidate, 0)
	for branchName, pull := range latestMerged {
		if branchName == repository.DefaultBranch {
			continue
		}
		if _, protected := protectedByOpenPR[branchName]; protected {
			continue
		}
		branch, exists := current[branchName]
		if !exists {
			continue
		}
		if branch.Protected {
			continue
		}
		if branch.SHA() == "" || branch.SHA() != pull.Head.SHA {
			continue
		}
		candidates = append(candidates, Candidate{
			Repository:  repository.FullName,
			Branch:      branchName,
			PullRequest: pull.Number,
			HeadSHA:     pull.Head.SHA,
			MergedAt:    *pull.MergedAt,
		})
	}

	sort.Slice(candidates, func(i, j int) bool {
		if candidates[i].MergedAt.Equal(candidates[j].MergedAt) {
			return candidates[i].Branch < candidates[j].Branch
		}
		return candidates[i].MergedAt.After(candidates[j].MergedAt)
	})

	return Result{
		Repository:    repository.FullName,
		DefaultBranch: repository.DefaultBranch,
		BranchCount:   len(branches),
		OpenPRCount:   len(openPRs),
		Candidates:    candidates,
	}
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
			results = append(results, ApplyResult{
				Candidate: candidate,
				Status:    StatusSkipped,
				Reason:    "candidate repository mismatch",
			})
			continue
		}

		if candidate.Branch == repositoryState.DefaultBranch {
			results = append(results, ApplyResult{
				Candidate: candidate,
				Status:    StatusSkipped,
				Reason:    "branch is now the default branch",
			})
			continue
		}

		if _, exists := protected[candidate.Branch]; exists {
			results = append(results, ApplyResult{
				Candidate: candidate,
				Status:    StatusSkipped,
				Reason:    "branch is now used by an open pull request",
			})
			continue
		}

		if deleteAttempts > 0 && delay > 0 {
			select {
			case <-ctx.Done():
				return results, ctx.Err()
			case <-time.After(delay):
			}
		}

		branch, err := api.GetBranch(ctx, repository, candidate.Branch)
		if err != nil {
			var apiErr *githubapi.APIError
			if errors.As(err, &apiErr) && apiErr.StatusCode == 404 {
				results = append(results, ApplyResult{
					Candidate: candidate,
					Status:    StatusSkipped,
					Reason:    "branch no longer exists",
				})
				continue
			}
			results = append(results, ApplyResult{
				Candidate: candidate,
				Status:    StatusFailed,
				Reason:    err.Error(),
			})
			continue
		}

		if branch.Protected {
			results = append(results, ApplyResult{
				Candidate: candidate,
				Status:    StatusSkipped,
				Reason:    "branch is now protected",
			})
			continue
		}

		if branch.SHA() != candidate.HeadSHA {
			results = append(results, ApplyResult{
				Candidate: candidate,
				Status:    StatusSkipped,
				Reason:    "branch advanced after scan",
			})
			continue
		}

		deleteAttempts++
		if err := api.DeleteBranch(ctx, repository, candidate.Branch); err != nil {
			results = append(results, ApplyResult{
				Candidate: candidate,
				Status:    StatusFailed,
				Reason:    err.Error(),
			})
			continue
		}

		results = append(results, ApplyResult{
			Candidate: candidate,
			Status:    StatusDeleted,
		})
	}

	return results, nil
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

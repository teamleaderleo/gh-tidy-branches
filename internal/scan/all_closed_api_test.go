package scan

import (
	"context"
	"sync/atomic"
	"testing"
	"time"

	"github.com/teamleaderleo/gh-tidy-branches/internal/githubapi"
)

// Keep the shared Apply/Restore test double compatible with the scanner API. Tests that exercise
// Repository use repositoryScanAPI below so they can supply complete closed-PR history explicitly.
func (f *fakeAPI) ListAllClosedPullRequests(
	_ context.Context,
	_ string,
) ([]githubapi.PullRequest, error) {
	return nil, nil
}

func TestRepositoryUsesAllClosedPullRequestBasesForReuseSafety(t *testing.T) {
	mergedAt := time.Date(2026, 7, 20, 0, 0, 0, 0, time.UTC)
	api := &repositoryScanAPI{
		repository: githubapi.Repository{FullName: "owner/repo", DefaultBranch: "main"},
		branches: []githubapi.Branch{
			branch("feat/reused", "shared-sha"),
		},
		closed: []githubapi.PullRequest{
			pull(1, "feat/reused", "shared-sha", "main", "owner/repo", &mergedAt),
			pull(2, "feat/reused", "shared-sha", "release", "owner/repo", nil),
		},
	}

	result, err := Repository(context.Background(), api, "owner/repo")
	if err != nil {
		t.Fatal(err)
	}
	if api.allClosedCalls.Load() != 1 {
		t.Fatalf("expected one all-base closed-PR request, got %d", api.allClosedCalls.Load())
	}
	if len(result.Candidates) != 0 {
		t.Fatalf("newer cross-base closed reuse must block deletion: %#v", result.Candidates)
	}
}

type repositoryScanAPI struct {
	repository     githubapi.Repository
	branches       []githubapi.Branch
	open           []githubapi.PullRequest
	closed         []githubapi.PullRequest
	allClosedCalls atomic.Int32
}

func (f *repositoryScanAPI) GetRepository(
	_ context.Context,
	_ string,
) (githubapi.Repository, error) {
	return f.repository, nil
}

func (f *repositoryScanAPI) ListBranches(
	_ context.Context,
	_ string,
) ([]githubapi.Branch, error) {
	return append([]githubapi.Branch(nil), f.branches...), nil
}

func (f *repositoryScanAPI) GetBranch(
	_ context.Context,
	_ string,
	branchName string,
) (githubapi.Branch, error) {
	for _, value := range f.branches {
		if value.Name == branchName {
			return value, nil
		}
	}
	return githubapi.Branch{}, &githubapi.APIError{
		StatusCode: 404,
		Method:     "GET",
		URL:        branchName,
	}
}

func (f *repositoryScanAPI) ListOpenPullRequests(
	_ context.Context,
	_ string,
) ([]githubapi.PullRequest, error) {
	return append([]githubapi.PullRequest(nil), f.open...), nil
}

func (f *repositoryScanAPI) ListAllClosedPullRequests(
	_ context.Context,
	_ string,
) ([]githubapi.PullRequest, error) {
	f.allClosedCalls.Add(1)
	return append([]githubapi.PullRequest(nil), f.closed...), nil
}

func (f *repositoryScanAPI) DeleteBranch(
	_ context.Context,
	_ string,
	_ string,
) error {
	return nil
}

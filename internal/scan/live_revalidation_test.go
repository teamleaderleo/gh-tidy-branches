package scan

import (
	"context"
	"errors"
	"testing"

	"github.com/teamleaderleo/gh-tidy-branches/internal/githubapi"
)

func TestApplyRefreshesOpenPullRequestsBeforeEachDeletion(t *testing.T) {
	api := &liveRevalidationAPI{
		repositories: []githubapi.Repository{
			{FullName: "owner/repo", DefaultBranch: "main"},
			{FullName: "owner/repo", DefaultBranch: "main"},
		},
		openPulls: [][]githubapi.PullRequest{
			nil,
			{pull(10, "second", "second-sha", "main", "owner/repo", nil)},
		},
		branches: map[string]githubapi.Branch{
			"first":  branch("first", "first-sha"),
			"second": branch("second", "second-sha"),
		},
	}
	candidates := []Candidate{
		{Repository: "owner/repo", Branch: "first", HeadSHA: "first-sha"},
		{Repository: "owner/repo", Branch: "second", HeadSHA: "second-sha"},
	}

	results, err := Apply(context.Background(), api, "owner/repo", candidates, 0)
	if err != nil {
		t.Fatal(err)
	}
	if len(api.deleted) != 1 || api.deleted[0] != "first" {
		t.Fatalf("unexpected deleted branches: %#v", api.deleted)
	}
	if results[1].Status != StatusSkipped || results[1].Reason != "branch is now used by an open pull request" {
		t.Fatalf("unexpected second result: %#v", results[1])
	}
	if api.repositoryCalls != 2 || api.openPullCalls != 2 {
		t.Fatalf("expected per-candidate refreshes, got repository=%d open=%d", api.repositoryCalls, api.openPullCalls)
	}
}

func TestApplyRefreshesDefaultBranchBeforeEachDeletion(t *testing.T) {
	api := &liveRevalidationAPI{
		repositories: []githubapi.Repository{
			{FullName: "owner/repo", DefaultBranch: "main"},
			{FullName: "owner/repo", DefaultBranch: "second"},
		},
		openPulls: [][]githubapi.PullRequest{nil},
		branches: map[string]githubapi.Branch{
			"first":  branch("first", "first-sha"),
			"second": branch("second", "second-sha"),
		},
	}
	candidates := []Candidate{
		{Repository: "owner/repo", Branch: "first", HeadSHA: "first-sha"},
		{Repository: "owner/repo", Branch: "second", HeadSHA: "second-sha"},
	}

	results, err := Apply(context.Background(), api, "owner/repo", candidates, 0)
	if err != nil {
		t.Fatal(err)
	}
	if len(api.deleted) != 1 || api.deleted[0] != "first" {
		t.Fatalf("unexpected deleted branches: %#v", api.deleted)
	}
	if results[1].Status != StatusSkipped || results[1].Reason != "branch is now the default branch" {
		t.Fatalf("unexpected second result: %#v", results[1])
	}
}

func TestApplyRecordsOpenPullRefreshFailureAndContinues(t *testing.T) {
	refreshErr := errors.New("refresh open pull requests")
	api := &liveRevalidationAPI{
		repositories: []githubapi.Repository{
			{FullName: "owner/repo", DefaultBranch: "main"},
			{FullName: "owner/repo", DefaultBranch: "main"},
			{FullName: "owner/repo", DefaultBranch: "main"},
		},
		openPulls:      [][]githubapi.PullRequest{nil, nil, nil},
		openPullErrors: map[int]error{2: refreshErr},
		branches: map[string]githubapi.Branch{
			"first":  branch("first", "first-sha"),
			"second": branch("second", "second-sha"),
			"third":  branch("third", "third-sha"),
		},
	}
	assertRefreshFailureContinues(t, api, refreshErr)
}

func TestApplyRecordsRepositoryRefreshFailureAndContinues(t *testing.T) {
	refreshErr := errors.New("refresh repository")
	api := &liveRevalidationAPI{
		repositories: []githubapi.Repository{
			{FullName: "owner/repo", DefaultBranch: "main"},
			{FullName: "owner/repo", DefaultBranch: "main"},
			{FullName: "owner/repo", DefaultBranch: "main"},
		},
		repositoryErrors: map[int]error{2: refreshErr},
		openPulls:       [][]githubapi.PullRequest{nil, nil},
		branches: map[string]githubapi.Branch{
			"first":  branch("first", "first-sha"),
			"second": branch("second", "second-sha"),
			"third":  branch("third", "third-sha"),
		},
	}
	assertRefreshFailureContinues(t, api, refreshErr)
}

func assertRefreshFailureContinues(t *testing.T, api *liveRevalidationAPI, refreshErr error) {
	t.Helper()
	candidates := []Candidate{
		{Repository: "owner/repo", Branch: "first", HeadSHA: "first-sha"},
		{Repository: "owner/repo", Branch: "second", HeadSHA: "second-sha"},
		{Repository: "owner/repo", Branch: "third", HeadSHA: "third-sha"},
	}

	results, err := Apply(context.Background(), api, "owner/repo", candidates, 0)
	if err != nil {
		t.Fatal(err)
	}
	if len(api.deleted) != 2 || api.deleted[0] != "first" || api.deleted[1] != "third" {
		t.Fatalf("unexpected deleted branches: %#v", api.deleted)
	}
	if results[1].Status != StatusFailed || results[1].Reason != refreshErr.Error() {
		t.Fatalf("unexpected refresh failure result: %#v", results[1])
	}
	if results[2].Status != StatusDeleted {
		t.Fatalf("expected later candidate to continue, got %#v", results[2])
	}
}

type liveRevalidationAPI struct {
	repositories     []githubapi.Repository
	repositoryErrors map[int]error
	openPulls        [][]githubapi.PullRequest
	openPullErrors   map[int]error
	branches         map[string]githubapi.Branch
	deleted          []string
	repositoryCalls  int
	openPullCalls    int
}

func (f *liveRevalidationAPI) GetRepository(_ context.Context, _ string) (githubapi.Repository, error) {
	f.repositoryCalls++
	if err := f.repositoryErrors[f.repositoryCalls]; err != nil {
		return githubapi.Repository{}, err
	}
	index := f.repositoryCalls - 1
	if index >= len(f.repositories) {
		index = len(f.repositories) - 1
	}
	return f.repositories[index], nil
}

func (f *liveRevalidationAPI) ListBranches(context.Context, string) ([]githubapi.Branch, error) {
	return nil, nil
}

func (f *liveRevalidationAPI) GetBranch(_ context.Context, _ string, name string) (githubapi.Branch, error) {
	branch, ok := f.branches[name]
	if !ok {
		return githubapi.Branch{}, &githubapi.APIError{StatusCode: 404, Method: "GET", URL: name}
	}
	return branch, nil
}

func (f *liveRevalidationAPI) ListOpenPullRequests(context.Context, string) ([]githubapi.PullRequest, error) {
	f.openPullCalls++
	if err := f.openPullErrors[f.openPullCalls]; err != nil {
		return nil, err
	}
	index := f.openPullCalls - 1
	if index >= len(f.openPulls) {
		index = len(f.openPulls) - 1
	}
	return f.openPulls[index], nil
}

func (f *liveRevalidationAPI) ListClosedPullRequests(context.Context, string, string) ([]githubapi.PullRequest, error) {
	return nil, nil
}

func (f *liveRevalidationAPI) DeleteBranch(_ context.Context, _ string, name string) error {
	f.deleted = append(f.deleted, name)
	return nil
}

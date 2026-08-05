package scan

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/teamleaderleo/gh-tidy-branches/internal/githubapi"
)

func TestApplyRefreshesOpenPullRequestsBeforeEachDeletion(t *testing.T) {
	mergedAt := time.Date(2026, 8, 5, 0, 0, 0, 0, time.UTC)
	api := newLiveStateAPI(
		[]githubapi.Repository{
			{FullName: "owner/repo", DefaultBranch: "main"},
			{FullName: "owner/repo", DefaultBranch: "main"},
		},
		[][]githubapi.PullRequest{
			nil,
			{pull(10, "second", "second-sha", "main", "owner/repo", nil)},
		},
		map[string]githubapi.Branch{
			"first":  branch("first", "first-sha"),
			"second": branch("second", "second-sha"),
		},
		[]Candidate{
			liveCandidate(1, "first", "first-sha", mergedAt),
			liveCandidate(2, "second", "second-sha", mergedAt),
		},
	)

	results, err := Apply(context.Background(), api, "owner/repo", api.candidates, 0)
	if err != nil {
		t.Fatal(err)
	}
	if len(api.deleted) != 1 || api.deleted[0] != "first" {
		t.Fatalf("unexpected deleted branches: %#v", api.deleted)
	}
	if results[1].Status != StatusSkipped || results[1].Reason != "branch is now used by an open pull request" {
		t.Fatalf("unexpected second result: %#v", results[1])
	}
}

func TestApplyRefreshesDefaultBranchBeforeEachDeletion(t *testing.T) {
	mergedAt := time.Date(2026, 8, 5, 0, 0, 0, 0, time.UTC)
	api := newLiveStateAPI(
		[]githubapi.Repository{
			{FullName: "owner/repo", DefaultBranch: "main"},
			{FullName: "owner/repo", DefaultBranch: "second"},
		},
		[][]githubapi.PullRequest{nil},
		map[string]githubapi.Branch{
			"first":  branch("first", "first-sha"),
			"second": branch("second", "second-sha"),
		},
		[]Candidate{
			liveCandidate(1, "first", "first-sha", mergedAt),
			liveCandidate(2, "second", "second-sha", mergedAt),
		},
	)

	results, err := Apply(context.Background(), api, "owner/repo", api.candidates, 0)
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

func TestApplyRecordsRepositoryRefreshFailureAndContinues(t *testing.T) {
	refreshErr := errors.New("refresh repository")
	api := threeCandidateLiveStateAPI()
	api.repositoryErrors[2] = refreshErr

	results, err := Apply(context.Background(), api, "owner/repo", api.candidates, 0)
	if err != nil {
		t.Fatal(err)
	}
	if len(api.deleted) != 2 || api.deleted[0] != "first" || api.deleted[1] != "third" {
		t.Fatalf("unexpected deleted branches: %#v", api.deleted)
	}
	if results[1].Status != StatusFailed || results[1].Reason != refreshErr.Error() {
		t.Fatalf("unexpected repository refresh result: %#v", results[1])
	}
	if results[2].Status != StatusDeleted {
		t.Fatalf("expected later candidate to continue, got %#v", results[2])
	}
}

func TestApplyRecordsOpenPullRefreshFailureAndContinues(t *testing.T) {
	refreshErr := errors.New("refresh open pull requests")
	api := threeCandidateLiveStateAPI()
	api.openPullErrors[2] = refreshErr

	results, err := Apply(context.Background(), api, "owner/repo", api.candidates, 0)
	if err != nil {
		t.Fatal(err)
	}
	if len(api.deleted) != 2 || api.deleted[0] != "first" || api.deleted[1] != "third" {
		t.Fatalf("unexpected deleted branches: %#v", api.deleted)
	}
	if results[1].Status != StatusFailed || results[1].Reason != refreshErr.Error() {
		t.Fatalf("unexpected open pull refresh result: %#v", results[1])
	}
	if results[2].Status != StatusDeleted {
		t.Fatalf("expected later candidate to continue, got %#v", results[2])
	}
}

func threeCandidateLiveStateAPI() *liveStateAPI {
	mergedAt := time.Date(2026, 8, 5, 0, 0, 0, 0, time.UTC)
	return newLiveStateAPI(
		[]githubapi.Repository{{FullName: "owner/repo", DefaultBranch: "main"}},
		[][]githubapi.PullRequest{nil},
		map[string]githubapi.Branch{
			"first":  branch("first", "first-sha"),
			"second": branch("second", "second-sha"),
			"third":  branch("third", "third-sha"),
		},
		[]Candidate{
			liveCandidate(1, "first", "first-sha", mergedAt),
			liveCandidate(2, "second", "second-sha", mergedAt),
			liveCandidate(3, "third", "third-sha", mergedAt),
		},
	)
}

func liveCandidate(number int, name, sha string, mergedAt time.Time) Candidate {
	return Candidate{
		Repository:  "owner/repo",
		Branch:      name,
		PullRequest: number,
		HeadSHA:     sha,
		MergedAt:    mergedAt,
	}
}

type liveStateAPI struct {
	repositories     []githubapi.Repository
	openPulls        [][]githubapi.PullRequest
	branches         map[string]githubapi.Branch
	closedByHead     map[string][]githubapi.PullRequest
	candidates       []Candidate
	repositoryErrors map[int]error
	openPullErrors   map[int]error
	deleted          []string
	repositoryCalls  int
	openPullCalls    int
}

func newLiveStateAPI(
	repositories []githubapi.Repository,
	openPulls [][]githubapi.PullRequest,
	branches map[string]githubapi.Branch,
	candidates []Candidate,
) *liveStateAPI {
	closedByHead := make(map[string][]githubapi.PullRequest, len(candidates))
	for _, candidate := range candidates {
		mergedAt := candidate.MergedAt
		closedByHead[candidate.Branch] = []githubapi.PullRequest{
			pull(
				candidate.PullRequest,
				candidate.Branch,
				candidate.HeadSHA,
				"main",
				candidate.Repository,
				&mergedAt,
			),
		}
	}
	return &liveStateAPI{
		repositories:     repositories,
		openPulls:        openPulls,
		branches:         branches,
		closedByHead:     closedByHead,
		candidates:       candidates,
		repositoryErrors: make(map[int]error),
		openPullErrors:   make(map[int]error),
	}
}

func (f *liveStateAPI) GetRepository(context.Context, string) (githubapi.Repository, error) {
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

func (f *liveStateAPI) ListBranches(context.Context, string) ([]githubapi.Branch, error) {
	return nil, nil
}

func (f *liveStateAPI) GetBranch(_ context.Context, _ string, name string) (githubapi.Branch, error) {
	value, found := f.branches[name]
	if !found {
		return githubapi.Branch{}, &githubapi.APIError{StatusCode: 404, Method: "GET", URL: name}
	}
	return value, nil
}

func (f *liveStateAPI) ListOpenPullRequests(context.Context, string) ([]githubapi.PullRequest, error) {
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

func (f *liveStateAPI) ListAllClosedPullRequests(context.Context, string) ([]githubapi.PullRequest, error) {
	return nil, nil
}

func (f *liveStateAPI) ListClosedPullRequestsForHead(_ context.Context, _ string, branch string) ([]githubapi.PullRequest, error) {
	return f.closedByHead[branch], nil
}

func (f *liveStateAPI) DeleteBranch(_ context.Context, _ string, name string) error {
	f.deleted = append(f.deleted, name)
	return nil
}

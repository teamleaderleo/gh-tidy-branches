package scan

import (
	"context"
	"testing"
	"time"

	"github.com/teamleaderleo/gh-tidy-branches/internal/githubapi"
)

func forkRepository() githubapi.Repository {
	return githubapi.Repository{
		FullName:      "me/fork",
		DefaultBranch: "main",
		Parent:        &githubapi.RepositoryRef{FullName: "org/upstream", DefaultBranch: "main"},
	}
}

func TestEvaluateForkFindsBranchMergedIntoParent(t *testing.T) {
	mergedAt := time.Date(2026, 9, 1, 0, 0, 0, 0, time.UTC)
	result := EvaluateFork(
		forkRepository(),
		[]githubapi.Branch{branch("fix/a", "a-sha")},
		nil, nil, nil,
		[]githubapi.PullRequest{pull(500, "fix/a", "a-sha", "main", "me/fork", &mergedAt)},
	)
	if len(result.Candidates) != 1 {
		t.Fatalf("expected one candidate, got %#v", result.Candidates)
	}
	got := result.Candidates[0]
	if got.Branch != "fix/a" || got.PullRequest != 500 || got.PullRequestRepository != "org/upstream" {
		t.Fatalf("unexpected candidate: %#v", got)
	}
}

func TestEvaluateForkSkipsUnsafeParentHistory(t *testing.T) {
	mergedAt := time.Date(2026, 9, 1, 0, 0, 0, 0, time.UTC)
	cases := map[string]struct {
		branches       []githubapi.Branch
		ownClosed      []githubapi.PullRequest
		upstreamOpen   []githubapi.PullRequest
		upstreamClosed []githubapi.PullRequest
	}{
		"open parent pull request uses the branch": {
			branches:       []githubapi.Branch{branch("b", "b-sha")},
			upstreamOpen:   []githubapi.PullRequest{pull(601, "b", "b-sha", "main", "me/fork", nil)},
			upstreamClosed: []githubapi.PullRequest{pull(600, "b", "b-sha", "main", "me/fork", &mergedAt)},
		},
		"newer parent pull request closed unmerged": {
			branches: []githubapi.Branch{branch("b", "b-sha")},
			upstreamClosed: []githubapi.PullRequest{
				pull(600, "b", "b-sha", "main", "me/fork", &mergedAt),
				pull(601, "b", "b-sha", "main", "me/fork", nil),
			},
		},
		"merged into a non-default parent branch": {
			branches:       []githubapi.Branch{branch("b", "b-sha")},
			upstreamClosed: []githubapi.PullRequest{pull(600, "b", "b-sha", "release", "me/fork", &mergedAt)},
		},
		"branch advanced past the merged head": {
			branches:       []githubapi.Branch{branch("b", "new-sha")},
			upstreamClosed: []githubapi.PullRequest{pull(600, "b", "b-sha", "main", "me/fork", &mergedAt)},
		},
		"head belongs to a different fork": {
			branches:       []githubapi.Branch{branch("b", "b-sha")},
			upstreamClosed: []githubapi.PullRequest{pull(600, "b", "b-sha", "main", "someone/else", &mergedAt)},
		},
		"closed history in both repositories": {
			branches:       []githubapi.Branch{branch("b", "b-sha")},
			ownClosed:      []githubapi.PullRequest{pull(3, "b", "b-sha", "main", "me/fork", &mergedAt)},
			upstreamClosed: []githubapi.PullRequest{pull(600, "b", "b-sha", "main", "me/fork", &mergedAt)},
		},
	}
	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			result := EvaluateFork(forkRepository(), tc.branches, nil, tc.ownClosed, tc.upstreamOpen, tc.upstreamClosed)
			if len(result.Candidates) != 0 {
				t.Fatalf("expected no candidates, got %#v", result.Candidates)
			}
		})
	}
}

func TestEvaluateForkOpenParentPullRequestProtectsOwnMergedBranch(t *testing.T) {
	mergedAt := time.Date(2026, 9, 1, 0, 0, 0, 0, time.UTC)
	result := EvaluateFork(
		forkRepository(),
		[]githubapi.Branch{branch("b", "b-sha")},
		nil,
		[]githubapi.PullRequest{pull(3, "b", "b-sha", "main", "me/fork", &mergedAt)},
		[]githubapi.PullRequest{pull(601, "b", "b-sha", "main", "me/fork", nil)},
		nil,
	)
	if len(result.Candidates) != 0 {
		t.Fatalf("an open parent pull request must protect its head: %#v", result.Candidates)
	}
}

func TestEvaluateForkLeavesNonForkResultUnchanged(t *testing.T) {
	mergedAt := time.Date(2026, 9, 1, 0, 0, 0, 0, time.UTC)
	repository := githubapi.Repository{FullName: "owner/repo", DefaultBranch: "main"}
	branches := []githubapi.Branch{branch("done", "d-sha")}
	closed := []githubapi.PullRequest{pull(1, "done", "d-sha", "main", "owner/repo", &mergedAt)}
	want := Evaluate(repository, branches, nil, closed)
	got := EvaluateFork(repository, branches, nil, closed, nil, nil)
	if len(got.Candidates) != 1 || got.Candidates[0] != want.Candidates[0] {
		t.Fatalf("non-fork result changed: got %#v want %#v", got.Candidates, want.Candidates)
	}
}

type forkApplyAPI struct {
	repository     githubapi.Repository
	branches       map[string]githubapi.Branch
	upstreamOpen   []githubapi.PullRequest
	upstreamClosed []githubapi.PullRequest
	deleted        []string
}

func (f *forkApplyAPI) GetRepository(context.Context, string) (githubapi.Repository, error) {
	return f.repository, nil
}

func (f *forkApplyAPI) ListBranches(context.Context, string) ([]githubapi.Branch, error) {
	return nil, nil
}

func (f *forkApplyAPI) GetBranch(_ context.Context, _ string, name string) (githubapi.Branch, error) {
	value, ok := f.branches[name]
	if !ok {
		return githubapi.Branch{}, &githubapi.APIError{StatusCode: 404}
	}
	return value, nil
}

func (f *forkApplyAPI) ListOpenPullRequests(_ context.Context, fullName string) ([]githubapi.PullRequest, error) {
	if fullName == f.repository.Parent.FullName {
		return f.upstreamOpen, nil
	}
	return nil, nil
}

func (f *forkApplyAPI) ListAllClosedPullRequests(context.Context, string) ([]githubapi.PullRequest, error) {
	return nil, nil
}

func (f *forkApplyAPI) ListClosedPullRequestsForHead(context.Context, string, string) ([]githubapi.PullRequest, error) {
	return nil, nil
}

func (f *forkApplyAPI) ListClosedPullRequestsForForkHead(_ context.Context, fullName, owner, name string) ([]githubapi.PullRequest, error) {
	if fullName != f.repository.Parent.FullName || owner != "me" {
		return nil, nil
	}
	var pulls []githubapi.PullRequest
	for _, pull := range f.upstreamClosed {
		if pull.Head.Ref == name {
			pulls = append(pulls, pull)
		}
	}
	return pulls, nil
}

func (f *forkApplyAPI) DeleteBranch(_ context.Context, _ string, name string) error {
	f.deleted = append(f.deleted, name)
	return nil
}

// withoutForkAPI hides ListClosedPullRequestsForForkHead so Apply sees a plain API.
type withoutForkAPI struct{ API }

func forkCandidate(number int, name, sha string, mergedAt time.Time) Candidate {
	return Candidate{
		Repository:            "me/fork",
		Branch:                name,
		PullRequest:           number,
		HeadSHA:               sha,
		MergedAt:              mergedAt,
		PullRequestRepository: "org/upstream",
	}
}

func TestApplyRevalidatesForkBranchAgainstParent(t *testing.T) {
	mergedAt := time.Date(2026, 9, 1, 0, 0, 0, 0, time.UTC)
	api := &forkApplyAPI{
		repository: forkRepository(),
		branches: map[string]githubapi.Branch{
			"merged":   branch("merged", "m-sha"),
			"reopened": branch("reopened", "r-sha"),
			"reused":   branch("reused", "u-sha"),
		},
		upstreamOpen: []githubapi.PullRequest{pull(701, "reopened", "r-sha", "main", "me/fork", nil)},
		upstreamClosed: []githubapi.PullRequest{
			pull(700, "merged", "m-sha", "main", "me/fork", &mergedAt),
			pull(702, "reopened", "r-sha", "main", "me/fork", &mergedAt),
			pull(703, "reused", "u-sha", "main", "me/fork", &mergedAt),
			pull(704, "reused", "u-sha", "main", "me/fork", nil),
		},
	}
	candidates := []Candidate{
		forkCandidate(700, "merged", "m-sha", mergedAt),
		forkCandidate(702, "reopened", "r-sha", mergedAt),
		forkCandidate(703, "reused", "u-sha", mergedAt),
	}

	results, err := Apply(context.Background(), api, "me/fork", candidates, 0)
	if err != nil {
		t.Fatal(err)
	}
	if len(api.deleted) != 1 || api.deleted[0] != "merged" {
		t.Fatalf("unexpected deleted branches: %#v", api.deleted)
	}
	if results[1].Status != StatusSkipped || results[1].Reason != "branch is now used by an open parent pull request" {
		t.Fatalf("unexpected reopened result: %#v", results[1])
	}
	if results[2].Status != StatusSkipped || results[2].Reason != "branch ownership changed after scan" {
		t.Fatalf("unexpected reused result: %#v", results[2])
	}
}

func TestApplyFailsClosedWithoutForkLookup(t *testing.T) {
	mergedAt := time.Date(2026, 9, 1, 0, 0, 0, 0, time.UTC)
	api := &forkApplyAPI{
		repository:     forkRepository(),
		branches:       map[string]githubapi.Branch{"merged": branch("merged", "m-sha")},
		upstreamClosed: []githubapi.PullRequest{pull(700, "merged", "m-sha", "main", "me/fork", &mergedAt)},
	}

	results, err := Apply(context.Background(), withoutForkAPI{api}, "me/fork", []Candidate{forkCandidate(700, "merged", "m-sha", mergedAt)}, 0)
	if err != nil {
		t.Fatal(err)
	}
	if len(api.deleted) != 0 || results[0].Status != StatusFailed {
		t.Fatalf("expected a failed, undeleted result: deleted=%#v results=%#v", api.deleted, results)
	}
}

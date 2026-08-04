package scan

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/teamleaderleo/gh-tidy-branches/internal/githubapi"
)

func TestApplySkipsWhenClosedOwnershipChangesAfterScan(t *testing.T) {
	mergedAt := time.Date(2026, 7, 20, 0, 0, 0, 0, time.UTC)
	api := &fakeAPI{
		repository: githubapi.Repository{FullName: "owner/repo", DefaultBranch: "main"},
		branches: map[string]githubapi.Branch{
			"feat/reused": branch("feat/reused", "shared-sha"),
		},
		closedPRs: []githubapi.PullRequest{
			pull(1, "feat/reused", "shared-sha", "main", "owner/repo", &mergedAt),
			pull(2, "feat/reused", "shared-sha", "main", "owner/repo", nil),
		},
	}
	candidate := Candidate{
		Repository:  "owner/repo",
		Branch:      "feat/reused",
		PullRequest: 1,
		HeadSHA:     "shared-sha",
		MergedAt:    mergedAt,
	}

	results, err := Apply(context.Background(), api, "owner/repo", []Candidate{candidate}, 0)
	if err != nil {
		t.Fatal(err)
	}
	if len(api.deleted) != 0 {
		t.Fatalf("branch with newer closed ownership was deleted: %#v", api.deleted)
	}
	if len(results) != 1 || results[0].Status != StatusSkipped || results[0].Reason != "branch ownership changed after scan" {
		t.Fatalf("unexpected ownership result: %#v", results)
	}
}

func TestApplySkipsWhenNewestOwnershipMergedIntoAnotherBase(t *testing.T) {
	olderMergedAt := time.Date(2026, 7, 20, 0, 0, 0, 0, time.UTC)
	newerMergedAt := time.Date(2026, 7, 25, 0, 0, 0, 0, time.UTC)
	api := &fakeAPI{
		repository: githubapi.Repository{FullName: "owner/repo", DefaultBranch: "main"},
		branches: map[string]githubapi.Branch{
			"feat/reused": branch("feat/reused", "shared-sha"),
		},
		closedPRs: []githubapi.PullRequest{
			pull(1, "feat/reused", "shared-sha", "main", "owner/repo", &olderMergedAt),
			pull(2, "feat/reused", "shared-sha", "release", "owner/repo", &newerMergedAt),
		},
	}
	candidate := Candidate{
		Repository:  "owner/repo",
		Branch:      "feat/reused",
		PullRequest: 1,
		HeadSHA:     "shared-sha",
		MergedAt:    olderMergedAt,
	}

	results, err := Apply(context.Background(), api, "owner/repo", []Candidate{candidate}, 0)
	if err != nil {
		t.Fatal(err)
	}
	if len(api.deleted) != 0 || results[0].Reason != "branch ownership changed after scan" {
		t.Fatalf("newer non-default merge must block deletion: results=%#v deleted=%#v", results, api.deleted)
	}
}

func TestApplyDeletesWhenClosedOwnershipIsUnchanged(t *testing.T) {
	mergedAt := time.Date(2026, 7, 20, 0, 0, 0, 0, time.UTC)
	api := &fakeAPI{
		repository: githubapi.Repository{FullName: "owner/repo", DefaultBranch: "main"},
		branches: map[string]githubapi.Branch{
			"feat/delete": branch("feat/delete", "delete-sha"),
		},
		closedPRs: []githubapi.PullRequest{
			pull(7, "feat/delete", "delete-sha", "main", "owner/repo", &mergedAt),
		},
	}
	candidate := Candidate{
		Repository:  "owner/repo",
		Branch:      "feat/delete",
		PullRequest: 7,
		HeadSHA:     "delete-sha",
		MergedAt:    mergedAt,
	}

	results, err := Apply(context.Background(), api, "owner/repo", []Candidate{candidate}, 0)
	if err != nil {
		t.Fatal(err)
	}
	if len(api.deleted) != 1 || api.deleted[0] != "feat/delete" || results[0].Status != StatusDeleted {
		t.Fatalf("unchanged ownership should delete: results=%#v deleted=%#v", results, api.deleted)
	}
}

func TestApplyRecordsClosedOwnershipRefreshFailure(t *testing.T) {
	refreshErr := errors.New("refresh branch ownership")
	api := &ownershipErrorAPI{
		fakeAPI: &fakeAPI{
			repository: githubapi.Repository{FullName: "owner/repo", DefaultBranch: "main"},
			branches: map[string]githubapi.Branch{
				"feat/delete": branch("feat/delete", "delete-sha"),
			},
		},
		err: refreshErr,
	}
	candidate := Candidate{Repository: "owner/repo", Branch: "feat/delete", PullRequest: 7, HeadSHA: "delete-sha"}

	results, err := Apply(context.Background(), api, "owner/repo", []Candidate{candidate}, 0)
	if err != nil {
		t.Fatal(err)
	}
	if len(api.deleted) != 0 || len(results) != 1 || results[0].Status != StatusFailed || results[0].Reason != refreshErr.Error() {
		t.Fatalf("unexpected refresh failure result: results=%#v deleted=%#v", results, api.deleted)
	}
}

func (f *fakeAPI) ListClosedPullRequestsForHead(
	_ context.Context,
	repository string,
	branchName string,
) ([]githubapi.PullRequest, error) {
	return closedPullRequestsForBranch(repository, branchName, f.closedPRs), nil
}

func (f *repositoryScanAPI) ListClosedPullRequestsForHead(
	_ context.Context,
	repository string,
	branchName string,
) ([]githubapi.PullRequest, error) {
	return closedPullRequestsForBranch(repository, branchName, f.closed), nil
}

func closedPullRequestsForBranch(
	repository string,
	branchName string,
	pulls []githubapi.PullRequest,
) []githubapi.PullRequest {
	result := make([]githubapi.PullRequest, 0)
	for _, pull := range pulls {
		if pull.Head.Ref == branchName && pull.Head.Repo != nil && pull.Head.Repo.FullName == repository {
			result = append(result, pull)
		}
	}
	return result
}

type ownershipErrorAPI struct {
	*fakeAPI
	err error
}

func (f *ownershipErrorAPI) ListClosedPullRequestsForHead(
	context.Context,
	string,
	string,
) ([]githubapi.PullRequest, error) {
	return nil, f.err
}

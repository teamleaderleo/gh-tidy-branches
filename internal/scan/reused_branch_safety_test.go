package scan

import (
	"testing"
	"time"

	"github.com/teamleaderleo/gh-tidy-branches/internal/githubapi"
)

func TestEvaluateProtectsNewerClosedUnmergedBranchReuse(t *testing.T) {
	repository := githubapi.Repository{FullName: "owner/repo", DefaultBranch: "main"}
	mergedAt := time.Date(2026, 7, 20, 0, 0, 0, 0, time.UTC)
	olderMerged := pull(1, "feat/reused", "shared-sha", "main", "owner/repo", &mergedAt)
	newerUnmerged := pull(2, "feat/reused", "shared-sha", "main", "owner/repo", nil)

	assertNoCandidatesForClosedOrderings(t, repository, olderMerged, newerUnmerged)
}

func TestEvaluateProtectsNewerClosedUnmergedCrossBaseReuse(t *testing.T) {
	repository := githubapi.Repository{FullName: "owner/repo", DefaultBranch: "main"}
	mergedAt := time.Date(2026, 7, 20, 0, 0, 0, 0, time.UTC)
	olderMerged := pull(1, "feat/reused", "shared-sha", "main", "owner/repo", &mergedAt)
	newerUnmerged := pull(2, "feat/reused", "shared-sha", "release", "owner/repo", nil)

	assertNoCandidatesForClosedOrderings(t, repository, olderMerged, newerUnmerged)
}

func TestEvaluateProtectsNewerMergedNonDefaultBaseReuse(t *testing.T) {
	repository := githubapi.Repository{FullName: "owner/repo", DefaultBranch: "main"}
	olderMergedAt := time.Date(2026, 7, 20, 0, 0, 0, 0, time.UTC)
	newerMergedAt := time.Date(2026, 7, 25, 0, 0, 0, 0, time.UTC)
	olderMerged := pull(1, "feat/reused", "shared-sha", "main", "owner/repo", &olderMergedAt)
	newerMergedElsewhere := pull(2, "feat/reused", "shared-sha", "release", "owner/repo", &newerMergedAt)

	assertNoCandidatesForClosedOrderings(t, repository, olderMerged, newerMergedElsewhere)
}

func TestEvaluateUsesNewerMergedBranchReuseRegardlessOfInputOrder(t *testing.T) {
	repository := githubapi.Repository{FullName: "owner/repo", DefaultBranch: "main"}
	newerMergedAt := time.Date(2026, 7, 25, 0, 0, 0, 0, time.UTC)
	olderUnmerged := pull(1, "feat/reused", "old-sha", "main", "owner/repo", nil)
	newerMerged := pull(2, "feat/reused", "new-sha", "main", "owner/repo", &newerMergedAt)

	for _, closed := range [][]githubapi.PullRequest{
		{olderUnmerged, newerMerged},
		{newerMerged, olderUnmerged},
	} {
		result := Evaluate(
			repository,
			[]githubapi.Branch{branch("feat/reused", "new-sha")},
			nil,
			closed,
		)
		if len(result.Candidates) != 1 || result.Candidates[0].PullRequest != 2 {
			t.Fatalf("newer merged reuse should remain eligible: %#v", result.Candidates)
		}
	}
}

func TestEvaluateIgnoresNewerForkReuseWithSameBranchName(t *testing.T) {
	repository := githubapi.Repository{FullName: "owner/repo", DefaultBranch: "main"}
	mergedAt := time.Date(2026, 7, 20, 0, 0, 0, 0, time.UTC)
	olderMerged := pull(1, "feat/reused", "shared-sha", "main", "owner/repo", &mergedAt)
	newerFork := pull(2, "feat/reused", "fork-sha", "main", "someone/fork", nil)

	for _, closed := range [][]githubapi.PullRequest{
		{olderMerged, newerFork},
		{newerFork, olderMerged},
	} {
		result := Evaluate(
			repository,
			[]githubapi.Branch{branch("feat/reused", "shared-sha")},
			nil,
			closed,
		)
		if len(result.Candidates) != 1 || result.Candidates[0].PullRequest != 1 {
			t.Fatalf("fork reuse must not replace same-repository ownership: %#v", result.Candidates)
		}
	}
}

func assertNoCandidatesForClosedOrderings(
	t *testing.T,
	repository githubapi.Repository,
	older githubapi.PullRequest,
	newer githubapi.PullRequest,
) {
	t.Helper()
	for _, closed := range [][]githubapi.PullRequest{
		{older, newer},
		{newer, older},
	} {
		result := Evaluate(
			repository,
			[]githubapi.Branch{branch("feat/reused", "shared-sha")},
			nil,
			closed,
		)
		if len(result.Candidates) != 0 {
			t.Fatalf("newer same-repository reuse must block deletion: %#v", result.Candidates)
		}
	}
}

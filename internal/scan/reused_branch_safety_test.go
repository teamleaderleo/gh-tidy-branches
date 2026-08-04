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

	for _, closed := range [][]githubapi.PullRequest{
		{olderMerged, newerUnmerged},
		{newerUnmerged, olderMerged},
	} {
		result := Evaluate(
			repository,
			[]githubapi.Branch{branch("feat/reused", "shared-sha")},
			nil,
			closed,
		)
		if len(result.Candidates) != 0 {
			t.Fatalf("newer closed-unmerged reuse must block deletion: %#v", result.Candidates)
		}
	}
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

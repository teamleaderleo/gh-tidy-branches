package scan

import (
	"testing"
	"time"

	"github.com/teamleaderleo/gh-tidy-branches/internal/githubapi"
)

func TestEvaluateProtectsAdvancedAndOpenBranches(t *testing.T) {
	repository := githubapi.Repository{
		FullName:      "owner/repo",
		DefaultBranch: "main",
	}

	branches := []githubapi.Branch{
		branch("main", "main-sha"),
		branch("feat/delete", "delete-sha"),
		branch("feat/advanced", "new-sha"),
		branch("feat/open", "open-sha"),
		branch("integration", "integration-sha"),
	}

	mergedAt := time.Date(2026, 7, 26, 12, 0, 0, 0, time.UTC)
	closed := []githubapi.PullRequest{
		pull(1, "feat/delete", "delete-sha", "main", "owner/repo", &mergedAt),
		pull(2, "feat/advanced", "old-sha", "main", "owner/repo", &mergedAt),
		pull(3, "feat/open", "open-sha", "main", "owner/repo", &mergedAt),
		pull(4, "from-fork", "fork-sha", "main", "someone/fork", &mergedAt),
		pull(5, "integration", "integration-sha", "integration", "owner/repo", &mergedAt),
	}

	open := []githubapi.PullRequest{
		pull(6, "feat/open", "open-sha", "main", "owner/repo", nil),
		pull(7, "main", "main-sha", "integration", "owner/repo", nil),
	}

	result := Evaluate(repository, branches, open, closed)

	if len(result.Candidates) != 1 {
		t.Fatalf("expected 1 candidate, got %d: %#v", len(result.Candidates), result.Candidates)
	}
	if result.Candidates[0].Branch != "feat/delete" {
		t.Fatalf("expected feat/delete, got %s", result.Candidates[0].Branch)
	}
}

func TestEvaluateUsesNewestMergedPRForReusedBranch(t *testing.T) {
	repository := githubapi.Repository{
		FullName:      "owner/repo",
		DefaultBranch: "main",
	}
	older := time.Date(2026, 7, 20, 0, 0, 0, 0, time.UTC)
	newer := time.Date(2026, 7, 25, 0, 0, 0, 0, time.UTC)

	result := Evaluate(
		repository,
		[]githubapi.Branch{branch("feat/reused", "new-sha")},
		nil,
		[]githubapi.PullRequest{
			pull(1, "feat/reused", "old-sha", "main", "owner/repo", &older),
			pull(2, "feat/reused", "new-sha", "main", "owner/repo", &newer),
		},
	)

	if len(result.Candidates) != 1 {
		t.Fatalf("expected 1 candidate, got %d", len(result.Candidates))
	}
	if result.Candidates[0].PullRequest != 2 {
		t.Fatalf("expected newest PR #2, got #%d", result.Candidates[0].PullRequest)
	}
}

func branch(name, sha string) githubapi.Branch {
	value := githubapi.Branch{Name: name}
	value.Commit.SHA = sha
	return value
}

func pull(number int, head, sha, base, headRepo string, mergedAt *time.Time) githubapi.PullRequest {
	value := githubapi.PullRequest{
		Number:   number,
		MergedAt: mergedAt,
		Head: githubapi.PullRef{
			Ref: head,
			SHA: sha,
		},
		Base: githubapi.PullRef{
			Ref: base,
		},
	}
	value.Head.Repo = &struct {
		FullName string `json:"full_name"`
	}{FullName: headRepo}
	return value
}

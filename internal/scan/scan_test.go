package scan

import (
	"context"
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

func TestApplyRevalidatesDefaultProtectedOpenAdvancedAndOwnership(t *testing.T) {
	mergedAt := time.Date(2026, 7, 26, 12, 0, 0, 0, time.UTC)
	api := &fakeAPI{
		repository: githubapi.Repository{FullName: "owner/repo", DefaultBranch: "new-default"},
		branches: map[string]githubapi.Branch{
			"protected": protectedBranch("protected", "protected-sha"),
			"open":      branch("open", "open-sha"),
			"advanced":  branch("advanced", "new-sha"),
			"delete":    branch("delete", "delete-sha"),
		},
		openPRs: []githubapi.PullRequest{
			pull(9, "open", "open-sha", "main", "owner/repo", nil),
		},
		closedPRs: []githubapi.PullRequest{
			pull(1, "new-default", "default-sha", "new-default", "owner/repo", &mergedAt),
			pull(2, "protected", "protected-sha", "new-default", "owner/repo", &mergedAt),
			pull(3, "open", "open-sha", "new-default", "owner/repo", &mergedAt),
			pull(4, "advanced", "old-sha", "new-default", "owner/repo", &mergedAt),
			pull(5, "delete", "delete-sha", "new-default", "owner/repo", &mergedAt),
		},
	}

	candidates := []Candidate{
		{Repository: "owner/repo", Branch: "new-default", PullRequest: 1, HeadSHA: "default-sha", MergedAt: mergedAt},
		{Repository: "owner/repo", Branch: "protected", PullRequest: 2, HeadSHA: "protected-sha", MergedAt: mergedAt},
		{Repository: "owner/repo", Branch: "open", PullRequest: 3, HeadSHA: "open-sha", MergedAt: mergedAt},
		{Repository: "owner/repo", Branch: "advanced", PullRequest: 4, HeadSHA: "old-sha", MergedAt: mergedAt},
		{Repository: "owner/repo", Branch: "delete", PullRequest: 5, HeadSHA: "delete-sha", MergedAt: mergedAt},
	}

	results, err := Apply(context.Background(), api, "owner/repo", candidates, 0)
	if err != nil {
		t.Fatal(err)
	}
	if len(results) != len(candidates) {
		t.Fatalf("expected %d results, got %d", len(candidates), len(results))
	}
	if len(api.deleted) != 1 || api.deleted[0] != "delete" {
		t.Fatalf("unexpected deleted branches: %#v", api.deleted)
	}
	if results[0].Reason != "branch is now the default branch" {
		t.Fatalf("unexpected default-branch result: %#v", results[0])
	}
	if results[1].Reason != "branch is now protected" {
		t.Fatalf("unexpected protected result: %#v", results[1])
	}
	if results[2].Reason != "branch is now used by an open pull request" {
		t.Fatalf("unexpected open-PR result: %#v", results[2])
	}
	if results[3].Reason != "branch advanced after scan" {
		t.Fatalf("unexpected advanced result: %#v", results[3])
	}
	if results[4].Status != StatusDeleted {
		t.Fatalf("expected deleted status, got %#v", results[4])
	}
}

type fakeAPI struct {
	repository githubapi.Repository
	branches   map[string]githubapi.Branch
	openPRs    []githubapi.PullRequest
	closedPRs  []githubapi.PullRequest
	deleted    []string
}

func (f *fakeAPI) GetRepository(_ context.Context, _ string) (githubapi.Repository, error) {
	return f.repository, nil
}

func (f *fakeAPI) ListBranches(_ context.Context, _ string) ([]githubapi.Branch, error) {
	return nil, nil
}

func (f *fakeAPI) GetBranch(_ context.Context, _ string, branchName string) (githubapi.Branch, error) {
	value, ok := f.branches[branchName]
	if !ok {
		return githubapi.Branch{}, &githubapi.APIError{StatusCode: 404, Method: "GET", URL: branchName}
	}
	return value, nil
}

func (f *fakeAPI) ListOpenPullRequests(_ context.Context, _ string) ([]githubapi.PullRequest, error) {
	return f.openPRs, nil
}

func (f *fakeAPI) ListClosedPullRequests(_ context.Context, _, _ string) ([]githubapi.PullRequest, error) {
	return f.closedPRs, nil
}

func (f *fakeAPI) DeleteBranch(_ context.Context, _ string, branchName string) error {
	f.deleted = append(f.deleted, branchName)
	return nil
}

func protectedBranch(name, sha string) githubapi.Branch {
	value := branch(name, sha)
	value.Protected = true
	return value
}

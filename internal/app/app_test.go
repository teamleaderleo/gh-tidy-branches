package app

import (
	"context"
	"reflect"
	"testing"
	"time"

	"github.com/teamleaderleo/gh-tidy-branches/internal/githubapi"
	"github.com/teamleaderleo/gh-tidy-branches/internal/receipt"
	"github.com/teamleaderleo/gh-tidy-branches/internal/scan"
)

func TestParseSupportsFlagsAnywhere(t *testing.T) {
	options, err := parse([]string{"owner/repo", "--jobs=3", "--preview", "-R", "owner/other", "--repo=owner/third", "--delete-delay", "250ms"})
	if err != nil {
		t.Fatal(err)
	}
	if options.Jobs != 3 || !options.DryRun || options.DeleteDelay != 250*time.Millisecond {
		t.Fatalf("unexpected options: %#v", options)
	}
	want := []string{"owner/other", "owner/repo", "owner/third"}
	if !reflect.DeepEqual(options.Repositories, want) {
		t.Fatalf("got %#v, want %#v", options.Repositories, want)
	}
}

func TestParseRejectsConflictingMutationFlags(t *testing.T) {
	if _, err := parse([]string{"--dry-run", "--yes"}); err == nil {
		t.Fatal("expected error")
	}
}

func TestParseRejectsAllWithExplicitRepositories(t *testing.T) {
	if _, err := parse([]string{"--all", "--repo", "owner/repo"}); err == nil {
		t.Fatal("expected error")
	}
}

func TestParseRejectsEmptyRepositoryFlag(t *testing.T) {
	if _, err := parse([]string{"--repo="}); err == nil {
		t.Fatal("expected error")
	}
}

func TestPromptDisabledUsesGitHubCLIEnvironment(t *testing.T) {
	t.Setenv("GH_PROMPT_DISABLED", "1")
	if !promptDisabled() {
		t.Fatal("expected prompts to be disabled")
	}
	t.Setenv("GH_PROMPT_DISABLED", "")
	if promptDisabled() {
		t.Fatal("expected prompts to be enabled")
	}
}

func TestRemainingReceiptEntriesKeepsUnsafeRestores(t *testing.T) {
	entries := []receipt.Entry{{Repository: "o/r", Branch: "a"}, {Repository: "o/r", Branch: "b"}, {Repository: "o/r", Branch: "c"}}
	results := []scan.RestoreResult{
		{Candidate: scan.RestoreCandidate{Repository: "o/r", Branch: "a"}, Status: scan.StatusRestored},
		{Candidate: scan.RestoreCandidate{Repository: "o/r", Branch: "b"}, Status: scan.StatusRestoreSkipped},
		{Candidate: scan.RestoreCandidate{Repository: "o/r", Branch: "c"}, Status: scan.StatusAlreadyPresent},
	}
	remaining := remainingReceiptEntries(entries, results)
	if len(remaining) != 1 || remaining[0].Branch != "b" {
		t.Fatalf("unexpected remaining: %#v", remaining)
	}
}

func TestApplyScanResultsPreservesCompletedDeletesWhenLaterWaitIsCanceled(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	mergedAt := time.Date(2026, 8, 5, 12, 0, 0, 0, time.UTC)
	candidates := []scan.Candidate{
		{Repository: "o/r", Branch: "first", PullRequest: 10, HeadSHA: "sha-first", MergedAt: mergedAt},
		{Repository: "o/r", Branch: "second", PullRequest: 11, HeadSHA: "sha-second", MergedAt: mergedAt.Add(time.Minute)},
	}
	api := &cancelAfterFirstDeleteAPI{
		cancel: cancel,
		candidates: map[string]scan.Candidate{
			"first":  candidates[0],
			"second": candidates[1],
		},
	}

	applied, repositoryErrors := applyScanResults(
		ctx,
		api,
		[]scan.Result{{Repository: "o/r", Candidates: candidates}},
		time.Hour,
	)

	if len(applied) != 1 || applied[0].Status != scan.StatusDeleted || applied[0].Candidate.Branch != "first" {
		t.Fatalf("completed deletion was not preserved: %#v", applied)
	}
	if len(repositoryErrors) != 1 || repositoryErrors[0].Error != context.Canceled.Error() {
		t.Fatalf("expected cancellation error alongside partial results, got %#v", repositoryErrors)
	}
	entries := receiptEntries(applied)
	if len(entries) != 1 || entries[0].Branch != "first" || entries[0].SHA != "sha-first" {
		t.Fatalf("completed deletion was omitted from undo entries: %#v", entries)
	}
}

type cancelAfterFirstDeleteAPI struct {
	cancel     context.CancelFunc
	candidates map[string]scan.Candidate
	deleted    []string
}

func (api *cancelAfterFirstDeleteAPI) GetRepository(context.Context, string) (githubapi.Repository, error) {
	return githubapi.Repository{FullName: "o/r", DefaultBranch: "main"}, nil
}

func (api *cancelAfterFirstDeleteAPI) ListBranches(context.Context, string) ([]githubapi.Branch, error) {
	return nil, nil
}

func (api *cancelAfterFirstDeleteAPI) GetBranch(_ context.Context, _ string, branch string) (githubapi.Branch, error) {
	candidate := api.candidates[branch]
	value := githubapi.Branch{Name: branch}
	value.Commit.SHA = candidate.HeadSHA
	return value, nil
}

func (api *cancelAfterFirstDeleteAPI) ListOpenPullRequests(context.Context, string) ([]githubapi.PullRequest, error) {
	return nil, nil
}

func (api *cancelAfterFirstDeleteAPI) ListAllClosedPullRequests(context.Context, string) ([]githubapi.PullRequest, error) {
	return nil, nil
}

func (api *cancelAfterFirstDeleteAPI) ListClosedPullRequestsForHead(_ context.Context, repository, branch string) ([]githubapi.PullRequest, error) {
	candidate := api.candidates[branch]
	repo := &struct {
		FullName string `json:"full_name"`
	}{FullName: repository}
	mergedAt := candidate.MergedAt
	return []githubapi.PullRequest{{
		Number:   candidate.PullRequest,
		MergedAt: &mergedAt,
		Head: githubapi.PullRef{
			Ref:  candidate.Branch,
			SHA:  candidate.HeadSHA,
			Repo: repo,
		},
		Base: githubapi.PullRef{Ref: "main"},
	}}, nil
}

func (api *cancelAfterFirstDeleteAPI) DeleteBranch(_ context.Context, _ string, branch string) error {
	api.deleted = append(api.deleted, branch)
	if len(api.deleted) == 1 {
		api.cancel()
	}
	return nil
}

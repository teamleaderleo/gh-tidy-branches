package app

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/teamleaderleo/gh-tidy-branches/internal/githubapi"
	"github.com/teamleaderleo/gh-tidy-branches/internal/receipt"
	"github.com/teamleaderleo/gh-tidy-branches/internal/scan"
)

func TestRemainingReceiptEntriesRetainsUnattemptedRestoreAfterCancellation(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	candidates := []scan.RestoreCandidate{
		{Repository: "o/r", Branch: "first", SHA: "sha-first"},
		{Repository: "o/r", Branch: "second", SHA: "sha-second"},
	}
	api := &cancelAfterFirstRestoreAPI{cancel: cancel}

	results, err := scan.Restore(ctx, api, candidates, time.Hour)
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("expected cancellation after first restore, got %v", err)
	}
	if len(results) != 1 || results[0].Status != scan.StatusRestored || results[0].Candidate.Branch != "first" {
		t.Fatalf("completed restore was not preserved: %#v", results)
	}

	entries := []receipt.Entry{
		{Repository: "o/r", Branch: "first", SHA: "sha-first"},
		{Repository: "o/r", Branch: "second", SHA: "sha-second"},
	}
	remaining := remainingReceiptEntries(entries, results)
	if len(remaining) != 1 || remaining[0].Branch != "second" || remaining[0].SHA != "sha-second" {
		t.Fatalf("unattempted restore was not retained: %#v", remaining)
	}
}

type cancelAfterFirstRestoreAPI struct {
	cancel  context.CancelFunc
	created []string
}

func (api *cancelAfterFirstRestoreAPI) GetBranch(context.Context, string, string) (githubapi.Branch, error) {
	return githubapi.Branch{}, &githubapi.APIError{StatusCode: 404, Method: "GET"}
}

func (api *cancelAfterFirstRestoreAPI) CreateBranch(_ context.Context, _ string, branch, _ string) error {
	api.created = append(api.created, branch)
	if len(api.created) == 1 {
		api.cancel()
	}
	return nil
}

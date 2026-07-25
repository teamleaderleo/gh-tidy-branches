package app

import (
	"reflect"
	"testing"
	"time"

	"github.com/teamleaderleo/gh-tidy-branches/internal/receipt"
	"github.com/teamleaderleo/gh-tidy-branches/internal/scan"
)

func TestParseSupportsFlagsAnywhere(t *testing.T) {
	options, err := parse([]string{"owner/repo", "--jobs=3", "--preview", "owner/other", "--delete-delay", "250ms"})
	if err != nil {
		t.Fatal(err)
	}
	if options.Jobs != 3 || !options.DryRun || options.DeleteDelay != 250*time.Millisecond {
		t.Fatalf("unexpected options: %#v", options)
	}
	want := []string{"owner/other", "owner/repo"}
	if !reflect.DeepEqual(options.Repositories, want) {
		t.Fatalf("got %#v, want %#v", options.Repositories, want)
	}
}

func TestParseRejectsConflictingMutationFlags(t *testing.T) {
	if _, err := parse([]string{"--dry-run", "--yes"}); err == nil {
		t.Fatal("expected error")
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

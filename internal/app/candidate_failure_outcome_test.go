package app

import (
	"bytes"
	"encoding/json"
	"errors"
	"strings"
	"testing"

	"github.com/teamleaderleo/gh-tidy-branches/internal/receipt"
	"github.com/teamleaderleo/gh-tidy-branches/internal/scan"
)

func TestApplyCandidateFailuresReturnErrorAndKeepDeletedReceipts(t *testing.T) {
	results := []scan.ApplyResult{
		{Candidate: scan.Candidate{Repository: "o/r", Branch: "deleted", HeadSHA: "sha-deleted"}, Status: scan.StatusDeleted},
		{Candidate: scan.Candidate{Repository: "o/r", Branch: "safe-skip"}, Status: scan.StatusSkipped},
		{Candidate: scan.Candidate{Repository: "o/r", Branch: "failed"}, Status: scan.StatusFailed, Reason: "delete rejected"},
	}

	err := errorIfApplyFailures(results)
	if err == nil || !strings.Contains(err.Error(), "1 branch deletion(s) failed") {
		t.Fatalf("candidate failure did not select a nonzero outcome: %v", err)
	}
	entries := receiptEntries(results)
	if len(entries) != 1 || entries[0].Branch != "deleted" || entries[0].SHA != "sha-deleted" {
		t.Fatalf("successful deletion was not retained for undo: %#v", entries)
	}
}

func TestSafeApplyStatusesReturnSuccess(t *testing.T) {
	results := []scan.ApplyResult{
		{Status: scan.StatusDeleted},
		{Status: scan.StatusSkipped},
	}
	if err := errorIfApplyFailures(results); err != nil {
		t.Fatalf("safe apply statuses returned an error: %v", err)
	}
}

func TestRestoreCandidateFailuresReturnErrorAndKeepReceiptEntries(t *testing.T) {
	entries := []receipt.Entry{
		{Repository: "o/r", Branch: "restored", SHA: "sha-restored"},
		{Repository: "o/r", Branch: "failed", SHA: "sha-failed"},
	}
	results := []scan.RestoreResult{
		{Candidate: scan.RestoreCandidate{Repository: "o/r", Branch: "restored", SHA: "sha-restored"}, Status: scan.StatusRestored},
		{Candidate: scan.RestoreCandidate{Repository: "o/r", Branch: "failed", SHA: "sha-failed"}, Status: scan.StatusRestoreFailed, Reason: "create rejected"},
	}

	err := errorIfRestoreFailures(results)
	if err == nil || !strings.Contains(err.Error(), "1 branch restoration(s) failed") {
		t.Fatalf("candidate failure did not select a nonzero outcome: %v", err)
	}
	remaining := remainingReceiptEntries(entries, results)
	if len(remaining) != 1 || remaining[0].Branch != "failed" || remaining[0].SHA != "sha-failed" {
		t.Fatalf("failed restore entry was not retained: %#v", remaining)
	}
}

func TestSafeRestoreStatusesReturnSuccess(t *testing.T) {
	results := []scan.RestoreResult{
		{Status: scan.StatusRestored},
		{Status: scan.StatusAlreadyPresent},
		{Status: scan.StatusRestoreSkipped},
	}
	if err := errorIfRestoreFailures(results); err != nil {
		t.Fatalf("safe restore statuses returned an error: %v", err)
	}
}

func TestJSONIsWrittenBeforeOutcomeErrorIsReturned(t *testing.T) {
	outcomeErr := errors.New("candidate operation failed")
	var output bytes.Buffer

	err := writeJSONWithOutcome(&output, map[string]string{"status": "failed"}, outcomeErr)
	if !errors.Is(err, outcomeErr) {
		t.Fatalf("outcome error was hidden: %v", err)
	}
	if !json.Valid(output.Bytes()) || !strings.Contains(output.String(), `"status": "failed"`) {
		t.Fatalf("JSON was not emitted before the nonzero outcome: %q", output.String())
	}
}

func TestJSONAndTerminalErrorsRemainJointlyVisible(t *testing.T) {
	writeErr := errors.New("output unavailable")
	outcomeErr := errors.New("candidate operation failed")
	err := writeJSONWithOutcome(errorWriter{err: writeErr}, map[string]string{"status": "failed"}, outcomeErr)
	if !errors.Is(err, writeErr) || !errors.Is(err, outcomeErr) {
		t.Fatalf("joined output/candidate errors were not preserved: %v", err)
	}
}

type errorWriter struct {
	err error
}

func (writer errorWriter) Write([]byte) (int, error) {
	return 0, writer.err
}

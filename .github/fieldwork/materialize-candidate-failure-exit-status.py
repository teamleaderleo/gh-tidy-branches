from pathlib import Path

app_path = Path('internal/app/app.go')
app = app_path.read_text(encoding='utf-8')

old_json = '''\tif options.JSON {\n\t\tif err := writeJSON(stdout, output); err != nil {\n\t\t\treturn err\n\t\t}\n\t\treturn errorIfRepositoryFailures(repositoryErrors)\n\t}\n\tprintApplyResults(stdout, applied)\n'''
new_json = '''\toutcomeErr := errors.Join(\n\t\terrorIfRepositoryFailures(repositoryErrors),\n\t\terrorIfApplyFailures(applied),\n\t)\n\tif options.JSON {\n\t\treturn writeJSONWithOutcome(stdout, output, outcomeErr)\n\t}\n\tprintApplyResults(stdout, applied)\n'''
if app.count(old_json) != 1:
    raise SystemExit(f'expected one apply JSON settlement block, found {app.count(old_json)}')
app = app.replace(old_json, new_json, 1)

old_tail = '''\tfmt.Fprintf(stdout, "Completed in %s. API requests: %d; retries: %d.\\n", formatDuration(time.Duration(output.ElapsedMilliseconds)*time.Millisecond), output.RequestStats.Requests, output.RequestStats.Retries)\n\treturn errorIfRepositoryFailures(repositoryErrors)\n}\n'''
new_tail = '''\tfmt.Fprintf(stdout, "Completed in %s. API requests: %d; retries: %d.\\n", formatDuration(time.Duration(output.ElapsedMilliseconds)*time.Millisecond), output.RequestStats.Requests, output.RequestStats.Retries)\n\treturn outcomeErr\n}\n'''
if app.count(old_tail) != 1:
    raise SystemExit(f'expected one apply text settlement block, found {app.count(old_tail)}')
app = app.replace(old_tail, new_tail, 1)

helper_anchor = '''func errorIfRepositoryFailures(repositoryErrors []RepositoryError) error {\n\tif len(repositoryErrors) == 0 {\n\t\treturn nil\n\t}\n\treturn fmt.Errorf("%d repository scan or apply operation(s) failed", len(repositoryErrors))\n}\n\n'''
helper_replacement = helper_anchor + '''func errorIfApplyFailures(results []scan.ApplyResult) error {\n\tfailed := 0\n\tfor _, result := range results {\n\t\tif result.Status == scan.StatusFailed {\n\t\t\tfailed++\n\t\t}\n\t}\n\tif failed == 0 {\n\t\treturn nil\n\t}\n\treturn fmt.Errorf("%d branch deletion(s) failed", failed)\n}\n\nfunc writeJSONWithOutcome(writer io.Writer, value any, outcomeErr error) error {\n\treturn errors.Join(writeJSON(writer, value), outcomeErr)\n}\n\n'''
if app.count(helper_anchor) != 1:
    raise SystemExit(f'expected one repository failure helper, found {app.count(helper_anchor)}')
app = app.replace(helper_anchor, helper_replacement, 1)
app_path.write_text(app, encoding='utf-8')

undo_path = Path('internal/app/undo.go')
undo = undo_path.read_text(encoding='utf-8')

old_settlement = '''\tif err := errors.Join(restoreErr, receiptErr); err != nil {\n\t\treturn err\n\t}\n\toutput := UndoOutput{SchemaVersion: "tidy-branches.undo-result.v1", Receipt: path, Results: results, RequestStats: client.SnapshotStats()}\n\tif jsonOutput {\n\t\treturn writeJSON(stdout, output)\n\t}\n'''
new_settlement = '''\toutcomeErr := errors.Join(restoreErr, receiptErr, errorIfRestoreFailures(results))\n\toutput := UndoOutput{SchemaVersion: "tidy-branches.undo-result.v1", Receipt: path, Results: results, RequestStats: client.SnapshotStats()}\n\tif jsonOutput {\n\t\treturn writeJSONWithOutcome(stdout, output, outcomeErr)\n\t}\n'''
if undo.count(old_settlement) != 1:
    raise SystemExit(f'expected one undo early-return settlement block, found {undo.count(old_settlement)}')
undo = undo.replace(old_settlement, new_settlement, 1)

old_receipt_tail = '''\tif len(remaining) == 0 {\n\t\tfmt.Fprintln(stdout, style.green("✓ Undo receipt cleared."))\n\t} else {\n\t\tfmt.Fprintf(stderr, "%d item(s) remain in the undo receipt because they could not be safely restored.\\n", len(remaining))\n\t}\n\treturn nil\n}\n'''
new_receipt_tail = '''\tif receiptErr != nil {\n\t\tfmt.Fprintf(stderr, "WARNING: undo receipt could not be reconciled: %v\\n", receiptErr)\n\t} else if len(remaining) == 0 {\n\t\tfmt.Fprintln(stdout, style.green("✓ Undo receipt cleared."))\n\t} else {\n\t\tfmt.Fprintf(stderr, "%d item(s) remain in the undo receipt because they could not be safely restored.\\n", len(remaining))\n\t}\n\treturn outcomeErr\n}\n'''
if undo.count(old_receipt_tail) != 1:
    raise SystemExit(f'expected one undo receipt tail, found {undo.count(old_receipt_tail)}')
undo = undo.replace(old_receipt_tail, new_receipt_tail, 1)

remaining_anchor = '''func remainingReceiptEntries(entries []receipt.Entry, results []scan.RestoreResult) []receipt.Entry {\n'''
restore_helper = '''func errorIfRestoreFailures(results []scan.RestoreResult) error {\n\tfailed := 0\n\tfor _, result := range results {\n\t\tif result.Status == scan.StatusRestoreFailed {\n\t\t\tfailed++\n\t\t}\n\t}\n\tif failed == 0 {\n\t\treturn nil\n\t}\n\treturn fmt.Errorf("%d branch restoration(s) failed", failed)\n}\n\n'''
if undo.count(remaining_anchor) != 1:
    raise SystemExit(f'expected one remaining-receipt helper, found {undo.count(remaining_anchor)}')
undo = undo.replace(remaining_anchor, restore_helper + remaining_anchor, 1)
undo_path.write_text(undo, encoding='utf-8')

test_path = Path('internal/app/candidate_failure_outcome_test.go')
if test_path.exists():
    raise SystemExit(f'{test_path} already exists')
test_path.write_text(r'''package app

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
''', encoding='utf-8')

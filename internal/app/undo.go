package app

import (
	"bufio"
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"strings"
	"time"

	"github.com/teamleaderleo/gh-tidy-branches/internal/githubapi"
	"github.com/teamleaderleo/gh-tidy-branches/internal/receipt"
	"github.com/teamleaderleo/gh-tidy-branches/internal/scan"
)

type UndoOutput struct {
	SchemaVersion string                 `json:"schema_version"`
	Receipt       string                 `json:"receipt"`
	Results       []scan.RestoreResult   `json:"results"`
	RequestStats  githubapi.RequestStats `json:"request_stats"`
}

func receiptEntries(results []scan.ApplyResult) []receipt.Entry {
	now := time.Now().UTC()
	entries := make([]receipt.Entry, 0)
	for _, result := range results {
		if result.Status == scan.StatusDeleted {
			entries = append(entries, receipt.Entry{Repository: result.Candidate.Repository, Branch: result.Candidate.Branch, SHA: result.Candidate.HeadSHA, PullRequest: result.Candidate.PullRequest, DeletedAt: now})
		}
	}
	return entries
}

func runUndo(ctx context.Context, args []string, stdin io.Reader, stdout, stderr io.Writer) error {
	yes, jsonOutput, err := parseUndo(args)
	if err != nil {
		return err
	}
	style := newTerminalStyle(stdout)
	stored, path, err := receipt.Load()
	if errors.Is(err, os.ErrNotExist) {
		return fmt.Errorf("no undo receipt found at %s", path)
	}
	if err != nil {
		return err
	}
	candidates := make([]scan.RestoreCandidate, 0, len(stored.Entries))
	for _, entry := range stored.Entries {
		candidates = append(candidates, scan.RestoreCandidate{Repository: entry.Repository, Branch: entry.Branch, SHA: entry.SHA})
	}
	if !jsonOutput {
		fmt.Fprintln(stdout, style.bold("Undo preview"))
		fmt.Fprintf(stdout, "%s\n", style.dim("Branches are recreated only when the name is free. Existing different SHAs are never overwritten."))
		fmt.Fprintf(stdout, "%s\n", style.dim("Receipt: "+path))
		for _, candidate := range candidates {
			fmt.Fprintf(stdout, "  %s  %-36s  %-38s  %s\n", style.cyan("RESTORE"), candidate.Repository, candidate.Branch, style.dim(shortSHA(candidate.SHA)))
		}
	}
	if !yes {
		fmt.Fprintf(stdout, "\nRestore %d branch(es)? [y/N] ", len(candidates))
		confirmed, err := readConfirmation(stdin)
		if err != nil {
			return err
		}
		if !confirmed {
			fmt.Fprintln(stdout, "Cancelled.")
			return nil
		}
	}
	client, err := githubapi.NewFromEnvironment(ctx)
	if err != nil {
		return err
	}
	results, err := scan.Restore(ctx, client, candidates, time.Second)
	if err != nil {
		return err
	}
	remaining := remainingReceiptEntries(stored.Entries, results)
	if len(remaining) == 0 {
		if err := receipt.Clear(); err != nil {
			return err
		}
	} else if _, err := receipt.Write(remaining); err != nil {
		return err
	}
	output := UndoOutput{SchemaVersion: "tidy-branches.undo-result.v1", Receipt: path, Results: results, RequestStats: client.SnapshotStats()}
	if jsonOutput {
		return writeJSON(stdout, output)
	}
	fmt.Fprintln(stdout, "\n"+style.bold("Undo results"))
	for _, result := range results {
		icon := style.yellow("↷")
		status := style.yellow(strings.ToLower(string(result.Status)))
		switch result.Status {
		case scan.StatusRestored, scan.StatusAlreadyPresent:
			icon = style.green("✓")
			status = style.green(strings.ToLower(string(result.Status)))
		case scan.StatusRestoreFailed:
			icon = style.red("✗")
			status = style.red(strings.ToLower(string(result.Status)))
		}
		fmt.Fprintf(stdout, "%s %-15s %s %s", icon, status, style.cyan(result.Candidate.Repository), result.Candidate.Branch)
		if result.Reason != "" {
			fmt.Fprintf(stdout, "\n  %s", style.dim(result.Reason))
		}
		fmt.Fprintln(stdout)
	}
	if len(remaining) == 0 {
		fmt.Fprintln(stdout, style.green("✓ Undo receipt cleared."))
	} else {
		fmt.Fprintf(stderr, "%d item(s) remain in the undo receipt because they could not be safely restored.\n", len(remaining))
	}
	return nil
}

func parseUndo(args []string) (bool, bool, error) {
	yes, jsonOutput := false, false
	for _, arg := range args {
		switch arg {
		case "--yes", "-y":
			yes = true
		case "--json":
			jsonOutput = true
		default:
			return false, false, fmt.Errorf("unknown undo option: %s", arg)
		}
	}
	return yes, jsonOutput, nil
}

func remainingReceiptEntries(entries []receipt.Entry, results []scan.RestoreResult) []receipt.Entry {
	keep := make(map[string]bool)
	for _, result := range results {
		if result.Status == scan.StatusRestoreSkipped || result.Status == scan.StatusRestoreFailed {
			keep[result.Candidate.Repository+"\x00"+result.Candidate.Branch] = true
		}
	}
	remaining := make([]receipt.Entry, 0)
	for _, entry := range entries {
		if keep[entry.Repository+"\x00"+entry.Branch] {
			remaining = append(remaining, entry)
		}
	}
	return remaining
}

func readConfirmation(reader io.Reader) (bool, error) {
	answer, err := bufio.NewReader(reader).ReadString('\n')
	if err != nil && !errors.Is(err, io.EOF) {
		return false, fmt.Errorf("read confirmation: %w", err)
	}
	switch strings.ToLower(strings.TrimSpace(answer)) {
	case "y", "yes":
		return true, nil
	default:
		return false, nil
	}
}

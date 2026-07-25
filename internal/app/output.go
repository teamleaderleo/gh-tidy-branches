package app

import (
	"encoding/json"
	"fmt"
	"io"
	"strings"
	"time"

	"github.com/teamleaderleo/gh-tidy-branches/internal/githubapi"
	"github.com/teamleaderleo/gh-tidy-branches/internal/scan"
)

func printPreview(writer io.Writer, results []scan.Result, repositoryErrors []RepositoryError, elapsedMS int64, stats githubapi.RequestStats) {
	fmt.Fprintln(writer, "\nDeletion preview")
	fmt.Fprintln(writer, "================")
	fmt.Fprintln(writer, "Only merged same-repository PR branches with an unchanged exact head SHA are shown.")
	fmt.Fprintln(writer, "Every item is checked again immediately before deletion.")
	for _, result := range results {
		fmt.Fprintf(writer, "\n%s: %d branch(es), %d open PR(s), %d eligible\n", result.Repository, result.BranchCount, result.OpenPRCount, len(result.Candidates))
		for _, candidate := range result.Candidates {
			fmt.Fprintf(writer, "  [delete] %-38s PR #%-6d merged %s  SHA %s\n", candidate.Branch, candidate.PullRequest, candidate.MergedAt.Format("2006-01-02"), shortSHA(candidate.HeadSHA))
		}
	}
	for _, repositoryError := range repositoryErrors {
		fmt.Fprintf(writer, "\n%s: ERROR: %s\n", repositoryError.Repository, repositoryError.Error)
	}
	fmt.Fprintf(writer, "\nScanned in %s using %d API request(s) and %d retry/retries.\n", formatDuration(time.Duration(elapsedMS)*time.Millisecond), stats.Requests, stats.Retries)
}

func printApplyResults(writer io.Writer, applied []scan.ApplyResult) {
	fmt.Fprintln(writer, "\nApply results")
	fmt.Fprintln(writer, "=============")
	for _, result := range applied {
		fmt.Fprintf(writer, "%-8s %-36s %s", strings.ToUpper(string(result.Status)), result.Candidate.Repository, result.Candidate.Branch)
		if result.Reason != "" {
			fmt.Fprintf(writer, ": %s", result.Reason)
		}
		fmt.Fprintln(writer)
	}
}

func countCandidates(results []scan.Result) int {
	total := 0
	for _, result := range results {
		total += len(result.Candidates)
	}
	return total
}

func shortSHA(value string) string {
	if len(value) > 10 {
		return value[:10]
	}
	return value
}

func formatDuration(value time.Duration) string {
	if value < time.Second {
		return fmt.Sprintf("%dms", value.Milliseconds())
	}
	return value.Round(10 * time.Millisecond).String()
}

func writeJSON(writer io.Writer, output any) error {
	encoder := json.NewEncoder(writer)
	encoder.SetIndent("", "  ")
	return encoder.Encode(output)
}

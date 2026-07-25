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
	style := newTerminalStyle(writer)
	candidateCount := countCandidates(results)

	fmt.Fprintln(writer)
	if candidateCount == 0 {
		fmt.Fprintln(writer, style.bold("Repository check"))
		fmt.Fprintln(writer, style.dim("No branch will be changed."))
	} else {
		fmt.Fprintln(writer, style.bold("Deletion preview"))
		fmt.Fprintln(writer, style.dim("Every branch below matches the exact head SHA of a merged pull request."))
		fmt.Fprintln(writer, style.dim("The branch and open pull requests are checked again immediately before deletion."))
	}

	for _, result := range results {
		status := style.green("✓")
		summary := style.green("tidy")
		if len(result.Candidates) > 0 {
			status = style.yellow("!")
			summary = style.yellow(fmt.Sprintf("%d eligible", len(result.Candidates)))
		}
		fmt.Fprintf(writer, "\n%s %s\n", status, style.cyan(result.Repository))
		fmt.Fprintf(writer, "  %d branches · %d open PRs · %s\n", result.BranchCount, result.OpenPRCount, summary)
		for _, candidate := range result.Candidates {
			fmt.Fprintf(writer, "  %s  %-36s  PR #%-5d  merged %s  %s\n",
				style.red("DELETE"),
				candidate.Branch,
				candidate.PullRequest,
				candidate.MergedAt.Format("2006-01-02"),
				style.dim(shortSHA(candidate.HeadSHA)),
			)
		}
	}

	for _, repositoryError := range repositoryErrors {
		fmt.Fprintf(writer, "\n%s %s\n  %s\n", style.red("✗"), style.cyan(repositoryError.Repository), repositoryError.Error)
	}

	footer := fmt.Sprintf("Scanned in %s · %d API request(s)", formatDuration(time.Duration(elapsedMS)*time.Millisecond), stats.Requests)
	if stats.Retries > 0 {
		footer += fmt.Sprintf(" · %d retry/retries", stats.Retries)
	}
	fmt.Fprintf(writer, "\n%s\n", style.dim(footer))
}

func printApplyResults(writer io.Writer, applied []scan.ApplyResult) {
	style := newTerminalStyle(writer)
	fmt.Fprintln(writer, "\n"+style.bold("Results"))
	for _, result := range applied {
		icon := style.yellow("↷")
		status := strings.ToLower(string(result.Status))
		switch result.Status {
		case scan.StatusDeleted:
			icon = style.green("✓")
			status = style.green(status)
		case scan.StatusFailed:
			icon = style.red("✗")
			status = style.red(status)
		default:
			status = style.yellow(status)
		}
		fmt.Fprintf(writer, "%s %-7s %s %s", icon, status, style.cyan(result.Candidate.Repository), result.Candidate.Branch)
		if result.Reason != "" {
			fmt.Fprintf(writer, "\n  %s", style.dim(result.Reason))
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

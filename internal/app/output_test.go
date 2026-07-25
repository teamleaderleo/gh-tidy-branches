package app

import (
	"bytes"
	"strings"
	"testing"
	"time"

	"github.com/teamleaderleo/gh-tidy-branches/internal/githubapi"
	"github.com/teamleaderleo/gh-tidy-branches/internal/scan"
)

func TestPrintPreviewIsReadableWithoutColour(t *testing.T) {
	t.Setenv("NO_COLOR", "1")
	mergedAt := time.Date(2026, 7, 20, 12, 0, 0, 0, time.UTC)
	results := []scan.Result{{
		Repository:  "owner/repo",
		BranchCount: 3,
		OpenPRCount: 1,
		Candidates: []scan.Candidate{{
			Repository:  "owner/repo",
			Branch:      "feat/finished-work",
			PullRequest: 123,
			HeadSHA:     "0123456789abcdef",
			MergedAt:    mergedAt,
		}},
	}}

	var output bytes.Buffer
	printPreview(&output, results, nil, 1250, githubapi.RequestStats{Requests: 6, Retries: 1})
	text := output.String()

	for _, expected := range []string{
		"Deletion preview",
		"owner/repo",
		"3 branches · 1 open PRs · 1 eligible",
		"DELETE",
		"feat/finished-work",
		"PR #123",
		"merged 2026-07-20",
		"0123456789",
		"Scanned in 1.25s · 6 API request(s) · 1 retry/retries",
	} {
		if !strings.Contains(text, expected) {
			t.Fatalf("output did not contain %q:\n%s", expected, text)
		}
	}
	if strings.Contains(text, "\x1b[") {
		t.Fatalf("NO_COLOR output contained ANSI escapes: %q", text)
	}
}

func TestPrintApplyResultsColourCanBeForced(t *testing.T) {
	t.Setenv("NO_COLOR", "")
	t.Setenv("TERM", "xterm-256color")
	t.Setenv("CLICOLOR_FORCE", "1")

	var output bytes.Buffer
	printApplyResults(&output, []scan.ApplyResult{{
		Candidate: scan.Candidate{Repository: "owner/repo", Branch: "feat/done"},
		Status:    scan.StatusDeleted,
	}})

	text := output.String()
	if !strings.Contains(text, "\x1b[32m✓\x1b[0m") {
		t.Fatalf("expected forced colour in output: %q", text)
	}
	if !strings.Contains(text, "owner/repo") || !strings.Contains(text, "feat/done") {
		t.Fatalf("missing apply result details: %q", text)
	}
}

func TestColourDisabledForDumbTerminal(t *testing.T) {
	t.Setenv("NO_COLOR", "")
	t.Setenv("CLICOLOR_FORCE", "")
	t.Setenv("TERM", "dumb")

	var output bytes.Buffer
	style := newTerminalStyle(&output)
	if style.color {
		t.Fatal("expected colour to be disabled for TERM=dumb")
	}
}

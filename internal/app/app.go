package app

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/teamleaderleo/gh-tidy-branches/internal/config"
	"github.com/teamleaderleo/gh-tidy-branches/internal/githubapi"
	"github.com/teamleaderleo/gh-tidy-branches/internal/receipt"
	"github.com/teamleaderleo/gh-tidy-branches/internal/scan"
)

const Version = "0.1.0-dev"

type Options struct {
	All          bool
	DryRun       bool
	Yes          bool
	JSON         bool
	Jobs         int
	DeleteDelay  time.Duration
	Repositories []string
}

type Output struct {
	SchemaVersion       string                 `json:"schema_version"`
	ElapsedMilliseconds int64                  `json:"elapsed_ms"`
	CandidateCount      int                    `json:"candidate_count"`
	Results             []scan.Result          `json:"results"`
	ApplyResults        []scan.ApplyResult     `json:"apply_results,omitempty"`
	Errors              []RepositoryError      `json:"errors,omitempty"`
	RequestStats        githubapi.RequestStats `json:"request_stats"`
	UndoReceipt         string                 `json:"undo_receipt,omitempty"`
}

type RepositoryError struct {
	Repository string `json:"repository"`
	Error      string `json:"error"`
}

func Run(ctx context.Context, args []string, stdin io.Reader, stdout, stderr io.Writer) error {
	if len(args) > 0 {
		switch args[0] {
		case "config":
			return runConfig(args[1:], stdout)
		case "doctor":
			return runDoctor(ctx, args[1:], stdout)
		case "undo":
			return runUndo(ctx, args[1:], stdin, stdout, stderr)
		case "help", "--help", "-h":
			printHelp(stdout)
			return nil
		case "version", "--version":
			fmt.Fprintln(stdout, Version)
			return nil
		}
	}

	started := time.Now()
	options, err := parse(args)
	if err != nil {
		return err
	}
	client, err := githubapi.NewFromEnvironment(ctx)
	if err != nil {
		return err
	}
	repositories, err := selectRepositories(ctx, options)
	if err != nil {
		return err
	}

	progress := io.Discard
	if !options.JSON {
		progress = stderr
	}
	fmt.Fprintf(progress, "Scanning %d repository(s) with up to %d concurrent worker(s)...\n", len(repositories), options.Jobs)
	results, repositoryErrors := scanRepositories(ctx, client, repositories, options.Jobs, progress)
	candidateCount := countCandidates(results)
	output := Output{
		SchemaVersion:       "tidy-branches.output.v1",
		ElapsedMilliseconds: time.Since(started).Milliseconds(),
		CandidateCount:      candidateCount,
		Results:             results,
		Errors:              repositoryErrors,
		RequestStats:        client.SnapshotStats(),
	}

	if options.JSON && !options.Yes {
		if err := writeJSON(stdout, output); err != nil {
			return err
		}
		return errorIfRepositoryFailures(repositoryErrors)
	}
	if !options.JSON {
		printPreview(stdout, results, repositoryErrors, output.ElapsedMilliseconds, output.RequestStats)
	}
	if candidateCount == 0 {
		if options.JSON {
			if err := writeJSON(stdout, output); err != nil {
				return err
			}
		}
		if !options.JSON {
			fmt.Fprintln(stdout, "\nEverything is tidy.")
		}
		return errorIfRepositoryFailures(repositoryErrors)
	}
	if options.DryRun {
		if options.JSON {
			return writeJSON(stdout, output)
		}
		fmt.Fprintf(stdout, "\nPreview only: %d branch(es) eligible. No changes made.\n", candidateCount)
		return errorIfRepositoryFailures(repositoryErrors)
	}
	if !options.Yes {
		if promptDisabled() {
			return errors.New("interactive prompting is disabled; use --preview to inspect candidates or --yes to delete them")
		}
		fmt.Fprintf(stdout, "\nDelete these %d remote branch(es)? [y/N] ", candidateCount)
		confirmed, err := readConfirmation(stdin)
		if err != nil {
			return err
		}
		if !confirmed {
			fmt.Fprintln(stdout, "Cancelled.")
			return errorIfRepositoryFailures(repositoryErrors)
		}
	}

	applied, applyErrors := applyScanResults(ctx, client, results, options.DeleteDelay)
	repositoryErrors = append(repositoryErrors, applyErrors...)
	output.ApplyResults = applied
	output.Errors = repositoryErrors
	output.RequestStats = client.SnapshotStats()
	deletedEntries := receiptEntries(applied)
	if len(deletedEntries) > 0 {
		path, receiptErr := receipt.Write(deletedEntries)
		if receiptErr != nil {
			fmt.Fprintf(stderr, "WARNING: branches were deleted but the undo receipt could not be saved: %v\n", receiptErr)
			repositoryErrors = append(repositoryErrors, RepositoryError{Repository: "undo-receipt", Error: receiptErr.Error()})
			output.Errors = repositoryErrors
		} else {
			output.UndoReceipt = path
		}
	}
	output.ElapsedMilliseconds = time.Since(started).Milliseconds()

	if options.JSON {
		if err := writeJSON(stdout, output); err != nil {
			return err
		}
		return errorIfRepositoryFailures(repositoryErrors)
	}
	printApplyResults(stdout, applied)
	if output.UndoReceipt != "" {
		fmt.Fprintf(stdout, "\nUndo receipt: %s\n", output.UndoReceipt)
		fmt.Fprintln(stdout, "Restore the deleted branches with: gh tidy-branches undo")
	}
	fmt.Fprintf(stdout, "Completed in %s. API requests: %d; retries: %d.\n", formatDuration(time.Duration(output.ElapsedMilliseconds)*time.Millisecond), output.RequestStats.Requests, output.RequestStats.Retries)
	return errorIfRepositoryFailures(repositoryErrors)
}

func parse(args []string) (Options, error) {
	options := Options{Jobs: 2, DeleteDelay: time.Second}
	for index := 0; index < len(args); index++ {
		arg := args[index]
		switch {
		case arg == "--all":
			options.All = true
		case arg == "--dry-run" || arg == "--preview" || arg == "-n":
			options.DryRun = true
		case arg == "--yes" || arg == "-y":
			options.Yes = true
		case arg == "--json":
			options.JSON = true
		case arg == "--repo" || arg == "-R":
			index++
			if index >= len(args) || strings.TrimSpace(args[index]) == "" {
				return Options{}, errors.New("--repo requires an owner/repo value")
			}
			options.Repositories = append(options.Repositories, args[index])
		case strings.HasPrefix(arg, "--repo="):
			value := strings.TrimSpace(strings.TrimPrefix(arg, "--repo="))
			if value == "" {
				return Options{}, errors.New("--repo requires an owner/repo value")
			}
			options.Repositories = append(options.Repositories, value)
		case arg == "--jobs":
			index++
			if index >= len(args) {
				return Options{}, errors.New("--jobs requires a value")
			}
			value, err := parseJobs(args[index])
			if err != nil {
				return Options{}, err
			}
			options.Jobs = value
		case strings.HasPrefix(arg, "--jobs="):
			value, err := parseJobs(strings.TrimPrefix(arg, "--jobs="))
			if err != nil {
				return Options{}, err
			}
			options.Jobs = value
		case arg == "--delete-delay":
			index++
			if index >= len(args) {
				return Options{}, errors.New("--delete-delay requires a duration")
			}
			value, err := parseDelay(args[index])
			if err != nil {
				return Options{}, err
			}
			options.DeleteDelay = value
		case strings.HasPrefix(arg, "--delete-delay="):
			value, err := parseDelay(strings.TrimPrefix(arg, "--delete-delay="))
			if err != nil {
				return Options{}, err
			}
			options.DeleteDelay = value
		case strings.HasPrefix(arg, "-"):
			return Options{}, fmt.Errorf("unknown option: %s", arg)
		default:
			options.Repositories = append(options.Repositories, arg)
		}
	}
	options.Repositories = unique(options.Repositories)
	if options.DryRun && options.Yes {
		return Options{}, errors.New("--preview/--dry-run and --yes cannot be used together")
	}
	if options.All && len(options.Repositories) > 0 {
		return Options{}, errors.New("--all cannot be combined with explicit repositories")
	}
	return options, nil
}

func promptDisabled() bool {
	return os.Getenv("GH_PROMPT_DISABLED") != ""
}

func parseJobs(value string) (int, error) {
	jobs, err := strconv.Atoi(value)
	if err != nil || jobs < 1 || jobs > 16 {
		return 0, errors.New("--jobs must be between 1 and 16")
	}
	return jobs, nil
}

func parseDelay(value string) (time.Duration, error) {
	delay, err := time.ParseDuration(value)
	if err != nil || delay < 0 {
		return 0, errors.New("--delete-delay must be a non-negative duration")
	}
	return delay, nil
}

func selectRepositories(ctx context.Context, options Options) ([]string, error) {
	if len(options.Repositories) > 0 {
		return options.Repositories, nil
	}
	if options.All {
		repositories, err := config.Load()
		if err != nil {
			return nil, err
		}
		if len(repositories) == 0 {
			return nil, errors.New("no configured repositories: use `gh tidy-branches config add owner/repo`")
		}
		return repositories, nil
	}
	if current, err := currentRepository(ctx); err == nil && current != "" {
		return []string{current}, nil
	}
	repositories, err := config.Load()
	if err != nil {
		return nil, err
	}
	if len(repositories) == 0 {
		return nil, errors.New("no current or configured repository")
	}
	return repositories, nil
}

func currentRepository(ctx context.Context) (string, error) {
	command := exec.CommandContext(ctx, "gh", "repo", "view", "--json", "nameWithOwner", "--jq", ".nameWithOwner")
	output, err := command.Output()
	if err != nil {
		return "", err
	}
	return strings.TrimSpace(string(output)), nil
}

func scanRepositories(ctx context.Context, client *githubapi.Client, repositories []string, jobs int, progress io.Writer) ([]scan.Result, []RepositoryError) {
	type indexedResult struct {
		index  int
		result scan.Result
		err    error
	}
	semaphore := make(chan struct{}, jobs)
	channel := make(chan indexedResult, len(repositories))
	var wg sync.WaitGroup
	var progressMu sync.Mutex
	for index, repository := range repositories {
		wg.Add(1)
		go func(index int, repository string) {
			defer wg.Done()
			select {
			case semaphore <- struct{}{}:
				defer func() { <-semaphore }()
			case <-ctx.Done():
				channel <- indexedResult{index: index, err: ctx.Err()}
				return
			}
			result, err := scan.Repository(ctx, client, repository)
			progressMu.Lock()
			if err != nil {
				fmt.Fprintf(progress, "  x %s: %v\n", repository, err)
			} else {
				fmt.Fprintf(progress, "  ✓ %s: %d eligible in %s\n", repository, len(result.Candidates), formatDuration(time.Duration(result.ElapsedMilliseconds)*time.Millisecond))
			}
			progressMu.Unlock()
			channel <- indexedResult{index: index, result: result, err: err}
		}(index, repository)
	}
	wg.Wait()
	close(channel)
	ordered := make([]indexedResult, len(repositories))
	for item := range channel {
		ordered[item.index] = item
	}
	results := make([]scan.Result, 0, len(repositories))
	var repositoryErrors []RepositoryError
	for index, item := range ordered {
		if item.err != nil {
			repositoryErrors = append(repositoryErrors, RepositoryError{Repository: repositories[index], Error: item.err.Error()})
			continue
		}
		results = append(results, item.result)
	}
	return results, repositoryErrors
}

func applyScanResults(ctx context.Context, api scan.API, results []scan.Result, delay time.Duration) ([]scan.ApplyResult, []RepositoryError) {
	var applied []scan.ApplyResult
	var repositoryErrors []RepositoryError
	for _, result := range results {
		if len(result.Candidates) == 0 {
			continue
		}
		repositoryResults, err := scan.Apply(ctx, api, result.Repository, result.Candidates, delay)
		applied = append(applied, repositoryResults...)
		if err != nil {
			repositoryErrors = append(repositoryErrors, RepositoryError{Repository: result.Repository, Error: err.Error()})
		}
	}
	return applied, repositoryErrors
}

func errorIfRepositoryFailures(repositoryErrors []RepositoryError) error {
	if len(repositoryErrors) == 0 {
		return nil
	}
	return fmt.Errorf("%d repository scan or apply operation(s) failed", len(repositoryErrors))
}

func unique(values []string) []string {
	seen := make(map[string]struct{}, len(values))
	result := make([]string, 0, len(values))
	for _, value := range values {
		value = strings.TrimSpace(value)
		if value == "" {
			continue
		}
		if _, exists := seen[value]; exists {
			continue
		}
		seen[value] = struct{}{}
		result = append(result, value)
	}
	sort.Strings(result)
	return result
}

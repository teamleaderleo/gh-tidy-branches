package app

import (
	"bufio"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os/exec"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/teamleaderleo/gh-tidy-branches/internal/config"
	"github.com/teamleaderleo/gh-tidy-branches/internal/githubapi"
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
	SchemaVersion string             `json:"schema_version"`
	Results       []scan.Result      `json:"results"`
	ApplyResults  []scan.ApplyResult `json:"apply_results,omitempty"`
	Errors        []RepositoryError  `json:"errors,omitempty"`
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
			return runDoctor(ctx, stdout)
		case "help", "--help", "-h":
			printHelp(stdout)
			return nil
		case "version", "--version":
			fmt.Fprintln(stdout, Version)
			return nil
		}
	}

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

	results, repositoryErrors := scanRepositories(ctx, client, repositories, options.Jobs)
	output := Output{
		SchemaVersion: "tidy-branches.output.v1",
		Results:       results,
		Errors:        repositoryErrors,
	}

	candidateCount := 0
	for _, result := range results {
		candidateCount += len(result.Candidates)
	}

	if options.JSON && !options.Yes {
		if err := writeJSON(stdout, output); err != nil {
			return err
		}
		return errorIfRepositoryFailures(repositoryErrors)
	}

	if !options.JSON {
		printResults(stdout, results, repositoryErrors)
	}

	if candidateCount == 0 {
		if options.JSON {
			if err := writeJSON(stdout, output); err != nil {
				return err
			}
			return errorIfRepositoryFailures(repositoryErrors)
		}
		fmt.Fprintln(stdout)
		fmt.Fprintln(stdout, "Everything is tidy.")
		return errorIfRepositoryFailures(repositoryErrors)
	}

	if options.DryRun {
		if options.JSON {
			return writeJSON(stdout, output)
		}
		fmt.Fprintln(stdout)
		fmt.Fprintf(stdout, "Dry run: %d branch(es) eligible.\n", candidateCount)
		return errorIfRepositoryFailures(repositoryErrors)
	}

	if !options.Yes {
		fmt.Fprintln(stdout)
		fmt.Fprintf(stdout, "Delete %d eligible branch(es)? [y/N] ", candidateCount)
		answer, err := bufio.NewReader(stdin).ReadString('\n')
		if err != nil && !errors.Is(err, io.EOF) {
			return fmt.Errorf("read confirmation: %w", err)
		}
		switch strings.ToLower(strings.TrimSpace(answer)) {
		case "y", "yes":
		default:
			fmt.Fprintln(stdout, "Cancelled.")
			return errorIfRepositoryFailures(repositoryErrors)
		}
	}

	var applied []scan.ApplyResult
	for _, result := range results {
		if len(result.Candidates) == 0 {
			continue
		}
		repositoryResults, err := scan.Apply(ctx, client, result.Repository, result.Candidates, options.DeleteDelay)
		if err != nil {
			repositoryErrors = append(repositoryErrors, RepositoryError{
				Repository: result.Repository,
				Error:      err.Error(),
			})
			continue
		}
		applied = append(applied, repositoryResults...)
	}

	output.ApplyResults = applied
	output.Errors = repositoryErrors

	if options.JSON {
		if err := writeJSON(stdout, output); err != nil {
			return err
		}
		return errorIfRepositoryFailures(repositoryErrors)
	}

	fmt.Fprintln(stdout)
	for _, result := range applied {
		fmt.Fprintf(stdout, "%-8s %-36s %s", strings.ToUpper(string(result.Status)), result.Candidate.Repository, result.Candidate.Branch)
		if result.Reason != "" {
			fmt.Fprintf(stdout, ": %s", result.Reason)
		}
		fmt.Fprintln(stdout)
	}

	return errorIfRepositoryFailures(repositoryErrors)
}

func parse(args []string) (Options, error) {
	options := Options{
		Jobs:        2,
		DeleteDelay: time.Second,
	}

	for index := 0; index < len(args); index++ {
		arg := args[index]
		switch {
		case arg == "--all":
			options.All = true
		case arg == "--dry-run" || arg == "-n":
			options.DryRun = true
		case arg == "--yes" || arg == "-y":
			options.Yes = true
		case arg == "--json":
			options.JSON = true
		case arg == "--jobs":
			index++
			if index >= len(args) {
				return Options{}, errors.New("--jobs requires a value")
			}
			value, err := strconv.Atoi(args[index])
			if err != nil || value < 1 || value > 16 {
				return Options{}, errors.New("--jobs must be between 1 and 16")
			}
			options.Jobs = value
		case strings.HasPrefix(arg, "--jobs="):
			value, err := strconv.Atoi(strings.TrimPrefix(arg, "--jobs="))
			if err != nil || value < 1 || value > 16 {
				return Options{}, errors.New("--jobs must be between 1 and 16")
			}
			options.Jobs = value
		case arg == "--delete-delay":
			index++
			if index >= len(args) {
				return Options{}, errors.New("--delete-delay requires a duration")
			}
			value, err := time.ParseDuration(args[index])
			if err != nil || value < 0 {
				return Options{}, errors.New("--delete-delay must be a non-negative duration")
			}
			options.DeleteDelay = value
		case strings.HasPrefix(arg, "--delete-delay="):
			value, err := time.ParseDuration(strings.TrimPrefix(arg, "--delete-delay="))
			if err != nil || value < 0 {
				return Options{}, errors.New("--delete-delay must be a non-negative duration")
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
		return Options{}, errors.New("--dry-run and --yes cannot be used together")
	}
	return options, nil
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

func scanRepositories(ctx context.Context, client *githubapi.Client, repositories []string, jobs int) ([]scan.Result, []RepositoryError) {
	type indexedResult struct {
		index  int
		result scan.Result
		err    error
	}

	semaphore := make(chan struct{}, jobs)
	channel := make(chan indexedResult, len(repositories))
	var wg sync.WaitGroup

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
			repositoryErrors = append(repositoryErrors, RepositoryError{
				Repository: repositories[index],
				Error:      item.err.Error(),
			})
			continue
		}
		results = append(results, item.result)
	}
	return results, repositoryErrors
}

func printResults(writer io.Writer, results []scan.Result, repositoryErrors []RepositoryError) {
	for _, result := range results {
		fmt.Fprintln(writer)
		fmt.Fprintf(writer, "%s: %d branch(es), %d open PR(s), %d eligible\n",
			result.Repository,
			result.BranchCount,
			result.OpenPRCount,
			len(result.Candidates),
		)
		for _, candidate := range result.Candidates {
			fmt.Fprintf(writer, "  %-44s PR #%-6d merged %s\n",
				candidate.Branch,
				candidate.PullRequest,
				candidate.MergedAt.Format("2006-01-02"),
			)
		}
	}
	for _, repositoryError := range repositoryErrors {
		fmt.Fprintln(writer)
		fmt.Fprintf(writer, "%s: ERROR: %s\n", repositoryError.Repository, repositoryError.Error)
	}
}

func writeJSON(writer io.Writer, output Output) error {
	encoder := json.NewEncoder(writer)
	encoder.SetIndent("", "  ")
	return encoder.Encode(output)
}

func runConfig(args []string, stdout io.Writer) error {
	if len(args) == 0 || args[0] == "list" {
		repositories, err := config.Load()
		if err != nil {
			return err
		}
		for _, repository := range repositories {
			fmt.Fprintln(stdout, repository)
		}
		return nil
	}
	if len(args) != 2 {
		return errors.New("usage: gh tidy-branches config add|remove owner/repo")
	}
	switch args[0] {
	case "add":
		if err := config.Add(args[1]); err != nil {
			return err
		}
		fmt.Fprintf(stdout, "Added %s\n", args[1])
		return nil
	case "remove":
		if err := config.Remove(args[1]); err != nil {
			return err
		}
		fmt.Fprintf(stdout, "Removed %s\n", args[1])
		return nil
	default:
		return errors.New("usage: gh tidy-branches config add|remove|list")
	}
}

func runDoctor(ctx context.Context, stdout io.Writer) error {
	client, err := githubapi.NewFromEnvironment(ctx)
	if err != nil {
		return err
	}
	configPath, err := config.Path()
	if err != nil {
		return err
	}

	fmt.Fprintf(stdout, "Version: %s\n", Version)
	fmt.Fprintf(stdout, "Host: %s\n", client.Host)
	fmt.Fprintf(stdout, "API: %s\n", client.BaseURL)
	fmt.Fprintf(stdout, "Config: %s\n", configPath)
	if repository, err := currentRepository(ctx); err == nil {
		fmt.Fprintf(stdout, "Current repository: %s\n", repository)
	} else {
		fmt.Fprintln(stdout, "Current repository: none")
	}
	return nil
}

func printHelp(writer io.Writer) {
	fmt.Fprintln(writer, `Tidy Branches safely removes remote branches whose pull requests merged.

Usage:
  gh tidy-branches [flags] [owner/repo ...]
  gh tidy-branches config add owner/repo
  gh tidy-branches config remove owner/repo
  gh tidy-branches config list
  gh tidy-branches doctor

Flags:
  --all                 scan configured repositories
  -n, --dry-run         display eligible branches without deleting
  -y, --yes             delete eligible branches without prompting
  --jobs N              concurrent repository scans, default 2
  --json                machine-readable output
  --delete-delay 1s     delay between successful delete requests`)
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

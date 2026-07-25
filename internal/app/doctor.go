package app

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"time"

	"github.com/teamleaderleo/gh-tidy-branches/internal/config"
	"github.com/teamleaderleo/gh-tidy-branches/internal/githubapi"
	"github.com/teamleaderleo/gh-tidy-branches/internal/receipt"
)

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

func runDoctor(ctx context.Context, args []string, stdout io.Writer) error {
	client, err := githubapi.NewFromEnvironment(ctx)
	if err != nil {
		return err
	}
	configPath, err := config.Path()
	if err != nil {
		return err
	}
	configured, err := config.Load()
	if err != nil {
		return err
	}
	receiptPath, err := receipt.Path()
	if err != nil {
		return err
	}
	current, _ := currentRepository(ctx)
	fmt.Fprintf(stdout, "Version: %s\n", Version)
	fmt.Fprintf(stdout, "Authentication: OK\nHost: %s\nAPI: %s\n", client.Host, client.BaseURL)
	fmt.Fprintf(stdout, "Config: %s (%d repositories)\n", configPath, len(configured))
	if current == "" {
		fmt.Fprintln(stdout, "Current repository: none")
	} else {
		fmt.Fprintf(stdout, "Current repository: %s\n", current)
	}
	if stored, _, loadErr := receipt.Load(); loadErr == nil {
		fmt.Fprintf(stdout, "Undo receipt: %s (%d branches)\n", receiptPath, len(stored.Entries))
	} else if errors.Is(loadErr, os.ErrNotExist) {
		fmt.Fprintf(stdout, "Undo receipt: none (%s)\n", receiptPath)
	} else {
		fmt.Fprintf(stdout, "Undo receipt: ERROR: %v\n", loadErr)
	}

	repositories := unique(args)
	if len(repositories) == 0 && current != "" {
		repositories = []string{current}
	}
	if len(repositories) == 0 {
		repositories = configured
	}
	for _, repositoryName := range repositories {
		repository, repoErr := client.GetRepository(ctx, repositoryName)
		if repoErr != nil {
			fmt.Fprintf(stdout, "\n%s: ERROR: %v\n", repositoryName, repoErr)
			continue
		}
		access := "read"
		if repository.Permissions.Admin {
			access = "admin"
		} else if repository.Permissions.Push {
			access = "write"
		}
		autoDelete := "disabled"
		if repository.DeleteBranchOnMerge {
			autoDelete = "enabled"
		}
		fmt.Fprintf(stdout, "\n%s\n  Access: %s\n  Default branch: %s\n  Archived: %t\n  Delete head branches after merge: %s\n", repository.FullName, access, repository.DefaultBranch, repository.Archived, autoDelete)
		if !repository.DeleteBranchOnMerge && repository.Permissions.Admin {
			fmt.Fprintf(stdout, "  Recommendation: gh repo edit %s --delete-branch-on-merge\n", repository.FullName)
		}
	}
	stats := client.SnapshotStats()
	fmt.Fprintf(stdout, "\nAPI requests: %d; retries: %d", stats.Requests, stats.Retries)
	if !stats.RateLimitReset.IsZero() {
		fmt.Fprintf(stdout, "; rate limit remaining: %d (resets %s)", stats.RateLimitRemaining, stats.RateLimitReset.Local().Format(time.RFC3339))
	}
	fmt.Fprintln(stdout)
	return nil
}

func printHelp(writer io.Writer) {
	fmt.Fprintln(writer, `Tidy Branches safely removes remote branches whose pull requests merged.

Usage:
  gh tidy-branches [flags] [owner/repo ...]
  gh tidy-branches undo [--yes] [--json]
  gh tidy-branches config add owner/repo
  gh tidy-branches config remove owner/repo
  gh tidy-branches config list
  gh tidy-branches doctor [owner/repo ...]

Flags:
  --all                 scan configured repositories
  -n, --preview         display eligible branches without deleting
      --dry-run         alias for --preview
  -y, --yes             delete eligible branches without prompting
  --jobs N              concurrent repository scans, default 2
  --json                machine-readable output
  --delete-delay 1s     delay between delete requests

After a successful deletion, gh tidy-branches undo can recreate branches at their
recorded SHAs when the names are still available and GitHub retains the commits.`)
}

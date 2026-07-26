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
	style := newTerminalStyle(stdout)
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

	fmt.Fprintln(stdout, style.bold("Tidy Branches doctor"))
	fmt.Fprintf(stdout, "%s Authentication  %s\n", style.green("✓"), style.dim(client.Host))
	fmt.Fprintf(stdout, "  Version          %s\n", Version)
	fmt.Fprintf(stdout, "  API              %s\n", client.BaseURL)
	fmt.Fprintf(stdout, "  Config           %s (%d repositories)\n", configPath, len(configured))
	if current == "" {
		fmt.Fprintf(stdout, "  Current repo     %s\n", style.dim("none"))
	} else {
		fmt.Fprintf(stdout, "  Current repo     %s\n", current)
	}
	if stored, _, loadErr := receipt.Load(); loadErr == nil {
		fmt.Fprintf(stdout, "  Undo receipt     %s (%d branches)\n", receiptPath, len(stored.Entries))
	} else if errors.Is(loadErr, os.ErrNotExist) {
		fmt.Fprintf(stdout, "  Undo receipt     %s\n", style.dim("none"))
	} else {
		fmt.Fprintf(stdout, "  Undo receipt     %s\n", style.red(loadErr.Error()))
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
			fmt.Fprintf(stdout, "\n%s %s\n  %s\n", style.red("✗"), style.cyan(repositoryName), repoErr)
			continue
		}
		access := "read"
		if repository.Permissions.Admin {
			access = "admin"
		} else if repository.Permissions.Push {
			access = "write"
		}

		fmt.Fprintf(stdout, "\n%s %s\n", style.green("✓"), style.cyan(repository.FullName))
		fmt.Fprintf(stdout, "  Access           %s\n", access)
		fmt.Fprintf(stdout, "  Default branch   %s\n", repository.DefaultBranch)
		fmt.Fprintf(stdout, "  Archived         %t\n", repository.Archived)
		if repository.DeleteBranchOnMerge {
			fmt.Fprintf(stdout, "  Auto-delete      %s\n", style.green("enabled"))
		} else {
			fmt.Fprintf(stdout, "  Auto-delete      %s\n", style.yellow("disabled"))
			if repository.Permissions.Admin {
				fmt.Fprintf(stdout, "  Tip              gh repo edit %s --delete-branch-on-merge\n", repository.FullName)
			}
		}
	}
	stats := client.SnapshotStats()
	fmt.Fprintf(stdout, "\n%s\n", style.dim(fmt.Sprintf("API requests: %d · retries: %d", stats.Requests, stats.Retries)))
	if !stats.RateLimitReset.IsZero() {
		fmt.Fprintf(stdout, "%s\n", style.dim(fmt.Sprintf("Rate limit remaining: %d · resets %s", stats.RateLimitRemaining, stats.RateLimitReset.Local().Format(time.RFC3339))))
	}
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
  -R, --repo REPO       scan an explicit repository; repeat for more
  -n, --preview         display eligible branches without deleting
      --dry-run         alias for --preview
  -y, --yes             delete eligible branches without prompting
  --jobs N              concurrent repository scans, default 2
  --json                machine-readable output
  --delete-delay 1s     delay between delete requests

Positional owner/repo arguments remain supported. Do not combine explicit
repositories with --all.

Colour follows GitHub CLI conventions. NO_COLOR=1 disables it and GH_FORCE_TTY=1
forces terminal presentation. GH_PROMPT_DISABLED=1 prevents interactive deletion;
use --preview or --yes explicitly in that environment.

After a successful deletion, gh tidy-branches undo can recreate branches at their
recorded SHAs when the names are still available and GitHub retains the commits.`)
}

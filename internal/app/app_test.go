package app

import (
	"reflect"
	"testing"
	"time"
)

func TestParseSupportsFlagsAnywhere(t *testing.T) {
	options, err := parse([]string{
		"owner/repo",
		"--jobs=3",
		"--dry-run",
		"owner/other",
		"--delete-delay",
		"250ms",
	})
	if err != nil {
		t.Fatal(err)
	}

	if options.Jobs != 3 {
		t.Fatalf("expected jobs 3, got %d", options.Jobs)
	}
	if !options.DryRun {
		t.Fatal("expected dry run")
	}
	if options.DeleteDelay != 250*time.Millisecond {
		t.Fatalf("unexpected delay: %s", options.DeleteDelay)
	}
	want := []string{"owner/other", "owner/repo"}
	if !reflect.DeepEqual(options.Repositories, want) {
		t.Fatalf("got %#v, want %#v", options.Repositories, want)
	}
}

func TestParseRejectsConflictingMutationFlags(t *testing.T) {
	if _, err := parse([]string{"--dry-run", "--yes"}); err == nil {
		t.Fatal("expected error")
	}
}

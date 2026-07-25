package receipt

import (
	"errors"
	"os"
	"testing"
	"time"
)

func TestWriteLoadAndClear(t *testing.T) {
	root := t.TempDir()
	t.Setenv("XDG_CACHE_HOME", root)
	t.Setenv("HOME", root)
	entries := []Entry{{Repository: "owner/repo", Branch: "feat/a", SHA: "abc", DeletedAt: time.Now().UTC()}}
	path, err := Write(entries)
	if err != nil {
		t.Fatal(err)
	}
	info, err := os.Stat(path)
	if err != nil {
		t.Fatal(err)
	}
	if info.Mode().Perm() != 0o600 {
		t.Fatalf("mode %o", info.Mode().Perm())
	}
	loaded, _, err := Load()
	if err != nil {
		t.Fatal(err)
	}
	if len(loaded.Entries) != 1 || loaded.Entries[0].SHA != "abc" {
		t.Fatalf("unexpected receipt: %#v", loaded)
	}
	if err := Clear(); err != nil {
		t.Fatal(err)
	}
	if _, _, err := Load(); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("expected not exist, got %v", err)
	}
}

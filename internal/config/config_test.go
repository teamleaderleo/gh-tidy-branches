package config

import (
	"os"
	"path/filepath"
	"reflect"
	"testing"
)

func TestAddRemoveAndLoad(t *testing.T) {
	configRoot := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", configRoot)
	t.Setenv("HOME", configRoot)

	if err := Add("owner/b"); err != nil {
		t.Fatal(err)
	}
	if err := Add("owner/a"); err != nil {
		t.Fatal(err)
	}
	if err := Add("owner/a"); err != nil {
		t.Fatal(err)
	}

	got, err := Load()
	if err != nil {
		t.Fatal(err)
	}
	want := []string{"owner/a", "owner/b"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("got %#v, want %#v", got, want)
	}

	if err := Remove("owner/a"); err != nil {
		t.Fatal(err)
	}
	got, err = Load()
	if err != nil {
		t.Fatal(err)
	}
	want = []string{"owner/b"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("got %#v, want %#v", got, want)
	}

	path, err := Path()
	if err != nil {
		t.Fatal(err)
	}
	info, err := os.Stat(path)
	if err != nil {
		t.Fatal(err)
	}
	if info.Mode().Perm() != 0o600 {
		t.Fatalf("expected config mode 0600, got %o", info.Mode().Perm())
	}
	if filepath.Base(path) != "repos" {
		t.Fatalf("unexpected config path: %s", path)
	}
}

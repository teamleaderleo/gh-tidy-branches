package receipt

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"time"
)

const SchemaVersion = "tidy-branches.undo.v1"

type Entry struct {
	Repository  string    `json:"repository"`
	Branch      string    `json:"branch"`
	SHA         string    `json:"sha"`
	PullRequest int       `json:"pull_request,omitempty"`
	DeletedAt   time.Time `json:"deleted_at"`
}

type Receipt struct {
	SchemaVersion string    `json:"schema_version"`
	CreatedAt     time.Time `json:"created_at"`
	Entries       []Entry   `json:"entries"`
}

func Path() (string, error) {
	root, err := os.UserCacheDir()
	if err != nil {
		return "", fmt.Errorf("resolve user cache directory: %w", err)
	}
	return filepath.Join(root, "tidy-branches", "last-delete.json"), nil
}

func Write(entries []Entry) (string, error) {
	path, err := Path()
	if err != nil {
		return "", err
	}
	if len(entries) == 0 {
		return path, Clear()
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		return "", fmt.Errorf("create undo receipt directory: %w", err)
	}
	payload, err := json.MarshalIndent(Receipt{SchemaVersion: SchemaVersion, CreatedAt: time.Now().UTC(), Entries: entries}, "", "  ")
	if err != nil {
		return "", fmt.Errorf("encode undo receipt: %w", err)
	}
	payload = append(payload, '\n')
	temporary := path + ".tmp"
	if err := os.WriteFile(temporary, payload, 0o600); err != nil {
		return "", fmt.Errorf("write undo receipt: %w", err)
	}
	if err := os.Rename(temporary, path); err != nil {
		return "", fmt.Errorf("replace undo receipt: %w", err)
	}
	return path, nil
}

func Load() (Receipt, string, error) {
	path, err := Path()
	if err != nil {
		return Receipt{}, "", err
	}
	payload, err := os.ReadFile(path)
	if errors.Is(err, os.ErrNotExist) {
		return Receipt{}, path, os.ErrNotExist
	}
	if err != nil {
		return Receipt{}, path, fmt.Errorf("read undo receipt: %w", err)
	}
	var value Receipt
	if err := json.Unmarshal(payload, &value); err != nil {
		return Receipt{}, path, fmt.Errorf("decode undo receipt: %w", err)
	}
	if value.SchemaVersion != SchemaVersion {
		return Receipt{}, path, fmt.Errorf("unsupported undo receipt schema %q", value.SchemaVersion)
	}
	return value, path, nil
}

func Clear() error {
	path, err := Path()
	if err != nil {
		return err
	}
	if err := os.Remove(path); err != nil && !errors.Is(err, os.ErrNotExist) {
		return fmt.Errorf("remove undo receipt: %w", err)
	}
	return nil
}

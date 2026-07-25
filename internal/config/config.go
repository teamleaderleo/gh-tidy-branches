package config

import (
	"bufio"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

func Path() (string, error) {
	root, err := os.UserConfigDir()
	if err != nil {
		return "", fmt.Errorf("resolve user config directory: %w", err)
	}
	return filepath.Join(root, "tidy-branches", "repos"), nil
}

func Load() ([]string, error) {
	path, err := Path()
	if err != nil {
		return nil, err
	}

	file, err := os.Open(path)
	if os.IsNotExist(err) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("open repository config: %w", err)
	}
	defer file.Close()

	var repositories []string
	seen := make(map[string]struct{})
	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		if _, exists := seen[line]; exists {
			continue
		}
		seen[line] = struct{}{}
		repositories = append(repositories, line)
	}
	if err := scanner.Err(); err != nil {
		return nil, fmt.Errorf("read repository config: %w", err)
	}
	sort.Strings(repositories)
	return repositories, nil
}

func Add(repository string) error {
	repositories, err := Load()
	if err != nil {
		return err
	}
	for _, existing := range repositories {
		if existing == repository {
			return nil
		}
	}
	repositories = append(repositories, repository)
	sort.Strings(repositories)
	return write(repositories)
}

func Remove(repository string) error {
	repositories, err := Load()
	if err != nil {
		return err
	}
	filtered := repositories[:0]
	for _, existing := range repositories {
		if existing != repository {
			filtered = append(filtered, existing)
		}
	}
	return write(filtered)
}

func write(repositories []string) error {
	path, err := Path()
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		return fmt.Errorf("create repository config directory: %w", err)
	}

	temporary := path + ".tmp"
	file, err := os.OpenFile(temporary, os.O_CREATE|os.O_TRUNC|os.O_WRONLY, 0o600)
	if err != nil {
		return fmt.Errorf("create temporary repository config: %w", err)
	}

	writer := bufio.NewWriter(file)
	if _, err := writer.WriteString("# One GitHub repository per line.\n"); err != nil {
		file.Close()
		return err
	}
	for _, repository := range repositories {
		if _, err := writer.WriteString(repository + "\n"); err != nil {
			file.Close()
			return err
		}
	}
	if err := writer.Flush(); err != nil {
		file.Close()
		return err
	}
	if err := file.Close(); err != nil {
		return err
	}
	if err := os.Rename(temporary, path); err != nil {
		return fmt.Errorf("replace repository config: %w", err)
	}
	return nil
}

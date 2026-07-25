package githubapi

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"
	"time"
)

func TestGetBranchPreservesSlashInBranchName(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		if request.RequestURI != "/repos/owner/repo/branches/feat%2Fthing" {
			t.Fatalf("unexpected request URI: %s", request.RequestURI)
		}
		writer.Header().Set("Content-Type", "application/json")
		fmt.Fprint(writer, `{"name":"feat/thing","commit":{"sha":"abc"}}`)
	}))
	defer server.Close()
	client := testClient(server.URL)
	branch, err := client.GetBranch(context.Background(), "owner/repo", "feat/thing")
	if err != nil {
		t.Fatal(err)
	}
	if branch.SHA() != "abc" {
		t.Fatalf("unexpected SHA: %s", branch.SHA())
	}
}

func TestListBranchesPaginates(t *testing.T) {
	var mu sync.Mutex
	pages := make([]string, 0, 2)
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		mu.Lock()
		pages = append(pages, request.URL.Query().Get("page"))
		mu.Unlock()
		writer.Header().Set("Content-Type", "application/json")
		if request.URL.Query().Get("page") == "1" {
			fmt.Fprint(writer, "[")
			for index := 0; index < 100; index++ {
				if index > 0 {
					fmt.Fprint(writer, ",")
				}
				fmt.Fprintf(writer, `{"name":"branch-%d","commit":{"sha":"sha-%d"}}`, index, index)
			}
			fmt.Fprint(writer, "]")
			return
		}
		fmt.Fprint(writer, `[{"name":"last","commit":{"sha":"last-sha"}}]`)
	}))
	defer server.Close()
	client := testClient(server.URL)
	branches, err := client.ListBranches(context.Background(), "owner/repo")
	if err != nil {
		t.Fatal(err)
	}
	if len(branches) != 101 {
		t.Fatalf("expected 101 branches, got %d", len(branches))
	}
	if len(pages) != 2 || pages[0] != "1" || pages[1] != "2" {
		t.Fatalf("unexpected pages: %#v", pages)
	}
}

func TestGetRetriesTransientResponses(t *testing.T) {
	attempts := 0
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		attempts++
		if attempts == 1 {
			http.Error(writer, `{"message":"temporary"}`, http.StatusServiceUnavailable)
			return
		}
		fmt.Fprint(writer, `{"full_name":"owner/repo","default_branch":"main"}`)
	}))
	defer server.Close()
	client := testClient(server.URL)
	if _, err := client.GetRepository(context.Background(), "owner/repo"); err != nil {
		t.Fatal(err)
	}
	if attempts != 2 {
		t.Fatalf("expected 2 attempts, got %d", attempts)
	}
	stats := client.SnapshotStats()
	if stats.Requests != 2 || stats.Retries != 1 {
		t.Fatalf("unexpected stats: %#v", stats)
	}
}

func TestGetRespectsRetryAfter(t *testing.T) {
	attempts := 0
	var slept time.Duration
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		attempts++
		if attempts == 1 {
			writer.Header().Set("Retry-After", "2")
			writer.WriteHeader(http.StatusTooManyRequests)
			fmt.Fprint(writer, `{"message":"slow down"}`)
			return
		}
		fmt.Fprint(writer, `{"full_name":"owner/repo","default_branch":"main"}`)
	}))
	defer server.Close()
	client := testClient(server.URL)
	client.Sleep = func(_ context.Context, delay time.Duration) error {
		slept = delay
		return nil
	}
	if _, err := client.GetRepository(context.Background(), "owner/repo"); err != nil {
		t.Fatal(err)
	}
	if slept != 2*time.Second {
		t.Fatalf("expected 2s sleep, got %s", slept)
	}
}

func TestDeleteIsNeverRetried(t *testing.T) {
	attempts := 0
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		attempts++
		writer.WriteHeader(http.StatusServiceUnavailable)
	}))
	defer server.Close()
	client := testClient(server.URL)
	if err := client.DeleteBranch(context.Background(), "owner/repo", "feat/thing"); err == nil {
		t.Fatal("expected error")
	}
	if attempts != 1 {
		t.Fatalf("delete attempted %d times", attempts)
	}
	if client.SnapshotStats().Retries != 0 {
		t.Fatal("delete must not increment retries")
	}
}

func TestCreateBranchPayload(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		if request.Method != http.MethodPost || request.URL.Path != "/repos/owner/repo/git/refs" {
			t.Fatalf("unexpected request: %s %s", request.Method, request.URL.Path)
		}
		var body struct {
			Ref string `json:"ref"`
			SHA string `json:"sha"`
		}
		if err := json.NewDecoder(request.Body).Decode(&body); err != nil {
			t.Fatal(err)
		}
		if body.Ref != "refs/heads/feat/restored" || body.SHA != "abc123" {
			t.Fatalf("unexpected body: %#v", body)
		}
		writer.WriteHeader(http.StatusCreated)
		fmt.Fprint(writer, `{}`)
	}))
	defer server.Close()
	if err := testClient(server.URL).CreateBranch(context.Background(), "owner/repo", "feat/restored", "abc123"); err != nil {
		t.Fatal(err)
	}
}

func testClient(baseURL string) *Client {
	return &Client{
		BaseURL:        baseURL,
		Host:           "example.test",
		Token:          "token",
		UserAgent:      "test",
		HTTPClient:     http.DefaultClient,
		RetryMax:       3,
		RetryBaseDelay: time.Millisecond,
		Sleep:          func(context.Context, time.Duration) error { return nil },
		Jitter:         func(delay time.Duration) time.Duration { return delay },
		Now:            time.Now,
	}
}

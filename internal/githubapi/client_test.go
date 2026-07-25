package githubapi

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"
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

func testClient(baseURL string) *Client {
	return &Client{
		BaseURL:    baseURL,
		Host:       "example.test",
		Token:      "token",
		UserAgent:  "test",
		HTTPClient: http.DefaultClient,
	}
}

package scan

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"

	"github.com/teamleaderleo/gh-tidy-branches/internal/githubapi"
)

func TestRepositoryUsesBulkPaginatedRequestsWithoutPerBranchDiscovery(t *testing.T) {
	var mu sync.Mutex
	counts := map[string]int{}
	mergedAt := "2026-07-25T12:00:00Z"
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		key := request.URL.Path + "?state=" + request.URL.Query().Get("state") + "&page=" + request.URL.Query().Get("page")
		mu.Lock()
		counts[key]++
		mu.Unlock()
		writer.Header().Set("Content-Type", "application/json")
		switch {
		case request.URL.Path == "/repos/owner/repo":
			fmt.Fprint(writer, `{"full_name":"owner/repo","default_branch":"main"}`)
		case request.URL.Path == "/repos/owner/repo/branches" && request.URL.Query().Get("page") == "1":
			fmt.Fprint(writer, "[")
			for index := 0; index < 100; index++ {
				if index > 0 {
					fmt.Fprint(writer, ",")
				}
				fmt.Fprintf(writer, `{"name":"branch-%d","commit":{"sha":"sha-%d"}}`, index, index)
			}
			fmt.Fprint(writer, "]")
		case request.URL.Path == "/repos/owner/repo/branches":
			fmt.Fprint(writer, `[{"name":"eligible","commit":{"sha":"eligible-sha"}}]`)
		case request.URL.Path == "/repos/owner/repo/pulls" && request.URL.Query().Get("state") == "open":
			fmt.Fprint(writer, `[]`)
		case request.URL.Path == "/repos/owner/repo/pulls" && request.URL.Query().Get("state") == "closed" && request.URL.Query().Get("page") == "1":
			fmt.Fprint(writer, "[")
			for index := 0; index < 100; index++ {
				if index > 0 {
					fmt.Fprint(writer, ",")
				}
				fmt.Fprintf(writer, `{"number":%d,"merged_at":null,"head":{"ref":"old-%d","sha":"x","repo":{"full_name":"owner/repo"}},"base":{"ref":"main"}}`, index+1, index)
			}
			fmt.Fprint(writer, "]")
		case request.URL.Path == "/repos/owner/repo/pulls":
			fmt.Fprintf(writer, `[{"number":101,"merged_at":%q,"head":{"ref":"eligible","sha":"eligible-sha","repo":{"full_name":"owner/repo"}},"base":{"ref":"main"}}]`, mergedAt)
		default:
			http.NotFound(writer, request)
		}
	}))
	defer server.Close()

	client := &githubapi.Client{
		BaseURL:    server.URL,
		Host:       "test",
		Token:      "x",
		UserAgent:  "test",
		HTTPClient: server.Client(),
		RetryMax:   -1,
	}
	result, err := Repository(context.Background(), client, "owner/repo")
	if err != nil {
		t.Fatal(err)
	}
	if len(result.Candidates) != 1 || result.Candidates[0].Branch != "eligible" {
		t.Fatalf("unexpected candidates: %#v", result.Candidates)
	}
	stats := client.SnapshotStats()
	if stats.Requests != 6 {
		t.Fatalf("expected 6 requests, got %d counts=%#v", stats.Requests, counts)
	}
	for key := range counts {
		if key == "/repos/owner/repo/branches/eligible?state=&page=" {
			t.Fatal("unexpected per-branch discovery request")
		}
	}
}

package githubapi

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestListAllClosedPullRequestsDoesNotFilterByBase(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		if request.URL.Path != "/repos/owner/repo/pulls" {
			t.Fatalf("unexpected path: %s", request.URL.Path)
		}
		query := request.URL.Query()
		if query.Get("state") != "closed" || query.Get("sort") != "created" || query.Get("direction") != "desc" {
			t.Fatalf("unexpected closed pull request query: %s", request.URL.RawQuery)
		}
		if _, present := query["base"]; present {
			t.Fatalf("all-base closed pull request query unexpectedly contains base: %s", request.URL.RawQuery)
		}
		if _, present := query["head"]; present {
			t.Fatalf("all-history closed pull request query unexpectedly contains head: %s", request.URL.RawQuery)
		}
		if query.Get("per_page") != "100" || query.Get("page") != "1" {
			t.Fatalf("pagination parameters missing: %s", request.URL.RawQuery)
		}
		writer.Header().Set("Content-Type", "application/json")
		fmt.Fprint(writer, `[]`)
	}))
	defer server.Close()

	pulls, err := testClient(server.URL).ListAllClosedPullRequests(context.Background(), "owner/repo")
	if err != nil {
		t.Fatal(err)
	}
	if len(pulls) != 0 {
		t.Fatalf("expected no pulls, got %#v", pulls)
	}
}

func TestListClosedPullRequestsForHeadFiltersOwnerAndBranchWithoutBase(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		if request.URL.Path != "/repos/owner/repo/pulls" {
			t.Fatalf("unexpected path: %s", request.URL.Path)
		}
		query := request.URL.Query()
		if query.Get("state") != "closed" || query.Get("head") != "owner:feat/reused" {
			t.Fatalf("unexpected branch ownership query: %s", request.URL.RawQuery)
		}
		if query.Get("sort") != "created" || query.Get("direction") != "desc" {
			t.Fatalf("unexpected branch ownership ordering: %s", request.URL.RawQuery)
		}
		if _, present := query["base"]; present {
			t.Fatalf("branch ownership query unexpectedly contains base: %s", request.URL.RawQuery)
		}
		if query.Get("per_page") != "100" || query.Get("page") != "1" {
			t.Fatalf("pagination parameters missing: %s", request.URL.RawQuery)
		}
		writer.Header().Set("Content-Type", "application/json")
		fmt.Fprint(writer, `[]`)
	}))
	defer server.Close()

	pulls, err := testClient(server.URL).ListClosedPullRequestsForHead(
		context.Background(),
		"owner/repo",
		"feat/reused",
	)
	if err != nil {
		t.Fatal(err)
	}
	if len(pulls) != 0 {
		t.Fatalf("expected no pulls, got %#v", pulls)
	}
}

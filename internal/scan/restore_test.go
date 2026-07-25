package scan

import (
	"context"
	"testing"

	"github.com/teamleaderleo/gh-tidy-branches/internal/githubapi"
)

func TestRestoreNeverOverwritesExistingDifferentSHA(t *testing.T) {
	api := &fakeRestoreAPI{branches: map[string]githubapi.Branch{
		"different": branch("different", "new"),
		"same":      branch("same", "old"),
	}}
	results, err := Restore(context.Background(), api, []RestoreCandidate{
		{Repository: "o/r", Branch: "missing", SHA: "abc"},
		{Repository: "o/r", Branch: "different", SHA: "old"},
		{Repository: "o/r", Branch: "same", SHA: "old"},
	}, 0)
	if err != nil {
		t.Fatal(err)
	}
	if len(api.created) != 1 || api.created[0] != "missing@abc" {
		t.Fatalf("unexpected creates: %#v", api.created)
	}
	if results[0].Status != StatusRestored || results[1].Status != StatusRestoreSkipped || results[2].Status != StatusAlreadyPresent {
		t.Fatalf("unexpected results: %#v", results)
	}
}

type fakeRestoreAPI struct {
	branches map[string]githubapi.Branch
	created  []string
}

func (f *fakeRestoreAPI) GetBranch(_ context.Context, _ string, name string) (githubapi.Branch, error) {
	value, ok := f.branches[name]
	if !ok {
		return githubapi.Branch{}, &githubapi.APIError{StatusCode: 404, Method: "GET", URL: name}
	}
	return value, nil
}

func (f *fakeRestoreAPI) CreateBranch(_ context.Context, _ string, name, sha string) error {
	f.created = append(f.created, name+"@"+sha)
	return nil
}

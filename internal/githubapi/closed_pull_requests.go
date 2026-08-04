package githubapi

import (
	"context"
	"net/url"
)

// ListAllClosedPullRequests returns closed pull requests across every base branch.
//
// Branch deletion safety needs the newest use of a same-repository head branch before deciding
// whether an older default-branch merge still owns that branch name. Filtering the request by base
// would hide a newer closed-unmerged or non-default-base reuse.
func (c *Client) ListAllClosedPullRequests(ctx context.Context, fullName string) ([]PullRequest, error) {
	return c.listPullRequests(ctx, fullName, url.Values{
		"state":     {"closed"},
		"sort":      {"created"},
		"direction": {"desc"},
	})
}

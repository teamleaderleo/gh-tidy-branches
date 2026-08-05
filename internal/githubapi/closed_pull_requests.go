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

// ListClosedPullRequestsForHead returns closed pull requests for one same-repository branch name
// across every base branch.
//
// Apply uses this immediately before deletion so a branch reuse after the original scan cannot be
// hidden by the earlier preview or by a deletion delay.
func (c *Client) ListClosedPullRequestsForHead(
	ctx context.Context,
	fullName string,
	branch string,
) ([]PullRequest, error) {
	owner, _, err := splitRepository(fullName)
	if err != nil {
		return nil, err
	}
	return c.listPullRequests(ctx, fullName, url.Values{
		"state":     {"closed"},
		"head":      {owner + ":" + branch},
		"sort":      {"created"},
		"direction": {"desc"},
	})
}

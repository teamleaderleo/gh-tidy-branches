package githubapi

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"os/exec"
	"strings"
	"time"
)

const apiVersion = "2022-11-28"

type Client struct {
	BaseURL    string
	Host       string
	Token      string
	UserAgent  string
	HTTPClient *http.Client
}

func NewFromEnvironment(ctx context.Context) (*Client, error) {
	host := strings.TrimSpace(os.Getenv("GH_HOST"))
	if host == "" {
		host = "github.com"
	}

	token := firstNonEmpty(
		os.Getenv("GH_TOKEN"),
		os.Getenv("GITHUB_TOKEN"),
		os.Getenv("GH_ENTERPRISE_TOKEN"),
	)

	if token == "" {
		cmd := exec.CommandContext(ctx, "gh", "auth", "token", "--hostname", host)
		output, err := cmd.Output()
		if err != nil {
			return nil, fmt.Errorf("read GitHub token from gh: %w", err)
		}
		token = strings.TrimSpace(string(output))
	}

	if token == "" {
		return nil, errors.New("GitHub token is empty")
	}

	baseURL := "https://api.github.com"
	if host != "github.com" {
		baseURL = "https://" + host + "/api/v3"
	}

	return &Client{
		BaseURL:   baseURL,
		Host:      host,
		Token:     token,
		UserAgent: "gh-tidy-branches",
		HTTPClient: &http.Client{
			Timeout: 45 * time.Second,
		},
	}, nil
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if strings.TrimSpace(value) != "" {
			return strings.TrimSpace(value)
		}
	}
	return ""
}

func splitRepository(fullName string) (string, string, error) {
	parts := strings.Split(fullName, "/")
	if len(parts) != 2 || strings.TrimSpace(parts[0]) == "" || strings.TrimSpace(parts[1]) == "" {
		return "", "", fmt.Errorf("repository must be owner/name: %q", fullName)
	}
	return parts[0], parts[1], nil
}

func repositoryPath(fullName string) (string, error) {
	owner, repo, err := splitRepository(fullName)
	if err != nil {
		return "", err
	}
	return "/repos/" + url.PathEscape(owner) + "/" + url.PathEscape(repo), nil
}

func (c *Client) GetRepository(ctx context.Context, fullName string) (Repository, error) {
	repoPath, err := repositoryPath(fullName)
	if err != nil {
		return Repository{}, err
	}

	var repository Repository
	if err := c.getJSON(ctx, repoPath, &repository); err != nil {
		return Repository{}, err
	}
	return repository, nil
}

func (c *Client) ListBranches(ctx context.Context, fullName string) ([]Branch, error) {
	repoPath, err := repositoryPath(fullName)
	if err != nil {
		return nil, err
	}

	var branches []Branch
	for pageNumber := 1; ; pageNumber++ {
		var pageItems []Branch
		query := url.Values{
			"per_page": {"100"},
			"page":     {fmt.Sprintf("%d", pageNumber)},
		}
		if err := c.getJSON(ctx, repoPath+"/branches?"+query.Encode(), &pageItems); err != nil {
			return nil, err
		}
		branches = append(branches, pageItems...)
		if len(pageItems) < 100 {
			break
		}
	}
	return branches, nil
}

func (c *Client) GetBranch(ctx context.Context, fullName, branch string) (Branch, error) {
	repoPath, err := repositoryPath(fullName)
	if err != nil {
		return Branch{}, err
	}

	var result Branch
	if err := c.getJSON(ctx, repoPath+"/branches/"+url.PathEscape(branch), &result); err != nil {
		return Branch{}, err
	}
	return result, nil
}

func (c *Client) ListOpenPullRequests(ctx context.Context, fullName string) ([]PullRequest, error) {
	return c.listPullRequests(ctx, fullName, url.Values{
		"state":     {"open"},
		"sort":      {"updated"},
		"direction": {"desc"},
	})
}

func (c *Client) ListClosedPullRequests(ctx context.Context, fullName, base string) ([]PullRequest, error) {
	return c.listPullRequests(ctx, fullName, url.Values{
		"state":     {"closed"},
		"base":      {base},
		"sort":      {"updated"},
		"direction": {"desc"},
	})
}

func (c *Client) listPullRequests(ctx context.Context, fullName string, query url.Values) ([]PullRequest, error) {
	repoPath, err := repositoryPath(fullName)
	if err != nil {
		return nil, err
	}

	var pulls []PullRequest
	for pageNumber := 1; ; pageNumber++ {
		pageQuery := cloneValues(query)
		pageQuery.Set("per_page", "100")
		pageQuery.Set("page", fmt.Sprintf("%d", pageNumber))

		var pageItems []PullRequest
		if err := c.getJSON(ctx, repoPath+"/pulls?"+pageQuery.Encode(), &pageItems); err != nil {
			return nil, err
		}
		pulls = append(pulls, pageItems...)
		if len(pageItems) < 100 {
			break
		}
	}
	return pulls, nil
}

func cloneValues(values url.Values) url.Values {
	cloned := make(url.Values, len(values))
	for key, items := range values {
		cloned[key] = append([]string(nil), items...)
	}
	return cloned
}

func (c *Client) DeleteBranch(ctx context.Context, fullName, branch string) error {
	repoPath, err := repositoryPath(fullName)
	if err != nil {
		return err
	}
	refPath := repoPath + "/git/refs/heads/" + url.PathEscape(branch)
	return c.doJSON(ctx, http.MethodDelete, refPath, nil, nil)
}

func (c *Client) getJSON(ctx context.Context, requestPath string, target any) error {
	return c.doJSON(ctx, http.MethodGet, requestPath, nil, target)
}

func (c *Client) doJSON(ctx context.Context, method, requestPath string, body any, target any) error {
	requestURL := strings.TrimRight(c.BaseURL, "/") + "/" + strings.TrimLeft(requestPath, "/")
	parsedURL, err := url.Parse(requestURL)
	if err != nil {
		return fmt.Errorf("parse GitHub API URL: %w", err)
	}

	var reader io.Reader
	if body != nil {
		payload, err := json.Marshal(body)
		if err != nil {
			return fmt.Errorf("encode request body: %w", err)
		}
		reader = bytes.NewReader(payload)
	}

	request, err := http.NewRequestWithContext(ctx, method, parsedURL.String(), reader)
	if err != nil {
		return fmt.Errorf("create GitHub API request: %w", err)
	}

	request.Header.Set("Accept", "application/vnd.github+json")
	request.Header.Set("Authorization", "Bearer "+c.Token)
	request.Header.Set("X-GitHub-Api-Version", apiVersion)
	request.Header.Set("User-Agent", c.UserAgent)
	if body != nil {
		request.Header.Set("Content-Type", "application/json")
	}

	response, err := c.HTTPClient.Do(request)
	if err != nil {
		return fmt.Errorf("GitHub API request failed: %w", err)
	}
	defer response.Body.Close()

	if response.StatusCode < 200 || response.StatusCode >= 300 {
		message := ""
		var apiBody struct {
			Message string `json:"message"`
		}
		if err := json.NewDecoder(io.LimitReader(response.Body, 1<<20)).Decode(&apiBody); err == nil {
			message = apiBody.Message
		}
		return &APIError{
			StatusCode: response.StatusCode,
			Method:     method,
			URL:        parsedURL.String(),
			Message:    message,
		}
	}

	if target == nil || response.StatusCode == http.StatusNoContent {
		_, _ = io.Copy(io.Discard, response.Body)
		return nil
	}

	if err := json.NewDecoder(response.Body).Decode(target); err != nil {
		return fmt.Errorf("decode GitHub API response: %w", err)
	}
	return nil
}

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
	"strconv"
	"strings"
	"sync"
	"time"
)

const apiVersion = "2022-11-28"

type RequestStats struct {
	Requests               int       `json:"requests"`
	Retries                int       `json:"retries"`
	RateLimitRemaining     int       `json:"rate_limit_remaining,omitempty"`
	RateLimitReset         time.Time `json:"rate_limit_reset,omitempty"`
	LastResponseStatusCode int       `json:"last_response_status_code,omitempty"`
}

type Client struct {
	BaseURL        string
	Host           string
	Token          string
	UserAgent      string
	HTTPClient     *http.Client
	RetryMax       int
	RetryBaseDelay time.Duration
	Sleep          func(context.Context, time.Duration) error
	Jitter         func(time.Duration) time.Duration
	Now            func() time.Time

	statsMu sync.Mutex
	stats   RequestStats
}

func NewFromEnvironment(ctx context.Context) (*Client, error) {
	host := strings.TrimSpace(os.Getenv("GH_HOST"))
	if host == "" {
		host = "github.com"
	}

	token := firstNonEmpty(os.Getenv("GH_TOKEN"), os.Getenv("GITHUB_TOKEN"), os.Getenv("GH_ENTERPRISE_TOKEN"))
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
		BaseURL:        baseURL,
		Host:           host,
		Token:          token,
		UserAgent:      "gh-tidy-branches",
		HTTPClient:     &http.Client{Timeout: 45 * time.Second},
		RetryMax:       3,
		RetryBaseDelay: 250 * time.Millisecond,
		Sleep:          sleepContext,
		Jitter:         defaultJitter,
		Now:            time.Now,
	}, nil
}

func sleepContext(ctx context.Context, delay time.Duration) error {
	timer := time.NewTimer(delay)
	defer timer.Stop()
	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-timer.C:
		return nil
	}
}

func defaultJitter(delay time.Duration) time.Duration {
	if delay <= 0 {
		return 0
	}
	spread := delay / 5
	if spread == 0 {
		return delay
	}
	offset := time.Now().UnixNano()%(int64(spread)*2+1) - int64(spread)
	return delay + time.Duration(offset)
}

func (c *Client) SnapshotStats() RequestStats {
	c.statsMu.Lock()
	defer c.statsMu.Unlock()
	return c.stats
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
		query := url.Values{"per_page": {"100"}, "page": {fmt.Sprintf("%d", pageNumber)}}
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
	return c.listPullRequests(ctx, fullName, url.Values{"state": {"open"}, "sort": {"updated"}, "direction": {"desc"}})
}

func (c *Client) ListClosedPullRequests(ctx context.Context, fullName, base string) ([]PullRequest, error) {
	return c.listPullRequests(ctx, fullName, url.Values{"state": {"closed"}, "base": {base}, "sort": {"updated"}, "direction": {"desc"}})
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
	return c.doJSON(ctx, http.MethodDelete, repoPath+"/git/refs/heads/"+url.PathEscape(branch), nil, nil)
}

func (c *Client) CreateBranch(ctx context.Context, fullName, branch, sha string) error {
	repoPath, err := repositoryPath(fullName)
	if err != nil {
		return err
	}
	body := struct {
		Ref string `json:"ref"`
		SHA string `json:"sha"`
	}{Ref: "refs/heads/" + branch, SHA: sha}
	return c.doJSON(ctx, http.MethodPost, repoPath+"/git/refs", body, nil)
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

	var payload []byte
	if body != nil {
		payload, err = json.Marshal(body)
		if err != nil {
			return fmt.Errorf("encode request body: %w", err)
		}
	}

	maxAttempts := 1
	if method == http.MethodGet {
		maxAttempts += c.retryMax()
	}
	for attempt := 0; attempt < maxAttempts; attempt++ {
		var reader io.Reader
		if payload != nil {
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
		if payload != nil {
			request.Header.Set("Content-Type", "application/json")
		}

		c.recordRequest()
		response, requestErr := c.httpClient().Do(request)
		if requestErr != nil {
			if method == http.MethodGet && attempt+1 < maxAttempts {
				c.recordRetry()
				if err := c.sleep(ctx, c.backoff(attempt, nil)); err != nil {
					return err
				}
				continue
			}
			return fmt.Errorf("GitHub API request failed: %w", requestErr)
		}

		responseBody, readErr := io.ReadAll(io.LimitReader(response.Body, 4<<20))
		closeErr := response.Body.Close()
		c.recordResponse(response)
		if readErr != nil {
			return fmt.Errorf("read GitHub API response: %w", readErr)
		}
		if closeErr != nil {
			return fmt.Errorf("close GitHub API response: %w", closeErr)
		}

		if response.StatusCode >= 200 && response.StatusCode < 300 {
			if target == nil || response.StatusCode == http.StatusNoContent || len(bytes.TrimSpace(responseBody)) == 0 {
				return nil
			}
			if err := json.Unmarshal(responseBody, target); err != nil {
				return fmt.Errorf("decode GitHub API response: %w", err)
			}
			return nil
		}

		message := ""
		var apiBody struct {
			Message string `json:"message"`
		}
		if json.Unmarshal(responseBody, &apiBody) == nil {
			message = apiBody.Message
		}
		apiErr := &APIError{StatusCode: response.StatusCode, Method: method, URL: parsedURL.String(), Message: message}
		if method == http.MethodGet && attempt+1 < maxAttempts && retryableResponse(response, message) {
			c.recordRetry()
			if err := c.sleep(ctx, c.backoff(attempt, response)); err != nil {
				return err
			}
			continue
		}
		return apiErr
	}
	return errors.New("GitHub API retry loop exhausted")
}

func (c *Client) retryMax() int {
	if c.RetryMax < 0 {
		return 0
	}
	if c.RetryMax == 0 {
		return 3
	}
	return c.RetryMax
}

func (c *Client) httpClient() *http.Client {
	if c.HTTPClient != nil {
		return c.HTTPClient
	}
	return http.DefaultClient
}

func (c *Client) sleep(ctx context.Context, delay time.Duration) error {
	if delay <= 0 {
		return nil
	}
	if c.Sleep != nil {
		return c.Sleep(ctx, delay)
	}
	return sleepContext(ctx, delay)
}

func (c *Client) backoff(attempt int, response *http.Response) time.Duration {
	now := time.Now
	if c.Now != nil {
		now = c.Now
	}
	if response != nil {
		if value := strings.TrimSpace(response.Header.Get("Retry-After")); value != "" {
			if seconds, err := strconv.Atoi(value); err == nil && seconds >= 0 {
				return time.Duration(seconds) * time.Second
			}
			if when, err := http.ParseTime(value); err == nil {
				if delay := when.Sub(now()); delay > 0 {
					return delay
				}
			}
		}
		if response.Header.Get("X-RateLimit-Remaining") == "0" {
			if resetUnix, err := strconv.ParseInt(response.Header.Get("X-RateLimit-Reset"), 10, 64); err == nil {
				if delay := time.Unix(resetUnix, 0).Sub(now()); delay > 0 {
					return delay
				}
			}
		}
	}
	base := c.RetryBaseDelay
	if base <= 0 {
		base = 250 * time.Millisecond
	}
	delay := base << attempt
	if delay > 8*time.Second {
		delay = 8 * time.Second
	}
	if c.Jitter != nil {
		return c.Jitter(delay)
	}
	return defaultJitter(delay)
}

func retryableResponse(response *http.Response, message string) bool {
	switch response.StatusCode {
	case http.StatusTooManyRequests, http.StatusInternalServerError, http.StatusBadGateway, http.StatusServiceUnavailable, http.StatusGatewayTimeout:
		return true
	case http.StatusForbidden:
		lower := strings.ToLower(message)
		return response.Header.Get("Retry-After") != "" || response.Header.Get("X-RateLimit-Remaining") == "0" || strings.Contains(lower, "secondary rate limit") || strings.Contains(lower, "abuse detection")
	default:
		return false
	}
}

func (c *Client) recordRequest() {
	c.statsMu.Lock()
	c.stats.Requests++
	c.statsMu.Unlock()
}

func (c *Client) recordRetry() {
	c.statsMu.Lock()
	c.stats.Retries++
	c.statsMu.Unlock()
}

func (c *Client) recordResponse(response *http.Response) {
	c.statsMu.Lock()
	defer c.statsMu.Unlock()
	c.stats.LastResponseStatusCode = response.StatusCode
	if value := response.Header.Get("X-RateLimit-Remaining"); value != "" {
		if remaining, err := strconv.Atoi(value); err == nil {
			c.stats.RateLimitRemaining = remaining
		}
	}
	if value := response.Header.Get("X-RateLimit-Reset"); value != "" {
		if reset, err := strconv.ParseInt(value, 10, 64); err == nil {
			c.stats.RateLimitReset = time.Unix(reset, 0)
		}
	}
}

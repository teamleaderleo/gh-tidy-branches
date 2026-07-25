package githubapi

import (
	"strconv"
	"time"
)

type Repository struct {
	FullName      string `json:"full_name"`
	DefaultBranch string `json:"default_branch"`
	Archived      bool   `json:"archived"`
}

type Branch struct {
	Name      string `json:"name"`
	Protected bool   `json:"protected"`
	Commit    struct {
		SHA string `json:"sha"`
	} `json:"commit"`
}

func (b Branch) SHA() string {
	return b.Commit.SHA
}

type PullRequest struct {
	Number   int        `json:"number"`
	MergedAt *time.Time `json:"merged_at"`
	Head     PullRef    `json:"head"`
	Base     PullRef    `json:"base"`
}

type PullRef struct {
	Ref  string `json:"ref"`
	SHA  string `json:"sha"`
	Repo *struct {
		FullName string `json:"full_name"`
	} `json:"repo"`
}

type APIError struct {
	StatusCode int
	Method     string
	URL        string
	Message    string
}

func (e *APIError) Error() string {
	if e.Message == "" {
		return e.Method + " " + e.URL + " returned HTTP " + statusText(e.StatusCode)
	}
	return e.Method + " " + e.URL + " returned HTTP " + statusText(e.StatusCode) + ": " + e.Message
}

func statusText(code int) string {
	switch code {
	case 401:
		return "401 Unauthorized"
	case 403:
		return "403 Forbidden"
	case 404:
		return "404 Not Found"
	case 409:
		return "409 Conflict"
	case 422:
		return "422 Unprocessable Entity"
	default:
		return strconv.Itoa(code)
	}
}

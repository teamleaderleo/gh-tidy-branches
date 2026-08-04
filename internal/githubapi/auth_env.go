package githubapi

import (
	"os"
	"strings"
)

const (
	defaultHost   = "github.com"
	localhostHost = "github.localhost"
	tenancyHost   = "ghe.com"
)

func tokenFromEnvironment(host string) string {
	normalizedHost := normalizeHost(host)
	if normalizedHost == defaultHost || normalizedHost == localhostHost || isTenancyHost(normalizedHost) {
		return firstNonEmpty(os.Getenv("GH_TOKEN"), os.Getenv("GITHUB_TOKEN"))
	}
	return firstNonEmpty(os.Getenv("GH_ENTERPRISE_TOKEN"), os.Getenv("GITHUB_ENTERPRISE_TOKEN"))
}

func apiBaseURL(host string) string {
	normalizedHost := normalizeHost(host)
	switch {
	case normalizedHost == defaultHost:
		return "https://api.github.com"
	case normalizedHost == localhostHost:
		return "http://api.github.localhost"
	case isTenancyHost(normalizedHost):
		return "https://api." + normalizedHost
	default:
		return "https://" + normalizedHost + "/api/v3"
	}
}

func normalizeHost(host string) string {
	host = strings.ToLower(strings.TrimSpace(host))
	if host == defaultHost || strings.HasSuffix(host, "."+defaultHost) {
		return defaultHost
	}
	if host == localhostHost || strings.HasSuffix(host, "."+localhostHost) {
		return localhostHost
	}
	if before, found := strings.CutSuffix(host, "."+tenancyHost); found {
		if separator := strings.LastIndex(before, "."); separator >= 0 {
			before = before[separator+1:]
		}
		return before + "." + tenancyHost
	}
	return host
}

func isTenancyHost(host string) bool {
	return strings.HasSuffix(host, "."+tenancyHost)
}

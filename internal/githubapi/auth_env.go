package githubapi

import (
	"os"
	"strings"
)

func tokenFromEnvironment(host string) string {
	normalizedHost := normalizeHost(host)
	if normalizedHost == "github.com" || isTenancyHost(normalizedHost) {
		return firstNonEmpty(os.Getenv("GH_TOKEN"), os.Getenv("GITHUB_TOKEN"))
	}
	return firstNonEmpty(os.Getenv("GH_ENTERPRISE_TOKEN"), os.Getenv("GITHUB_ENTERPRISE_TOKEN"))
}

func apiBaseURL(host string) string {
	normalizedHost := normalizeHost(host)
	if normalizedHost == "github.com" {
		return "https://api.github.com"
	}
	if isTenancyHost(normalizedHost) {
		return "https://api." + normalizedHost
	}
	return "https://" + normalizedHost + "/api/v3"
}

func normalizeHost(host string) string {
	return strings.ToLower(strings.TrimSpace(host))
}

func isTenancyHost(host string) bool {
	return strings.HasSuffix(host, ".ghe.com")
}

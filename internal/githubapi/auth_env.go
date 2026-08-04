package githubapi

import (
	"os"
	"strings"
)

func tokenFromEnvironment(host string) string {
	normalizedHost := strings.ToLower(strings.TrimSpace(host))
	if normalizedHost == "github.com" || normalizedHost == "ghe.com" || strings.HasSuffix(normalizedHost, ".ghe.com") {
		return firstNonEmpty(os.Getenv("GH_TOKEN"), os.Getenv("GITHUB_TOKEN"))
	}
	return firstNonEmpty(os.Getenv("GH_ENTERPRISE_TOKEN"), os.Getenv("GITHUB_ENTERPRISE_TOKEN"))
}

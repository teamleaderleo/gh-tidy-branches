package githubapi

import "testing"

func TestTokenFromEnvironmentUsesHostScopedFamilies(t *testing.T) {
	tests := []struct {
		name                  string
		host                  string
		ghToken               string
		githubToken           string
		ghEnterpriseToken     string
		githubEnterpriseToken string
		want                  string
	}{
		{
			name:                  "github dot com prefers GH token",
			host:                  "github.com",
			ghToken:               "gh-token",
			githubToken:           "github-token",
			ghEnterpriseToken:     "enterprise-token",
			githubEnterpriseToken: "github-enterprise-token",
			want:                  "gh-token",
		},
		{
			name:        "github dot com falls back to GITHUB token",
			host:        "github.com",
			githubToken: "github-token",
			want:        "github-token",
		},
		{
			name:              "ghe dot com subdomain uses public token family",
			host:              "tenant.ghe.com",
			ghToken:           "gh-token",
			ghEnterpriseToken: "enterprise-token",
			want:              "gh-token",
		},
		{
			name:              "enterprise server prefers GH enterprise token",
			host:              "github.example.com",
			ghToken:           "public-token",
			ghEnterpriseToken: "enterprise-token",
			want:              "enterprise-token",
		},
		{
			name:                  "enterprise server falls back to GITHUB enterprise token",
			host:                  "github.example.com",
			githubEnterpriseToken: "github-enterprise-token",
			want:                  "github-enterprise-token",
		},
		{
			name:        "enterprise server ignores public token family",
			host:        "github.example.com",
			ghToken:     "public-token",
			githubToken: "github-token",
			want:        "",
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Setenv("GH_TOKEN", test.ghToken)
			t.Setenv("GITHUB_TOKEN", test.githubToken)
			t.Setenv("GH_ENTERPRISE_TOKEN", test.ghEnterpriseToken)
			t.Setenv("GITHUB_ENTERPRISE_TOKEN", test.githubEnterpriseToken)

			if got := tokenFromEnvironment(test.host); got != test.want {
				t.Fatalf("tokenFromEnvironment(%q) = %q, want %q", test.host, got, test.want)
			}
		})
	}
}

package connectors

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"
)

// GitHubPAT is GitHub authed with a personal access token instead of a GitHub App
// installation — for repos a user can read but can't install the App on (the App
// needs org/repo admin). It reuses the App connector's issue/Project sync wholesale
// (embedded *GitHub); only auth and repo discovery differ. The stored PAT is used
// directly as a bearer token by the existing token path (no installation, no
// refresh), so the token manager needs no special case.
//
// This is a fine-grained, read-only token by recommendation; gated behind the
// `github_pat` feature flag (the App stays the default, more secure path).
type GitHubPAT struct {
	*GitHub
}

func NewGitHubPAT() *GitHubPAT { return &GitHubPAT{GitHub: NewGitHub()} }

func (g *GitHubPAT) ID() string   { return "github_pat" }
func (g *GitHubPAT) Name() string { return "GitHub (token)" }

// DiscoverUnits lists the repos the token can reach via /user/repos (owned,
// collaborator, or org-member — which covers a repo you have read access to but
// don't own), plus each owning account's Projects v2 boards (best-effort). The App
// connector instead uses /installation/repositories. SyncUnit and RateBudget are
// inherited from the embedded *GitHub.
func (g *GitHubPAT) DiscoverUnits(ctx context.Context, hc *http.Client) ([]Unit, error) {
	var repos []string
	owners := map[string]string{} // login -> type, for Projects discovery
	for page := 1; len(repos) < ghMaxRepos; page++ {
		url := g.apiBaseURL + "/user/repos?affiliation=owner,collaborator,organization_member&" + pageParams(page)
		body, err := httpGet(ctx, hc, url)
		if err != nil {
			return nil, err
		}
		var rs []ghRepo
		if err := json.Unmarshal(body, &rs); err != nil {
			return nil, fmt.Errorf("decode repos: %w", err)
		}
		for _, r := range rs {
			repos = append(repos, r.FullName)
			if r.Owner.Login != "" {
				owners[r.Owner.Login] = r.Owner.Type
			}
		}
		if len(rs) < ghPerPage {
			break
		}
	}
	out := g.repoUnits(repos)

	seen := map[string]bool{}
	for login, typ := range owners {
		projects, err := g.listProjects(ctx, hc, login, typ)
		if err != nil {
			slog.Warn("github_projects_discover_failed", "account", login, "error", err.Error())
			continue
		}
		for _, p := range projects {
			if seen[p.ID] {
				continue
			}
			seen[p.ID] = true
			out = append(out, p)
		}
	}
	return out, nil
}

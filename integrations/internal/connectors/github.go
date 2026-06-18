package connectors

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"net/url"
	"strconv"
	"strings"

	"golang.org/x/oauth2"
	"golang.org/x/oauth2/github"
	"golang.org/x/sync/errgroup"

	"integrations/internal/metrics"
)

// v0.1 sync caps — discover-and-sync-all-accessible is bounded so a large account
// can't run unbounded (design-doc 0009/0010). Hitting a cap is logged + metered,
// never silent (review #17). Selection UI is a later refinement.
const (
	ghMaxRepos        = 100
	ghPerPage         = 100
	ghMaxPagesPerRepo = 10
	// Hard ceiling on pages in a single issue-sync run. Only reached when an
	// updatedAt plateau (>per-run-budget issues sharing one timestamp) must be
	// drained so the cursor can advance past it (design-doc 0011 P1).
	ghMaxPagesHardCap = 200
	// Pages of comments to pull per issue before stopping (a guard against a
	// pathological mega-thread); a cap hit is logged + metered.
	ghMaxCommentPages = 10
	// Bounded in-activity fan-out for comment fetches on ONE page of issues. The
	// comment N+1 was the throughput bug (review L3): ~1000 serialized round-trips
	// for a comment-heavy repo (~4.5 artifacts/sec). We fetch a page's comments
	// concurrently and emit in order; the activity-level admission limiter already
	// gates per-connection concurrency, so this stays modest to not blow the rate
	// budget. (A full GraphQL issues+comments query would cut calls further — a
	// larger change left as a future option.)
	ghCommentConcurrency = 8
	// Per-batch repo/project count for EstimateTotals GraphQL alias batching — kept
	// well under GraphQL node/complexity limits.
	ghEstimateBatchSize = 25
)

// GitHub is the GitHub issues connector (record archetype). Each repo is its own
// sync container with its own cursor, so a deleted/huge/quiet repo syncs and
// retries in isolation (ADR-0029). apiBaseURL is a field so tests can point it at
// an httptest server; perPage/maxPages are fields so tests can exercise paging
// (they default to the constants in NewGitHub).
type GitHub struct {
	apiBaseURL string
	perPage    int
	maxPages   int
}

func NewGitHub() *GitHub {
	return &GitHub{apiBaseURL: GitHubAPIBase, perPage: ghPerPage, maxPages: ghMaxPagesPerRepo}
}

// NewGitHubWithBaseURL builds a GitHub connector pointed at a custom API base —
// used by tests in other packages (the activity layer) to aim it at an httptest
// source. Production code uses NewGitHub.
func NewGitHubWithBaseURL(base string) *GitHub {
	g := NewGitHub()
	g.apiBaseURL = base
	return g
}

func (g *GitHub) ID() string        { return "github" }
func (g *GitHub) Name() string      { return "GitHub (App)" }
func (g *GitHub) Archetype() string { return "record" }

func (g *GitHub) Scopes() []string { return []string{"repo", "read:user"} }

// RateBudget seeds the adaptive limiter: GitHub's REST primary limit is 5000
// req/hour per token, so start conservative and let AIMD discover the secondary
// ceiling (ADR-0036).
func (g *GitHub) RateBudget() RateBudget {
	return RateBudget{StartConcurrency: 4, MaxConcurrency: 8, KnownReqPerHour: 5000}
}

// AuthorizeURL routes to the GitHub App install page when an app slug is set —
// that's where the user selects which repos to grant, and (with "Request user
// authorization during installation") it also issues the user token, redirecting
// to our callback with code+state. Falls back to plain OAuth for a classic app.
func (g *GitHub) AuthorizeURL(creds Creds, state, redirectURL string) string {
	if creds.AppSlug != "" {
		return fmt.Sprintf("https://github.com/apps/%s/installations/new?state=%s",
			creds.AppSlug, url.QueryEscape(state))
	}
	return g.OAuthConfig(creds.ClientID, creds.ClientSecret, redirectURL).AuthCodeURL(state)
}

func (g *GitHub) OAuthConfig(clientID, clientSecret, redirectURL string) *oauth2.Config {
	return &oauth2.Config{
		ClientID:     clientID,
		ClientSecret: clientSecret,
		RedirectURL:  redirectURL,
		Scopes:       g.Scopes(),
		Endpoint:     github.Endpoint,
	}
}

func (g *GitHub) Account(ctx context.Context, hc *http.Client) (string, error) {
	body, err := httpGet(ctx, hc, g.apiBaseURL+"/user")
	if err != nil {
		return "", err
	}
	var u struct {
		Login string `json:"login"`
	}
	if err := json.Unmarshal(body, &u); err != nil {
		return "", err
	}
	return u.Login, nil
}

type ghRepo struct {
	FullName string `json:"full_name"`
	Owner    struct {
		Login string `json:"login"`
		Type  string `json:"type"` // "Organization" | "User"
	} `json:"owner"`
}

type ghInstallationRepos struct {
	Repositories []ghRepo `json:"repositories"`
}

type ghIssue struct {
	Number      int              `json:"number"`
	HTMLURL     string           `json:"html_url"`
	CreatedAt   string           `json:"created_at"`
	UpdatedAt   string           `json:"updated_at"`
	Comments    int              `json:"comments"` // comment count — skip the fetch when 0
	PullRequest *json.RawMessage `json:"pull_request"`
}

// DiscoverUnits lists the sync units via the App installation token: ALL repos the
// installation is granted (/installation/repositories — not filtered to the
// connecting user, so the whole org's granted repos appear) plus each owning
// account's GitHub Projects v2 boards. The http client carries an installation
// token (TokenManager); Projects are best-effort (need Organization Projects:Read).
func (g *GitHub) DiscoverUnits(ctx context.Context, hc *http.Client) ([]Unit, error) {
	var repos []string
	owners := map[string]string{} // account login -> type, for Projects discovery
	for page := 1; len(repos) < ghMaxRepos; page++ {
		body, err := httpGet(ctx, hc, g.apiBaseURL+"/installation/repositories?"+pageParams(page))
		if err != nil {
			return nil, err
		}
		var resp ghInstallationRepos
		if err := json.Unmarshal(body, &resp); err != nil {
			return nil, fmt.Errorf("decode installation repos: %w", err)
		}
		for _, r := range resp.Repositories {
			repos = append(repos, r.FullName)
			if r.Owner.Login != "" {
				owners[r.Owner.Login] = r.Owner.Type
			}
		}
		if len(resp.Repositories) < ghPerPage {
			break
		}
	}
	out := g.repoUnits(repos)

	// Projects v2 (best-effort) — one unit per board, per owning account.
	seen := map[string]bool{}
	for login, typ := range owners {
		projects, err := g.listProjects(ctx, hc, login, typ)
		if err != nil {
			slog.Warn("github_projects_discover_failed", "account", login, "error", err.Error())
			continue // likely missing Projects:Read — don't fail repo discovery
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

// repoUnits dedups repo full-names and builds units, capped.
func (g *GitHub) repoUnits(repos []string) []Unit {
	seen := make(map[string]bool, len(repos))
	out := make([]Unit, 0, len(repos))
	for _, full := range repos {
		if seen[full] {
			continue // a repo can appear under more than one installation
		}
		seen[full] = true
		out = append(out, Unit{ID: full, Kind: "github_repo", Name: full})
	}
	if len(out) > ghMaxRepos {
		slog.Warn("github_repo_cap_hit", "cap", ghMaxRepos)
		metrics.ConnectorCapHits.WithLabelValues("github", "repos").Inc()
		out = out[:ghMaxRepos]
	}
	return out
}

// listProjects lists an account's GitHub Projects v2 boards via GraphQL. Needs the
// App's Organization/User → Projects: Read permission; without it GraphQL errors
// and the caller skips this account.
func (g *GitHub) listProjects(ctx context.Context, hc *http.Client, login, accountType string) ([]Unit, error) {
	owner := "user"
	if accountType == "Organization" {
		owner = "organization"
	}
	query := fmt.Sprintf(`query($login:String!,$cursor:String){%s(login:$login){projectsV2(first:100,after:$cursor){nodes{id title number} pageInfo{hasNextPage endCursor}}}}`, owner)

	var out []Unit
	cursor := ""
	for {
		vars := map[string]any{"login": login}
		if cursor != "" {
			vars["cursor"] = cursor
		}
		var resp struct {
			Organization *ghProjectsConn `json:"organization"`
			User         *ghProjectsConn `json:"user"`
		}
		if err := g.graphqlPost(ctx, hc, query, vars, &resp); err != nil {
			return nil, err
		}
		conn := resp.Organization
		if conn == nil {
			conn = resp.User
		}
		if conn == nil {
			break
		}
		for _, n := range conn.ProjectsV2.Nodes {
			out = append(out, Unit{ID: n.ID, Kind: "github_project", Name: n.Title})
		}
		if !conn.ProjectsV2.PageInfo.HasNextPage {
			break
		}
		cursor = conn.ProjectsV2.PageInfo.EndCursor
	}
	return out, nil
}

type ghProjectsConn struct {
	ProjectsV2 struct {
		Nodes []struct {
			ID     string `json:"id"`
			Title  string `json:"title"`
			Number int    `json:"number"`
		} `json:"nodes"`
		PageInfo struct {
			HasNextPage bool   `json:"hasNextPage"`
			EndCursor   string `json:"endCursor"`
		} `json:"pageInfo"`
	} `json:"projectsV2"`
}

// EstimateTotals estimates each unit's item count via GraphQL totalCount, batched
// with aliases to minimize calls. Repo units count their issues (issues.totalCount
// excludes PRs — which the connector skips anyway); project units count their board
// items. Best-effort: a GraphQL error on a sub-batch logs and skips those units
// (they stay "unknown"), never failing discovery. GitHubPAT inherits this via the
// embedded *GitHub (same GraphQL surface).
func (g *GitHub) EstimateTotals(ctx context.Context, hc *http.Client, units []Unit) (map[string]int, error) {
	out := make(map[string]int)
	var repos, projects []Unit
	for _, u := range units {
		switch u.Kind {
		case "github_repo":
			repos = append(repos, u)
		case "github_project":
			projects = append(projects, u)
		}
	}
	g.estimateRepoTotals(ctx, hc, repos, out)
	g.estimateProjectTotals(ctx, hc, projects, out)
	return out, nil
}

// estimateRepoTotals batches repo issue counts: r0: repository(owner:..,name:..){
// issues(states:[OPEN,CLOSED]){ totalCount } } r1: ... — ~ghEstimateBatchSize repos
// per query. A sub-batch GraphQL error is logged and skipped (those repos stay
// unknown), so a single bad repo or a permission gap can't fail discovery.
func (g *GitHub) estimateRepoTotals(ctx context.Context, hc *http.Client, repos []Unit, out map[string]int) {
	for start := 0; start < len(repos); start += ghEstimateBatchSize {
		end := start + ghEstimateBatchSize
		if end > len(repos) {
			end = len(repos)
		}
		batch := repos[start:end]
		var b strings.Builder
		b.WriteString("query{")
		alias := make([]string, len(batch))
		valid := 0
		for i, u := range batch {
			owner, name, ok := strings.Cut(u.ID, "/")
			if !ok {
				continue // not "owner/name" — skip (stays unknown)
			}
			a := fmt.Sprintf("r%d", i)
			alias[i] = a
			fmt.Fprintf(&b, "%s: repository(owner:%s,name:%s){issues(states:[OPEN,CLOSED]){totalCount}} ",
				a, strconv.Quote(owner), strconv.Quote(name))
			valid++
		}
		b.WriteString("}")
		if valid == 0 {
			continue
		}
		var resp map[string]*struct {
			Issues struct {
				TotalCount int `json:"totalCount"`
			} `json:"issues"`
		}
		if err := g.graphqlPost(ctx, hc, b.String(), nil, &resp); err != nil {
			slog.Warn("github_estimate_repos_failed", "count", valid, "error", err.Error())
			continue // best-effort — these repos stay unknown
		}
		for i, u := range batch {
			if alias[i] == "" {
				continue
			}
			if r := resp[alias[i]]; r != nil {
				out[u.ID] = r.Issues.TotalCount
			}
		}
	}
}

// estimateProjectTotals batches Projects v2 item counts: p0: node(id:..){ ... on
// ProjectV2 { items { totalCount } } } p1: ... — same best-effort batching.
func (g *GitHub) estimateProjectTotals(ctx context.Context, hc *http.Client, projects []Unit, out map[string]int) {
	for start := 0; start < len(projects); start += ghEstimateBatchSize {
		end := start + ghEstimateBatchSize
		if end > len(projects) {
			end = len(projects)
		}
		batch := projects[start:end]
		var b strings.Builder
		b.WriteString("query{")
		for i, u := range batch {
			fmt.Fprintf(&b, "p%d: node(id:%s){... on ProjectV2{items{totalCount}}} ", i, strconv.Quote(u.ID))
		}
		b.WriteString("}")
		var resp map[string]*struct {
			Items struct {
				TotalCount int `json:"totalCount"`
			} `json:"items"`
		}
		if err := g.graphqlPost(ctx, hc, b.String(), nil, &resp); err != nil {
			slog.Warn("github_estimate_projects_failed", "count", len(batch), "error", err.Error())
			continue // best-effort — these projects stay unknown
		}
		for i, u := range batch {
			if n := resp[fmt.Sprintf("p%d", i)]; n != nil {
				out[u.ID] = n.Items.TotalCount
			}
		}
	}
}

func pageParams(page int) string {
	p := url.Values{}
	p.Set("per_page", strconv.Itoa(ghPerPage))
	p.Set("page", strconv.Itoa(page))
	return p.Encode()
}

// SyncUnit dispatches by unit kind: a repo syncs its issues; a Project v2 board
// syncs its items.
func (g *GitHub) SyncUnit(ctx context.Context, hc *http.Client, unit Unit, cursor string, emit EmitFunc) (bool, error) {
	if unit.Kind == "github_project" {
		return g.syncProjectItems(ctx, hc, unit, cursor, emit)
	}
	return g.syncRepoIssues(ctx, hc, unit, cursor, emit)
}

// syncRepoIssues syncs one repo's issues updated since the position (an RFC3339
// cursor; GitHub's `since` is inclusive). Pull requests are skipped. A 404 (e.g. a
// deleted repo) surfaces as an error so this job alone retries/DLQs.
//
// Issues are fetched updated-ascending so the cursor is a true high-water mark: it
// advances to an item's updatedAt only after that item is committed (the emit layer
// writes art.Cursor in the same transaction as the manifest — ADR-0038), so an
// interruption (Stop, crash, DLQ) leaves a gapless resume point (design-doc 0011).
// A cancelled context returns done=false — the committed work is kept and the rest
// resumes from the persisted cursor. The local newPos here is only the plateau
// detector (has the high-water cleared this run's starting position?).
func (g *GitHub) syncRepoIssues(ctx context.Context, hc *http.Client, unit Unit, cursor string, emit EmitFunc) (bool, error) {
	perPage := g.perPage
	if perPage <= 0 {
		perPage = ghPerPage
	}
	maxPages := g.maxPages
	if maxPages <= 0 {
		maxPages = ghMaxPagesPerRepo
	}
	hardCap := ghMaxPagesHardCap
	if maxPages > hardCap {
		hardCap = maxPages
	}

	// For GitHub the per-artifact manifest container equals the sync unit (the repo).
	container := Container{ID: unit.ID, Kind: unit.Kind, Name: unit.Name}

	newPos := cursor
	for page := 1; page <= hardCap; page++ {
		if ctx.Err() != nil {
			return false, nil // cooperative stop between pages — more to fetch
		}
		params := url.Values{}
		params.Set("state", "all")
		params.Set("sort", "updated")
		params.Set("direction", "asc") // ascending → cursor is a real high-water mark
		params.Set("per_page", strconv.Itoa(perPage))
		params.Set("page", strconv.Itoa(page))
		if cursor != "" {
			params.Set("since", cursor)
		}
		body, err := httpGet(ctx, hc, g.apiBaseURL+"/repos/"+unit.ID+"/issues?"+params.Encode())
		if err != nil {
			return false, err
		}
		var raws []json.RawMessage
		if err := json.Unmarshal(body, &raws); err != nil {
			return false, fmt.Errorf("decode issues: %w", err)
		}
		// Fold this page's issues+comments concurrently (bounded), then emit in the
		// page's existing ascending-updatedAt order. The concurrency is only the
		// FETCH — emit order and per-item Cursor stay strictly ascending so the
		// high-water-mark contract holds (ADR-0031/0038): a fan-out that emitted out
		// of order could advance the cursor past an un-emitted earlier item.
		arts, err := g.foldIssuePage(ctx, hc, unit.ID, container, raws)
		if err != nil {
			if ctx.Err() != nil {
				return false, nil // cooperative stop mid-page — committed tail resumes
			}
			return false, err
		}
		for _, art := range arts {
			if err := emit(ctx, art); err != nil {
				return false, err
			}
			if art.Cursor > newPos {
				newPos = art.Cursor
			}
		}
		if len(raws) < perPage {
			return true, nil // natural end of the stream — caught up
		}
		// Per-run page budget reached. Normally stop and resume from the watermark
		// next run. But if the watermark hasn't advanced past this run's starting
		// position, the whole budget was a single updatedAt plateau (>budget issues
		// share one timestamp); GitHub's `since` is inclusive and can't sub-filter a
		// timestamp, so stopping now would re-fetch the same head forever. Keep
		// paging (up to the hard ceiling) until the cursor clears the plateau
		// (design-doc 0011 P1).
		if page >= maxPages {
			if newPos != cursor {
				return false, nil // budget hit, watermark advanced — more next run
			}
			if page == maxPages {
				slog.Warn("github_issue_plateau_draining", "repo", unit.ID, "position", cursor)
				metrics.ConnectorCapHits.WithLabelValues("github", "issues").Inc()
			}
		}
	}
	// Hit the hard ceiling — a same-timestamp plateau larger than the ceiling, which
	// is practically impossible. The cursor advanced as far as it could; the next
	// run continues from here.
	slog.Warn("github_issue_plateau_hard_cap", "repo", unit.ID, "cap", hardCap, "position", newPos)
	return false, nil
}

// foldIssuePage builds the artifacts for one page of issues, fetching each issue's
// comments concurrently (bounded by ghCommentConcurrency) but returning them in the
// page's original order — which is ascending-updatedAt, so emitting in this order
// keeps the cursor a true high-water mark (ADR-0031/0038). PRs are skipped (the
// issues endpoint returns them too). Per-issue comment errors are degraded or
// propagated by issueArtifact exactly as in the sequential path: a terminal
// per-issue error yields an issue-without-comments artifact; an auth/rate-limit/
// parent-ctx/transient error fails the whole page (and thus the repo, for retry).
// On cancellation we stop launching new fetches.
func (g *GitHub) foldIssuePage(ctx context.Context, hc *http.Client, repo string, container Container, raws []json.RawMessage) ([]Artifact, error) {
	// Decode and filter to real issues first, preserving order.
	type pageIssue struct {
		raw json.RawMessage
		iss ghIssue
	}
	items := make([]pageIssue, 0, len(raws))
	for _, raw := range raws {
		var iss ghIssue
		if err := json.Unmarshal(raw, &iss); err != nil {
			return nil, fmt.Errorf("decode issue: %w", err)
		}
		if iss.PullRequest != nil {
			continue // the issues endpoint also returns PRs; skip them
		}
		items = append(items, pageIssue{raw: raw, iss: iss})
	}

	// Fan out the comment fold over a bounded worker pool; results land in a
	// per-index slot so the output order matches the input (ascending updatedAt).
	docs := make([][]byte, len(items))
	g0, gctx := errgroup.WithContext(ctx)
	g0.SetLimit(ghCommentConcurrency)
	for i := range items {
		if gctx.Err() != nil {
			break // cancelled (parent ctx or a sibling failed) — don't start more
		}
		i, it := i, items[i]
		g0.Go(func() error {
			doc, err := g.issueArtifact(gctx, hc, repo, it.raw, it.iss)
			if err != nil {
				return err // issueArtifact already classified degrade-vs-propagate
			}
			docs[i] = doc
			return nil
		})
	}
	if err := g0.Wait(); err != nil {
		return nil, err
	}
	// A parent cancellation surfaces as a nil error from issueArtifact callers only
	// when ctx wasn't yet observed; guard so a cancelled fold doesn't emit partial
	// artifacts as if complete.
	if ctx.Err() != nil {
		return nil, ctx.Err()
	}

	arts := make([]Artifact, 0, len(items))
	for i, it := range items {
		arts = append(arts, Artifact{
			SourceNativeKind: "github_issue",
			Container:        container,
			ExternalID:       fmt.Sprintf("%s#%d", repo, it.iss.Number),
			SourceURL:        it.iss.HTMLURL,
			SourceCreatedAt:  parseRFC3339(it.iss.CreatedAt),
			SourceUpdatedAt:  parseRFC3339(it.iss.UpdatedAt),
			ContentType:      "application/json",
			Raw:              docs[i],
			// Ascending order makes updatedAt monotonic, so advancing the cursor to
			// the item just committed never skips an earlier one (the high-water-mark
			// contract); the emit layer persists this in the manifest's transaction.
			Cursor: it.iss.UpdatedAt,
		})
	}
	return arts, nil
}

// issueArtifact folds an issue and its comments into one raw document, so the
// stored artifact carries the whole discussion — for a company brain the comment
// thread is the high-value content, and the bare list-endpoint issue object has
// none. An issue's updated_at bumps when a comment is added, so the cursor
// re-fetches the issue and we re-pull comments; content-hash dedup absorbs an
// unchanged thread. Comments are fetched only when the issue has any, to avoid an
// extra request per comment-less issue (design-doc 0011 P2).
func (g *GitHub) issueArtifact(ctx context.Context, hc *http.Client, repo string, issue json.RawMessage, iss ghIssue) ([]byte, error) {
	comments := []json.RawMessage{}
	if iss.Comments > 0 {
		c, err := g.fetchIssueComments(ctx, hc, repo, iss.Number)
		if err != nil {
			// Classify the failure (design-doc 0011 P3-5):
			//
			//  - Parent context cancelled = a Stop/shutdown → propagate. A Stop is not
			//    a comment failure; degrading here would tick the drop metric on every
			//    Stop and rely on a downstream re-check to actually halt.
			//  - Connection-wide (credentials revoked, rate limited) → propagate. It
			//    affects every container, so the run must stop / go needs_reauth.
			//  - Terminal per-issue (404/410 — deleted issue race; size cap; or a
			//    per-request timeout on one slow thread) → degrade to the issue
			//    without comments and continue, so one bad issue can't DLQ the whole
			//    repo (ADR-0029 isolation). Retrying these wouldn't help.
			//  - Anything else is transient (a 5xx, a network blip) → propagate, so
			//    River retries the repo and the (high-value) comment thread is
			//    preserved rather than stickily dropped.
			if ctx.Err() != nil {
				return nil, err
			}
			if _, ok := AsAuth(err); ok {
				return nil, err
			}
			if _, ok := AsRateLimit(err); ok {
				return nil, err
			}
			if !isTerminalCommentError(err) {
				return nil, err // transient → let the repo retry, keep the thread
			}
			slog.Warn("github_issue_comments_degraded", "repo", repo, "issue", iss.Number, "error", err.Error())
			metrics.ConnectorCapHits.WithLabelValues("github", "issue_comments_dropped").Inc()
			comments = c // keep whatever pages were fetched before the failure
			if comments == nil {
				comments = []json.RawMessage{}
			}
		} else {
			comments = c
		}
	}
	return json.Marshal(struct {
		Issue    json.RawMessage   `json:"issue"`
		Comments []json.RawMessage `json:"comments"`
	}{Issue: issue, Comments: comments})
}

// fetchIssueComments pulls all comments for one issue, paginated and capped.
func (g *GitHub) fetchIssueComments(ctx context.Context, hc *http.Client, repo string, number int) ([]json.RawMessage, error) {
	perPage := g.perPage
	if perPage <= 0 {
		perPage = ghPerPage
	}
	var out []json.RawMessage
	for page := 1; page <= ghMaxCommentPages; page++ {
		if ctx.Err() != nil {
			return out, ctx.Err()
		}
		params := url.Values{}
		params.Set("per_page", strconv.Itoa(perPage))
		params.Set("page", strconv.Itoa(page))
		// Pin the order so the folded thread (and thus the content hash) is stable
		// regardless of GitHub's default, avoiding spurious re-write churn (L2).
		params.Set("sort", "created")
		params.Set("direction", "asc")
		body, err := httpGet(ctx, hc, fmt.Sprintf("%s/repos/%s/issues/%d/comments?%s", g.apiBaseURL, repo, number, params.Encode()))
		if err != nil {
			return out, err
		}
		var pageItems []json.RawMessage
		if err := json.Unmarshal(body, &pageItems); err != nil {
			return out, fmt.Errorf("decode issue comments: %w", err)
		}
		out = append(out, pageItems...)
		if len(pageItems) < perPage {
			return out, nil
		}
		if page == ghMaxCommentPages {
			slog.Warn("github_issue_comments_cap_hit", "repo", repo, "issue", number, "cap", ghMaxCommentPages)
			metrics.ConnectorCapHits.WithLabelValues("github", "issue_comments").Inc()
		}
	}
	return out, nil
}

// isTerminalCommentError reports whether a comment-fetch error is permanent for
// this one issue — so degrading to issue-only is correct — versus transient, where
// retrying the repo would preserve the thread. The caller has already handled the
// parent-ctx (Stop) and connection-wide (auth / rate-limit) cases. Terminal: a
// 404/410 (the issue was deleted between the list and the fetch), an oversized
// thread (size cap), or a per-request timeout (one slow thread mustn't block or
// endlessly re-fetch the whole repo). Everything else (5xx, transport) is transient.
func isTerminalCommentError(err error) bool {
	if se, ok := AsStatus(err); ok {
		return se.Status == http.StatusNotFound || se.Status == http.StatusGone
	}
	var te *TooLargeError
	if errors.As(err, &te) {
		return true
	}
	return errors.Is(err, context.DeadlineExceeded)
}

// syncProjectItems syncs one Project v2 board's items via GraphQL. Projects v2 has
// no server-side updatedAt filter, so it fetches all items (capped) each run and
// relies on the emit layer's content-hash dedup to skip unchanged ones — the unit
// is cursorless, so every emit carries an empty Artifact.Cursor and the cursor is
// never advanced. Returns done=true when the board is fully fetched this call,
// false if it stopped at the page cap mid-board.
func (g *GitHub) syncProjectItems(ctx context.Context, hc *http.Client, unit Unit, _ string, emit EmitFunc) (bool, error) {
	const query = `query($id:ID!,$cursor:String){node(id:$id){... on ProjectV2{items(first:100,after:$cursor){nodes{id updatedAt content{__typename ... on Issue{url} ... on PullRequest{url} ... on DraftIssue{title}}} pageInfo{hasNextPage endCursor}}}}}`
	// For GitHub the per-artifact manifest container equals the sync unit (the board).
	container := Container{ID: unit.ID, Kind: unit.Kind, Name: unit.Name}
	gqlCursor := ""
	for page := 1; page <= ghMaxPagesPerRepo; page++ {
		if ctx.Err() != nil {
			return false, nil // cooperative stop; a board is re-fetched whole next run
		}
		vars := map[string]any{"id": unit.ID}
		if gqlCursor != "" {
			vars["cursor"] = gqlCursor
		}
		var resp struct {
			Node struct {
				Items struct {
					Nodes    []json.RawMessage `json:"nodes"`
					PageInfo struct {
						HasNextPage bool   `json:"hasNextPage"`
						EndCursor   string `json:"endCursor"`
					} `json:"pageInfo"`
				} `json:"items"`
			} `json:"node"`
		}
		if err := g.graphqlPost(ctx, hc, query, vars, &resp); err != nil {
			return false, err
		}
		for _, raw := range resp.Node.Items.Nodes {
			var item struct {
				ID        string `json:"id"`
				UpdatedAt string `json:"updatedAt"`
				Content   struct {
					URL string `json:"url"`
				} `json:"content"`
			}
			if err := json.Unmarshal(raw, &item); err != nil {
				return false, fmt.Errorf("decode project item: %w", err)
			}
			if err := emit(ctx, Artifact{
				SourceNativeKind: "github_project_item",
				Container:        container,
				ExternalID:       item.ID,
				SourceURL:        item.Content.URL,
				SourceUpdatedAt:  parseRFC3339(item.UpdatedAt),
				ContentType:      "application/json",
				Raw:              raw,
				// Cursorless: no high-water value; dedup absorbs unchanged items.
				Cursor: "",
			}); err != nil {
				return false, err
			}
		}
		if !resp.Node.Items.PageInfo.HasNextPage {
			return true, nil // board fully fetched this call
		}
		gqlCursor = resp.Node.Items.PageInfo.EndCursor
		if page == ghMaxPagesPerRepo {
			slog.Warn("github_project_page_cap_hit", "project", unit.ID, "cap", ghMaxPagesPerRepo)
			metrics.ConnectorCapHits.WithLabelValues("github", "project_items").Inc()
		}
	}
	return false, nil // hit the page cap mid-board — more next run
}

// graphqlPost runs a GitHub GraphQL query, classifying transport errors like
// httpGet (rate-limit → snooze, auth → needs_reauth) and surfacing GraphQL-level
// errors (e.g. a missing Projects:Read permission) as a plain error.
func (g *GitHub) graphqlPost(ctx context.Context, hc *http.Client, query string, vars map[string]any, out any) error {
	payload, err := json.Marshal(map[string]any{"query": query, "variables": vars})
	if err != nil {
		return err
	}
	rctx, cancel := context.WithTimeout(ctx, requestTimeout)
	defer cancel()
	req, err := http.NewRequestWithContext(rctx, http.MethodPost, g.apiBaseURL+"/graphql", bytes.NewReader(payload))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")
	resp, err := hc.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	body, err := io.ReadAll(io.LimitReader(resp.Body, maxArtifactBytes+1))
	if err != nil {
		return err
	}
	switch {
	case resp.StatusCode == http.StatusTooManyRequests || (resp.StatusCode == http.StatusForbidden && isRateLimited(resp, body)):
		return &RateLimitError{RetryAfter: retryAfter(resp)}
	// Only 401 is terminal-auth; a 403 without rate-limit signals is a per-resource
	// failure (restricted resource / SAML / secondary limit without headers), not a
	// connection-wide needs_reauth (H3 fix). It surfaces as a plain status error here.
	case resp.StatusCode == http.StatusUnauthorized:
		return &AuthError{Status: resp.StatusCode}
	case resp.StatusCode != http.StatusOK:
		return fmt.Errorf("graphql: status %d", resp.StatusCode)
	}
	var env struct {
		Data   json.RawMessage `json:"data"`
		Errors []struct {
			Message string `json:"message"`
		} `json:"errors"`
	}
	if err := json.Unmarshal(body, &env); err != nil {
		return fmt.Errorf("graphql decode: %w", err)
	}
	if len(env.Errors) > 0 {
		return fmt.Errorf("graphql: %s", env.Errors[0].Message)
	}
	if out != nil && len(env.Data) > 0 {
		return json.Unmarshal(env.Data, out)
	}
	return nil
}

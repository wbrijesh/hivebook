package connectors

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"

	"golang.org/x/oauth2"
	"golang.org/x/oauth2/google"

	"integrations/internal/metrics"
)

// Per-run pagination bounds for the Drive listing (design-doc 0011 P2). pageSize
// and maxPages are fields on GoogleDocs (defaulted here) so tests can exercise the
// cap; the hard ceiling drains a single-modifiedTime plateau bigger than the
// budget so the cursor never stalls (the same guarantee as GitHub issues).
const (
	gdocsPageSize        = 100
	gdocsMaxPages        = 10
	gdocsMaxPagesHardCap = 200
)

// GoogleDocs is the Google Docs connector (document archetype). A Drive is one
// modifiedTime stream, so it syncs as a single container ("drive") with one
// cursor; each artifact still records its parent folder as the manifest container
// for filing priors. Docs are exported as HTML — reasonably full-fidelity raw for
// v0.1. Base URLs are fields so tests can point them at httptest servers.
type GoogleDocs struct {
	driveBaseURL    string
	userinfoBaseURL string
	pageSize        int // Drive list page size (a field for tests)
	maxPages        int // per-run page budget (a field for tests)
}

func NewGoogleDocs() *GoogleDocs {
	return &GoogleDocs{
		driveBaseURL:    "https://www.googleapis.com",
		userinfoBaseURL: "https://www.googleapis.com",
		pageSize:        gdocsPageSize,
		maxPages:        gdocsMaxPages,
	}
}

func (g *GoogleDocs) ID() string        { return "gdocs" }
func (g *GoogleDocs) Name() string      { return "Google Docs" }
func (g *GoogleDocs) Archetype() string { return "document" }

// RateBudget seeds the adaptive limiter. Drive has no single published hourly cap
// (quota is per-project, per-minute), so KnownReqPerHour is 0 and the platform
// learns the ceiling from feedback (ADR-0036).
func (g *GoogleDocs) RateBudget() RateBudget {
	return RateBudget{StartConcurrency: 10, MaxConcurrency: 20, KnownReqPerHour: 0}
}

func (g *GoogleDocs) Scopes() []string {
	return []string{
		"https://www.googleapis.com/auth/drive.readonly",
		"https://www.googleapis.com/auth/userinfo.email",
	}
}

func (g *GoogleDocs) OAuthConfig(clientID, clientSecret, redirectURL string) *oauth2.Config {
	return &oauth2.Config{
		ClientID:     clientID,
		ClientSecret: clientSecret,
		RedirectURL:  redirectURL,
		Scopes:       g.Scopes(),
		Endpoint:     google.Endpoint,
	}
}

// AuthorizeURL is the standard OAuth consent URL with offline access + a forced
// consent prompt, so Google returns a refresh token the sync loop needs.
func (g *GoogleDocs) AuthorizeURL(creds Creds, state, redirectURL string) string {
	cfg := g.OAuthConfig(creds.ClientID, creds.ClientSecret, redirectURL)
	return cfg.AuthCodeURL(state, oauth2.AccessTypeOffline, oauth2.SetAuthURLParam("prompt", "consent"))
}

func (g *GoogleDocs) Account(ctx context.Context, hc *http.Client) (string, error) {
	body, err := httpGet(ctx, hc, g.userinfoBaseURL+"/oauth2/v2/userinfo")
	if err != nil {
		return "", err
	}
	var info struct {
		Email string `json:"email"`
	}
	if err := json.Unmarshal(body, &info); err != nil {
		return "", err
	}
	return info.Email, nil
}

// DiscoverUnits: a Drive is a single sync unit (its modifiedTime stream is one
// sequence; folders are manifest metadata, not partitions — ADR-0035).
func (g *GoogleDocs) DiscoverUnits(_ context.Context, _ *http.Client) ([]Unit, error) {
	return []Unit{{ID: "drive", Kind: "google_drive", Name: "Drive"}}, nil
}

// EstimateTotals returns an empty map: the Drive changes feed has no cheap total
// (counting every doc would cost a full listing), so every unit stays "unknown" —
// which the platform tolerates (best-effort).
func (g *GoogleDocs) EstimateTotals(_ context.Context, _ *http.Client, _ []Unit) (map[string]int, error) {
	return map[string]int{}, nil
}

type driveFile struct {
	ID           string   `json:"id"`
	Name         string   `json:"name"`
	ModifiedTime string   `json:"modifiedTime"`
	CreatedTime  string   `json:"createdTime"`
	WebViewLink  string   `json:"webViewLink"`
	Parents      []string `json:"parents"`
}

type driveListResp struct {
	NextPageToken string      `json:"nextPageToken"`
	Files         []driveFile `json:"files"`
}

// SyncUnit lists Google Docs modified since the cursor (an RFC3339 high-water mark),
// across My Drive and Shared Drives, exports each as HTML, and emits it. Each emit
// carries the file's modifiedTime as Artifact.Cursor, which the emit layer persists
// in the same transaction as the manifest (ADR-0038); `>=` plus content-hash dedup
// means boundary items are re-fetched harmlessly, never skipped. Returns done=true
// when the stream is fully drained this call, false at a page/runtime budget. The
// local newPos is only the plateau detector.
func (g *GoogleDocs) SyncUnit(ctx context.Context, hc *http.Client, _ Unit, cursor string, emit EmitFunc) (bool, error) {
	pageSize := g.pageSize
	if pageSize <= 0 {
		pageSize = gdocsPageSize
	}
	maxPages := g.maxPages
	if maxPages <= 0 {
		maxPages = gdocsMaxPages
	}
	hardCap := gdocsMaxPagesHardCap
	if maxPages > hardCap {
		hardCap = maxPages
	}

	q := "mimeType='application/vnd.google-apps.document' and trashed=false"
	if cursor != "" {
		q += fmt.Sprintf(" and modifiedTime >= '%s'", cursor)
	}

	newPos := cursor
	pageToken := ""
	for page := 1; page <= hardCap; page++ {
		if ctx.Err() != nil {
			return false, nil // cooperative stop between pages — more to fetch
		}
		params := url.Values{}
		params.Set("q", q)
		params.Set("orderBy", "modifiedTime")
		params.Set("pageSize", strconv.Itoa(pageSize))
		params.Set("fields", "nextPageToken,files(id,name,modifiedTime,createdTime,webViewLink,parents)")
		// Include Shared Drives, not just My Drive.
		params.Set("includeItemsFromAllDrives", "true")
		params.Set("supportsAllDrives", "true")
		params.Set("corpora", "allDrives")
		if pageToken != "" {
			params.Set("pageToken", pageToken)
		}

		body, err := httpGet(ctx, hc, g.driveBaseURL+"/drive/v3/files?"+params.Encode())
		if err != nil {
			return false, err
		}
		var resp driveListResp
		if err := json.Unmarshal(body, &resp); err != nil {
			return false, fmt.Errorf("decode file list: %w", err)
		}

		for _, f := range resp.Files {
			if ctx.Err() != nil {
				return false, nil // cooperative stop between items — more to fetch
			}
			raw, err := httpGet(ctx, hc, g.driveBaseURL+"/drive/v3/files/"+url.PathEscape(f.ID)+"/export?mimeType=text/html&supportsAllDrives=true")
			if err != nil {
				return false, err
			}
			if err := emit(ctx, Artifact{
				SourceNativeKind: "gdoc",
				Container:        docContainer(f),
				ExternalID:       f.ID,
				SourceURL:        f.WebViewLink,
				SourceCreatedAt:  parseRFC3339(f.CreatedTime),
				SourceUpdatedAt:  parseRFC3339(f.ModifiedTime),
				ContentType:      "text/html",
				Raw:              raw,
				// modifiedTime is this item's high-water value; the emit layer advances
				// the unit cursor to it atomically with the manifest write (ADR-0038).
				Cursor: f.ModifiedTime,
			}); err != nil {
				return false, err
			}
			if f.ModifiedTime > newPos {
				newPos = f.ModifiedTime
			}
		}

		if resp.NextPageToken == "" {
			return true, nil // natural end of the stream — caught up
		}
		pageToken = resp.NextPageToken
		// Per-run page budget — same plateau-aware logic as GitHub issues (design-doc
		// 0011 P1/P2): stop and resume from the watermark next run, unless the
		// watermark hasn't advanced past this run's start (a single modifiedTime
		// plateau larger than the budget), in which case keep paging (to the hard
		// ceiling) so the cursor can clear the plateau instead of stalling.
		if page >= maxPages {
			if newPos != cursor {
				return false, nil // budget hit, watermark advanced — more next run
			}
			if page == maxPages {
				slog.Warn("gdocs_page_cap_hit", "position", cursor)
				metrics.ConnectorCapHits.WithLabelValues("gdocs", "docs").Inc()
			}
		}
	}
	slog.Warn("gdocs_plateau_hard_cap", "cap", hardCap, "position", newPos)
	return false, nil
}

// docContainer is the per-artifact manifest container (the parent folder), used
// downstream for filing priors — distinct from the sync container above.
func docContainer(f driveFile) Container {
	id := "root"
	if len(f.Parents) > 0 {
		id = f.Parents[0]
	}
	return Container{ID: id, Kind: "drive_folder"}
}

// --- shared HTTP helpers -------------------------------------------------

const (
	requestTimeout   = 30 * time.Second
	maxArtifactBytes = 32 << 20 // 32 MiB
)

// httpGet does a GET with a per-request timeout and classifies failures so the
// worker can react: RateLimitError (snooze), AuthError (terminal → needs_reauth),
// or a plain retryable error. It detects oversize bodies instead of silently
// truncating them.
func httpGet(ctx context.Context, hc *http.Client, rawURL string) ([]byte, error) {
	rctx, cancel := context.WithTimeout(ctx, requestTimeout)
	defer cancel()
	req, err := http.NewRequestWithContext(rctx, http.MethodGet, rawURL, nil)
	if err != nil {
		return nil, err
	}
	resp, err := hc.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(io.LimitReader(resp.Body, maxArtifactBytes+1))
	if err != nil {
		return nil, err
	}

	switch {
	case resp.StatusCode == http.StatusTooManyRequests || (resp.StatusCode == http.StatusForbidden && isRateLimited(resp, body)):
		return nil, &RateLimitError{RetryAfter: retryAfter(resp)}
	// Only a 401 is a terminal credentials failure. A 403 is NOT — GitHub returns 403
	// for one restricted repo, SAML enforcement, and secondary rate limits; flipping the
	// whole connection to needs_reauth on any 403 was the H3 over-broad bug. A 403 with
	// rate-limit signals is handled above; every other 403 is a per-resource StatusError
	// (retryable/degradable), not connection-terminal.
	case resp.StatusCode == http.StatusUnauthorized:
		return nil, &AuthError{Status: resp.StatusCode}
	case resp.StatusCode < 200 || resp.StatusCode >= 300:
		return nil, &StatusError{Status: resp.StatusCode, URL: rawURL}
	}

	if len(body) > maxArtifactBytes {
		return nil, &TooLargeError{URL: rawURL, Limit: maxArtifactBytes}
	}
	return body, nil
}

// isRateLimited reports whether a 403/429 is actually a rate limit (so it snoozes) rather
// than an auth/permission failure. GitHub signals a primary limit via Retry-After /
// X-RateLimit-Remaining: 0, and a *secondary* limit with a 403 whose body says so (often
// WITHOUT the standard headers) — both must classify as a rate limit, not needs_reauth.
func isRateLimited(resp *http.Response, body []byte) bool {
	if resp.Header.Get("Retry-After") != "" || resp.Header.Get("X-RateLimit-Remaining") == "0" {
		return true
	}
	b := strings.ToLower(string(body))
	return strings.Contains(b, "secondary rate limit") || strings.Contains(b, "rate limit")
}

// retryAfter reads Retry-After (seconds) or X-RateLimit-Reset (unix seconds),
// defaulting to a minute.
func retryAfter(resp *http.Response) time.Duration {
	if v := resp.Header.Get("Retry-After"); v != "" {
		if secs, err := strconv.Atoi(v); err == nil {
			return time.Duration(secs) * time.Second
		}
	}
	if v := resp.Header.Get("X-RateLimit-Reset"); v != "" {
		if epoch, err := strconv.ParseInt(v, 10, 64); err == nil {
			if d := time.Until(time.Unix(epoch, 0)); d > 0 {
				return d
			}
		}
	}
	return time.Minute
}

func parseRFC3339(s string) *time.Time {
	if s == "" {
		return nil
	}
	t, err := time.Parse(time.RFC3339, s)
	if err != nil {
		return nil
	}
	return &t
}

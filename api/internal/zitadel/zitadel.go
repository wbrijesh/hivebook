// Package zitadel is a thin client for ZITADEL's management API — the parts we
// need beyond token verification (auth handles that). Today: listing the members
// of an organization, so the workspace can show who's in it.
//
// ZITADEL is the directory (ADR-0012: a tenant is a ZITADEL org); Postgres stays
// the system of record for everything else (ADR-0013). Authentication is a
// service-account token (a machine user's PAT) scoped to read across orgs; the
// per-org scope is selected with the x-zitadel-orgid header.
package zitadel

import (
	"bytes"
	"context"
	"crypto/tls"
	"crypto/x509"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"os"
	"strings"
	"time"
)

// ErrNotConfigured is returned when no management token is set — the caller turns
// this into a "directory integration not set up" state rather than an error.
var ErrNotConfigured = errors.New("zitadel management token not configured")

// Client talks to a ZITADEL instance's management API.
type Client struct {
	base  string // issuer base URL, e.g. https://id.hivebook.localhost
	token string // service-account PAT; empty disables the client
	hc    *http.Client
}

// New builds a client. token may be empty (the client then reports
// Configured()==false and every call returns ErrNotConfigured). caFile, if set,
// is trusted for the issuer's TLS, mirroring the auth package so the mkcert-signed
// local issuer works in-cluster.
func New(issuer, token, caFile string) *Client {
	return &Client{
		base:  strings.TrimRight(issuer, "/"),
		token: token,
		hc:    buildHTTPClient(caFile),
	}
}

// Configured reports whether a management token is set.
func (c *Client) Configured() bool { return c.token != "" }

// Member is one person in an organization.
type Member struct {
	ID    string
	Name  string
	Email string
	Roles []string
}

// listMembersResponse mirrors the fields we read from ListOrgMembers.
type listMembersResponse struct {
	Result []struct {
		UserID             string   `json:"userId"`
		DisplayName        string   `json:"displayName"`
		PreferredLoginName string   `json:"preferredLoginName"`
		Email              string   `json:"email"`
		Roles              []string `json:"roles"`
	} `json:"result"`
}

// ListOrgMembers returns the members of one organization. The org is selected by
// the x-zitadel-orgid header; the management token must be allowed to read it.
func (c *Client) ListOrgMembers(ctx context.Context, orgID string) ([]Member, error) {
	if c.token == "" {
		return nil, ErrNotConfigured
	}
	// A generous page; workspaces are small at this stage and we don't paginate yet.
	body := []byte(`{"query":{"limit":500}}`)
	url := c.base + "/management/v1/orgs/me/members/_search"
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(body))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Authorization", "Bearer "+c.token)
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("x-zitadel-orgid", orgID)

	resp, err := c.hc.Do(req)
	if err != nil {
		return nil, fmt.Errorf("list org members: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		snippet, _ := io.ReadAll(io.LimitReader(resp.Body, 512))
		return nil, fmt.Errorf("list org members: status %d: %s", resp.StatusCode, strings.TrimSpace(string(snippet)))
	}
	var out listMembersResponse
	if err := json.NewDecoder(resp.Body).Decode(&out); err != nil {
		return nil, fmt.Errorf("list org members: decode: %w", err)
	}
	members := make([]Member, 0, len(out.Result))
	for _, r := range out.Result {
		name := r.DisplayName
		if name == "" {
			name = r.PreferredLoginName
		}
		members = append(members, Member{
			ID:    r.UserID,
			Name:  name,
			Email: r.Email,
			Roles: r.Roles,
		})
	}
	return members, nil
}

// buildHTTPClient trusts the system pool plus an optional extra CA (the mkcert CA
// for the local issuer), mirroring auth.buildHTTPClient.
func buildHTTPClient(caFile string) *http.Client {
	pool, err := x509.SystemCertPool()
	if err != nil || pool == nil {
		pool = x509.NewCertPool()
	}
	if caFile != "" {
		if pem, err := os.ReadFile(caFile); err == nil {
			pool.AppendCertsFromPEM(pem)
		} else {
			slog.Warn("zitadel_ca_file_unreadable", "path", caFile, "error", err.Error())
		}
	}
	return &http.Client{
		Timeout:   10 * time.Second,
		Transport: &http.Transport{TLSClientConfig: &tls.Config{RootCAs: pool, MinVersion: tls.VersionTLS12}},
	}
}

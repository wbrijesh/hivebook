package connectors

import (
	"bytes"
	"context"
	"crypto"
	"crypto/rand"
	"crypto/rsa"
	"crypto/sha256"
	"crypto/x509"
	"encoding/base64"
	"encoding/json"
	"encoding/pem"
	"fmt"
	"net/http"
	"strconv"
	"time"
)

// GitHub App server-to-server auth (design-doc 0010). A GitHub App authenticates
// as an *installation* — not as the connecting user — so it sees every repo and
// Project the org grants it, regardless of that user's personal access, and can
// be uninstalled programmatically. We mint installation access tokens from the
// App's private key: sign a short-lived App JWT (RS256), then exchange it for an
// installation token. No third-party JWT library — RS256 is a few lines here.

// GitHubAPIBase is the REST/GraphQL host for App-level calls.
const GitHubAPIBase = "https://api.github.com"

// ParseGitHubKey parses a PEM private key (PKCS#1, as GitHub issues, or PKCS#8).
func ParseGitHubKey(pemBytes []byte) (*rsa.PrivateKey, error) {
	block, _ := pem.Decode(pemBytes)
	if block == nil {
		return nil, fmt.Errorf("github key: not PEM")
	}
	if k, err := x509.ParsePKCS1PrivateKey(block.Bytes); err == nil {
		return k, nil
	}
	k, err := x509.ParsePKCS8PrivateKey(block.Bytes)
	if err != nil {
		return nil, fmt.Errorf("github key: %w", err)
	}
	rk, ok := k.(*rsa.PrivateKey)
	if !ok {
		return nil, fmt.Errorf("github key: not RSA")
	}
	return rk, nil
}

func b64url(b []byte) string { return base64.RawURLEncoding.EncodeToString(b) }

// appJWT signs the App JWT used to authenticate as the App itself. iss is the
// App's Client ID (GitHub accepts it in place of the numeric App ID). Lifetime is
// kept short (<10m); iat is backdated 30s for clock skew.
func appJWT(clientID string, key *rsa.PrivateKey) (string, error) {
	now := time.Now()
	header, _ := json.Marshal(map[string]string{"alg": "RS256", "typ": "JWT"})
	claims, _ := json.Marshal(map[string]any{
		"iat": now.Add(-30 * time.Second).Unix(),
		"exp": now.Add(9 * time.Minute).Unix(),
		"iss": clientID,
	})
	signingInput := b64url(header) + "." + b64url(claims)
	digest := sha256.Sum256([]byte(signingInput))
	sig, err := rsa.SignPKCS1v15(rand.Reader, key, crypto.SHA256, digest[:])
	if err != nil {
		return "", err
	}
	return signingInput + "." + b64url(sig), nil
}

func appRequest(ctx context.Context, hc *http.Client, method, url, jwt string) (*http.Response, error) {
	req, err := http.NewRequestWithContext(ctx, method, url, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Authorization", "Bearer "+jwt)
	req.Header.Set("Accept", "application/vnd.github+json")
	return hc.Do(req)
}

// GitHubInstallationToken mints an installation access token (valid ~1h) for the
// installation, authenticating as the App via a freshly signed JWT.
func GitHubInstallationToken(ctx context.Context, hc *http.Client, clientID string, key *rsa.PrivateKey, installationID string) (string, time.Time, error) {
	jwt, err := appJWT(clientID, key)
	if err != nil {
		return "", time.Time{}, err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost,
		GitHubAPIBase+"/app/installations/"+installationID+"/access_tokens", bytes.NewReader([]byte("{}")))
	if err != nil {
		return "", time.Time{}, err
	}
	req.Header.Set("Authorization", "Bearer "+jwt)
	req.Header.Set("Accept", "application/vnd.github+json")
	resp, err := hc.Do(req)
	if err != nil {
		return "", time.Time{}, err
	}
	defer resp.Body.Close()
	if resp.StatusCode == http.StatusUnauthorized || resp.StatusCode == http.StatusForbidden {
		return "", time.Time{}, &AuthError{Status: resp.StatusCode}
	}
	if resp.StatusCode != http.StatusCreated {
		return "", time.Time{}, fmt.Errorf("installation token: status %d", resp.StatusCode)
	}
	var out struct {
		Token     string `json:"token"`
		ExpiresAt string `json:"expires_at"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&out); err != nil {
		return "", time.Time{}, err
	}
	exp, _ := time.Parse(time.RFC3339, out.ExpiresAt)
	return out.Token, exp, nil
}

// GitHubInstallationAccount returns the account (login + type) an installation is
// on — used to label the connection and to scope Projects v2 queries.
func GitHubInstallationAccount(ctx context.Context, hc *http.Client, clientID string, key *rsa.PrivateKey, installationID string) (login, accountType string, err error) {
	jwt, err := appJWT(clientID, key)
	if err != nil {
		return "", "", err
	}
	resp, err := appRequest(ctx, hc, http.MethodGet, GitHubAPIBase+"/app/installations/"+installationID, jwt)
	if err != nil {
		return "", "", err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return "", "", fmt.Errorf("installation lookup: status %d", resp.StatusCode)
	}
	var out struct {
		Account struct {
			Login string `json:"login"`
			Type  string `json:"type"`
		} `json:"account"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&out); err != nil {
		return "", "", err
	}
	return out.Account.Login, out.Account.Type, nil
}

// GitHubResolveInstallation returns the id of the App's *live* installation on an
// account (org or user), or "" if it isn't installed there. Disconnect uses this
// to remove the account's current installation even when our stored id has drifted
// — e.g. the user reinstalled or updated access outside a fresh connect, which
// mints a new installation id we never recorded.
func GitHubResolveInstallation(ctx context.Context, hc *http.Client, clientID string, key *rsa.PrivateKey, login string) (string, error) {
	if login == "" {
		return "", nil
	}
	jwt, err := appJWT(clientID, key)
	if err != nil {
		return "", err
	}
	// An account is either an org or a user; try both, precise (no listing).
	for _, prefix := range []string{"/orgs/", "/users/"} {
		id, err := func() (string, error) {
			resp, err := appRequest(ctx, hc, http.MethodGet, GitHubAPIBase+prefix+login+"/installation", jwt)
			if err != nil {
				return "", err
			}
			defer resp.Body.Close()
			if resp.StatusCode == http.StatusNotFound {
				return "", nil
			}
			if resp.StatusCode != http.StatusOK {
				return "", fmt.Errorf("resolve installation: status %d", resp.StatusCode)
			}
			var out struct {
				ID int64 `json:"id"`
			}
			if err := json.NewDecoder(resp.Body).Decode(&out); err != nil {
				return "", err
			}
			if out.ID == 0 {
				return "", nil
			}
			return strconv.FormatInt(out.ID, 10), nil
		}()
		if err != nil {
			return "", err
		}
		if id != "" {
			return id, nil
		}
	}
	return "", nil
}

// GitHubUninstall removes the App installation — so disconnecting in Hivebook
// actually uninstalls the App, not just drops our side. Best-effort.
func GitHubUninstall(ctx context.Context, hc *http.Client, clientID string, key *rsa.PrivateKey, installationID string) error {
	jwt, err := appJWT(clientID, key)
	if err != nil {
		return err
	}
	resp, err := appRequest(ctx, hc, http.MethodDelete, GitHubAPIBase+"/app/installations/"+installationID, jwt)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusNoContent && resp.StatusCode != http.StatusNotFound {
		return fmt.Errorf("uninstall: status %d", resp.StatusCode)
	}
	return nil
}

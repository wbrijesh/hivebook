package connectors

import (
	"os"
	"strings"
)

// Creds is a connector's OAuth client credentials, registered with the provider.
// AppSlug is set only for GitHub-App-style connectors — the app's URL name, used
// to route Connect to the install (repo-picker) flow instead of plain OAuth.
type Creds struct {
	ClientID     string
	ClientSecret string
	AppSlug      string
	PrivateKey   string // PEM — GitHub App private key, for installation tokens
}

// CredsFromEnv reads HIVEBOOK_<ID>_CLIENT_ID / _CLIENT_SECRET / _APP_SLUG for each
// registered connector (e.g. HIVEBOOK_GDOCS_CLIENT_ID, HIVEBOOK_GITHUB_APP_SLUG).
func CredsFromEnv(reg *Registry) map[string]Creds {
	m := make(map[string]Creds, len(reg.All()))
	for _, c := range reg.All() {
		up := strings.ToUpper(c.ID())
		m[c.ID()] = Creds{
			ClientID:     os.Getenv("HIVEBOOK_" + up + "_CLIENT_ID"),
			ClientSecret: os.Getenv("HIVEBOOK_" + up + "_CLIENT_SECRET"),
			AppSlug:      os.Getenv("HIVEBOOK_" + up + "_APP_SLUG"),
			PrivateKey:   os.Getenv("HIVEBOOK_" + up + "_PRIVATE_KEY"),
		}
	}
	return m
}

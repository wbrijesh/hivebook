#!/usr/bin/env bash
# Idempotent ZITADEL provisioning: project "Hivebook" + web (PKCE) and api apps,
# admin email, and the `hivebook-oidc` secret consumed by the web build + Go API.
# Authenticates with the iam-admin PAT that FirstInstance bootstraps. Re-runnable:
# finds existing objects instead of duplicating them.
set -uo pipefail

ISS="https://id.hivebook.localhost"
MGMT="$ISS/management/v1"
CA="$(mkcert -CAROOT)/rootCA.pem"
PAT="$(kubectl -n zitadel get secret iam-admin-pat -o jsonpath='{.data.pat}' 2>/dev/null | base64 -d)"
[ -z "$PAT" ] && { echo "!! no iam-admin PAT — is ZITADEL up (just auth)?"; exit 1; }

api() { curl -sS --cacert "$CA" -H "Authorization: Bearer $PAT" -H "Content-Type: application/json" "$@"; }

echo ">> ensuring project 'Hivebook'..."
PROJECT_ID=$(api -X POST "$MGMT/projects/_search" \
  -d '{"queries":[{"nameQuery":{"name":"Hivebook","method":"TEXT_QUERY_METHOD_EQUALS"}}]}' \
  | jq -r '.result[0].id // empty')
[ -z "$PROJECT_ID" ] && PROJECT_ID=$(api -X POST "$MGMT/projects" -d '{"name":"Hivebook"}' | jq -r '.id')
echo "   PROJECT_ID=$PROJECT_ID"

# clientId of an existing app by name (works for both oidc + api apps), or empty
app_client_id() {
  api -X POST "$MGMT/projects/$PROJECT_ID/apps/_search" -d '{}' \
    | jq -r --arg n "$1" '.result[]? | select(.name==$n) | (.oidcConfig.clientId // .apiConfig.clientId) // empty' | head -1
}

echo ">> ensuring web app (User-Agent / PKCE / JWT tokens)..."
WEB_CLIENT_ID=$(app_client_id web)
[ -z "$WEB_CLIENT_ID" ] && WEB_CLIENT_ID=$(api -X POST "$MGMT/projects/$PROJECT_ID/apps/oidc" -d '{
  "name":"web",
  "redirectUris":["https://app.hivebook.localhost/callback"],
  "postLogoutRedirectUris":["https://app.hivebook.localhost/"],
  "responseTypes":["OIDC_RESPONSE_TYPE_CODE"],
  "grantTypes":["OIDC_GRANT_TYPE_AUTHORIZATION_CODE"],
  "appType":"OIDC_APP_TYPE_USER_AGENT",
  "authMethodType":"OIDC_AUTH_METHOD_TYPE_NONE",
  "accessTokenType":"OIDC_TOKEN_TYPE_JWT",
  "devMode":false}' | jq -r '.clientId')
echo "   WEB_CLIENT_ID=$WEB_CLIENT_ID"

echo ">> ensuring api app..."
API_CLIENT_ID=$(app_client_id api)
[ -z "$API_CLIENT_ID" ] && API_CLIENT_ID=$(api -X POST "$MGMT/projects/$PROJECT_ID/apps/api" \
  -d '{"name":"api","authMethodType":"API_AUTH_METHOD_TYPE_PRIVATE_KEY_JWT"}' | jq -r '.clientId')
echo "   API_CLIENT_ID=$API_CLIENT_ID"

echo ">> setting admin email -> admin@hivebook.localhost..."
ADMIN_ID=$(api -X POST "$MGMT/users/_search" \
  -d '{"queries":[{"userNameQuery":{"userName":"admin@","method":"TEXT_QUERY_METHOD_CONTAINS"}}]}' \
  | jq -r '.result[]? | select(.userName|startswith("admin@")) | .id' | head -1)
[ -n "$ADMIN_ID" ] && api -X PUT "$MGMT/users/$ADMIN_ID/email" \
  -d '{"email":"admin@hivebook.localhost","isEmailVerified":true}' >/dev/null

if [ -z "$PROJECT_ID" ] || [ -z "$WEB_CLIENT_ID" ] || [ -z "$API_CLIENT_ID" ]; then
  echo "!! provisioning incomplete (PROJECT_ID/WEB_CLIENT_ID/API_CLIENT_ID empty)"; exit 1
fi

echo ">> writing hivebook-oidc secret..."
kubectl get ns hivebook >/dev/null 2>&1 || kubectl create ns hivebook
kubectl -n hivebook create secret generic hivebook-oidc \
  --from-literal=OIDC_ISSUER="$ISS" \
  --from-literal=PROJECT_ID="$PROJECT_ID" \
  --from-literal=WEB_CLIENT_ID="$WEB_CLIENT_ID" \
  --from-literal=API_CLIENT_ID="$API_CLIENT_ID" \
  --dry-run=client -o yaml | kubectl apply -f -
echo ">> done."

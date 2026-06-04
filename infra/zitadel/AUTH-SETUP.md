# ZITADEL provisioning (manual, day 1)

We provision the project and applications **by hand** in the console for now.
Automating it (Terraform / ZITADEL provider) comes later, once the auth model
is stable. These are the exact clicks; the result is four values dropped into the
`trenches-oidc` Secret (template: `oidc-secret.example.yaml`).

## Log in

- Console: <https://id.trenches.localhost/ui/console>
- User: `admin@trenches.localhost`
- Password: `Password1!`

(The cert is trusted via the mkcert CA in your macOS keychain — no warning.)

## 1. Create the project

Projects → **Create New Project** → name it **Trenches** → create.

Open the project and copy its **Resource Id** (the project ID) — this is
`PROJECT_ID`. The web app puts this ID into the access-token audience, and the Go
API validates tokens against it.

## 2. Create the `web` application (User Agent / PKCE)

In the Trenches project → Applications → **New**:

- Name: `web`
- Type: **User Agent**
- Authentication method: **PKCE**
- Redirect URIs: `https://app.trenches.localhost/callback`
- Post-logout URIs: `https://app.trenches.localhost/`

Create, then in the app's **Token settings**, set **Auth Token Type** to
**JWT** (so the SPA receives a JWT access token the API can validate offline) and
save.

Copy the app's **Client Id** — this is `WEB_CLIENT_ID`.

## 3. Create the `api` application (API)

In the Trenches project → Applications → **New**:

- Name: `api`
- Type: **API**
- Authentication method: **JWT** (or Basic)

Copy the **Client Id** — this is `API_CLIENT_ID`. (Not required for JWT audience
validation today — the API validates against `PROJECT_ID` — but it represents the
resource server and enables token introspection later.)

## 4. Write the Secret

Fill in `oidc-secret.example.yaml` with the four values and apply it:

```sh
kubectl apply -f infra/zitadel/oidc-secret.example.yaml   # after editing
```

Keys:

| Key             | From                          | Used by            |
| --------------- | ----------------------------- | ------------------ |
| `OIDC_ISSUER`   | `https://id.trenches.localhost` | web build, api   |
| `PROJECT_ID`    | step 1 (project Resource Id)  | web build, api (audience) |
| `WEB_CLIENT_ID` | step 2 (web app Client Id)    | web build          |
| `API_CLIENT_ID` | step 3 (api app Client Id)    | reference / future |

## 5. Build + deploy web, redeploy api

```sh
make -C infra web    # bakes WEB_CLIENT_ID/PROJECT_ID into the web image, deploys
make -C infra api    # redeploys api so it picks up PROJECT_ID as the audience
```

Then visit <https://app.trenches.localhost>, log in, and click **Call
GET /api/me** — you should see the token claims returned by the Go API.

---

## What was actually done the first time (API one-off)

The console steps above are the human-readable record. The first provisioning was
done via the management API as a one-off (not in the Makefile), using the
`iam-admin` machine user + PAT that the chart bootstraps (`FirstInstance.Org.Machine`,
secret `iam-admin-pat`). This is the basis for the future Terraform/provider
automation. The calls, all `Authorization: Bearer <iam-admin PAT>` against
`https://id.trenches.localhost/management/v1`:

- `POST /projects` `{"name":"Trenches"}` → `PROJECT_ID`
- `POST /projects/{PROJECT_ID}/apps/oidc` with appType `USER_AGENT`, auth method
  `NONE` (PKCE), `accessTokenType: OIDC_TOKEN_TYPE_JWT`, the redirect/post-logout
  URIs → `WEB_CLIENT_ID`
- `POST /projects/{PROJECT_ID}/apps/api` → `API_CLIENT_ID`

The `trenches-oidc` Secret was then created with `kubectl create secret generic`.
Verified end-to-end by minting a project-audience JWT (jwt-bearer grant with the
`iam-admin` key) and calling `/api/me` → 200 with claims.

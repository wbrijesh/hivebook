# Hivebook — local dev control panel (OrbStack Kubernetes). Run `just`.

set shell := ["bash", "-c"]
# justfile lives at the repo root, but the manifests/helm-values are under infra/,
# so recipes run from there and keep their infra-relative paths (and ../api, ../web).
set working-directory := 'infra'
# Single source of secret values (gitignored). Recipes inject these into the
# cluster; not required for start/stop/status so it's fine if it's missing.
set dotenv-load := true
set dotenv-path := 'infra/.env'
set dotenv-required := false

caroot := `mkcert -CAROOT 2>/dev/null`
ns := "traefik cert-manager zitadel hivebook observability"

[private]
default:
    @just help

# Show this help
help:
    #!/usr/bin/env bash
    . scripts/ui.sh
    ui_logo
    ui_subtle "local dev control panel · OrbStack Kubernetes (run from anywhere in the repo)"
    ui_subtle "the intended interface for working here — for humans and coding agents alike"
    echo
    ui_section "Commands"
    ui_box \
      "just setup                 first-time setup — local CA, secrets, full install" \
      "just start  [service]      turn ON  — everything, or one service" \
      "just stop   [service]      turn OFF — everything, or one service" \
      "just reset-data            wipe ALL local data for a clean test (then just start)" \
      "just update [service...]   rebuild image(s) & roll out — after code changes" \
      "just check                 run all checks (web + api) via Dagger — the gate" \
      "just status [service]      what's running (pod readiness)" \
      "just health                real health checks across services" \
      "just logs   [service]      follow one service's live log" \
      "just urls                  service URLs & logins"
    echo
    ui_section "Working in the repo"
    ui_info "First time?    →  just setup   trusts a local CA, writes infra/.env, deploys the whole stack."
    ui_info "Before pushing →  just check   runs the same Dagger gate as CI (web: prettier/eslint/tsc · api: build/vet/test)."
    ui_info "Changed code   →  just update web api   rebuilds the image(s) & rolls out — in parallel, zero-downtime."
    ui_info "Bring it up    →  just start, then just health to confirm everything's green."
    ui_info "Inspect state  →  just status (pod readiness) · just health (live endpoint + DB checks)."
    ui_info "Logs & metrics →  Grafana (just urls) is the observability surface — search, dashboards, history."
    ui_info "                  just logs is only a quick live tail of one service, not for investigating."
    ui_subtle "buildable: api · web · docs    ·    opt-in: prototype    ·    full set also: postgres · zitadel · zitadel-db · zitadel-login"

# Service URLs & logins
urls:
    #!/usr/bin/env bash
    . scripts/ui.sh
    ui_section "URLs & logins  (HTTPS via mkcert · local creds, never reuse)"
    ui_box \
      "Web app   https://app.hivebook.localhost" \
      "Docs      https://docs.hivebook.localhost       Antora site (no login)" \
      "Auth      https://id.hivebook.localhost         admin@hivebook.localhost / Password1!" \
      "API       https://api.hivebook.localhost        /health · /metrics · /api/me" \
      "Grafana   https://grafana.hivebook.localhost    admin / password1234"
    ui_subtle "App DB  hivebook / password1234    ·    ZITADEL DB  zitadel / zitadel-local-pw"
    ui_subtle "IAM PAT  kubectl -n zitadel get secret iam-admin-pat -o jsonpath='{.data.pat}' | base64 -d"

# Real health checks (HTTP endpoints + DB + datasources), grouped by tier
health:
    #!/usr/bin/env bash
    set -uo pipefail
    . scripts/ui.sh
    CA="$(mkcert -CAROOT 2>/dev/null)/rootCA.pem"
    GPW="${GRAFANA_ADMIN_PASSWORD:-admin}"
    G="https://grafana.hivebook.localhost"
    if ! kubectl get ns >/dev/null 2>&1; then ui_fail "cluster unreachable — run 'just start'."; exit 1; fi
    hc_ok=0; hc_total=0
    http() { curl -sf -o /dev/null --max-time 6 --cacert "$CA" "$1"; }          # 0 on 2xx/3xx
    gds()  { curl -sf --max-time 8 --cacert "$CA" -u "admin:$GPW" "$G/api/datasources/uid/$1/health" 2>/dev/null | grep -q '"status":"OK"'; }
    check() {
      local name="$1" detail="$2"; shift 2
      hc_total=$((hc_total+1))
      if "$@" >/dev/null 2>&1; then ui_ok "$(printf '%-16s %s' "$name" "$detail")"; hc_ok=$((hc_ok+1))
      else ui_fail "$(printf '%-16s %s' "$name" "$detail")"; fi
    }

    ui_header "System Health"

    ui_section "Infrastructure"
    check "Traefik" "TLS verified :443" bash -c "echo | openssl s_client -connect 127.0.0.1:443 -servername app.hivebook.localhost -CAfile '$CA' 2>/dev/null | grep -q 'Verify return code: 0 '"
    check "cert-manager" "wildcard cert Ready" bash -c "kubectl -n hivebook get certificate wildcard-hivebook -o jsonpath='{.status.conditions}' 2>/dev/null | grep -q '\"status\":\"True\"'"

    ui_section "Identity"
    check "ZITADEL" "OIDC discovery 200" http "https://id.hivebook.localhost/.well-known/openid-configuration"

    ui_section "Hivebook"
    check "API" "/health 200" http "https://api.hivebook.localhost/health"
    check "Web" "HTTP 200" http "https://app.hivebook.localhost/"
    check "Docs" "HTTP 200" http "https://docs.hivebook.localhost/"
    check "Postgres" "pg_isready" kubectl -n hivebook exec statefulset/postgres -- pg_isready -q

    ui_section "Observability"
    check "Grafana" "/api/health 200" http "$G/api/health"
    vm_uid=$(curl -sf --max-time 6 --cacert "$CA" -u "admin:$GPW" "$G/api/datasources" 2>/dev/null | tr '}' '\n' | grep '"type":"prometheus"' | grep -oE '"uid":"[^"]+"' | head -1 | cut -d'"' -f4)
    check "VictoriaMetrics" "datasource query" gds "${vm_uid:-VictoriaMetrics}"
    check "VictoriaLogs" "datasource query" gds "VictoriaLogs"

    # Workload readiness footer.
    wl_ready=0; wl_total=0
    for n in {{ns}}; do
      while read -r nm r d; do
        [ "$r" = "<none>" ] && r=0
        wl_total=$((wl_total+1))
        { [ "${r:-0}" -ge "${d:-1}" ] && [ "${d:-0}" -ge 1 ]; } 2>/dev/null && wl_ready=$((wl_ready+1))
      done < <(kubectl -n "$n" get deploy,statefulset -o custom-columns=N:.metadata.name,R:.status.readyReplicas,D:.spec.replicas --no-headers 2>/dev/null)
    done
    echo
    ui_subtle "$wl_ready/$wl_total workloads ready"
    if [ "$hc_ok" = "$hc_total" ]; then ui_ok "all $hc_total checks healthy"; else ui_fail "$((hc_total-hc_ok)) of $hc_total checks failing"; fi

# Turn ON — the whole app (default) or one service
start service="all":
    #!/usr/bin/env bash
    set -uo pipefail
    . scripts/ui.sh
    if ! kubectl get ns >/dev/null 2>&1; then
      ui_fail "Kubernetes isn't reachable — start OrbStack / enable Kubernetes first."
      exit 1
    fi
    if ! kubectl get ns hivebook >/dev/null 2>&1; then
      ui_fail "Hivebook isn't installed yet — run 'just setup' (first-time setup)."
      exit 1
    fi
    if [ "{{service}}" = all ]; then
      ui_header "Starting Hivebook"
      # Scale everything up first (fast), and un-taint our daemonsets.
      for n in {{ns}}; do
        kubectl -n "$n" scale deploy,statefulset --all --replicas=1 >/dev/null 2>&1 || true
        for ds in $(kubectl -n "$n" get ds -o name 2>/dev/null); do
          kubectl -n "$n" patch "$ds" --type merge -p '{"spec":{"template":{"spec":{"nodeSelector":{"hivebook.io/stopped":null}}}}}' >/dev/null 2>&1 || true
        done
      done
      # Prototype is opt-in (not part of the managed set, image often unbuilt) —
      # keep it parked unless started explicitly with `just start prototype`.
      kubectl -n hivebook scale deploy/prototype --replicas=0 >/dev/null 2>&1 || true
      # Then watch all workloads come up together, live.
      if ui_progress {{ns}}; then
        ui_ok "all workloads ready"
        ui_info "'just urls' for links & logins"
      else
        ui_info "some workloads are still coming up — 'just status' to watch."
      fi
    else
      n=$(just _ns {{service}}) || { ui_fail "unknown service '{{service}}'"; exit 1; }
      kubectl -n "$n" scale deploy/{{service}} --replicas=1 >/dev/null 2>&1 || kubectl -n "$n" scale statefulset/{{service}} --replicas=1 >/dev/null 2>&1
      if ui_step "starting {{service}}" bash -c "kubectl -n '$n' rollout status deploy/{{service}} --timeout=180s 2>/dev/null || kubectl -n '$n' rollout status statefulset/{{service}} --timeout=180s"; then
        ui_ok "{{service}} is up  ($n)"
      else
        ui_fail "{{service}} — still starting"
      fi
    fi

# Turn OFF — the whole app (default) or one service. Only our workloads; OrbStack
# itself is left running for you to manage.
stop service="all":
    #!/usr/bin/env bash
    set -uo pipefail
    . scripts/ui.sh
    if ! kubectl get ns >/dev/null 2>&1; then
      ui_info "Kubernetes isn't reachable (nothing to stop)."
      exit 0
    fi
    scale_down() {
      for n in {{ns}}; do
        kubectl -n "$n" scale deploy,statefulset --all --replicas=0 >/dev/null 2>&1 || true
        for ds in $(kubectl -n "$n" get ds -o name 2>/dev/null); do
          kubectl -n "$n" patch "$ds" --type merge -p '{"spec":{"template":{"spec":{"nodeSelector":{"hivebook.io/stopped":"true"}}}}}' >/dev/null 2>&1 || true
        done
      done
    }
    sweep_terminal_pods() {  # remove Completed/Failed one-shot pods (e.g. helm-hook Jobs)
      for n in {{ns}}; do
        kubectl -n "$n" delete pod --field-selector=status.phase=Succeeded >/dev/null 2>&1 || true
        kubectl -n "$n" delete pod --field-selector=status.phase=Failed >/dev/null 2>&1 || true
      done
    }
    if [ "{{service}}" = all ]; then
      ui_header "Stopping Hivebook"
      # Two passes: the first stops everything (incl. operators); after operators
      # are down the second re-scales anything they revived so it stays at zero.
      scale_down
      ui_step "letting operators settle" sleep 5
      scale_down
      sweep_terminal_pods
      ui_ok "all workloads scaled to zero"
      ui_box "Stopped — data preserved, OrbStack left running." "Run 'just start' to bring it back up."
    else
      n=$(just _ns {{service}}) || { ui_fail "unknown service '{{service}}'"; exit 1; }
      kubectl -n "$n" scale deploy/{{service}} --replicas=0 >/dev/null 2>&1 || kubectl -n "$n" scale statefulset/{{service}} --replicas=0 >/dev/null 2>&1
      ui_ok "{{service}} stopped  ($n)"
    fi

# Wipe ALL local app data for a clean test run: BOTH app databases — integrations
# (connections, projects, synced items) and hivebook (the workspace tenant: region,
# onboarding, feature flags, audit) — plus the object-storage volume and Temporal
# workflows. Only your login (Zitadel, a separate database) survives. Run
# `just reset-data`, then `just start`. Skip the prompt with `just reset-data force`.
reset-data force="":
    #!/usr/bin/env bash
    set -uo pipefail
    . scripts/ui.sh
    if ! kubectl get ns hivebook >/dev/null 2>&1; then
      ui_fail "cluster unreachable — start OrbStack / run 'just start' first."
      exit 1
    fi
    ui_header "Reset local data"
    ui_subtle "Deletes: BOTH app databases (workspace · region · onboarding · feature flags · connections · projects · synced items) · object storage · Temporal workflows."
    ui_subtle "Keeps:   only your login (Zitadel). You'll re-do onboarding (region) + re-enable feature flags after."
    if [ "{{force}}" != force ]; then
      read -r -p "  Type 'reset' to confirm: " ans
      [ "$ans" = reset ] || { ui_fail "aborted — nothing changed."; exit 1; }
    fi
    # Postgres must be RUNNING to drop the databases. Bring it up (idempotent — it's
    # scaled to zero if you ran 'just stop' first) and wait until it's ready; otherwise
    # the psql exec below hangs/fails against a missing pod.
    kubectl -n hivebook scale statefulset/postgres --replicas=1 >/dev/null 2>&1 || true
    if ! kubectl -n hivebook rollout status statefulset/postgres --timeout=120s >/dev/null 2>&1; then
      ui_fail "postgres didn't come up — can't reset. Run 'just start', then retry."
      exit 1
    fi
    ui_ok "postgres is up"
    # Stop everything that holds a Postgres connection (api owns the hivebook DB;
    # integrations + worker own the integrations DB) so the databases can be dropped.
    kubectl -n hivebook scale deploy/api deploy/integrations deploy/integrations-worker --replicas=0 >/dev/null 2>&1 || true
    sleep 2
    # Drop + recreate BOTH app databases (each re-migrated on next start). pg() runs
    # against the 'postgres' maintenance DB so it never holds open the DB being dropped.
    PW=$(kubectl -n hivebook get secret hivebook-db -o jsonpath='{.data.POSTGRES_PASSWORD}' | base64 -d)
    pg() { kubectl -n hivebook exec statefulset/postgres -- env PGPASSWORD="$PW" psql -U hivebook -d postgres -tAc "$1" >/dev/null 2>&1; }
    drop_db() {  # $1 = db name → terminate stragglers, drop, recreate owned by hivebook
      pg "SELECT pg_terminate_backend(pid) FROM pg_stat_activity WHERE datname='$1' AND pid<>pg_backend_pid();" || true
      pg "DROP DATABASE IF EXISTS \"$1\";" || true
      pg "CREATE DATABASE \"$1\" OWNER hivebook;"
    }
    if drop_db integrations; then ui_ok "integrations database wiped"; else ui_fail "couldn't recreate the integrations database"; fi
    if drop_db hivebook; then ui_ok "app database wiped (workspace · region · flags)"; else ui_fail "couldn't recreate the app database"; fi
    # Wipe the object-storage volume (recreated fresh on next start).
    kubectl -n hivebook scale statefulset/rustfs --replicas=0 >/dev/null 2>&1 || true
    kubectl -n hivebook wait --for=delete pod/rustfs-0 --timeout=60s >/dev/null 2>&1 || true
    if kubectl -n hivebook delete pvc data-rustfs-0 >/dev/null 2>&1; then ui_ok "object storage wiped"; else ui_subtle "object-storage volume already absent"; fi
    # Clear Temporal's in-memory workflows (dev server) by recreating the pod.
    kubectl -n hivebook rollout restart deploy/temporal >/dev/null 2>&1 || true
    ui_ok "Temporal workflows cleared"
    ui_box "Cleared — both app databases, object storage, and Temporal are empty." "Run 'just start', then re-do onboarding (region) + reconnect your source."

# Logs — follow one service, or recent lines from everything
logs service="all":
    #!/usr/bin/env bash
    set -uo pipefail
    . scripts/ui.sh
    if [ "{{service}}" = all ]; then
      ui_header "Recent logs"
      for n in {{ns}}; do
        for d in $(kubectl -n "$n" get deploy,statefulset -o name 2>/dev/null); do
          ui_section "$n / ${d##*/}"
          kubectl -n "$n" logs "$d" --tail=20 2>/dev/null || true
        done
      done
    else
      n=$(just _ns {{service}}) || { ui_fail "unknown service '{{service}}'"; exit 1; }
      ui_section "logs: {{service}}  ($n)  — following"
      kubectl -n "$n" logs deploy/{{service}} --tail=200 -f 2>/dev/null || kubectl -n "$n" logs statefulset/{{service}} --tail=200 -f
    fi

# What's running — all, or one service
status service="all":
    #!/usr/bin/env bash
    set -uo pipefail
    . scripts/ui.sh
    if ! kubectl get ns >/dev/null 2>&1; then
      ui_fail "cluster unreachable — run 'just start' (or start OrbStack)."
      exit 1
    fi
    show() {  # ns, pod, ready, status
      case "$3" in
        Running)             if [ "${2%/*}" = "${2#*/}" ]; then ui_ok "$1  ($2)"; else ui_fail "$1  ($2)  not ready"; fi ;;
        Completed|Succeeded) ui_info "$1  ($3)" ;;
        *)                   ui_fail "$1  ($2)  $3" ;;
      esac
    }
    if [ "{{service}}" = all ]; then
      ui_header "Hivebook status"
      any=0
      for n in {{ns}}; do
        pods=$(kubectl -n "$n" get pods --no-headers 2>/dev/null)
        [ -z "$pods" ] && continue
        any=1
        ui_section "$n"
        echo "$pods" | while read -r pod ready status _rest; do show "$pod" "$ready" "$status"; done
      done
      [ "$any" = 1 ] || ui_info "nothing running — 'just start' to bring it up."
    else
      ui_header "{{service}}"
      pods=$(kubectl get pods -A --no-headers 2>/dev/null | grep -E "[[:space:]]{{service}}|{{service}}-" || true)
      [ -z "$pods" ] && { ui_info "no pods matching '{{service}}'."; exit 0; }
      echo "$pods" | while read -r ns pod ready status _rest; do show "$ns/$pod" "$ready" "$status"; done
    fi

# The day-to-day "I changed code" command. Defaults to all app services; pass any
# subset (`just update web api`). Targets build and roll out in parallel — no waiting
# on one before the next — and each rollout is zero-downtime.
# Rebuild image(s) & roll out — parallel across the named services
update *services:
    #!/usr/bin/env bash
    set -uo pipefail
    . scripts/ui.sh
    if ! kubectl get ns >/dev/null 2>&1; then
      ui_fail "Kubernetes isn't reachable — run 'just start' first."
      exit 1
    fi
    buildable="api web integrations prototype docs"
    targets="{{services}}"; [ -z "${targets// /}" ] && targets="api web integrations docs" # prototype is opt-in
    for s in $targets; do
      case " $buildable " in *" $s "*) ;; *) ui_fail "can't build '$s' — buildable: $buildable"; exit 1 ;; esac
    done

    ui_header "Rebuild & deploy"
    tmp=$(mktemp -d); trap 'rm -rf "$tmp"' EXIT
    svcs=(); for s in $targets; do svcs+=("$s"); done
    for s in "${svcs[@]}"; do
      ( just _build "$s" >"$tmp/$s.log" 2>&1; echo $? >"$tmp/$s.rc" ) &
    done

    # Live, all-at-once status block: one row per service, redrawn in place.
    spin='⠋⠙⠹⠸⠼⠴⠦⠧⠇⠏'; frame=0; first=1; total=${#svcs[@]}
    OK=$'\033[38;5;42m'; RUN=$'\033[38;5;208m'; ERR=$'\033[38;5;196m'; DIM=$'\033[38;5;244m'; OFF=$'\033[0m'
    while :; do
      [ "$first" = 1 ] || printf '\033[%dA' "$total"; first=0
      done_n=0; sp=${spin:frame:1}
      for s in "${svcs[@]}"; do
        if [ -f "$tmp/$s.rc" ]; then
          done_n=$((done_n+1))
          if [ "$(cat "$tmp/$s.rc")" = 0 ]; then printf '\033[K  %s✓%s %-10s deployed\n' "$OK" "$OFF" "$s"
          else printf '\033[K  %s✗%s %-10s failed\n' "$ERR" "$OFF" "$s"; fi
        else
          printf '\033[K  %s%s%s %-10s %sbuilding & rolling out…%s\n' "$RUN" "$sp" "$OFF" "$s" "$DIM" "$OFF"
        fi
      done
      [ "$done_n" = "$total" ] && break
      frame=$(((frame+1)%10)); sleep 0.15
    done
    wait

    fails=0
    for s in "${svcs[@]}"; do
      [ "$(cat "$tmp/$s.rc" 2>/dev/null || echo 1)" = 0 ] && continue
      fails=$((fails+1))
      ui_section "$s — build/deploy log (tail)"
      tail -n 25 "$tmp/$s.log" 2>/dev/null
    done
    if [ "$fails" = 0 ]; then ui_ok "updated: ${svcs[*]}"; else ui_fail "$fails of $total failed — logs above"; exit 1; fi

# The enforcement gate: web (prettier, eslint, tsc) + api (build, vet, test),
# each in a clean pinned container, in parallel. Same pipeline runs in CI.
# Logic lives in Dagger (.dagger/), not shell — see docs/standards/ci.adoc.
check:
    @bash -c '. scripts/ui.sh && ui_header "Checks · Dagger"'
    cd .. && dagger call check

# Auto-fix web formatting (Prettier). Fast and local; the same prettier the
# `check` gate verifies. (Containerized equivalent: dagger call format export --path=web)
format:
    @bash -c '. scripts/ui.sh && ui_header "Format · web"'
    pnpm --dir ../web run format
    @bash -c '. scripts/ui.sh && ui_ok "formatted web/"'

# Regenerate typed query code from SQL (sqlc). Runs in Docker — no local sqlc.
# Both Go modules own queries; edit */internal/database/queries/*.sql, run this,
# commit the result.
gen:
    @bash -c '. scripts/ui.sh && ui_header "Generate · sqlc"'
    docker run --rm -v "{{justfile_directory()}}/api":/src -w /src sqlc/sqlc generate
    docker run --rm -v "{{justfile_directory()}}/integrations":/src -w /src sqlc/sqlc generate
    @bash -c '. scripts/ui.sh && ui_ok "generated api + integrations database/gen"'

# Regenerate the Connect contract (Go + TS) from proto/. Runs buf in Docker — no
# local buf/protoc. Edit proto/**/*.proto, run this, commit api/internal/gen +
# web/lib/gen. Deliberately does NOT run `buf dep update` (that's `just proto-deps`)
# so this regen is byte-identical to what the CI drift gate enforces from the
# committed buf.lock (design-doc 0006).
proto:
    @bash -c '. scripts/ui.sh && ui_header "Generate · buf"'
    docker run --rm -v "{{justfile_directory()}}":/work -w /work bufbuild/buf:1.70.0 lint
    docker run --rm -v "{{justfile_directory()}}":/work -w /work bufbuild/buf:1.70.0 generate --template buf.gen.go.yaml
    docker run --rm -v "{{justfile_directory()}}":/work -w /work bufbuild/buf:1.70.0 generate --template buf.gen.integrations.yaml --path proto/hivebook/integration
    docker run --rm -v "{{justfile_directory()}}":/work -w /work bufbuild/buf:1.70.0 generate --template buf.gen.es.yaml --include-imports --exclude-path proto/hivebook/integration/v1/integration_internal.proto
    @bash -c '. scripts/ui.sh && ui_ok "generated api/internal/gen + integrations/internal/gen + web/lib/gen"'

# Update proto dependencies (rewrites buf.lock). Separate from `just proto` so the
# regen path stays reproducible; bump deliberately, then re-run `just proto` and
# keep the protovalidate pin in api/go.mod in step (see buf.yaml).
proto-deps:
    @bash -c '. scripts/ui.sh && ui_header "Update · buf deps"'
    docker run --rm -v "{{justfile_directory()}}":/work -w /work bufbuild/buf:1.70.0 dep update
    @bash -c '. scripts/ui.sh && ui_ok "updated buf.lock — now run just proto"'

# First-time setup: trust a local CA, write local secrets, deploy the whole stack.
# Idempotent — safe to re-run to reconcile a half-broken cluster. After this,
# `just start` / `just stop` are the day-to-day controls.
setup:
    #!/usr/bin/env bash
    set -euo pipefail
    . scripts/ui.sh
    ui_header "Setup · Hivebook"
    if ! kubectl get ns >/dev/null 2>&1; then
      ui_fail "Kubernetes isn't reachable — start OrbStack and enable Kubernetes first."
      exit 1
    fi
    # 1. Local CA — cert-manager mints the *.hivebook.localhost certs from it and
    #    your browser trusts them. Idempotent; prompts for your password only the
    #    first time, when it adds the CA to the system trust store.
    ui_info "ensuring the local CA is installed (mkcert)…"
    mkcert -install || { ui_fail "mkcert -install failed"; exit 1; }
    # 2. Local secrets — dev defaults, gitignored, written only if absent.
    if [ -f .env ]; then
      ui_ok "infra/.env present"
    else
      ui_info "writing infra/.env with local dev defaults…"
      printf '%s\n' \
        "# Local secret values for the Hivebook dev stack. Gitignored — never committed." \
        "# Match the creds shown in 'just urls'. Edit to override." \
        "HIVEBOOK_DB_PASSWORD=password1234" \
        "ZITADEL_DB_PASSWORD=zitadel-local-pw" \
        "ZITADEL_ADMIN_PASSWORD=Password1!" \
        "GRAFANA_ADMIN_PASSWORD=password1234" > .env
    fi
    # 3. Deploy everything. A fresh `just` picks up the new .env and CA path.
    just install

# ========================================================================
# Hidden helpers + install/deploy recipes (no git backup, so kept here).
# Runnable (e.g. `just install`, `just api`) but not shown in `just`.
# We'll surface these as real commands if/when we need them.
# ========================================================================

# Build one service's image, apply its manifests, and roll it out (waiting for
# readiness). Single source of truth for per-service deploy — used by `update`
# and by the install/wrapper recipes below.
[private]
_build service:
    #!/usr/bin/env bash
    set -euo pipefail
    kubectl get ns hivebook >/dev/null 2>&1 || kubectl create ns hivebook
    case "{{service}}" in
      api)
        docker build -t hivebook-api:dev ../api
        kubectl -n hivebook create secret generic hivebook-db \
          --from-literal=POSTGRES_DB=hivebook --from-literal=POSTGRES_USER=hivebook \
          --from-literal=POSTGRES_PASSWORD="$HIVEBOOK_DB_PASSWORD" \
          --dry-run=client -o yaml | kubectl apply -f -
        kubectl apply -f postgres/
        # ZITADEL management token (Members surface), patched into the provisioned
        # hivebook-oidc secret so we don't clobber its other keys. Default to the
        # iam-admin PAT the FirstInstance bootstrap creates (IAM_OWNER — reads any
        # org's members); override with HIVEBOOK_ZITADEL_MGMT_TOKEN in infra/.env to
        # use a dedicated least-privilege machine user (recommended for prod).
        MGMT_TOKEN="${HIVEBOOK_ZITADEL_MGMT_TOKEN:-$(kubectl -n zitadel get secret iam-admin-pat -o jsonpath='{.data.pat}' 2>/dev/null | base64 -d)}"
        [ -n "$MGMT_TOKEN" ] && \
          kubectl -n hivebook patch secret hivebook-oidc -p "{\"stringData\":{\"MGMT_TOKEN\":\"$MGMT_TOKEN\"}}" || true
        kubectl apply -f api/
        kubectl -n hivebook rollout restart deployment/api
        kubectl -n hivebook rollout status deployment/api --timeout=180s
        ;;
      web)
        docker build -t hivebook-web:dev ../web \
          --build-arg NEXT_PUBLIC_OIDC_ISSUER=https://id.hivebook.localhost \
          --build-arg NEXT_PUBLIC_API_BASE=https://api.hivebook.localhost \
          --build-arg NEXT_PUBLIC_OIDC_CLIENT_ID="$(kubectl -n hivebook get secret hivebook-oidc -o jsonpath='{.data.WEB_CLIENT_ID}' | base64 -d)" \
          --build-arg NEXT_PUBLIC_OIDC_PROJECT_ID="$(kubectl -n hivebook get secret hivebook-oidc -o jsonpath='{.data.PROJECT_ID}' | base64 -d)"
        kubectl apply -f web/
        kubectl -n hivebook rollout restart deployment/web
        kubectl -n hivebook rollout status deployment/web --timeout=180s
        ;;
      prototype)
        docker build -t hivebook-prototype:dev ../prototype
        kubectl apply -f prototype/
        kubectl -n hivebook rollout restart deployment/prototype
        kubectl -n hivebook rollout status deployment/prototype --timeout=180s
        ;;
      docs)
        docker build -t hivebook-docs:dev -f ../docs/Dockerfile ..
        kubectl apply -f docs/
        kubectl -n hivebook rollout restart deployment/docs
        kubectl -n hivebook rollout status deployment/docs --timeout=180s
        ;;
      integrations)
        docker build -t hivebook-integrations:dev ../integrations
        # Object-storage credentials, shared by RustFS and the integrations pod.
        kubectl -n hivebook create secret generic rustfs-creds \
          --from-literal=ACCESS_KEY="${HIVEBOOK_S3_ACCESS_KEY:-rustfsadmin}" \
          --from-literal=SECRET_KEY="${HIVEBOOK_S3_SECRET_KEY:-rustfsadmin}" \
          --dry-run=client -o yaml | kubectl apply -f -
        # Token/state keys: create once, never rotate (rotating would orphan every
        # stored OAuth token). The connector OAuth creds, by contrast, are synced
        # from infra/.env on every deploy so you can change apps without recreating
        # the whole secret (registering/replacing an OAuth or GitHub App).
        kubectl -n hivebook get secret integrations-secrets >/dev/null 2>&1 || \
          kubectl -n hivebook create secret generic integrations-secrets \
            --from-literal=TOKEN_KEY="$(openssl rand -base64 32)" \
            --from-literal=STATE_KEY="$(openssl rand -base64 32)"
        kubectl -n hivebook patch secret integrations-secrets -p "{\"stringData\":{\"GDOCS_CLIENT_ID\":\"${HIVEBOOK_GDOCS_CLIENT_ID:-}\",\"GDOCS_CLIENT_SECRET\":\"${HIVEBOOK_GDOCS_CLIENT_SECRET:-}\",\"GITHUB_CLIENT_ID\":\"${HIVEBOOK_GITHUB_CLIENT_ID:-}\",\"GITHUB_CLIENT_SECRET\":\"${HIVEBOOK_GITHUB_CLIENT_SECRET:-}\",\"GITHUB_APP_SLUG\":\"${HIVEBOOK_GITHUB_APP_SLUG:-}\"}}"
        # GitHub App private key (PEM) for installation tokens — kept as a gitignored
        # file (multi-line, awkward in .env). Synced into the secret when present.
        [ -f github-app-private-key.pem ] && kubectl -n hivebook patch secret integrations-secrets --type merge -p "$(python3 -c "import json;print(json.dumps({'stringData':{'GITHUB_PRIVATE_KEY':open('github-app-private-key.pem').read()}}))")" || true
        kubectl apply -f rustfs/
        # The integrations service owns its own database on the shared instance.
        kubectl -n hivebook exec statefulset/postgres -- \
          psql -U hivebook -d hivebook -tc "SELECT 1 FROM pg_database WHERE datname='integrations'" | grep -q 1 || \
          kubectl -n hivebook exec statefulset/postgres -- psql -U hivebook -d hivebook -c "CREATE DATABASE integrations"
        # Server + worker + scrape configs (non-recursive: skips integrations/keda/).
        kubectl apply -f integrations/
        # Worker autoscaling. The ScaledObject needs the KEDA operator (just infra);
        # without it the worker just runs at its Deployment replica count.
        if kubectl get crd scaledobjects.keda.sh >/dev/null 2>&1; then
          kubectl apply -f integrations/keda/
        else
          echo "KEDA operator not found — worker runs at fixed replicas (run 'just infra' for scale-to-zero)"
        fi
        kubectl -n hivebook rollout restart deployment/integrations deployment/integrations-worker
        kubectl -n hivebook rollout status deployment/integrations --timeout=180s
        kubectl -n hivebook rollout status deployment/integrations-worker --timeout=180s
        ;;
      *) echo "unknown buildable service: {{service}}" >&2; exit 1 ;;
    esac

[private]
_ns service:
    #!/usr/bin/env bash
    n=$(kubectl get deploy,sts -A -o jsonpath="{range .items[?(@.metadata.name=='{{service}}')]}{.metadata.namespace}{'\n'}{end}" 2>/dev/null | head -1)
    [ -z "$n" ] && exit 1
    echo "$n"

[private]
install: infra coredns auth provision api integrations web docs observability
    #!/usr/bin/env bash
    . scripts/ui.sh
    ui_logo
    ui_header "Hivebook installed"
    ui_box \
      "Web app   https://app.hivebook.localhost" \
      "Docs      https://docs.hivebook.localhost" \
      "Auth      https://id.hivebook.localhost      admin@hivebook.localhost / Password1!" \
      "API       https://api.hivebook.localhost" \
      "Grafana   https://grafana.hivebook.localhost   admin / password1234"
    ui_info "ZITADEL provisioned automatically — see docs/operations/auth.adoc"

# Idempotent ZITADEL provisioning: project + web/api apps + hivebook-oidc secret.
[private]
provision:
    @bash -c '. scripts/ui.sh && ui_header "Provisioning ZITADEL (OIDC project + apps)"'
    bash scripts/provision-zitadel.sh
    @bash -c '. scripts/ui.sh && ui_ok "OIDC provisioned"'

[private]
destroy:
    #!/usr/bin/env bash
    set -uo pipefail
    . scripts/ui.sh
    ui_header "Destroying Hivebook"
    helm -n observability uninstall vector vm victoria-logs kube-prometheus-stack 2>/dev/null || true
    helm -n zitadel uninstall zitadel 2>/dev/null || true
    helm -n traefik uninstall traefik 2>/dev/null || true
    helm -n cert-manager uninstall cert-manager 2>/dev/null || true
    kubectl -n kube-system delete configmap coredns-custom 2>/dev/null || true
    kubectl -n kube-system rollout restart deployment/coredns 2>/dev/null || true
    kubectl delete -f namespaces.yaml 2>/dev/null || true
    kubectl get crd -o name 2>/dev/null | grep -E 'cert-manager\.io|traefik\.io|victoriametrics\.com|monitoring\.coreos\.com' | xargs -r kubectl delete 2>/dev/null || true
    kubectl delete mutatingwebhookconfiguration,validatingwebhookconfiguration -l app.kubernetes.io/instance=cert-manager 2>/dev/null || true
    ui_ok "destroyed (namespaces, charts, CRDs, webhooks removed)"

[private]
repos:
    @bash -c '. scripts/ui.sh && ui_info "updating helm repos…"'
    helm repo add jetstack https://charts.jetstack.io
    helm repo add traefik https://traefik.github.io/charts
    helm repo add zitadel https://charts.zitadel.com
    helm repo add victoriametrics https://victoriametrics.github.io/helm-charts/
    helm repo add vector https://helm.vector.dev
    helm repo add kedacore https://kedacore.github.io/charts
    helm repo update

[private]
infra: repos
    @bash -c '. scripts/ui.sh && ui_header "Infra · cert-manager + Traefik + wildcard cert"'
    kubectl apply -f namespaces.yaml
    kubectl -n cert-manager create secret tls mkcert-root --cert="{{caroot}}/rootCA.pem" --key="{{caroot}}/rootCA-key.pem" --dry-run=client -o yaml | kubectl apply -f -
    helm upgrade --install cert-manager jetstack/cert-manager --namespace cert-manager --set crds.enabled=true --wait
    kubectl apply -f cert-manager/clusterissuer-mkcert.yaml
    helm upgrade --install traefik traefik/traefik --namespace traefik --values helm-values/traefik.yaml --wait
    kubectl apply -f cert-manager/certificate-wildcard.yaml
    # KEDA: scales the integrations sync worker 0→N off the Temporal task-queue
    # backlog (ADR-0030, design-doc 0012; infra/integrations/keda/scaledobject.yaml).
    helm upgrade --install keda kedacore/keda --namespace keda --create-namespace --wait
    @bash -c '. scripts/ui.sh && ui_ok "infra ready"'

[private]
coredns:
    @bash -c '. scripts/ui.sh && ui_header "CoreDNS · in-cluster issuer rewrite"'
    kubectl apply -f coredns/rewrite-patch.yaml
    kubectl -n kube-system rollout restart deployment/coredns
    kubectl -n kube-system rollout status deployment/coredns
    @bash -c '. scripts/ui.sh && ui_ok "coredns patched"'

[private]
auth: repos
    #!/usr/bin/env bash
    set -euo pipefail
    . scripts/ui.sh
    ui_header "Auth · ZITADEL (Postgres + helm + ingress)"
    kubectl get ns zitadel >/dev/null 2>&1 || kubectl create ns zitadel
    # DB credentials from .env (not committed)
    kubectl -n zitadel create secret generic zitadel-db \
      --from-literal=POSTGRES_USER=zitadel --from-literal=POSTGRES_DB=postgres \
      --from-literal=POSTGRES_PASSWORD="$ZITADEL_DB_PASSWORD" \
      --dry-run=client -o yaml | kubectl apply -f -
    kubectl apply -f zitadel/postgres.yaml
    kubectl -n zitadel rollout status statefulset/zitadel-db --timeout 180s
    kubectl -n zitadel get secret zitadel-masterkey >/dev/null 2>&1 || kubectl -n zitadel create secret generic zitadel-masterkey --from-literal=masterkey="$(LC_ALL=C tr -dc A-Za-z0-9 </dev/urandom | head -c 32)"
    helm upgrade --install zitadel zitadel/zitadel --namespace zitadel --values helm-values/zitadel.yaml --wait --timeout 10m \
      --set zitadel.configmapConfig.FirstInstance.Org.Human.Password="$ZITADEL_ADMIN_PASSWORD" \
      --set zitadel.configmapConfig.Database.Postgres.User.Password="$ZITADEL_DB_PASSWORD" \
      --set zitadel.configmapConfig.Database.Postgres.Admin.Password="$ZITADEL_DB_PASSWORD"
    kubectl apply -f zitadel/ingressroute.yaml
    ui_ok "ZITADEL up on https://id.hivebook.localhost"

# Install-time wrappers: build + deploy one service with a gum header/footer.
# All real build logic lives in `_build` (the single source of truth, shared with
# `update`). Day-to-day, prefer `just update <svc...>`.
[private]
api:
    @bash -c '. scripts/ui.sh && ui_header "Deploy · API (Go)"'
    just _build api
    @bash -c '. scripts/ui.sh && ui_ok "API deployed on https://api.hivebook.localhost"'

[private]
web:
    @bash -c '. scripts/ui.sh && ui_header "Deploy · web (Next.js)"'
    just _build web
    @bash -c '. scripts/ui.sh && ui_ok "web deployed on https://app.hivebook.localhost"'

[private]
prototype:
    @bash -c '. scripts/ui.sh && ui_header "Deploy · prototype (Next.js)"'
    just _build prototype
    @bash -c '. scripts/ui.sh && ui_ok "prototype deployed on https://prototype.hivebook.localhost"'

[private]
docs:
    @bash -c '. scripts/ui.sh && ui_header "Deploy · docs (Antora)"'
    just _build docs
    @bash -c '. scripts/ui.sh && ui_ok "docs deployed on https://docs.hivebook.localhost"'

[private]
integrations: temporal
    @bash -c '. scripts/ui.sh && ui_header "Deploy · integrations (Go)"'
    just _build integrations
    @bash -c '. scripts/ui.sh && ui_ok "integrations deployed (internal ClusterIP)"'

# Deploy the Temporal dev server (sync orchestration backbone, design-doc 0012)
temporal:
    #!/usr/bin/env bash
    set -euo pipefail
    . scripts/ui.sh
    ui_header "Deploy · Temporal (dev server)"
    kubectl get ns hivebook >/dev/null 2>&1 || kubectl create ns hivebook
    kubectl apply -f temporal/
    kubectl -n hivebook rollout status deployment/temporal --timeout=180s
    ui_ok "Temporal up — frontend temporal-frontend:7233, UI via 'just temporal-ui'"

# Port-forward the Temporal Web UI to http://localhost:8233
temporal-ui:
    @bash -c '. scripts/ui.sh && ui_info "Temporal Web UI → http://localhost:8233 (Ctrl-C to stop)"'
    kubectl -n hivebook port-forward svc/temporal-ui 8233:8233

# Run the Temporal sync worker locally (set HIVEBOOK_TEMPORAL_HOSTPORT, default localhost:7233)
worker-run:
    #!/usr/bin/env bash
    set -euo pipefail
    . scripts/ui.sh
    ui_header "Run · Temporal sync worker (local)"
    : "${HIVEBOOK_TEMPORAL_HOSTPORT:=localhost:7233}"
    ui_info "dialing Temporal at $HIVEBOOK_TEMPORAL_HOSTPORT (override HIVEBOOK_TEMPORAL_HOSTPORT)"
    cd ../integrations && HIVEBOOK_TEMPORAL_HOSTPORT="$HIVEBOOK_TEMPORAL_HOSTPORT" go run ./cmd/worker

[private]
observability: repos
    #!/usr/bin/env bash
    set -euo pipefail
    . scripts/ui.sh
    ui_header "Observability · VictoriaMetrics + VictoriaLogs + Grafana"
    helm -n observability uninstall kube-prometheus-stack 2>/dev/null || true
    helm upgrade --install victoria-logs victoriametrics/victoria-logs-single --namespace observability --values helm-values/victoria-logs.yaml --wait
    helm upgrade --install vm victoriametrics/victoria-metrics-k8s-stack --namespace observability --values helm-values/victoria-metrics-k8s-stack.yaml --wait --timeout 15m --set grafana.adminPassword="$GRAFANA_ADMIN_PASSWORD"
    helm upgrade --install vector vector/vector --namespace observability --values helm-values/vector.yaml --wait
    # VictoriaLogs datasource (API-created so it doesn't race the plugin load; persists on the PVC)
    CA="$(mkcert -CAROOT)/rootCA.pem"; G="https://grafana.hivebook.localhost"
    i=0; until curl -sf --cacert "$CA" -u "admin:$GRAFANA_ADMIN_PASSWORD" -o /dev/null "$G/api/health" 2>/dev/null; do i=$((i+1)); [ $i -gt 60 ] && break; sleep 3; done
    curl -sf --cacert "$CA" -u "admin:$GRAFANA_ADMIN_PASSWORD" -X POST "$G/api/datasources" -H 'Content-Type: application/json' \
      -d '{"uid":"VictoriaLogs","name":"VictoriaLogs","type":"victoriametrics-logs-datasource","access":"proxy","url":"http://victorialogs.observability.svc.cluster.local:9428"}' >/dev/null 2>&1 || true
    kubectl apply -f observability/vmservicescrape-api.yaml -f observability/vmservicescrape-zitadel.yaml
    kubectl apply -f observability/grafana-ingressroute.yaml
    for d in observability/dashboards/*/; do
      folder=$(basename "$d")
      for f in "$d"*.json; do
        name=$(echo "dash-$folder-$(basename "$f" .json)" | tr 'A-Z' 'a-z')
        kubectl -n observability create configmap "$name" --from-file="$f" --dry-run=client -o yaml | kubectl apply -f -
        kubectl -n observability label configmap "$name" grafana_dashboard=1 --overwrite
        kubectl -n observability annotate configmap "$name" grafana_folder="$folder" --overwrite
      done
    done
    ui_ok "observability up — Grafana on https://grafana.hivebook.localhost"

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
    @echo ""
    @echo "  Hivebook — local dev (run from anywhere in the repo)"
    @echo ""
    @echo "  COMMANDS"
    @echo "    just start  [service]   turn ON  — everything, or one service"
    @echo "    just stop   [service]   turn OFF — everything, or one service"
    @echo "    just logs   [service]   logs     — follow one service, or recent from all"
    @echo "    just status [service]   what's running"
    @echo ""
    @echo "    services: api · web · postgres · zitadel · zitadel-db · zitadel-login"
    @echo "    (Kubernetes/OrbStack must be running — you manage that yourself.)"
    @echo ""
    @echo "  URLs & LOGINS  (HTTPS via mkcert · local creds, never reuse)"
    @echo "    Web app   https://app.hivebook.localhost       — sign in via Auth below"
    @echo "    Auth      https://id.hivebook.localhost        admin@hivebook.localhost / Password1!"
    @echo "    Grafana   https://grafana.hivebook.localhost   admin / admin"
    @echo "    API       https://api.hivebook.localhost       /health · /metrics · /api/me"
    @echo ""
    @echo "    App DB       hivebook / password1234            (postgres, hivebook ns)"
    @echo "    ZITADEL DB   zitadel / zitadel-local-pw         (postgres, zitadel ns)"
    @echo "    IAM PAT      kubectl -n zitadel get secret iam-admin-pat -o jsonpath='{.data.pat}' | base64 -d"
    @echo ""

# Turn ON — the whole app (default) or one service
start service="all":
    #!/usr/bin/env bash
    set -uo pipefail
    kubectl get ns >/dev/null 2>&1 || { echo ">> Kubernetes isn't reachable — start OrbStack / enable Kubernetes first."; exit 1; }
    if [ "{{service}}" = all ]; then
      for n in {{ns}}; do
        kubectl -n "$n" scale deploy,statefulset --all --replicas=1 >/dev/null 2>&1 || true
        for ds in $(kubectl -n "$n" get ds -o name 2>/dev/null); do
          kubectl -n "$n" patch "$ds" --type merge -p '{"spec":{"template":{"spec":{"nodeSelector":{"hivebook.io/stopped":null}}}}}' >/dev/null 2>&1 || true
        done
      done
      echo ">> ON. 'just status' to watch."
    else
      n=$(just _ns {{service}}) || { echo "unknown service '{{service}}'"; exit 1; }
      kubectl -n "$n" scale deploy/{{service}} --replicas=1 2>/dev/null || kubectl -n "$n" scale statefulset/{{service}} --replicas=1
      echo ">> started {{service}}."
    fi

# Turn OFF — the whole app (default) or one service. Only our workloads; OrbStack
# itself is left running for you to manage.
stop service="all":
    #!/usr/bin/env bash
    set -uo pipefail
    kubectl get ns >/dev/null 2>&1 || { echo ">> Kubernetes isn't reachable (nothing to stop)."; exit 0; }
    if [ "{{service}}" = all ]; then
      # Two passes: the first stops everything (incl. operators); after operators
      # are down the second re-scales anything they revived so it stays at zero.
      for pass in 1 2; do
        for n in {{ns}}; do
          kubectl -n "$n" scale deploy,statefulset --all --replicas=0 >/dev/null 2>&1 || true
          for ds in $(kubectl -n "$n" get ds -o name 2>/dev/null); do
            kubectl -n "$n" patch "$ds" --type merge -p '{"spec":{"template":{"spec":{"nodeSelector":{"hivebook.io/stopped":"true"}}}}}' >/dev/null 2>&1 || true
          done
        done
        [ "$pass" = 1 ] && sleep 5
      done
      echo ">> OFF (all workloads scaled to zero; data preserved). OrbStack left running."
    else
      n=$(just _ns {{service}}) || { echo "unknown service '{{service}}'"; exit 1; }
      kubectl -n "$n" scale deploy/{{service}} --replicas=0 2>/dev/null || kubectl -n "$n" scale statefulset/{{service}} --replicas=0
      echo ">> stopped {{service}}."
    fi

# Logs — follow one service, or recent lines from everything
logs service="all":
    #!/usr/bin/env bash
    set -uo pipefail
    if [ "{{service}}" = all ]; then
      for n in {{ns}}; do
        for d in $(kubectl -n "$n" get deploy,statefulset -o name 2>/dev/null); do
          echo "==== $n/$d ===="; kubectl -n "$n" logs "$d" --tail=20 2>/dev/null || true
        done
      done
    else
      n=$(just _ns {{service}}) || { echo "unknown service '{{service}}'"; exit 1; }
      kubectl -n "$n" logs deploy/{{service}} --tail=200 -f 2>/dev/null || kubectl -n "$n" logs statefulset/{{service}} --tail=200 -f
    fi

# What's running — all, or one service
status service="all":
    #!/usr/bin/env bash
    f="^(traefik|cert-manager|zitadel|hivebook|observability) "
    [ "{{service}}" != all ] && f="{{service}}"
    kubectl get pods -A 2>/dev/null | grep -E "NAMESPACE|$f" || echo ">> cluster unreachable — run 'just start'."

# ========================================================================
# Hidden helpers + install/deploy recipes (no git backup, so kept here).
# Runnable (e.g. `just install`, `just api`) but not shown in `just`.
# We'll surface these as real commands if/when we need them.
# ========================================================================

[private]
_ns service:
    #!/usr/bin/env bash
    n=$(kubectl get deploy,sts -A -o jsonpath="{range .items[?(@.metadata.name=='{{service}}')]}{.metadata.namespace}{'\n'}{end}" 2>/dev/null | head -1)
    [ -z "$n" ] && exit 1
    echo "$n"

[private]
install: infra coredns auth provision api web observability
    @echo ">> installed. (ZITADEL provisioned automatically — see docs/auth-setup.adoc)"

# Idempotent ZITADEL provisioning: project + web/api apps + hivebook-oidc secret.
[private]
provision:
    bash scripts/provision-zitadel.sh

[private]
destroy:
    #!/usr/bin/env bash
    set -uo pipefail
    helm -n observability uninstall vector vm victoria-logs kube-prometheus-stack 2>/dev/null || true
    helm -n zitadel uninstall zitadel 2>/dev/null || true
    helm -n traefik uninstall traefik 2>/dev/null || true
    helm -n cert-manager uninstall cert-manager 2>/dev/null || true
    kubectl -n kube-system delete configmap coredns-custom 2>/dev/null || true
    kubectl -n kube-system rollout restart deployment/coredns 2>/dev/null || true
    kubectl delete -f namespaces.yaml 2>/dev/null || true
    kubectl get crd -o name 2>/dev/null | grep -E 'cert-manager\.io|traefik\.io|victoriametrics\.com|monitoring\.coreos\.com' | xargs -r kubectl delete 2>/dev/null || true
    kubectl delete mutatingwebhookconfiguration,validatingwebhookconfiguration -l app.kubernetes.io/instance=cert-manager 2>/dev/null || true
    echo ">> destroyed."

[private]
repos:
    helm repo add jetstack https://charts.jetstack.io
    helm repo add traefik https://traefik.github.io/charts
    helm repo add zitadel https://charts.zitadel.com
    helm repo add victoriametrics https://victoriametrics.github.io/helm-charts/
    helm repo add vector https://helm.vector.dev
    helm repo update

[private]
infra: repos
    kubectl apply -f namespaces.yaml
    kubectl -n cert-manager create secret tls mkcert-root --cert="{{caroot}}/rootCA.pem" --key="{{caroot}}/rootCA-key.pem" --dry-run=client -o yaml | kubectl apply -f -
    helm upgrade --install cert-manager jetstack/cert-manager --namespace cert-manager --set crds.enabled=true --wait
    kubectl apply -f cert-manager/clusterissuer-mkcert.yaml
    helm upgrade --install traefik traefik/traefik --namespace traefik --values helm-values/traefik.yaml --wait
    kubectl apply -f cert-manager/certificate-wildcard.yaml

[private]
coredns:
    kubectl apply -f coredns/rewrite-patch.yaml
    kubectl -n kube-system rollout restart deployment/coredns
    kubectl -n kube-system rollout status deployment/coredns

[private]
auth: repos
    #!/usr/bin/env bash
    set -uo pipefail
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

[private]
api:
    #!/usr/bin/env bash
    set -uo pipefail
    docker build -t hivebook-api:dev ../api
    kubectl get ns hivebook >/dev/null 2>&1 || kubectl create ns hivebook
    # App DB credentials from .env (not committed)
    kubectl -n hivebook create secret generic hivebook-db \
      --from-literal=POSTGRES_DB=hivebook --from-literal=POSTGRES_USER=hivebook \
      --from-literal=POSTGRES_PASSWORD="$HIVEBOOK_DB_PASSWORD" \
      --dry-run=client -o yaml | kubectl apply -f -
    kubectl apply -f postgres/
    kubectl apply -f api/
    kubectl -n hivebook rollout restart deployment/api

[private]
web:
    docker build -t hivebook-web:dev ../web --build-arg NEXT_PUBLIC_OIDC_ISSUER=https://id.hivebook.localhost --build-arg NEXT_PUBLIC_API_BASE=https://api.hivebook.localhost --build-arg NEXT_PUBLIC_OIDC_CLIENT_ID="$(kubectl -n hivebook get secret hivebook-oidc -o jsonpath='{.data.WEB_CLIENT_ID}' | base64 -d)" --build-arg NEXT_PUBLIC_OIDC_PROJECT_ID="$(kubectl -n hivebook get secret hivebook-oidc -o jsonpath='{.data.PROJECT_ID}' | base64 -d)"
    kubectl apply -f web/
    kubectl -n hivebook rollout restart deployment/web

[private]
observability: repos
    #!/usr/bin/env bash
    set -uo pipefail
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

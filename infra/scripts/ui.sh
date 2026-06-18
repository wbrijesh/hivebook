# Shared pretty-output helpers for justfile recipes.
# Uses `gum` (https://github.com/charmbracelet/gum) when available, and
# degrades to plain echo otherwise. Source from a recipe: `. scripts/ui.sh`

if command -v gum >/dev/null 2>&1; then UI_GUM=1; else UI_GUM=0; fi
UI_ACCENT="${UI_ACCENT:-208}"   # Apex orange
UI_DIR="$(cd "$(dirname "${BASH_SOURCE[0]:-scripts/ui.sh}")" 2>/dev/null && pwd)"

# The Hivebook wordmark, in the accent colour.
ui_logo() {
  local logo="$UI_DIR/logo.txt"
  [ -f "$logo" ] || return 0
  if [ "$UI_GUM" = 1 ]; then gum style --foreground "$UI_ACCENT" "$(cat "$logo")"; else cat "$logo"; fi
}

# A bordered, accented title block.
ui_header() {
  if [ "$UI_GUM" = 1 ]; then
    gum style --border rounded --margin "1 0" --padding "0 2" \
      --border-foreground "$UI_ACCENT" --foreground "$UI_ACCENT" "$1"
  else
    printf '\n=== %s ===\n' "$1"
  fi
}

# A bold accent section label (no border).
ui_section() {
  if [ "$UI_GUM" = 1 ]; then gum style --bold --foreground "$UI_ACCENT" "$1"; else printf '\n-- %s --\n' "$1"; fi
}

ui_ok()     { if [ "$UI_GUM" = 1 ]; then gum style --foreground 42  "  ✓ $1"; else echo "  [ok]   $1"; fi; }
ui_fail()   { if [ "$UI_GUM" = 1 ]; then gum style --foreground 196 "  ✗ $1"; else echo "  [fail] $1"; fi; }
ui_info()   { if [ "$UI_GUM" = 1 ]; then gum style --foreground 244 "  · $1"; else echo "  - $1"; fi; }
ui_subtle() { if [ "$UI_GUM" = 1 ]; then gum style --foreground 244 "$1"; else echo "$1"; fi; }

# ui_step "title" cmd args...  — spinner while the command runs; returns its exit code.
ui_step() {
  local title="$1"; shift
  if [ "$UI_GUM" = 1 ]; then
    gum spin --spinner dot --title " $title" -- "$@"
  else
    printf '  ... %s\n' "$title"; "$@" >/dev/null 2>&1
  fi
}

# ui_box line1 line2 ...  — a bordered box of lines.
ui_box() {
  if [ "$UI_GUM" = 1 ]; then
    gum style --border rounded --padding "1 2" --border-foreground 244 "$@"
  else
    printf '  %s\n' "$@"
  fi
}

# ui_progress ns1 ns2 ...  — live, all-at-once readiness of every deploy/statefulset
# in the given namespaces. Renders one multi-line block that updates in place: each
# row shows a spinner + ready/desired count until that workload is ready, then a ✓.
# Returns 0 when all are ready, 1 on timeout. (Portable bash; no associative arrays.)
ui_progress() {
  local namespaces="$*"
  local max_ticks=1200 poll_every=8 tick=0 frame=0 nframes=10
  local spin='⠋⠙⠹⠸⠼⠴⠦⠧⠇⠏'
  local OK=$'\033[38;5;42m' RUN=$'\033[38;5;208m' DIM=$'\033[38;5;244m' OFF=$'\033[0m'
  local items=() rr=() dd=() ns nm total i j it line r d sp allok first=1 data

  for ns in $namespaces; do
    while IFS= read -r nm; do
      [ -n "$nm" ] && items+=("$ns/${nm##*/}")
    done < <(kubectl -n "$ns" get deploy,statefulset -o name 2>/dev/null)
  done
  total=${#items[@]}
  [ "$total" -eq 0 ] && return 0

  while :; do
    if [ $((tick % poll_every)) -eq 0 ]; then
      data=""
      for ns in $namespaces; do
        while read -r nm r d; do
          [ "$r" = "<none>" ] && r=0
          data="$data$ns/$nm ${r:-0} ${d:-1}"$'\n'
        done < <(kubectl -n "$ns" get deploy,statefulset \
          -o custom-columns=N:.metadata.name,R:.status.readyReplicas,D:.spec.replicas --no-headers 2>/dev/null)
      done
      i=0
      for it in "${items[@]}"; do
        line=$(printf '%s' "$data" | grep -m1 -- "^$it ")
        set -- $line
        rr[$i]="${2:-0}"; dd[$i]="${3:-1}"
        i=$((i+1))
      done
    fi
    [ "$first" = 1 ] || printf '\033[%dA' "$total"
    first=0
    allok=1; j=0; sp=${spin:frame:1}
    for it in "${items[@]}"; do
      r=${rr[$j]:-0}; d=${dd[$j]:-1}
      # Ready when ready >= desired. A desired of 0 (e.g. a KEDA-scaled worker
      # idling at zero) is ready too — there's nothing to wait for.
      if [ "${r:-0}" -ge "${d:-0}" ] 2>/dev/null; then
        printf '\033[K  %s✓%s %s\n' "$OK" "$OFF" "$it"
      else
        printf '\033[K  %s%s%s %s %s(%s/%s)%s\n' "$RUN" "$sp" "$OFF" "$it" "$DIM" "$r" "$d" "$OFF"
        allok=0
      fi
      j=$((j+1))
    done
    [ "$allok" = 1 ] && return 0
    [ "$tick" -ge "$max_ticks" ] && return 1
    frame=$(((frame+1)%nframes)); tick=$((tick+1)); sleep 0.15
  done
}

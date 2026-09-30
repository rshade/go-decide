#!/usr/bin/env bash
set -euo pipefail

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
LIST="$ROOT/scripts/jev-clients.txt"
SAVE=false
[[ "${1:-}" == "--save" ]] && SAVE=true

row() {
  local repo="$1" meta tag proxy
  meta="$(gh api "repos/$repo" --jq '[.stargazers_count, .open_issues_count, (.license.spdx_id // "none"), .pushed_at[0:10], .archived] | @tsv' 2>/dev/null)" || {
    printf '%s\tERROR\t\t\t\t\t\t\n' "$repo"
    return
  }
  tag="$(gh api "repos/$repo/tags?per_page=1" --jq '.[0].name // "none"' 2>/dev/null || echo none)"
  proxy="$(curl -fsS "https://proxy.golang.org/$(echo "github.com/$repo" | sed -E 's/[A-Z]/!\L&/g')/@latest" 2>/dev/null | sed -n 's/.*"Version":"\([^"]*\)".*/\1/p' || true)"
  printf '%s\t%s\t%s\t%s\n' "$repo" "$meta" "$tag" "${proxy:-not-on-proxy}"
}

output="$(
  printf 'repo\tstars\topen_issues\tlicense\tlast_push\tarchived\tlatest_tag\tproxy_latest\n'
  while IFS= read -r repo; do
    [[ -z "$repo" || "$repo" == \#* ]] && continue
    row "$repo"
  done <"$LIST"
)"

if $SAVE; then
  out="$ROOT/docs/snapshots/$(date +%F).tsv"
  printf '%s\n' "$output" >"$out"
  echo "saved ${out#"$ROOT"/}" >&2
fi

printf '%s\n' "$output" | column -t -s "$(printf '\t')"

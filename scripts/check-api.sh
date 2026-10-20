#!/usr/bin/env bash
set -euo pipefail
cd "$(dirname "$0")/.."
export GOWORK=off
baseline_ref="${1:?usage: check-api.sh BASELINE_COMMIT_OR_TAG}"
baseline_sha=$(git rev-parse --verify --end-of-options "$baseline_ref^{commit}")
report_dir="${GONEX_API_REPORT_DIR:-.tools/api}"
mkdir -p "$report_dir"
report_dir=$(cd "$report_dir" && pwd)
task_tmp=$(mktemp -d)
trap 'git worktree remove --force "$task_tmp/baseline" >/dev/null 2>&1 || true; rm -rf "$task_tmp"' EXIT
git worktree add --detach "$task_tmp/baseline" "$baseline_sha"
(cd "$task_tmp/baseline" && apidiff -m -w "$report_dir/baseline.api" github.com/amkargar/gonex)
apidiff -m -w "$report_dir/current.api" github.com/amkargar/gonex
apidiff -m -incompatible "$report_dir/baseline.api" "$report_dir/current.api" > "$report_dir/incompatible.txt"
printf '%s\n' "$baseline_sha" > "$report_dir/baseline-commit.txt"
if [[ -s "$report_dir/incompatible.txt" ]]; then cat "$report_dir/incompatible.txt"; exit 1; fi

#!/usr/bin/env bash
set -euo pipefail
cd "$(dirname "$0")/.."
export GOWORK=off
report_dir="${GONEX_SECURITY_REPORT_DIR:-.tools/security}"
mkdir -p "$report_dir"
command -v govulncheck >/dev/null
command -v gitleaks >/dev/null
modules=(. contracts integration/rpc examples/microservices/services/identity examples/microservices/services/orders)
if [[ -f examples/production/go.mod ]]; then modules+=(examples/production); fi
failed=0
for module in "${modules[@]}"; do
  name="${module//\//-}"
  if ! (cd "$module" && govulncheck -json ./...) > "$report_dir/vuln-$name.json"; then failed=1; fi
  if ! go run ./internal/tooling/vulncheck "$report_dir/vuln-$name.json"; then failed=1; fi
done
# Only redacted findings may be uploaded; never publish a detected credential.
if ! gitleaks git --redact --report-format json --report-path "$report_dir/secrets.json"; then failed=1; fi
exit "$failed"

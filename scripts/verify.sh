#!/usr/bin/env bash
set -euo pipefail
cd "$(dirname "$0")/.."
export GOWORK=off GIN_MODE=release
if [[ -n "$(gofmt -l .)" ]]; then gofmt -l .; exit 1; fi
modules=(. contracts integration/rpc examples/microservices/services/identity examples/microservices/services/orders)
if [[ -f examples/production/go.mod ]]; then modules+=(examples/production); fi
for module in "${modules[@]}"; do
  go -C "$module" mod verify
  go -C "$module" test ./...
  go -C "$module" vet ./...
  go -C "$module" build ./...
done
go run ./cmd/gonex grpc check
go run ./cmd/gonex openapi validate --file tests/testdata/basic.openapi.json --against tests/testdata/releases/v0.1.0.openapi.json
if [[ "${GONEX_RACE:-0}" == 1 ]]; then go test -race ./...; fi

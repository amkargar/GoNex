#!/usr/bin/env bash
set -euo pipefail
cd "$(dirname "$0")/.."
export GOWORK=off GIN_MODE=release
if [[ -n "$(gofmt -l .)" ]]; then gofmt -l .; exit 1; fi
for module in . contracts integration/rpc examples/microservices/services/identity examples/microservices/services/orders; do
  go -C "$module" mod verify
  go -C "$module" test ./...
  go -C "$module" vet ./...
  go -C "$module" build ./...
done
go run ./cmd/gonex grpc check
go run ./cmd/gonex openapi validate --file tests/testdata/basic.openapi.json --against tests/testdata/releases/v0.1.0.openapi.json
if [[ "${GONEX_RACE:-0}" == 1 ]]; then go test -race ./...; fi

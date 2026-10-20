$ErrorActionPreference = 'Stop'
Set-Location (Join-Path $PSScriptRoot '..')
$env:GOWORK = 'off'
$env:GIN_MODE = 'release'
function Invoke-Go { & go @args; if ($LASTEXITCODE -ne 0) { throw "go $args failed ($LASTEXITCODE)" } }
$format = & gofmt -l .
if ($LASTEXITCODE -ne 0 -or $format) { throw "Run gofmt: $format" }
foreach ($module in @('.', 'contracts', 'integration/rpc', 'examples/microservices/services/identity', 'examples/microservices/services/orders')) {
    Invoke-Go -C $module mod verify
    Invoke-Go -C $module test ./...
    Invoke-Go -C $module vet ./...
    Invoke-Go -C $module build ./...
}
Invoke-Go run ./cmd/gonex grpc check
Invoke-Go run ./cmd/gonex openapi validate --file tests/testdata/basic.openapi.json --against tests/testdata/releases/v0.1.0.openapi.json
if ($env:GONEX_RACE -eq '1') { Invoke-Go test -race ./... }

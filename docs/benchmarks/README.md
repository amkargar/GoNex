# Paired benchmark methodology

[Project home](../../README.md) · [Documentation index](../README.md)

## Contents

- [Run paired benchmarks](#run-paired-benchmarks)
- [Workload boundaries](#workload-boundaries)
- [Recorded measurements](#recorded-measurements)
- [Regression budgets](#regression-budgets)

## Run paired benchmarks

Run from the repository root:

```sh
go test ./tests -run '^$' -bench BenchmarkComparison -benchmem -benchtime=200ms -count=3
go run ./internal/tooling/benchcheck benchmark.txt
```

## Workload boundaries

Save the first command's output as benchmark.txt for the checker. HTTP compares the same small JSON GET through bare Gin and a registered typed Gonex endpoint using httptest; registration/schema construction are outside the loop. Database compares direct GORM and the Gonex repository adapter reading an existing row from real file SQLite. It does not measure PostgreSQL, network ingress, TLS or production concurrency. Observability is disabled in the HTTP comparison.

## Recorded measurements

[Recorded output](windows-amd64.txt): Go 1.27.1, Windows/amd64, Intel Core i5-8350U at 1.70 GHz, four cores/eight logical processors, CGO disabled. Three 200ms samples produced these median results:

| Workload | Direct ns/op | Gonex ns/op | Ratio |
|---|---:|---:|---:|
| HTTP JSON GET | 1,554 | 2,001 | 1.288 |
| SQLite row read | 121,206 | 122,830 | 1.013 |

## Regression budgets

The checker compares medians from the same run/hardware and rejects HTTP ratios above 5x or database ratios above 2x. These deliberately coarse initial budgets catch major regressions while allowing runner noise; tighten them only with repeated comparable evidence. CI retains raw samples. Short local microbenchmarks are evidence for these workloads, not throughput or latency promises for applications.

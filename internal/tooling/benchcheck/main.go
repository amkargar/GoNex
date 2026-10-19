// Command benchcheck checks paired same-run overhead, not cross-machine timings.
package main

import (
	"fmt"
	"os"
	"regexp"
	"sort"
	"strconv"
)

func main() {
	if len(os.Args) != 2 {
		fmt.Fprintln(os.Stderr, "usage: benchcheck BENCHMARK_OUTPUT")
		os.Exit(1)
	}
	if err := check(os.Args[1]); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}
func check(path string) error {
	data, err := os.ReadFile(path)
	if err != nil {
		return err
	}
	pattern := regexp.MustCompile(`BenchmarkComparison(HTTP|Database)/(Gin|GORM|Gonex)-\d+\s+\d+\s+([\d.]+)\s+ns/op`)
	samples := map[string][]float64{}
	for _, match := range pattern.FindAllStringSubmatch(string(data), -1) {
		value, err := strconv.ParseFloat(match[3], 64)
		if err != nil {
			return err
		}
		key := match[1] + "/" + match[2]
		samples[key] = append(samples[key], value)
	}
	for _, comparison := range []struct {
		group, baseline string
		budget          float64
	}{{"HTTP", "Gin", 5}, {"Database", "GORM", 2}} {
		baseline := samples[comparison.group+"/"+comparison.baseline]
		framework := samples[comparison.group+"/Gonex"]
		if len(baseline) == 0 || len(framework) == 0 {
			return fmt.Errorf("missing measurements for %s", comparison.group)
		}
		base := median(baseline)
		if base <= 0 {
			return fmt.Errorf("invalid benchmark clock for %s", comparison.group)
		}
		ratio := median(framework) / base
		fmt.Printf("%s Gonex/%s = %.3f; budget <= %.1f\n", comparison.group, comparison.baseline, ratio, comparison.budget)
		if ratio > comparison.budget {
			return fmt.Errorf("investigate %s overhead regression before release", comparison.group)
		}
	}
	return nil
}
func median(values []float64) float64 {
	sort.Float64s(values)
	middle := len(values) / 2
	if len(values)%2 == 0 {
		return (values[middle-1] + values[middle]) / 2
	}
	return values[middle]
}

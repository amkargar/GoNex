// Command vulncheck fails closed on reachable findings in govulncheck JSON output.
package main

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
)

func main() {
	if len(os.Args) != 2 {
		fmt.Fprintln(os.Stderr, "usage: vulncheck REPORT.json")
		os.Exit(2)
	}
	file, err := os.Open(os.Args[1])
	if err == nil {
		defer file.Close()
		err = check(file)
	}
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}
func check(reader io.Reader) error {
	decoder := json.NewDecoder(io.LimitReader(reader, 64<<20))
	configured := false
	seen := map[string]bool{}
	for {
		var record struct {
			Config  json.RawMessage `json:"config"`
			Finding *struct {
				OSV   string `json:"osv"`
				Trace []struct {
					Function string `json:"function"`
				} `json:"trace"`
			} `json:"finding"`
		}
		err := decoder.Decode(&record)
		if err == io.EOF {
			break
		}
		if err != nil {
			return fmt.Errorf("invalid/incomplete scan report: %w", err)
		}
		if record.Config != nil {
			configured = true
		}
		if record.Finding != nil {
			for _, frame := range record.Finding.Trace {
				if frame.Function != "" {
					seen[record.Finding.OSV] = true
				}
			}
		}
	}
	if !configured {
		return fmt.Errorf("missing scan configuration: scan evidence required")
	}
	if len(seen) > 0 {
		return fmt.Errorf("release blocked: %d reachable vulnerabilities; inspect the JSON report", len(seen))
	}
	return nil
}

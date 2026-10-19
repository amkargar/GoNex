// Command bufinstall fetches and checksum-verifies an official pinned Buf executable.
package main

import (
	"crypto/sha256"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"time"
)

func main() {
	directory := ".tools"
	if len(os.Args) > 1 {
		directory = os.Args[1]
	}
	if err := install(directory); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}
func install(directory string) error {
	systems := map[string]string{"linux": "Linux", "windows": "Windows", "darwin": "Darwin"}
	arches := map[string]string{"amd64": "x86_64", "arm64": "aarch64"}
	if systems[runtime.GOOS] == "" || arches[runtime.GOARCH] == "" {
		return fmt.Errorf("unsupported Buf download platform")
	}
	name := "buf-" + systems[runtime.GOOS] + "-" + arches[runtime.GOARCH]
	local := "buf"
	if runtime.GOOS == "windows" {
		name += ".exe"
		local += ".exe"
	}
	const base = "https://github.com/bufbuild/buf/releases/download/v1.64.0/"
	client := http.Client{Timeout: 60 * time.Second}
	fetch := func(name string) ([]byte, error) {
		response, err := client.Get(base + name)
		if err != nil {
			return nil, err
		}
		defer response.Body.Close()
		if response.StatusCode != 200 {
			return nil, fmt.Errorf("Buf download failed: HTTP %d", response.StatusCode)
		}
		return io.ReadAll(io.LimitReader(response.Body, 128<<20))
	}
	checksums, err := fetch("sha256.txt")
	if err != nil {
		return err
	}
	expected := ""
	for _, line := range strings.Split(string(checksums), "\n") {
		fields := strings.Fields(line)
		if len(fields) == 2 && strings.TrimPrefix(fields[1], "*") == name {
			expected = fields[0]
		}
	}
	if len(expected) != 64 {
		return fmt.Errorf("executable missing from official checksums")
	}
	destination := filepath.Join(directory, local)
	data, err := os.ReadFile(destination)
	if err != nil || fmt.Sprintf("%x", sha256.Sum256(data)) != expected {
		data, err = fetch(name)
		if err != nil {
			return err
		}
	}
	if fmt.Sprintf("%x", sha256.Sum256(data)) != expected {
		return fmt.Errorf("Buf checksum mismatch; executable not written")
	}
	if err = os.MkdirAll(directory, 0755); err != nil {
		return err
	}
	if err = os.WriteFile(destination, data, 0755); err != nil {
		return err
	}
	absolute, err := filepath.Abs(destination)
	if err != nil {
		return err
	}
	fmt.Println(absolute)
	return nil
}

// Command consumer verifies a public, versioned installation outside the checkout.
package main

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"time"
)

func main() {
	if err := run(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}
func run() error {
	version := os.Getenv("GONEX_SMOKE_VERSION")
	if version == "" {
		return fmt.Errorf("GONEX_SMOKE_VERSION must identify a public commit or tag")
	}
	root, err := os.MkdirTemp("", "gonex-consumer-")
	if err != nil {
		return err
	}
	defer os.RemoveAll(root)
	ctx, cancel := context.WithTimeout(context.Background(), 4*time.Minute)
	defer cancel()
	bin := filepath.Join(root, "bin")
	if err = os.Mkdir(bin, 0700); err != nil {
		return err
	}
	env := append(os.Environ(), "GOWORK=off", "GOBIN="+bin, "GIN_MODE=release")
	command := func(dir, name string, args ...string) error {
		c := exec.CommandContext(ctx, name, args...)
		c.Dir = dir
		c.Env = env
		c.Stdout = os.Stdout
		c.Stderr = os.Stderr
		return c.Run()
	}
	if err = command(root, "go", "install", "github.com/amkargar/gonex/cmd/gonex@"+bootstrapVersion(version)); err != nil {
		return err
	}
	suffix := ""
	if runtime.GOOS == "windows" {
		suffix = ".exe"
	}
	cli := filepath.Join(bin, "gonex"+suffix)
	if err = command(root, cli, "--version"); err != nil {
		return err
	}
	if err = command(root, cli, "new", "shop", "--module", "example.com/shop"); err != nil {
		return err
	}
	app := filepath.Join(root, "shop")
	// Create a real baseline project, preserve a handwritten marker, then update
	// both the selected tool dependency and the generator's version metadata.
	for _, args := range [][]string{{"mod", "tidy"}, {"tool", "gonex", "create", "module", "users"}} {
		if err = command(app, "go", args...); err != nil {
			return err
		}
	}
	servicePath := filepath.Join(app, "modules", "users", "service.go")
	old, err := os.ReadFile(servicePath)
	if err != nil {
		return err
	}
	edited := append(old, []byte("\n// Consumer upgrade rehearsal marker.\n")...)
	if err = os.WriteFile(servicePath, edited, 0644); err != nil {
		return err
	}
	if err = command(app, "go", "get", "-tool", "github.com/amkargar/gonex/cmd/gonex@"+version); err != nil {
		return err
	}
	resolve := exec.CommandContext(ctx, "go", "list", "-m", "-f", "{{.Version}}", "github.com/amkargar/gonex")
	resolve.Dir = app
	resolve.Env = env
	resolved, err := resolve.Output()
	if err != nil {
		return err
	}
	if err = command(app, "go", "tool", "gonex", "upgrade", "--framework-version", strings.TrimSpace(string(resolved))); err != nil {
		return err
	}
	for _, args := range [][]string{{"mod", "tidy"}, {"tool", "gonex", "create", "module", "products"}, {"tool", "gonex", "check"}, {"tool", "gonex", "openapi", "generate"}, {"build", "-o", filepath.Join(root, "server"+suffix), "."}} {
		if err = command(app, "go", args...); err != nil {
			return err
		}
	}
	after, err := os.ReadFile(servicePath)
	if err != nil || string(after) != string(edited) {
		return fmt.Errorf("consumer handwritten source changed during upgrade")
	}

	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		return err
	}
	address := listener.Addr().String()
	listener.Close()
	// Bind collision during this local smoke is reported as a failure, never hidden by a retry.
	process := exec.CommandContext(ctx, filepath.Join(root, "server"+suffix))
	process.Dir = app
	process.Env = append(env, "HTTP_ADDRESS="+address)
	process.Stdout = os.Stdout
	process.Stderr = os.Stderr
	if err = process.Start(); err != nil {
		return err
	}
	defer func() { _ = process.Process.Kill(); _ = process.Wait() }()
	client := http.Client{Timeout: time.Second}
	deadline := time.Now().Add(15 * time.Second)
	for {
		res, e := client.Get("http://" + address + "/healthz")
		if e == nil {
			io.Copy(io.Discard, res.Body)
			res.Body.Close()
			if res.StatusCode != 200 {
				return fmt.Errorf("health status %d", res.StatusCode)
			}
			break
		}
		if time.Now().After(deadline) {
			return e
		}
		time.Sleep(50 * time.Millisecond)
	}
	for _, path := range []string{"/users", "/openapi.json"} {
		res, e := client.Get("http://" + address + path)
		if e != nil {
			return e
		}
		data, e := io.ReadAll(res.Body)
		res.Body.Close()
		if e != nil {
			return e
		}
		if res.StatusCode != 200 || !json.Valid(data) {
			return fmt.Errorf("invalid response at %s", path)
		}
	}
	fmt.Println("PASS: public install, project-local tool, generated API, health and OpenAPI; version", version)
	return nil
}

func bootstrapVersion(_ string) string {
	if baseline := os.Getenv("GONEX_SMOKE_BASELINE"); baseline != "" {
		return baseline
	}
	return "e835251029c429b599c10da1b06e71dae37fab05"
}

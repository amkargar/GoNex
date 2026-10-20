// Command clismoke builds a generated app and checks the actual executable.
package main

import (
	"bytes"
	"context"
	"debug/elf"
	"debug/pe"
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
	framework, err := os.Getwd()
	if err != nil {
		return err
	}
	root, err := os.MkdirTemp("", "gonex-cli-smoke-")
	if err != nil {
		return err
	}
	defer os.RemoveAll(root)
	ctx, cancel := context.WithTimeout(context.Background(), 12*time.Minute)
	defer cancel()
	targetOS := os.Getenv("SMOKE_GOOS")
	targetArch := os.Getenv("SMOKE_GOARCH")
	if targetOS == "" {
		targetOS = runtime.GOOS
	}
	if targetArch == "" {
		targetArch = runtime.GOARCH
	}
	env := append(os.Environ(), "GOWORK=off", "CGO_ENABLED=0", "GIN_MODE=release")
	command := func(dir, name string, args ...string) error {
		cmd := exec.CommandContext(ctx, name, args...)
		cmd.Dir, cmd.Env = dir, env
		cmd.Stdout, cmd.Stderr = os.Stdout, os.Stderr
		return cmd.Run()
	}
	suffix := ""
	if runtime.GOOS == "windows" {
		suffix = ".exe"
	}
	cli := filepath.Join(root, "gonex"+suffix)
	if err = command(framework, "go", "build", "-o", cli, "./cmd/gonex"); err != nil {
		return err
	}
	for _, args := range [][]string{{"-h"}, {"build", "--help"}, {"help", "create", "module"}} {
		if err = command(root, cli, args...); err != nil {
			return err
		}
	}
	if err = command(root, cli, "new", "shop", "--module", "example.com/shop", "--framework-version", "v0.0.0"); err != nil {
		return err
	}
	appDir := filepath.Join(root, "shop")
	for _, args := range [][]string{
		{"mod", "edit", "-replace=github.com/amkargar/gonex=" + filepath.ToSlash(framework)},
		{"mod", "tidy"},
		{"tool", "gonex", "create", "module", "users"},
		{"tool", "gonex", "create", "module", "messages", "--db"},
		{"tool", "gonex", "build", "--os", targetOS, "--arch", targetArch},
	} {
		if err = command(appDir, "go", args...); err != nil {
			return err
		}
	}
	appSuffix := ""
	if targetOS == "windows" {
		appSuffix = ".exe"
	}
	binary := filepath.Join(appDir, "dist", targetOS+"-"+targetArch, "shop"+appSuffix)
	if err = verifyBinary(binary, targetOS, targetArch); err != nil {
		return err
	}
	if targetOS != runtime.GOOS || targetArch != runtime.GOARCH {
		fmt.Printf("PASS: generated app executable verified for %s/%s (cross-compiled; not run)\n", targetOS, targetArch)
		return nil
	}
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		return err
	}
	address := listener.Addr().String()
	listener.Close()
	logFile, err := os.Create(filepath.Join(root, "server.log"))
	if err != nil {
		return err
	}
	defer logFile.Close()
	process := exec.CommandContext(ctx, binary)
	process.Dir = appDir
	process.Env = append(env, "HTTP_ADDRESS="+address, "APP_NAME=shop", "GONEX_LOG_FORMAT=pretty", "GONEX_LOG_COLOR=never", "DATABASE_DIALECT=sqlite", "DATABASE_URL=file:"+filepath.ToSlash(filepath.Join(root, "smoke.db")))
	process.Stdout, process.Stderr = logFile, logFile
	if err = process.Start(); err != nil {
		return err
	}
	stopped := false
	defer func() {
		if !stopped {
			_ = process.Process.Kill()
			_ = process.Wait()
		}
	}()
	client := http.Client{Timeout: time.Second}
	deadline := time.Now().Add(30 * time.Second)
	baseURL := "http://" + address
	for {
		res, requestErr := client.Get(baseURL + "/healthz")
		if requestErr == nil {
			io.Copy(io.Discard, res.Body)
			res.Body.Close()
			if res.StatusCode != 200 {
				return fmt.Errorf("unexpected health status %d", res.StatusCode)
			}
			break
		}
		if time.Now().After(deadline) {
			return fmt.Errorf("executable did not start: %w", requestErr)
		}
		time.Sleep(100 * time.Millisecond)
	}
	for _, path := range []string{"/users", "/openapi.json", "/readyz"} {
		res, err := client.Get(baseURL + path)
		if err != nil {
			return err
		}
		body, err := io.ReadAll(res.Body)
		res.Body.Close()
		if err != nil || res.StatusCode != 200 || len(body) == 0 {
			return fmt.Errorf("invalid executable response %s: %d %v", path, res.StatusCode, err)
		}
	}
	if runtime.GOOS != "windows" {
		if err = process.Process.Signal(os.Interrupt); err != nil {
			return err
		}
		if err = process.Wait(); err != nil {
			return fmt.Errorf("graceful shutdown: %w", err)
		}
	} else {
		// Windows does not support os.Interrupt through Process.Signal.
		_ = process.Process.Kill()
		_ = process.Wait()
	}
	stopped = true
	if err = logFile.Close(); err != nil {
		return err
	}
	logs, err := os.ReadFile(logFile.Name())
	if err != nil {
		return err
	}
	os.Stdout.Write(logs)
	for _, want := range []string{"GONEX", "Ready", baseURL, baseURL + "/openapi.json", "Listening", "Environment", "Version", "request_id="} {
		if !bytes.Contains(logs, []byte(want)) {
			return fmt.Errorf("server output missing %q", want)
		}
	}
	if bytes.Contains(logs, []byte("\x1b")) {
		return fmt.Errorf("redirected logs contain ANSI escape sequences")
	}
	fmt.Printf("PASS: %s/%s executable, health, typed route, OpenAPI, readiness and server output\n", targetOS, targetArch)
	return nil
}

func verifyBinary(path, targetOS, targetArch string) error {
	switch targetOS {
	case "windows":
		file, err := pe.Open(path)
		if err != nil {
			return err
		}
		defer file.Close()
		want := map[string]uint16{"amd64": pe.IMAGE_FILE_MACHINE_AMD64, "arm64": pe.IMAGE_FILE_MACHINE_ARM64}[targetArch]
		if want == 0 || file.Machine != want {
			return fmt.Errorf("incorrect PE architecture: %x", file.Machine)
		}
	case "linux":
		file, err := elf.Open(path)
		if err != nil {
			return err
		}
		defer file.Close()
		want := map[string]elf.Machine{"amd64": elf.EM_X86_64, "arm64": elf.EM_AARCH64}[targetArch]
		if want == 0 || file.Machine != want {
			return fmt.Errorf("incorrect ELF architecture: %s", file.Machine)
		}
	default:
		return fmt.Errorf("smoke target unsupported: %s", strings.Join([]string{targetOS, targetArch}, "/"))
	}
	return nil
}

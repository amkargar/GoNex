// Package devrunner supervises only explicitly selected local development processes.
package devrunner

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
	"time"
)

type Command struct {
	Name, Dir, HTTP, GRPC string
	Args, Env             []string
}
type Options struct {
	Commands []Command
	Output   io.Writer
	Grace    time.Duration
}
type running struct {
	owner *processOwner
	done  chan struct{}
	err   error
	name  string
}

// Supervise starts direct executables, never go run wrappers. A child exiting
// stops its siblings, including workers which exit successfully unexpectedly.
func Supervise(ctx context.Context, o Options) error {
	if len(o.Commands) == 0 {
		return fmt.Errorf("no services selected")
	}
	if o.Output == nil {
		o.Output = io.Discard
	}
	if o.Grace <= 0 {
		o.Grace = 10 * time.Second
	}
	if err := ctx.Err(); err != nil {
		return err
	}
	var mu sync.Mutex
	var children []*running
	events := make(chan *running, len(o.Commands))
	defer func() {
		defer func() {
			for _, p := range children {
				p.owner.close()
			}
		}()
		for _, p := range children {
			select {
			case <-p.done:
			default:
				p.owner.interrupt()
			}
		}
		deadline := time.NewTimer(o.Grace)
		defer deadline.Stop()
		for _, p := range children {
			select {
			case <-p.done:
			case <-deadline.C:
				for _, q := range children {
					q.owner.kill()
				}
				for _, q := range children {
					<-q.done
				}
				return
			}
		}
	}()
	for _, spec := range o.Commands {
		if err := ctx.Err(); err != nil {
			return err
		}
		if len(spec.Args) == 0 {
			return fmt.Errorf("service %s has no executable", spec.Name)
		}
		cmd := exec.Command(spec.Args[0], spec.Args[1:]...)
		cmd.Dir = spec.Dir
		cmd.Env = spec.Env
		cmd.WaitDelay = o.Grace
		out := &prefixWriter{out: o.Output, mu: &mu, prefix: "[" + spec.Name + "] "}
		errout := &prefixWriter{out: o.Output, mu: &mu, prefix: "[" + spec.Name + " stderr] "}
		cmd.Stdout = out
		cmd.Stderr = errout
		owner, err := prepare(cmd)
		if err != nil {
			return fmt.Errorf("service %s process ownership unavailable", spec.Name)
		}
		if err = cmd.Start(); err != nil {
			owner.close()
			return fmt.Errorf("service %s failed to start", spec.Name)
		}
		if err = owner.attach(); err != nil {
			_ = cmd.Process.Kill()
			_ = cmd.Wait()
			owner.close()
			return fmt.Errorf("service %s process ownership failed", spec.Name)
		}
		p := &running{owner: owner, done: make(chan struct{}), name: spec.Name}
		children = append(children, p)
		go func() {
			p.err = cmd.Wait()
			out.flush()
			errout.flush()
			owner.kill()
			close(p.done)
			events <- p
		}()
		mu.Lock()
		fmt.Fprintf(o.Output, "[%s] started", spec.Name)
		if spec.HTTP != "" {
			fmt.Fprintf(o.Output, "  HTTP http://%s", spec.HTTP)
		}
		if spec.GRPC != "" {
			fmt.Fprintf(o.Output, "  gRPC %s", spec.GRPC)
		}
		if spec.HTTP == "" && spec.GRPC == "" {
			fmt.Fprint(o.Output, "  worker (no listener)")
		}
		fmt.Fprintln(o.Output)
		mu.Unlock()
	}
	select {
	case <-ctx.Done():
		return nil
	case p := <-events:
		if p.err != nil {
			return fmt.Errorf("service %s exited with failure", p.name)
		}
		return fmt.Errorf("service %s exited; stopped workspace", p.name)
	}
}

// prefixWriter bounds partial lines and serializes every stream. Output is
// application-owned; the runner never prints process environments or secrets.
type prefixWriter struct {
	out    io.Writer
	mu     *sync.Mutex
	prefix string
	buffer []byte
}

func (w *prefixWriter) Write(b []byte) (int, error) {
	n := len(b)
	for len(b) > 0 {
		index := bytes.IndexByte(b, '\n')
		take := len(b)
		line := false
		if index >= 0 {
			take = index + 1
			line = true
		}
		if take > 4096-len(w.buffer) {
			take = 4096 - len(w.buffer)
			line = false
		}
		w.buffer = append(w.buffer, b[:take]...)
		b = b[take:]
		if line || len(w.buffer) == 4096 {
			w.flush()
		}
	}
	return n, nil
}
func (w *prefixWriter) flush() {
	if len(w.buffer) == 0 {
		return
	}
	w.mu.Lock()
	defer w.mu.Unlock()
	fmt.Fprint(w.out, w.prefix, string(w.buffer))
	if w.buffer[len(w.buffer)-1] != '\n' {
		fmt.Fprintln(w.out)
	}
	w.buffer = w.buffer[:0]
}

// Build disables network/module edits and implicit toolchain installation. Users
// prepare dependencies explicitly; go.work is optional local convenience.
func Build(ctx context.Context, root, temp, name string, isolated bool, output io.Writer) (string, error) {
	suffix := ""
	if runtime.GOOS == "windows" {
		suffix = ".exe"
	}
	binary := filepath.Join(temp, name+suffix)
	env := environment(os.Environ(), map[string]string{"GOPROXY": "off", "GOSUMDB": "off", "GOTOOLCHAIN": "local"})
	if isolated {
		env = environment(env, map[string]string{"GOWORK": "off"})
	}
	cmd := exec.Command("go", "build", "-mod=readonly", "-o", binary, ".")
	cmd.Dir = root
	cmd.Env = env
	cmd.Stdout = output
	cmd.Stderr = output
	cmd.WaitDelay = 10 * time.Second
	owner, err := prepare(cmd)
	if err != nil {
		return "", err
	}
	defer owner.close()
	if err = cmd.Start(); err != nil {
		return "", fmt.Errorf("build %s failed; install the supported Go toolchain", name)
	}
	if err = owner.attach(); err != nil {
		cmd.Process.Kill()
		cmd.Wait()
		return "", err
	}
	done := make(chan error, 1)
	go func() { done <- cmd.Wait() }()
	select {
	case err = <-done:
		owner.kill()
		if err != nil {
			return "", fmt.Errorf("build %s failed; prepare go.mod/go.sum and cached dependencies explicitly", name)
		}
		return binary, nil
	case <-ctx.Done():
		owner.kill()
		<-done
		return "", ctx.Err()
	}
}
func environment(base []string, values map[string]string) []string {
	result := make([]string, 0, len(base)+len(values))
	for _, v := range base {
		key, _, _ := strings.Cut(v, "=")
		skip := false
		for k := range values {
			if strings.EqualFold(key, k) {
				skip = true
				break
			}
		}
		if !skip {
			result = append(result, v)
		}
	}
	for k, v := range values {
		result = append(result, k+"="+v)
	}
	return result
}

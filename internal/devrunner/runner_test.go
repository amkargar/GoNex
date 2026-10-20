package devrunner

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"net"
	"os"
	"os/exec"
	"strconv"
	"strings"
	"sync"
	"testing"
	"time"
)

// The helper binds a real socket; a grandchild proves that cancellation kills
// descendants, not just the direct process. Nothing is mocked at the OS boundary.
func TestProcessHelper(t *testing.T) {
	if os.Getenv("GONEX_RUNNER_HELPER") != "1" {
		return
	}
	mode := os.Getenv("GONEX_RUNNER_MODE")
	if mode == "failure" {
		os.Exit(17)
	}
	l, e := net.Listen("tcp", os.Getenv("GONEX_RUNNER_ADDRESS"))
	if e != nil {
		os.Exit(18)
	}
	defer l.Close()
	fmt.Fprintln(os.Stdout, "helper ready")
	fmt.Fprintln(os.Stderr, "diagnostic")
	if mode == "tree" {
		time.Sleep(200 * time.Millisecond)
		cmd := exec.Command(os.Args[0], "-test.run=^TestProcessHelper$")
		cmd.Env = environment(os.Environ(), map[string]string{"GONEX_RUNNER_MODE": "worker", "GONEX_RUNNER_ADDRESS": os.Getenv("GONEX_RUNNER_CHILD")})
		if cmd.Start() != nil {
			os.Exit(19)
		}
		go cmd.Wait()
	}
	for {
		c, e := l.Accept()
		if e != nil {
			return
		}
		c.Close()
	}
}

type lockedBuffer struct {
	sync.Mutex
	b bytes.Buffer
}

func (b *lockedBuffer) Write(p []byte) (int, error) { b.Lock(); defer b.Unlock(); return b.b.Write(p) }
func (b *lockedBuffer) String() string              { b.Lock(); defer b.Unlock(); return b.b.String() }
func address(t *testing.T) string {
	t.Helper()
	l, e := net.Listen("tcp", "127.0.0.1:0")
	if e != nil {
		t.Fatal(e)
	}
	a := l.Addr().String()
	l.Close()
	return a
}
func ready(t *testing.T, a string) {
	t.Helper()
	until := time.Now().Add(10 * time.Second)
	for time.Now().Before(until) {
		c, e := net.DialTimeout("tcp", a, 100*time.Millisecond)
		if e == nil {
			c.Close()
			return
		}
		time.Sleep(20 * time.Millisecond)
	}
	t.Fatal("helper not ready", a)
}
func TestSuperviseCancellationRepeatedAndTree(t *testing.T) {
	for i := 0; i < 3; i++ {
		a, child := address(t), address(t)
		ctx, cancel := context.WithCancel(context.Background())
		out := &lockedBuffer{}
		done := make(chan error, 1)
		spec := Command{Name: "identity", Args: []string{os.Args[0], "-test.run=^TestProcessHelper$"}, Env: environment(os.Environ(), map[string]string{"GONEX_RUNNER_HELPER": "1", "GONEX_RUNNER_MODE": "tree", "GONEX_RUNNER_ADDRESS": a, "GONEX_RUNNER_CHILD": child})}
		go func() {
			done <- Supervise(ctx, Options{Commands: []Command{spec}, Output: out, Grace: 300 * time.Millisecond})
		}()
		ready(t, a)
		ready(t, child)
		cancel()
		select {
		case e := <-done:
			if e != nil {
				t.Fatal(e)
			}
		case <-time.After(10 * time.Second):
			t.Fatal("runner did not stop")
		}
		for _, target := range []string{a, child} {
			l, e := net.Listen("tcp", target)
			if e != nil {
				t.Fatal("orphan listener", e)
			}
			l.Close()
		}
		if !strings.Contains(out.String(), "[identity] helper ready") || !strings.Contains(out.String(), "[identity stderr] diagnostic") {
			t.Fatal("output not prefixed", out.String())
		}
	}
}
func TestSuperviseFailureStopsSibling(t *testing.T) {
	a := address(t)
	spec := Command{Name: "worker", Args: []string{os.Args[0], "-test.run=^TestProcessHelper$"}, Env: environment(os.Environ(), map[string]string{"GONEX_RUNNER_HELPER": "1", "GONEX_RUNNER_MODE": "worker", "GONEX_RUNNER_ADDRESS": a})}
	failure := Command{Name: "broken", Args: []string{os.Args[0], "-test.run=^TestProcessHelper$"}, Env: environment(os.Environ(), map[string]string{"GONEX_RUNNER_HELPER": "1", "GONEX_RUNNER_MODE": "failure"})}
	e := Supervise(context.Background(), Options{Commands: []Command{spec, failure}, Grace: 100 * time.Millisecond})
	if e == nil || !strings.Contains(e.Error(), "broken") {
		t.Fatal("child failure ignored", e)
	}
	l, e := net.Listen("tcp", a)
	if e != nil {
		t.Fatal("sibling listener left alive")
	}
	l.Close()
}
func TestPrefixWriterBoundsPartialRecords(t *testing.T) {
	var mu sync.Mutex
	var b bytes.Buffer
	w := prefixWriter{out: &b, mu: &mu, prefix: "[test] "}
	payload := strings.Repeat("x", 65536)
	n, e := io.WriteString(&w, payload)
	if e != nil || n != len(payload) || len(w.buffer) >= 4096 {
		t.Fatal(n, e)
	}
	w.flush()
	if count := strings.Count(b.String(), "[test]"); count != 16 {
		t.Fatal("unbounded chunking", strconv.Itoa(count))
	}
}

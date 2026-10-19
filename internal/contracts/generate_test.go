package contracts

import (
	"bytes"
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestGeneratedContractsReproducible(t *testing.T) {
	root, _ := filepath.Abs("../../contracts")
	ctx, cancel := context.WithTimeout(context.Background(), time.Minute)
	defer cancel()
	p, e := Generate(ctx, root, false)
	if e != nil {
		t.Fatal(e)
	}
	for _, c := range p.Changes {
		before, e := os.ReadFile(filepath.Join(root, c.Path))
		if e != nil {
			t.Fatal(e)
		}
		if !bytes.Equal(before, c.Data) {
			t.Fatalf("generated contract drift: %s", c.Path)
		}
	}
}

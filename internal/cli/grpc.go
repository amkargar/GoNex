package cli

import (
	"fmt"
	"github.com/amkargar/gonex/internal/contracts"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/types/descriptorpb"
	"os"
	"path/filepath"
)

func (c invocation) grpc(args []string) error {
	if len(args) == 0 || args[0] != "generate" && args[0] != "check" {
		return fmt.Errorf("usage: gonex grpc generate|check [--dry-run]")
	}
	f := c.flags("grpc")
	dry := f.Bool("dry-run", false, "compile/check only; no plugins or writes")
	if err := f.Parse(args[1:]); err != nil {
		return err
	}
	if f.NArg() != 0 {
		return fmt.Errorf("unexpected arguments")
	}
	root := filepath.Join(c.dir, "contracts")
	if _, err := os.Stat(filepath.Join(c.dir, "proto")); err == nil {
		root = c.dir
	}
	if args[0] == "check" {
		set, _, err := contracts.Compile(c.ctx, root)
		if err != nil {
			return err
		}
		data, err := os.ReadFile(filepath.Join(root, "baseline.binpb"))
		if err != nil {
			return fmt.Errorf("contract check needs an explicit release baseline.binpb: %w", err)
		}
		old := &descriptorpb.FileDescriptorSet{}
		if err = proto.Unmarshal(data, old); err != nil {
			return err
		}
		return contracts.Compatible(old, set)
	}
	p, err := contracts.Generate(c.ctx, root, *dry)
	if err != nil {
		return err
	}
	if *dry {
		fmt.Fprintln(c.out, "dry run: contracts compile; would run", contracts.GoPlugin, "and", contracts.GRPCPlugin)
		return nil
	}
	return c.plan(p, false)
}

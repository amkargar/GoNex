package contracts

import (
	"context"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/types/descriptorpb"
	"path/filepath"
	"testing"
)

func TestCompatibility(t *testing.T) {
	root, _ := filepath.Abs("../../contracts")
	old, _, e := Compile(context.Background(), root)
	if e != nil {
		t.Fatal(e)
	}
	if e = Compatible(old, old); e != nil {
		t.Fatal(e)
	}
	changed := proto.Clone(old).(*descriptorpb.FileDescriptorSet)
	changed.File[0].MessageType[0].Field[0].Name = proto.String("different")
	if e = Compatible(old, changed); e == nil {
		t.Fatal("breaking field rename accepted")
	}
	removed := proto.Clone(old).(*descriptorpb.FileDescriptorSet)
	removed.File[0].Service = nil
	if e = Compatible(old, removed); e == nil {
		t.Fatal("removed service accepted")
	}
}

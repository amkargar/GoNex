package contracts

import (
	"fmt"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/types/descriptorpb"
)

// Compatible conservatively rejects removals/changes within an existing API major.
// Additive files, messages, fields and methods are allowed; option changes may
// need a new major version even if wire-compatible.
func Compatible(old, next *descriptorpb.FileDescriptorSet) error {
	files := map[string]*descriptorpb.FileDescriptorProto{}
	for _, f := range next.File {
		files[f.GetName()] = f
	}
	for _, f := range old.File {
		n := files[f.GetName()]
		if n == nil || f.GetPackage() != n.GetPackage() || f.GetSyntax() != n.GetSyntax() || f.GetOptions().GetGoPackage() != n.GetOptions().GetGoPackage() {
			return fmt.Errorf("breaking contract file %s", f.GetName())
		}
		if err := messages(f.GetMessageType(), n.GetMessageType(), f.GetPackage()); err != nil {
			return err
		}
		if err := enums(f.GetEnumType(), n.GetEnumType()); err != nil {
			return err
		}
		services := map[string]*descriptorpb.ServiceDescriptorProto{}
		for _, s := range n.Service {
			services[s.GetName()] = s
		}
		for _, s := range f.Service {
			current := services[s.GetName()]
			if current == nil {
				return fmt.Errorf("removed service %s", s.GetName())
			}
			methods := map[string]*descriptorpb.MethodDescriptorProto{}
			for _, m := range current.Method {
				methods[m.GetName()] = m
			}
			for _, m := range s.Method {
				if !proto.Equal(m, methods[m.GetName()]) {
					return fmt.Errorf("breaking RPC %s/%s", s.GetName(), m.GetName())
				}
			}
		}
	}
	return nil
}
func messages(old, next []*descriptorpb.DescriptorProto, prefix string) error {
	byName := map[string]*descriptorpb.DescriptorProto{}
	for _, m := range next {
		byName[m.GetName()] = m
	}
	for _, m := range old {
		n := byName[m.GetName()]
		if n == nil {
			return fmt.Errorf("removed message %s.%s", prefix, m.GetName())
		}
		fields := map[int32]*descriptorpb.FieldDescriptorProto{}
		for _, f := range n.Field {
			fields[f.GetNumber()] = f
		}
		for _, f := range m.Field {
			if !proto.Equal(f, fields[f.GetNumber()]) {
				return fmt.Errorf("breaking field %s.%s.%s", prefix, m.GetName(), f.GetName())
			}
		}
		if err := messages(m.NestedType, n.NestedType, prefix+"."+m.GetName()); err != nil {
			return err
		}
		if err := enums(m.EnumType, n.EnumType); err != nil {
			return err
		}
	}
	return nil
}
func enums(old, next []*descriptorpb.EnumDescriptorProto) error {
	byName := map[string]*descriptorpb.EnumDescriptorProto{}
	for _, e := range next {
		byName[e.GetName()] = e
	}
	for _, e := range old {
		n := byName[e.GetName()]
		if n == nil {
			return fmt.Errorf("removed enum %s", e.GetName())
		}
		values := map[string]int32{}
		for _, v := range n.Value {
			values[v.GetName()] = v.GetNumber()
		}
		for _, v := range e.Value {
			number, ok := values[v.GetName()]
			if !ok || number != v.GetNumber() {
				return fmt.Errorf("breaking enum value %s", v.GetName())
			}
		}
	}
	return nil
}

// Package buildinfo identifies the Gonex dependency actually used by an executable.
package buildinfo

import "runtime/debug"

const Module = "github.com/amkargar/gonex"

func Version() string {
	info, ok := debug.ReadBuildInfo()
	if !ok {
		return "development"
	}
	return From(info)
}

// From works for an installed CLI as well as an application importing Gonex.
// A local replacement is deliberately not presented as a published version.
func From(info *debug.BuildInfo) string {
	modules := append([]*debug.Module{&info.Main}, info.Deps...)
	for _, m := range modules {
		if m.Path != Module {
			continue
		}
		if m.Replace != nil {
			m = m.Replace
		}
		if m.Version != "" && m.Version != "(devel)" {
			return m.Version
		}
	}
	return "development"
}

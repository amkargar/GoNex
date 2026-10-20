package buildinfo

import (
	"runtime/debug"
	"testing"
)

func TestModuleVersions(t *testing.T) {
	for _, tc := range []struct {
		name string
		info debug.BuildInfo
		want string
	}{
		{"installed CLI", debug.BuildInfo{Main: debug.Module{Path: Module, Version: "v0.2.0"}}, "v0.2.0"},
		{"consumer", debug.BuildInfo{Deps: []*debug.Module{{Path: Module, Version: "v0.0.0-20261019191310-670c8c5f427e"}}}, "v0.0.0-20261019191310-670c8c5f427e"},
		{"local CLI", debug.BuildInfo{Main: debug.Module{Path: Module, Version: "(devel)"}}, "development"},
		{"local replacement", debug.BuildInfo{Deps: []*debug.Module{{Path: Module, Version: "v0.2.0", Replace: &debug.Module{Path: "../gonex"}}}}, "development"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if got := From(&tc.info); got != tc.want {
				t.Fatalf("%s != %s", got, tc.want)
			}
		})
	}
}

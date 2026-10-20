package cli

import (
	"fmt"
	"strings"

	"github.com/amkargar/gonex/internal/terminal"
)

type helpTopic struct {
	usage       string
	description string
	options     string
	example     string
}

var helpTopics = map[string]helpTopic{
	"new": {
		usage:       "new NAME --module IMPORT_PATH [options]",
		description: "Create an application in a new directory.",
		options:     "  --module PATH               Go module import path (required)\n  --framework-version VERSION Pin a resolvable framework version\n  --arch TYPE                 monolith (default) or microservices\n  --dry-run                   Preview files without writing",
		example:     "gonex new shop --module example.com/shop",
	},
	"create": {
		usage:       "create <module|service|client> NAME [options]",
		description: "Generate a module, service or service client.",
		options:     "  module   Add a feature module\n  service  Add a workspace service\n  client   Add a typed service client",
		example:     "gonex create module users -h",
	},
	"create module": {
		usage:       "create module NAME [options]",
		description: "Add an editable feature module to your application.",
		options:     "  --transport TYPE   http (default), none, grpc or http,grpc\n  --service NAME     Select a workspace service\n  --http-version vN  Start with a versioned HTTP adapter\n  --version vN       gRPC contract version (default v1)\n  --db               Generate persistence files\n  --dry-run          Preview files without writing",
		example:     "go tool gonex create module users",
	},
	"create service": {
		usage:       "create service NAME [options]",
		description: "Add a service to a microservices workspace.",
		options:     "  --http             Enable HTTP\n  --grpc             Enable gRPC\n  --transport TYPE   none, http, grpc or http,grpc\n  --dry-run          Preview files without writing",
		example:     "go tool gonex create service orders --http",
	},
	"create client": {
		usage:       "create client PROVIDER --service CONSUMER --module MODULE [options]",
		description: "Generate an explicit client adapter in the consumer module.",
		options:     "  --service NAME     Consumer service (required)\n  --module NAME      Consumer module (required)\n  --version vN       Contract version (default v1)\n  --profile TYPE     skeleton (default), example or raw\n  --rpc METHOD       Explicit Service.Method for example mapping\n  --dry-run          Preview files without writing",
		example:     "go tool gonex create client identity --service orders --module orders",
	},
	"run": {
		usage:       "run [--service NAME]",
		description: "Start your application. Press Ctrl+C for graceful shutdown.",
		options:     "  --service NAME     Select a workspace service",
		example:     "go tool gonex run",
	},
	"build": {
		usage:       "build [options]",
		description: "Build the application executable using go build.",
		options:     "  --os OS            Target OS (default GOOS or current OS)\n  --arch ARCH        Target architecture (default GOARCH or current architecture)\n  --out FILE         Custom executable path\n  --service NAME     Select a workspace service",
		example:     "go tool gonex build --os windows --arch amd64",
	},
	"check": {
		usage:       "check [--service NAME]",
		description: "Run application tests, vet and package compilation checks.",
		options:     "  --service NAME     Select a workspace service",
		example:     "go tool gonex check",
	},
	"dev": {
		usage:       "dev [options]",
		description: "Build and supervise local workspace services.",
		options:     "  --service NAME     Select one service (default all)\n  --dev-insecure     Explicit local plaintext transport\n  --generate         Generate pinned contracts before startup\n  --isolated         Build with GOWORK=off",
		example:     "go tool gonex dev --dev-insecure",
	},
	"doctor": {
		usage:       "doctor [options]",
		description: "Diagnose local workspace configuration and contracts.",
		options:     "  --service NAME     Select one service (default all)\n  --dev-insecure     Check local plaintext configuration\n  --json             Emit structured diagnostics",
		example:     "go tool gonex doctor --dev-insecure",
	},
	"module": {
		usage:       "module <enable|version> NAME [options]",
		description: "Enable adapters or add an HTTP API version.",
		options:     "  enable    Enable gRPC on an existing module\n  version   Add an HTTP API version",
		example:     "go tool gonex module version users -h",
	},
	"module enable": {
		usage:       "module enable NAME --grpc [options]",
		description: "Enable a gRPC adapter for an existing module.",
		options:     "  --grpc             Enable gRPC (required)\n  --service NAME     Select a workspace service\n  --dry-run          Preview files without writing",
		example:     "go tool gonex module enable users --grpc",
	},
	"module version": {
		usage:       "module version NAME --http vN [options]",
		description: "Add a versioned HTTP adapter while preserving handwritten code.",
		options:     "  --http vN          HTTP version (required)\n  --policy POLICY    preserve (default) or replace legacy routes\n  --prefix PATH      URL prefix (default /api/vN)\n  --service NAME     Select a workspace service\n  --dry-run          Preview files without writing",
		example:     "go tool gonex module version users --http v1",
	},
	"upgrade": {
		usage:       "upgrade --framework-version VERSION [--dry-run]",
		description: "Update the selected framework version and generator metadata.",
		options:     "  --framework-version VERSION  Exact target module version\n  --dry-run                    Preview metadata changes",
		example:     "go tool gonex upgrade --framework-version <version>",
	},
	"openapi": {
		usage:       "openapi <generate|validate> [options]",
		description: "Generate or validate OpenAPI contracts.",
		options:     "  generate   Export your application contract\n  validate   Validate a contract and optional baseline",
		example:     "go tool gonex openapi generate -h",
	},
	"openapi generate": {
		usage:       "openapi generate [options]",
		description: "Export OpenAPI from your application.",
		options:     "  --out FILE         Relative JSON path (default openapi.json)\n  --http-version vN  Export an HTTP API version\n  --service NAME     Select a workspace service\n  --dry-run          Preview without exporting",
		example:     "go tool gonex openapi generate --out openapi.json",
	},
	"openapi validate": {
		usage:       "openapi validate [options]",
		description: "Validate OpenAPI and optionally check compatibility.",
		options:     "  --file FILE        JSON contract (default openapi.json)\n  --against FILE     Compatibility baseline\n  --service NAME     Select a workspace service",
		example:     "go tool gonex openapi validate --file openapi.json",
	},
	"db": {
		usage:       "db <migrate|status> [options]",
		description: "Manage database migrations.",
		options:     "  migrate   Apply pending migrations\n  status    Report migration status",
		example:     "go tool gonex db migrate -h",
	},
	"db migrate": {
		usage:       "db migrate [options]",
		description: "Apply pending SQL migrations.",
		options:     "  --service NAME     Select a workspace service\n  --dry-run          Preview without connecting",
		example:     "go tool gonex db migrate",
	},
	"db status": {
		usage:       "db status [options]",
		description: "Report database migration status.",
		options:     "  --service NAME     Select a workspace service\n  --dry-run          Preview without connecting",
		example:     "go tool gonex db status",
	},
	"grpc": {
		usage:       "grpc <generate|check> [options]",
		description: "Generate and check Protobuf contracts.",
		options:     "  generate   Generate contract artifacts\n  check      Check contracts against a release baseline",
		example:     "go tool gonex grpc generate -h",
	},
	"grpc generate": {
		usage:       "grpc generate [--check] [--dry-run]",
		description: "Generate pinned Protobuf contract artifacts.",
		options:     "  --check            Detect stale generated output\n  --dry-run          Preview generation",
		example:     "go tool gonex grpc generate",
	},
	"grpc check": {
		usage:       "grpc check",
		description: "Check contracts against the release baseline.",
		options:     "",
		example:     "go tool gonex grpc check",
	},
}

// helpRequest resolves a command before any project checks or side effects.
func helpRequest(args []string) (string, bool, error) {
	if len(args) == 0 {
		return "", true, nil
	}
	requested := args[0] == "help"
	if requested {
		args = args[1:]
	}
	for _, arg := range args {
		if arg == "--" {
			break
		}
		if arg == "-h" || arg == "--help" {
			requested = true
			break
		}
	}
	if !requested {
		return "", false, nil
	}
	if len(args) == 0 || args[0] == "-h" || args[0] == "--help" {
		return "", true, nil
	}
	key := args[0]
	if _, ok := helpTopics[key]; !ok {
		return "", true, fmt.Errorf("unknown command %q; run gonex -h", key)
	}
	if len(args) > 1 && !strings.HasPrefix(args[1], "-") {
		switch key {
		case "create", "module", "openapi", "db", "grpc":
			key += " " + args[1]
			if _, ok := helpTopics[key]; !ok {
				return "", true, fmt.Errorf("unknown command %q; run gonex -h", key)
			}
		}
	}
	return key, true, nil
}

func (c invocation) showHelp(topic string) {
	color := terminal.Color(c.out)
	heading := func(value string) string { return terminal.Paint(color, "1;33", value) }
	fmt.Fprintf(c.out, "\n%s  ·  API-first Go framework  %s\n\n", heading("GONEX"), terminal.Safe(version()))
	if topic != "" {
		h := helpTopics[topic]
		fmt.Fprintln(c.out, h.description)
		fmt.Fprintf(c.out, "\n%s\n  gonex %s\n", heading("Usage"), h.usage)
		fmt.Fprintf(c.out, "\n%s\n", heading("Options"))
		if h.options != "" {
			fmt.Fprintln(c.out, h.options)
		}
		fmt.Fprintln(c.out, "  -h, --help         Show command help")
		fmt.Fprintf(c.out, "\n%s\n  %s\n\n", heading("Example"), h.example)
		return
	}
	fmt.Fprintf(c.out, "%s\n  gonex <command> [options]\n\n", heading("Usage"))
	for _, group := range []struct{ title, commands string }{
		{"GET STARTED", "  new NAME        Create an application\n  create module   Add a feature module"},
		{"DEVELOPMENT", "  run             Start your application\n  build           Build an application executable\n  check           Run tests, vet and compilation checks"},
		{"API & DATA", "  openapi         Generate or validate API contracts\n  db              Manage database migrations"},
		{"MICROSERVICES", "  create service  Add a workspace service\n  create client   Connect services\n  dev             Run local services\n  doctor          Diagnose workspace configuration\n  grpc            Manage Protobuf contracts"},
		{"MAINTENANCE", "  module          Enable transports or API versions\n  upgrade         Upgrade the framework dependency"},
	} {
		fmt.Fprintf(c.out, "%s\n%s\n\n", heading(group.title), group.commands)
	}
	fmt.Fprintf(c.out, "%s\n  -h, --help      Show help\n      --version   Show version\n\n", heading("Options"))
	fmt.Fprintf(c.out, "%s\n  gonex new shop --module example.com/shop\n  go tool gonex create module users\n  go tool gonex run\n\n", heading("Examples"))
	fmt.Fprintln(c.out, "Use gonex <command> -h for command-specific help.")
	fmt.Fprintln(c.out, "Run project commands from the application root. Generators preserve handwritten files.")
	fmt.Fprintln(c.out, "Exit 0: success/help. Exit 1: failure. Child Go failures keep their exit status.")
	fmt.Fprintln(c.out)
}

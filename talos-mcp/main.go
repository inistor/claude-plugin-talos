package main

import (
	"fmt"
	"os"

	"github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"
)

// version is the server version reported over MCP. Overridden at build time:
//
//	go build -ldflags "-X main.version=$(git describe --tags --always)"
var version = "dev"

// toolListTTL is how long a client may cache tools/list and server/discover.
// The tool set is compiled in, so it cannot change while the process runs; the
// SDK default of 0 means "immediately stale" and forces refetches that can
// never return anything different. One hour is well short of a process lifetime
// while still bounding staleness after an upgrade.
const toolListTTL = 3_600_000 // ms

func main() {
	s := server.NewMCPServer("talos-mcp", version,
		// The tool set is static and identical for every caller, so the list is
		// cacheable and shareable. "public" is only correct because no tool is
		// filtered by credential — revisit if that ever changes.
		server.WithCacheHints(toolListTTL, mcp.CacheScopePublic),
		// listChanged=false: the tool set is fixed at compile time and
		// notifications/tools/list_changed is never sent. Claiming otherwise
		// invites clients to wait for a notification that will not arrive.
		server.WithToolCapabilities(false),
		server.WithInstructions(instructions),
		// Validate handler output against each tool's declared output schema so
		// a schema and its handler cannot drift apart silently.
		server.WithOutputSchemaValidation(),
		// A panic in one handler must not take the whole server down.
		server.WithRecovery(),
	)

	registerTools(s)

	// stdout is the JSON-RPC transport — anything written there that is not a
	// protocol frame corrupts the stream. Diagnostics go to stderr, and a
	// transport failure is a non-zero exit so the supervisor sees it.
	if err := server.ServeStdio(s); err != nil {
		fmt.Fprintf(os.Stderr, "talos-mcp: %v\n", err)
		os.Exit(1)
	}
}

// instructions is returned from server/discover and tells a model how to drive
// this server. It covers the two things not evident from the tool schemas
// alone: single-node targeting, and where the talosconfig comes from.
const instructions = `Talos Linux cluster management over the Talos gRPC API.

Node targeting: every node-aware tool takes exactly one "node" (a string). There
is no "nodes" array — to act on several nodes, issue one call per node. Omitting
"node" runs the request against whichever endpoint apid selects, which is fine
for cluster-wide reads (talos_health, talos_etcd_members) but means per-node
tools return the endpoint's own data rather than the intended node's.

Configuration: the talosconfig is fixed when this server starts, from
TALOSCONFIG or ~/.talos/config. Select a context within it per call with
"context", and override its endpoints with "endpoint" when the configured
control-plane endpoints are unreachable. To use a different talosconfig
entirely, start another server with TALOSCONFIG pointed at it.

Destructive tools are annotated as such: talos_reset, talos_wipe, talos_reboot,
talos_shutdown, talos_upgrade, talos_etcd_remove_member and talos_etcd_leave can
disrupt or destroy cluster state. Confirm with the operator before calling them.`

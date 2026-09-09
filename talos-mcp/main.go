package main

import (
	"fmt"
	"os"
	"os/signal"
	"syscall"

	"github.com/mark3labs/mcp-go/server"
)

// version is the server version reported over MCP. Overridden at build time:
//
//	go build -ldflags "-X main.version=$(git describe --tags --always)"
var version = "dev"

func main() {
	s := server.NewMCPServer("talos-mcp", version)

	registerTools(s)

	// The session talosconfig holds a client private key. Remove it on a normal
	// exit and on the usual termination signals rather than leaving it on disk.
	defer cleanupConfig()
	sigs := make(chan os.Signal, 1)
	signal.Notify(sigs, syscall.SIGINT, syscall.SIGTERM)
	go func() {
		<-sigs
		cleanupConfig()
		os.Exit(0)
	}()

	// stdout is the JSON-RPC transport — anything written there that is not a
	// protocol frame corrupts the stream. Diagnostics go to stderr, and a
	// transport failure is a non-zero exit so the supervisor sees it.
	if err := server.ServeStdio(s); err != nil {
		fmt.Fprintf(os.Stderr, "talos-mcp: %v\n", err)
		cleanupConfig()
		os.Exit(1)
	}
}

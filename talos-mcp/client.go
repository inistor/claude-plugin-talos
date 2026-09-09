package main

import (
	"context"
	"crypto/tls"
	"os"
	"path/filepath"

	"github.com/siderolabs/talos/pkg/machinery/client"
	"github.com/siderolabs/talos/pkg/machinery/constants"
)

// The server holds no per-session configuration. Which talosconfig is used is
// fixed when the process starts, via the TALOSCONFIG environment variable (or
// the default ~/.talos/config), and which context within it is selected per
// call. That matches how the Kubernetes MCP servers work with KUBECONFIG, and
// it satisfies MCP 2026-07-28, where a stdio connection is explicitly not a
// session and a server must not rely on earlier requests for context.
//
// To target a different cluster, point TALOSCONFIG at a different file in the
// server's launch configuration, or add a second server entry.

// configPath returns the talosconfig the client library will read: TALOSCONFIG
// when set, otherwise the default location. Used only for reporting, since
// client.WithDefaultConfig applies the same resolution itself.
func configPath() string {
	if p := os.Getenv(constants.TalosConfigEnvVar); p != "" {
		return p
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return ""
	}
	return filepath.Join(home, ".talos", "config")
}

// newClient creates a Talos client. A non-empty contextName selects a context
// within the talosconfig; a non-empty endpoint overrides that context's
// endpoints, which is what makes a node reachable when the configured
// control-plane endpoints are down.
func newClient(ctx context.Context, contextName, endpoint string) (*client.Client, error) {
	opts := []client.OptionFunc{client.WithDefaultConfig()}

	if contextName != "" {
		opts = append(opts, client.WithContextName(contextName))
	}
	if endpoint != "" {
		opts = append(opts, client.WithEndpoints(endpoint))
	}
	return client.New(ctx, opts...)
}

// newInsecureClient creates a Talos client for maintenance mode (no TLS auth).
// The node IP is used directly as the endpoint.
func newInsecureClient(ctx context.Context, node string) (*client.Client, error) {
	tlsConfig := &tls.Config{InsecureSkipVerify: true} //nolint:gosec
	return client.New(ctx,
		client.WithTLSConfig(tlsConfig),
		client.WithEndpoints(node),
	)
}

// nodeCtx returns a context targeting a specific node.
func nodeCtx(ctx context.Context, node string) context.Context {
	if node == "" {
		return ctx
	}
	return client.WithNode(ctx, node)
}

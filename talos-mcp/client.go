package main

import (
	"context"
	"sync"

	"github.com/siderolabs/talos/pkg/machinery/client"
)

var (
	configPath string
	configMu   sync.RWMutex
)

// setConfigPath sets a custom talosconfig path for all subsequent client calls.
func setConfigPath(path string) {
	configMu.Lock()
	defer configMu.Unlock()
	configPath = path
}

// getConfigPath returns the current talosconfig path, or empty for default.
func getConfigPath() string {
	configMu.RLock()
	defer configMu.RUnlock()
	return configPath
}

// newClient creates a Talos client from the configured talosconfig.
// If contextName is non-empty, it selects that talosconfig context.
func newClient(ctx context.Context, contextName string) (*client.Client, error) {
	var opts []client.OptionFunc

	if p := getConfigPath(); p != "" {
		opts = append(opts, client.WithConfigFromFile(p))
	} else {
		opts = append(opts, client.WithDefaultConfig())
	}
	if contextName != "" {
		opts = append(opts, client.WithContextName(contextName))
	}
	return client.New(ctx, opts...)
}

// nodeCtx returns a context targeting a specific node.
// If node is empty, returns the original context (uses default from talosconfig).
func nodeCtx(ctx context.Context, node string) context.Context {
	if node == "" {
		return ctx
	}
	return client.WithNode(ctx, node)
}

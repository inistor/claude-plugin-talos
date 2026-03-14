package main

import (
	"context"
	"os"
	"sync"

	"github.com/siderolabs/talos/pkg/machinery/client"
)

var (
	configPath string
	configMu   sync.RWMutex
)

// setConfigFromContent writes talosconfig YAML to a temp file and uses it for all subsequent calls.
func setConfigFromContent(content string) (string, error) {
	configMu.Lock()
	defer configMu.Unlock()

	if configPath != "" {
		os.Remove(configPath)
	}

	f, err := os.CreateTemp("", "talosconfig-*.yaml")
	if err != nil {
		return "", err
	}
	if _, err := f.WriteString(content); err != nil {
		f.Close()
		os.Remove(f.Name())
		return "", err
	}
	f.Close()

	configPath = f.Name()
	return configPath, nil
}

// getConfigPath returns the current talosconfig path, or empty for default.
func getConfigPath() string {
	configMu.RLock()
	defer configMu.RUnlock()
	return configPath
}

// newClient creates a Talos client from the configured talosconfig.
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
func nodeCtx(ctx context.Context, node string) context.Context {
	if node == "" {
		return ctx
	}
	return client.WithNode(ctx, node)
}

package main

import (
	"context"
	"crypto/tls"
	"os"
	"sync"

	"github.com/siderolabs/talos/pkg/machinery/client"
)

var (
	configPath string
	configDir  string
	configMu   sync.RWMutex
)

// setConfigFromContent writes talosconfig YAML to a temp file and uses it for
// all subsequent calls.
//
// The file contains the client private key, so it is created 0600 inside a 0700
// per-process directory rather than loose in a shared /tmp, and cleanupConfig
// removes it on shutdown.
func setConfigFromContent(content string) (string, error) {
	configMu.Lock()
	defer configMu.Unlock()

	if configPath != "" {
		os.Remove(configPath)
	}

	if configDir == "" {
		d, err := os.MkdirTemp("", "talos-mcp-")
		if err != nil {
			return "", err
		}
		configDir = d
	}

	f, err := os.CreateTemp(configDir, "talosconfig-*.yaml")
	if err != nil {
		return "", err
	}
	if err := f.Chmod(0o600); err != nil {
		f.Close()
		os.Remove(f.Name())
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

// cleanupConfig removes the session talosconfig and its directory. Safe to call
// when no config was ever set.
func cleanupConfig() {
	configMu.Lock()
	defer configMu.Unlock()

	if configPath != "" {
		os.Remove(configPath)
		configPath = ""
	}
	if configDir != "" {
		os.RemoveAll(configDir)
		configDir = ""
	}
}

// getConfigPath returns the current talosconfig path, or empty for default.
func getConfigPath() string {
	configMu.RLock()
	defer configMu.RUnlock()
	return configPath
}

// newClient creates a Talos client from the configured talosconfig. A non-empty
// endpoint overrides the endpoints from the talosconfig context, which is what
// makes a node reachable when the configured control-plane endpoints are down.
func newClient(ctx context.Context, contextName, endpoint string) (*client.Client, error) {
	var opts []client.OptionFunc

	if p := getConfigPath(); p != "" {
		opts = append(opts, client.WithConfigFromFile(p))
	} else {
		opts = append(opts, client.WithDefaultConfig())
	}
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

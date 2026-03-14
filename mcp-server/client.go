package main

import (
	"context"

	"github.com/siderolabs/talos/pkg/machinery/client"
)

// newClient creates a Talos client from the default talosconfig.
// If contextName is non-empty, it selects that talosconfig context.
func newClient(ctx context.Context, contextName string) (*client.Client, error) {
	opts := []client.OptionFunc{
		client.WithDefaultConfig(),
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

package main

import (
	"fmt"

	"github.com/mark3labs/mcp-go/server"
)

func main() {
	s := server.NewMCPServer("talos-mcp", "0.1.0")

	registerTools(s)

	if err := server.ServeStdio(s); err != nil {
		fmt.Printf("Error: %v\n", err)
	}
}

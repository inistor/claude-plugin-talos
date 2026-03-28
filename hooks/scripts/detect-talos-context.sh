#!/bin/bash
set -euo pipefail

# Detect the current Talos context and expose it to Claude
TALOSCONFIG="${TALOSCONFIG:-${HOME}/.talos/config}"

if [ ! -f "$TALOSCONFIG" ]; then
  echo '{"systemMessage": "No talosconfig found at '"$TALOSCONFIG"'. Talos MCP tools may not work until talosconfig is configured."}'
  exit 0
fi

# Extract current context info using yq
if command -v yq &>/dev/null; then
  CONTEXT=$(yq -r '.context // "default"' "$TALOSCONFIG")
  ENDPOINTS=$(yq -r ".contexts.${CONTEXT}.endpoints // [] | join(\", \")" "$TALOSCONFIG" 2>/dev/null || echo "unknown")
  NODES=$(yq -r ".contexts.${CONTEXT}.nodes // [] | join(\", \")" "$TALOSCONFIG" 2>/dev/null || echo "unknown")

  # Persist context info as env vars
  if [ -n "${CLAUDE_ENV_FILE:-}" ]; then
    echo "export TALOS_CONTEXT=\"${CONTEXT}\"" >> "$CLAUDE_ENV_FILE"
    echo "export TALOS_ENDPOINTS=\"${ENDPOINTS}\"" >> "$CLAUDE_ENV_FILE"
    echo "export TALOS_NODES=\"${NODES}\"" >> "$CLAUDE_ENV_FILE"
  fi

  echo "{\"systemMessage\": \"Talos context: ${CONTEXT} | Endpoints: ${ENDPOINTS} | Nodes: ${NODES}\"}"
else
  echo '{"systemMessage": "Talosconfig found but yq is not installed. Install yq for full context detection."}'
fi

exit 0

#!/bin/bash
# Wrapper script for proton
# Ensures the proton binary is run from the same directory as this script.

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
exec "$SCRIPT_DIR/proton" "$@"

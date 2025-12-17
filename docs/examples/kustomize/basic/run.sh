#!/usr/bin/env bash
# Script to run kustomize with the required flags for olm-extractor

set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"

echo "Building Kustomize manifests with olm-extractor..."
kustomize build --enable-alpha-plugins --network "${SCRIPT_DIR}"


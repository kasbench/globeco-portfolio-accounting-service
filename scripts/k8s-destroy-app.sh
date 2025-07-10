#!/bin/bash

# Destroy main app resources for GlobeCo Portfolio Accounting Service (excluding Hazelcast and Postgres)
set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
PROJECT_ROOT="$(dirname "$SCRIPT_DIR")"
DEPLOYMENTS_DIR="$PROJECT_ROOT/deployments"

DEFAULT_NAMESPACE="globeco"
NAMESPACE="${NAMESPACE:-$DEFAULT_NAMESPACE}"
FORCE="${FORCE:-false}"
DRY_RUN="${DRY_RUN:-false}"

RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
NC='\033[0m'

print_info() { echo -e "${BLUE}[INFO]${NC} $1"; }
print_success() { echo -e "${GREEN}[SUCCESS]${NC} $1"; }
print_warning() { echo -e "${YELLOW}[WARNING]${NC} $1"; }
print_error() { echo -e "${RED}[ERROR]${NC} $1"; }

show_help() {
    cat << EOF
Destroy main app resources for GlobeCo Portfolio Accounting Service (excluding Hazelcast and Postgres)

Usage: $0 [OPTIONS]

Options:
  -n, --namespace NAMESPACE     Namespace (default: $DEFAULT_NAMESPACE)
  -f, --force                   Force without prompt
  -d, --dry-run                 Dry run
  -h, --help                    Show help
EOF
}

parse_arguments() {
    while [[ $# -gt 0 ]]; do
        case $1 in
            -n|--namespace) NAMESPACE="$2"; shift 2;;
            -f|--force) FORCE="true"; shift;;
            -d|--dry-run) DRY_RUN="true"; shift;;
            -h|--help) show_help; exit 0;;
            *) print_error "Unknown option: $1"; show_help; exit 1;;
        esac
    done
}

main() {
    print_info "Deleting main app resources from namespace $NAMESPACE"
    if [[ "$FORCE" != "true" ]]; then
        read -p "Delete app resources in $NAMESPACE? (y/N): " -n 1 -r; echo
        if [[ ! $REPLY =~ ^[Yy]$ ]]; then print_info "Cancelled"; exit 0; fi
    fi
    kubectl delete -f "$DEPLOYMENTS_DIR/network-policy.yaml" -n "$NAMESPACE" --ignore-not-found=true || true
    kubectl delete -f "$DEPLOYMENTS_DIR/hpa.yaml" -n "$NAMESPACE" --ignore-not-found=true || true
    kubectl delete -f "$DEPLOYMENTS_DIR/ingress.yaml" -n "$NAMESPACE" --ignore-not-found=true || true
    kubectl delete -f "$DEPLOYMENTS_DIR/service.yaml" -n "$NAMESPACE" --ignore-not-found=true || true
    kubectl delete -f "$DEPLOYMENTS_DIR/deployment.yaml" -n "$NAMESPACE" --ignore-not-found=true || true
    kubectl delete -f "$DEPLOYMENTS_DIR/secrets.yaml" -n "$NAMESPACE" --ignore-not-found=true || true
    kubectl delete -f "$DEPLOYMENTS_DIR/configmap.yaml" -n "$NAMESPACE" --ignore-not-found=true || true
    kubectl delete -f "$DEPLOYMENTS_DIR/namespace.yaml" -n "$NAMESPACE" --ignore-not-found=true || true
    print_success "App resources deleted"
}

parse_arguments "$@"
main 
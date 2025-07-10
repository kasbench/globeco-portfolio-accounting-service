#!/bin/bash

# GlobeCo Portfolio Accounting Service - Kubernetes Destroy Script
# Author: Noah Krieger <noah@kasbench.org>
# Description: Deletes all resources created by k8s-deploy.sh except the namespace

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
NC='\033[0m' # No Color

print_info() {
    echo -e "${BLUE}[INFO]${NC} $1"
}

print_success() {
    echo -e "${GREEN}[SUCCESS]${NC} $1"
}

print_warning() {
    echo -e "${YELLOW}[WARNING]${NC} $1"
}

print_error() {
    echo -e "${RED}[ERROR]${NC} $1"
}

show_help() {
    cat << EOF
GlobeCo Portfolio Accounting Service - Kubernetes Destroy Script

Usage: $0 [OPTIONS]

Options:
  -n, --namespace NAMESPACE     Kubernetes namespace (default: $DEFAULT_NAMESPACE)
  -f, --force                   Force deletion without confirmation
  -d, --dry-run                 Show what would be deleted, but do not delete
  -h, --help                    Show this help message

Examples:
  $0                             # Delete with confirmation
  $0 -f                          # Force delete all resources
  $0 -n globeco                  # Delete from custom namespace
EOF
}

parse_arguments() {
    while [[ $# -gt 0 ]]; do
        case $1 in
            -n|--namespace)
                NAMESPACE="$2"
                shift 2
                ;;
            -f|--force)
                FORCE="true"
                shift
                ;;
            -d|--dry-run)
                DRY_RUN="true"
                shift
                ;;
            -h|--help)
                show_help
                exit 0
                ;;
            *)
                print_error "Unknown option: $1"
                show_help
                exit 1
                ;;
        esac
    done
}

main() {
    print_info "Preparing to delete all GlobeCo Portfolio Accounting resources in namespace: $NAMESPACE (except the namespace itself)"

    if [[ "$FORCE" != "true" ]]; then
        read -p "Are you sure you want to delete all resources in namespace $NAMESPACE (except the namespace itself)? (y/N): " -n 1 -r
        echo
        if [[ ! $REPLY =~ ^[Yy]$ ]]; then
            print_info "Deletion cancelled"
            exit 0
        fi
    fi

    local resources=(
        "ingress.yaml"
        "network-policy.yaml"
        "hpa.yaml"
        "service.yaml"
        "deployment.yaml"
        "hazelcast.yaml"
        "postgres.yaml"
        "secrets.yaml"
        "configmap.yaml"
    )

    for resource in "${resources[@]}"; do
        local resource_path="$DEPLOYMENTS_DIR/$resource"
        if [[ -f "$resource_path" ]]; then
            print_info "Deleting $resource ..."
            if [[ "$DRY_RUN" == "true" ]]; then
                print_info "DRY RUN: Would delete $resource_path"
                kubectl delete -f "$resource_path" -n "$NAMESPACE" --dry-run=client -o yaml || true
            else
                kubectl delete -f "$resource_path" -n "$NAMESPACE" --ignore-not-found=true || true
                print_success "$resource deleted"
            fi
        else
            print_warning "Resource file not found: $resource_path"
        fi
    done

    print_success "All resources deleted (namespace $NAMESPACE preserved)"
}

parse_arguments "$@"
main 
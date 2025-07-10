#!/bin/bash

# Deploy Postgres resources for GlobeCo Portfolio Accounting Service
set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
PROJECT_ROOT="$(dirname "$SCRIPT_DIR")"
DEPLOYMENTS_DIR="$PROJECT_ROOT/deployments"

DEFAULT_NAMESPACE="globeco"
NAMESPACE="${NAMESPACE:-$DEFAULT_NAMESPACE}"
DRY_RUN="${DRY_RUN:-false}"
FORCE="${FORCE:-false}"
WAIT_TIMEOUT="${WAIT_TIMEOUT:-600}"

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
Deploy Postgres resources for GlobeCo Portfolio Accounting Service

Usage: $0 [OPTIONS] deploy|destroy|status|logs

Options:
  -n, --namespace NAMESPACE     Namespace (default: $DEFAULT_NAMESPACE)
  -d, --dry-run                 Dry run
  -f, --force                   Force without prompt
  -w, --wait-timeout SECONDS    Wait timeout (default: $WAIT_TIMEOUT)
  -h, --help                    Show help
EOF
}

parse_arguments() {
    while [[ $# -gt 0 ]]; do
        case $1 in
            -n|--namespace) NAMESPACE="$2"; shift 2;;
            -d|--dry-run) DRY_RUN="true"; shift;;
            -f|--force) FORCE="true"; shift;;
            -w|--wait-timeout) WAIT_TIMEOUT="$2"; shift 2;;
            -h|--help) show_help; exit 0;;
            deploy|destroy|status|logs) COMMAND="$1"; shift;;
            *) print_error "Unknown option: $1"; show_help; exit 1;;
        esac
    done
}

apply_resource() {
    local file="$1"
    if [[ -f "$file" ]]; then
        print_info "Applying $(basename "$file")..."
        if [[ "$DRY_RUN" == "true" ]]; then
            kubectl apply -f "$file" --dry-run=client -o yaml
        else
            kubectl apply -f "$file" -n "$NAMESPACE"
            print_success "$(basename "$file") applied"
        fi
    else
        print_warning "File not found: $file"
    fi
}

deploy_postgres() {
    print_info "Deploying Postgres resources to namespace $NAMESPACE"
    apply_resource "$DEPLOYMENTS_DIR/postgres.yaml"
    # Apply Postgres network policy if present
    if grep -q 'name: postgresql-policy' "$DEPLOYMENTS_DIR/network-policy.yaml"; then
        kubectl apply -f <(awk '/name: postgresql-policy/{p=1} p; /---/{if(p){exit}}' "$DEPLOYMENTS_DIR/network-policy.yaml") -n "$NAMESPACE" || true
        print_success "Postgres network policy applied"
    fi
    print_success "Postgres deployment complete"
}

destroy_postgres() {
    print_info "Deleting Postgres resources from namespace $NAMESPACE"
    if [[ "$FORCE" != "true" ]]; then
        read -p "Delete Postgres resources in $NAMESPACE? (y/N): " -n 1 -r; echo
        if [[ ! $REPLY =~ ^[Yy]$ ]]; then print_info "Cancelled"; exit 0; fi
    fi
    kubectl delete -f "$DEPLOYMENTS_DIR/postgres.yaml" -n "$NAMESPACE" --ignore-not-found=true || true
    if grep -q 'name: postgresql-policy' "$DEPLOYMENTS_DIR/network-policy.yaml"; then
        kubectl delete -f <(awk '/name: postgresql-policy/{p=1} p; /---/{if(p){exit}}' "$DEPLOYMENTS_DIR/network-policy.yaml") -n "$NAMESPACE" --ignore-not-found=true || true
    fi
    print_success "Postgres resources deleted"
}

show_status() {
    kubectl get deployment,svc,pvc,configmap,secret -n "$NAMESPACE" | grep postgresql || true
}

show_logs() {
    local pods=$(kubectl get pods -n "$NAMESPACE" -l app.kubernetes.io/component=database -o name)
    for pod in $pods; do
        echo "=== Logs from $pod ==="
        kubectl logs "$pod" -n "$NAMESPACE" --tail=100
    done
}

main() {
    if [[ -z "${COMMAND:-}" ]]; then print_error "No command"; show_help; exit 1; fi
    case "$COMMAND" in
        deploy) deploy_postgres;;
        destroy) destroy_postgres;;
        status) show_status;;
        logs) show_logs;;
        *) print_error "Unknown command: $COMMAND"; show_help; exit 1;;
    esac
}

parse_arguments "$@"
main 
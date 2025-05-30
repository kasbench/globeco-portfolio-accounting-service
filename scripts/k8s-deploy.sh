#!/bin/bash

# GlobeCo Portfolio Accounting Service - Kubernetes Deployment Script
# Author: Noah Krieger <noah@kasbench.org>
# Description: Comprehensive deployment script for Kubernetes environments

set -euo pipefail

# Script configuration
SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
PROJECT_ROOT="$(dirname "$SCRIPT_DIR")"
DEPLOYMENTS_DIR="$PROJECT_ROOT/deployments"

# Default configuration
DEFAULT_NAMESPACE="globeco-portfolio-accounting"
DEFAULT_ENVIRONMENT="production"
DEFAULT_IMAGE_TAG="latest"
DEFAULT_REPLICAS="3"

# Color codes for output
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
NC='\033[0m' # No Color

# Configuration variables
NAMESPACE="${NAMESPACE:-$DEFAULT_NAMESPACE}"
ENVIRONMENT="${ENVIRONMENT:-$DEFAULT_ENVIRONMENT}"
IMAGE_TAG="${IMAGE_TAG:-$DEFAULT_IMAGE_TAG}"
REPLICAS="${REPLICAS:-$DEFAULT_REPLICAS}"
DRY_RUN="${DRY_RUN:-false}"
FORCE="${FORCE:-false}"
SKIP_TESTS="${SKIP_TESTS:-false}"
WAIT_TIMEOUT="${WAIT_TIMEOUT:-600}"

# Function to print colored output
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

# Function to display help
show_help() {
    cat << EOF
GlobeCo Portfolio Accounting Service - Kubernetes Deployment Script

Usage: $0 [OPTIONS] COMMAND

Commands:
  deploy      Deploy the complete service stack
  upgrade     Upgrade existing deployment
  rollback    Rollback to previous version
  destroy     Remove all resources
  status      Show deployment status
  logs        Show service logs
  test        Run deployment tests
  migration   Run database migrations

Options:
  -n, --namespace NAMESPACE     Kubernetes namespace (default: $DEFAULT_NAMESPACE)
  -e, --environment ENV         Environment (production|staging|development) (default: $DEFAULT_ENVIRONMENT)
  -t, --tag TAG                 Docker image tag (default: $DEFAULT_IMAGE_TAG)
  -r, --replicas COUNT          Number of replicas (default: $DEFAULT_REPLICAS)
  -d, --dry-run                 Perform dry run without applying changes
  -f, --force                   Force deployment without confirmation
  -s, --skip-tests              Skip deployment tests
  -w, --wait-timeout SECONDS    Wait timeout for deployment (default: $WAIT_TIMEOUT)
  -h, --help                    Show this help message

Environment Variables:
  KUBECONFIG                    Path to kubeconfig file
  NAMESPACE                     Override default namespace
  ENVIRONMENT                   Override default environment
  IMAGE_TAG                     Override default image tag
  REPLICAS                      Override default replicas
  DRY_RUN                       Enable dry run mode
  FORCE                         Enable force mode
  SKIP_TESTS                    Skip deployment tests
  WAIT_TIMEOUT                  Wait timeout in seconds

Examples:
  $0 deploy                                    # Deploy with defaults
  $0 deploy -e staging -t v1.2.3 -r 5        # Deploy staging environment
  $0 upgrade -t v1.2.4                       # Upgrade to new version
  $0 rollback                                 # Rollback to previous version
  $0 destroy -f                              # Force destroy all resources
  $0 status                                   # Show deployment status

EOF
}

# Function to check prerequisites
check_prerequisites() {
    print_info "Checking prerequisites..."
    
    # Check if kubectl is available
    if ! command -v kubectl &> /dev/null; then
        print_error "kubectl is not installed or not in PATH"
        exit 1
    fi
    
    # Check if kustomize is available
    if ! command -v kustomize &> /dev/null; then
        print_warning "kustomize not found, using kubectl kustomize"
    fi
    
    # Check if helm is available (optional)
    if ! command -v helm &> /dev/null; then
        print_warning "helm not found, some features may be unavailable"
    fi
    
    # Check kubectl connectivity
    if ! kubectl cluster-info &> /dev/null; then
        print_error "Cannot connect to Kubernetes cluster"
        exit 1
    fi
    
    # Check if Docker is available for building
    if ! command -v docker &> /dev/null; then
        print_warning "Docker not found, assuming images are pre-built"
    fi
    
    print_success "Prerequisites check completed"
}

# Function to create namespace if it doesn't exist
create_namespace() {
    print_info "Creating namespace: $NAMESPACE"
    
    if kubectl get namespace "$NAMESPACE" &> /dev/null; then
        print_info "Namespace $NAMESPACE already exists"
    else
        if [[ "$DRY_RUN" == "true" ]]; then
            print_info "DRY RUN: Would create namespace $NAMESPACE"
        else
            kubectl create namespace "$NAMESPACE"
            print_success "Namespace $NAMESPACE created"
        fi
    fi
}

# Function to apply configuration
apply_configuration() {
    local resource_file="$1"
    local resource_name=$(basename "$resource_file" .yaml)
    
    print_info "Applying $resource_name configuration..."
    
    if [[ "$DRY_RUN" == "true" ]]; then
        print_info "DRY RUN: Would apply $resource_file"
        kubectl apply -f "$resource_file" --dry-run=client -o yaml
    else
        kubectl apply -f "$resource_file" -n "$NAMESPACE"
        print_success "$resource_name applied successfully"
    fi
}

# Function to wait for deployment
wait_for_deployment() {
    local deployment_name="$1"
    local timeout="$2"
    
    print_info "Waiting for deployment $deployment_name to be ready (timeout: ${timeout}s)..."
    
    if [[ "$DRY_RUN" == "true" ]]; then
        print_info "DRY RUN: Would wait for deployment $deployment_name"
        return 0
    fi
    
    if kubectl wait --for=condition=available deployment/"$deployment_name" \
        -n "$NAMESPACE" --timeout="${timeout}s"; then
        print_success "Deployment $deployment_name is ready"
    else
        print_error "Deployment $deployment_name failed to become ready within ${timeout}s"
        return 1
    fi
}

# Function to deploy the service
deploy_service() {
    print_info "Starting deployment of Portfolio Accounting Service"
    print_info "Environment: $ENVIRONMENT"
    print_info "Namespace: $NAMESPACE"
    print_info "Image Tag: $IMAGE_TAG"
    print_info "Replicas: $REPLICAS"
    
    # Confirmation prompt (unless forced)
    if [[ "$FORCE" != "true" && "$DRY_RUN" != "true" ]]; then
        read -p "Continue with deployment? (y/N): " -n 1 -r
        echo
        if [[ ! $REPLY =~ ^[Yy]$ ]]; then
            print_info "Deployment cancelled"
            exit 0
        fi
    fi
    
    # Create namespace
    create_namespace
    
    # Apply resources in order
    local resources=(
        "namespace.yaml"
        "configmap.yaml"
        "secrets.yaml"
        "postgres.yaml"
        "hazelcast.yaml"
        "deployment.yaml"
        "service.yaml"
        "hpa.yaml"
        "network-policy.yaml"
        "ingress.yaml"
    )
    
    for resource in "${resources[@]}"; do
        local resource_path="$DEPLOYMENTS_DIR/$resource"
        if [[ -f "$resource_path" ]]; then
            apply_configuration "$resource_path"
        else
            print_warning "Resource file not found: $resource_path"
        fi
    done
    
    # Wait for deployments to be ready
    if [[ "$DRY_RUN" != "true" ]]; then
        wait_for_deployment "postgresql" "$WAIT_TIMEOUT"
        wait_for_deployment "hazelcast" "$WAIT_TIMEOUT"
        wait_for_deployment "portfolio-accounting-service" "$WAIT_TIMEOUT"
    fi
    
    print_success "Deployment completed successfully"
}

# Function to upgrade the service
upgrade_service() {
    print_info "Upgrading Portfolio Accounting Service to version $IMAGE_TAG"
    
    # Update image tag in deployment
    kubectl set image deployment/portfolio-accounting-service \
        portfolio-accounting=globeco/portfolio-accounting-service:$IMAGE_TAG \
        -n "$NAMESPACE"
    
    # Wait for rollout to complete
    kubectl rollout status deployment/portfolio-accounting-service -n "$NAMESPACE" --timeout="${WAIT_TIMEOUT}s"
    
    print_success "Upgrade completed successfully"
}

# Function to rollback the service
rollback_service() {
    print_info "Rolling back Portfolio Accounting Service"
    
    # Rollback deployment
    kubectl rollout undo deployment/portfolio-accounting-service -n "$NAMESPACE"
    
    # Wait for rollout to complete
    kubectl rollout status deployment/portfolio-accounting-service -n "$NAMESPACE" --timeout="${WAIT_TIMEOUT}s"
    
    print_success "Rollback completed successfully"
}

# Function to destroy the service
destroy_service() {
    print_info "Destroying Portfolio Accounting Service"
    print_warning "This will remove ALL resources in namespace: $NAMESPACE"
    
    # Confirmation prompt (unless forced)
    if [[ "$FORCE" != "true" ]]; then
        read -p "Are you sure you want to destroy the service? (y/N): " -n 1 -r
        echo
        if [[ ! $REPLY =~ ^[Yy]$ ]]; then
            print_info "Destruction cancelled"
            exit 0
        fi
    fi
    
    # Delete namespace (this removes all resources)
    kubectl delete namespace "$NAMESPACE" --ignore-not-found=true
    
    print_success "Service destroyed successfully"
}

# Function to show deployment status
show_status() {
    print_info "Portfolio Accounting Service Status"
    echo
    
    # Check if namespace exists
    if ! kubectl get namespace "$NAMESPACE" &> /dev/null; then
        print_warning "Namespace $NAMESPACE does not exist"
        return 1
    fi
    
    # Show deployments
    echo "=== Deployments ==="
    kubectl get deployments -n "$NAMESPACE" -o wide
    echo
    
    # Show pods
    echo "=== Pods ==="
    kubectl get pods -n "$NAMESPACE" -o wide
    echo
    
    # Show services
    echo "=== Services ==="
    kubectl get services -n "$NAMESPACE" -o wide
    echo
    
    # Show ingress
    echo "=== Ingress ==="
    kubectl get ingress -n "$NAMESPACE" -o wide
    echo
    
    # Show HPA
    echo "=== Horizontal Pod Autoscaler ==="
    kubectl get hpa -n "$NAMESPACE" -o wide
    echo
}

# Function to show logs
show_logs() {
    print_info "Portfolio Accounting Service Logs"
    
    # Get pod names
    local pods=$(kubectl get pods -n "$NAMESPACE" -l app.kubernetes.io/component=api-server -o name)
    
    if [[ -z "$pods" ]]; then
        print_error "No portfolio accounting pods found"
        return 1
    fi
    
    # Show logs from all pods
    for pod in $pods; do
        echo "=== Logs from $pod ==="
        kubectl logs "$pod" -n "$NAMESPACE" --tail=100
        echo
    done
}

# Function to run tests
run_tests() {
    if [[ "$SKIP_TESTS" == "true" ]]; then
        print_info "Skipping tests as requested"
        return 0
    fi
    
    print_info "Running deployment tests..."
    
    # Test 1: Check if all pods are running
    local failed_pods=$(kubectl get pods -n "$NAMESPACE" --field-selector=status.phase!=Running -o name | wc -l)
    if [[ "$failed_pods" -gt 0 ]]; then
        print_error "Some pods are not running"
        kubectl get pods -n "$NAMESPACE" --field-selector=status.phase!=Running
        return 1
    fi
    
    # Test 2: Check if services are accessible
    local service_endpoint=$(kubectl get service portfolio-accounting-service -n "$NAMESPACE" -o jsonpath='{.spec.clusterIP}')
    if [[ -n "$service_endpoint" ]]; then
        print_info "Service endpoint: $service_endpoint:8087"
    else
        print_error "Service endpoint not found"
        return 1
    fi
    
    # Test 3: Health check
    local health_check_pod=$(kubectl get pods -n "$NAMESPACE" -l app.kubernetes.io/component=api-server -o name | head -1)
    if [[ -n "$health_check_pod" ]]; then
        if kubectl exec "$health_check_pod" -n "$NAMESPACE" -- curl -f http://localhost:8087/health &> /dev/null; then
            print_success "Health check passed"
        else
            print_error "Health check failed"
            return 1
        fi
    fi
    
    print_success "All tests passed"
}

# Function to run database migrations
run_migrations() {
    print_info "Running database migrations..."
    
    # Create migration job
    kubectl create job --from=cronjob/portfolio-accounting-migration portfolio-accounting-migration-$(date +%s) -n "$NAMESPACE"
    
    print_success "Migration job created"
}

# Parse command line arguments
parse_arguments() {
    while [[ $# -gt 0 ]]; do
        case $1 in
            -n|--namespace)
                NAMESPACE="$2"
                shift 2
                ;;
            -e|--environment)
                ENVIRONMENT="$2"
                shift 2
                ;;
            -t|--tag)
                IMAGE_TAG="$2"
                shift 2
                ;;
            -r|--replicas)
                REPLICAS="$2"
                shift 2
                ;;
            -d|--dry-run)
                DRY_RUN="true"
                shift
                ;;
            -f|--force)
                FORCE="true"
                shift
                ;;
            -s|--skip-tests)
                SKIP_TESTS="true"
                shift
                ;;
            -w|--wait-timeout)
                WAIT_TIMEOUT="$2"
                shift 2
                ;;
            -h|--help)
                show_help
                exit 0
                ;;
            deploy|upgrade|rollback|destroy|status|logs|test|migration)
                COMMAND="$1"
                shift
                ;;
            *)
                print_error "Unknown option: $1"
                show_help
                exit 1
                ;;
        esac
    done
}

# Main function
main() {
    # Check if command is provided
    if [[ -z "${COMMAND:-}" ]]; then
        print_error "No command provided"
        show_help
        exit 1
    fi
    
    # Check prerequisites
    check_prerequisites
    
    # Execute command
    case "$COMMAND" in
        deploy)
            deploy_service
            if [[ "$SKIP_TESTS" != "true" ]]; then
                run_tests
            fi
            ;;
        upgrade)
            upgrade_service
            if [[ "$SKIP_TESTS" != "true" ]]; then
                run_tests
            fi
            ;;
        rollback)
            rollback_service
            ;;
        destroy)
            destroy_service
            ;;
        status)
            show_status
            ;;
        logs)
            show_logs
            ;;
        test)
            run_tests
            ;;
        migration)
            run_migrations
            ;;
        *)
            print_error "Unknown command: $COMMAND"
            show_help
            exit 1
            ;;
    esac
}

# Parse arguments and run main function
parse_arguments "$@"
main 
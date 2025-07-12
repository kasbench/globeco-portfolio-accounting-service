#!/bin/bash

# Docker Compose startup script for GlobeCo Portfolio Accounting Service
# Supports different deployment profiles and configurations

set -euo pipefail

# Configuration
COMPOSE_PROJECT_NAME="${COMPOSE_PROJECT_NAME:-globeco-portfolio-accounting}"
ENVIRONMENT="${ENVIRONMENT:-development}"
PROFILE="${PROFILE:-default}"
DETACH="${DETACH:-false}"

# Colors for output
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
NC='\033[0m' # No Color

# Logging functions
log_info() {
    echo -e "${GREEN}[INFO]${NC} $1"
}

log_warn() {
    echo -e "${YELLOW}[WARN]${NC} $1"
}

log_error() {
    echo -e "${RED}[ERROR]${NC} $1"
}

log_debug() {
    echo -e "${BLUE}[DEBUG]${NC} $1"
}

# Check prerequisites
check_prerequisites() {
    # Check Docker
    if ! command -v docker &> /dev/null; then
        log_error "Docker is not installed or not in PATH"
        exit 1
    fi

    # Check Docker Compose
    if ! command -v docker-compose &> /dev/null && ! docker compose version &> /dev/null; then
        log_error "Docker Compose is not installed or not in PATH"
        exit 1
    fi

    # Check if Docker daemon is running
    if ! docker info &> /dev/null; then
        log_error "Docker daemon is not running"
        exit 1
    fi

    log_info "Prerequisites check passed"
}

# Set up environment
setup_environment() {
    # Create necessary directories
    mkdir -p data output logs tmp

    # Set environment variables
    export COMPOSE_PROJECT_NAME="$COMPOSE_PROJECT_NAME"
    export COMPOSE_FILE="docker-compose.yml"
    
    # Add override file for development
    if [ "$ENVIRONMENT" = "development" ]; then
        export COMPOSE_FILE="docker-compose.yml:docker-compose.override.yml"
    fi

    log_info "Environment: $ENVIRONMENT"
    log_info "Profile: $PROFILE"
    log_info "Compose files: $COMPOSE_FILE"
}

# Start services based on profile
start_services() {
    local compose_args=()
    
    # Add profile-specific arguments
    case $PROFILE in
        default)
            log_info "Starting core services (PostgreSQL, Hazelcast, Kafka)"
            compose_args+=(postgres hazelcast kafka zookeeper)
            ;;
        full)
            log_info "Starting all services including external services"
            compose_args+=(--profile external-services)
            ;;
        development)
            log_info "Starting development services with tools"
            compose_args+=(postgres hazelcast kafka zookeeper portfolio-accounting-service)
            if [ "$ENVIRONMENT" = "development" ]; then
                compose_args+=(pgadmin kafka-ui redis)
            fi
            ;;
        infrastructure)
            log_info "Starting infrastructure services only"
            compose_args+=(postgres hazelcast kafka zookeeper)
            ;;
        cli)
            log_info "Starting services for CLI usage"
            compose_args+=(--profile cli postgres)
            ;;
        testing)
            log_info "Starting services for testing"
            compose_args+=(postgres hazelcast)
            ;;
        *)
            log_error "Unknown profile: $PROFILE"
            show_usage
            exit 1
            ;;
    esac

    # Add detach flag if specified
    if [ "$DETACH" = "true" ]; then
        compose_args=(up -d "${compose_args[@]}")
    else
        compose_args=(up "${compose_args[@]}")
    fi

    log_info "Starting services with: docker-compose ${compose_args[*]}"
    
    # Start services
    docker-compose "${compose_args[@]}"
}

# Wait for services to be healthy
wait_for_services() {
    if [ "$DETACH" = "true" ]; then
        log_info "Waiting for services to be healthy..."
        
        # Wait for PostgreSQL
        log_info "Waiting for PostgreSQL..."
        timeout 120 bash -c 'until docker-compose exec -T postgres pg_isready -U portfolio_user -d portfolio_accounting; do sleep 2; done' || {
            log_error "PostgreSQL failed to start"
            return 1
        }
        
        # Wait for Hazelcast
        log_info "Waiting for Hazelcast..."
        timeout 120 bash -c 'until curl -f http://localhost:5701/hazelcast/health &>/dev/null; do sleep 2; done' || {
            log_warn "Hazelcast health check failed, but continuing..."
        }
        
        # Wait for Kafka (if in profile)
        if [[ "$PROFILE" == *"kafka"* ]] || [ "$PROFILE" = "full" ] || [ "$PROFILE" = "development" ]; then
            log_info "Waiting for Kafka..."
            timeout 120 bash -c 'until docker-compose exec -T kafka kafka-broker-api-versions --bootstrap-server localhost:9092 &>/dev/null; do sleep 3; done' || {
                log_warn "Kafka health check failed, but continuing..."
            }
        fi
        
        # Wait for main application (if in profile)
        if [ "$PROFILE" = "development" ] || [ "$PROFILE" = "full" ]; then
            log_info "Waiting for Portfolio Accounting Service..."
            timeout 120 bash -c 'until curl -f http://localhost:8087/api/v1/health &>/dev/null; do sleep 3; done' || {
                log_warn "Portfolio Accounting Service health check failed"
            }
        fi
        
        log_info "All services are ready!"
        show_service_info
    fi
}

# Show service information
show_service_info() {
    echo ""
    echo "=== Service Information ==="
    echo ""
    
    # Service URLs
    echo "Core Services:"
    echo "  PostgreSQL:        localhost:5432 (dev: 5433)"
    echo "  Hazelcast:         localhost:5701"
    echo "  Kafka:             localhost:9092"
    echo ""
    
    if [ "$ENVIRONMENT" = "development" ]; then
        echo "Development Tools:"
        echo "  PgAdmin:           http://localhost:5050"
        echo "  Kafka UI:          http://localhost:8080"
        echo "  Redis:             localhost:6379"
        echo ""
    fi
    
    if [ "$PROFILE" = "development" ] || [ "$PROFILE" = "full" ]; then
        echo "Application Services:"
        echo "  Portfolio Accounting:  http://localhost:8087"
        echo "  - Health Check:        http://localhost:8087/api/v1/health"
        echo "  - Metrics:             http://localhost:8087/metrics"
        echo "  - Debug (pprof):       http://localhost:6060"
        echo ""
    fi
    
    if [ "$PROFILE" = "full" ]; then
        echo "External Services (Mocked):"
        echo "  Portfolio Service:     http://localhost:8001"
        echo "  Security Service:      http://localhost:8000"
        echo ""
    fi
    
    echo "Docker Compose Commands:"
    echo "  docker-compose ps              # Show running services"
    echo "  docker-compose logs -f         # Follow logs"
    echo "  docker-compose down            # Stop services"
    echo "  docker-compose down -v         # Stop and remove volumes"
    echo ""
}

# Show usage
show_usage() {
    echo "Usage: $0 [OPTIONS]"
    echo ""
    echo "Options:"
    echo "  -p, --profile PROFILE    Deployment profile (default: default)"
    echo "  -e, --env ENVIRONMENT    Environment (development/production)"
    echo "  -d, --detach            Run services in background"
    echo "  -h, --help              Show this help"
    echo ""
    echo "Profiles:"
    echo "  default        Core infrastructure services"
    echo "  development    Development environment with tools"
    echo "  full          All services including external dependencies"
    echo "  infrastructure Infrastructure services only"
    echo "  cli           Services for CLI usage"
    echo "  testing       Minimal services for testing"
    echo ""
    echo "Environment Variables:"
    echo "  COMPOSE_PROJECT_NAME   Docker Compose project name"
    echo "  ENVIRONMENT           deployment environment"
    echo "  PROFILE               Service profile"
    echo "  DETACH               Run in background (true/false)"
    echo ""
    echo "Examples:"
    echo "  $0                                    # Start default services"
    echo "  $0 -p development -d                  # Start development profile in background"
    echo "  $0 -p full -e production             # Start all services for production"
    echo "  PROFILE=cli $0                       # Start CLI services"
}

# Cleanup on exit
cleanup() {
    if [ "$DETACH" = "false" ]; then
        log_info "Cleaning up..."
        docker-compose down
    fi
}

# Parse command line arguments
parse_args() {
    while [[ $# -gt 0 ]]; do
        case $1 in
            -p|--profile)
                PROFILE="$2"
                shift 2
                ;;
            -e|--env)
                ENVIRONMENT="$2"
                shift 2
                ;;
            -d|--detach)
                DETACH="true"
                shift
                ;;
            -h|--help)
                show_usage
                exit 0
                ;;
            *)
                log_error "Unknown option: $1"
                show_usage
                exit 1
                ;;
        esac
    done
}

# Main execution
main() {
    parse_args "$@"
    
    log_info "Starting GlobeCo Portfolio Accounting Service"
    log_info "Profile: $PROFILE, Environment: $ENVIRONMENT"
    
    # Set up cleanup trap
    trap cleanup EXIT
    
    check_prerequisites
    setup_environment
    start_services
    wait_for_services
}

# Run main function with all arguments
main "$@" 
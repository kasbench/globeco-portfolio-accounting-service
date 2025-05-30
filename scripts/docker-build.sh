#!/bin/bash

# Docker build script for GlobeCo Portfolio Accounting Service
# Supports multiple build targets and platforms

set -euo pipefail

# Configuration
IMAGE_NAME="globeco/portfolio-accounting-service"
REGISTRY="${DOCKER_REGISTRY:-}"
VERSION="${VERSION:-latest}"
PLATFORM="${PLATFORM:-linux/amd64}"
BUILD_ARGS="${BUILD_ARGS:-}"

# Build targets
TARGET="${1:-production}"
PUSH="${2:-false}"

# Colors for output
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
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

# Validate build target
validate_target() {
    case $TARGET in
        production|development|testing|cli)
            log_info "Building target: $TARGET"
            ;;
        *)
            log_error "Invalid build target: $TARGET"
            log_info "Valid targets: production, development, testing, cli"
            exit 1
            ;;
    esac
}

# Build Docker image
build_image() {
    local image_tag="${IMAGE_NAME}:${VERSION}-${TARGET}"
    
    if [ -n "$REGISTRY" ]; then
        image_tag="${REGISTRY}/${image_tag}"
    fi

    log_info "Building Docker image: $image_tag"
    log_info "Platform: $PLATFORM"
    
    # Build command
    docker build \
        --platform="$PLATFORM" \
        --target="$TARGET" \
        --tag="$image_tag" \
        --tag="${IMAGE_NAME}:latest-${TARGET}" \
        --build-arg BUILDKIT_INLINE_CACHE=1 \
        --build-arg VERSION="$VERSION" \
        --build-arg BUILD_DATE="$(date -u +'%Y-%m-%dT%H:%M:%SZ')" \
        --build-arg VCS_REF="$(git rev-parse --short HEAD 2>/dev/null || echo 'unknown')" \
        $BUILD_ARGS \
        .

    log_info "Successfully built: $image_tag"
    
    # Show image size
    docker images "$image_tag" --format "table {{.Repository}}\t{{.Tag}}\t{{.Size}}"
}

# Push Docker image
push_image() {
    if [ "$PUSH" = "true" ]; then
        local image_tag="${IMAGE_NAME}:${VERSION}-${TARGET}"
        
        if [ -n "$REGISTRY" ]; then
            image_tag="${REGISTRY}/${image_tag}"
        fi
        
        log_info "Pushing Docker image: $image_tag"
        docker push "$image_tag"
        
        # Also push latest tag
        docker push "${IMAGE_NAME}:latest-${TARGET}"
        
        log_info "Successfully pushed: $image_tag"
    fi
}

# Security scan (if trivy is available)
security_scan() {
    if command -v trivy &> /dev/null; then
        local image_tag="${IMAGE_NAME}:${VERSION}-${TARGET}"
        
        log_info "Running security scan with Trivy..."
        trivy image \
            --severity HIGH,CRITICAL \
            --exit-code 1 \
            "$image_tag" || {
            log_warn "Security scan found vulnerabilities"
            log_info "Run 'trivy image $image_tag' for detailed report"
        }
    else
        log_warn "Trivy not found - skipping security scan"
        log_info "Install Trivy for security scanning: https://trivy.dev/"
    fi
}

# Test Docker image
test_image() {
    local image_tag="${IMAGE_NAME}:${VERSION}-${TARGET}"
    
    log_info "Testing Docker image: $image_tag"
    
    case $TARGET in
        production)
            # Test health check
            log_info "Testing health check..."
            docker run --rm "$image_tag" /usr/local/bin/server --version 2>/dev/null || {
                log_error "Health check failed"
                return 1
            }
            ;;
        development)
            # Test development tools
            log_info "Testing development tools..."
            docker run --rm --entrypoint="" "$image_tag" go version || {
                log_error "Go not available in development image"
                return 1
            }
            ;;
        cli)
            # Test CLI
            log_info "Testing CLI..."
            docker run --rm "$image_tag" --version || {
                log_error "CLI test failed"
                return 1
            }
            ;;
        testing)
            # Run tests
            log_info "Running tests in container..."
            docker run --rm "$image_tag" || {
                log_error "Tests failed"
                return 1
            }
            ;;
    esac
    
    log_info "Image tests passed"
}

# Build multi-platform images
build_multiplatform() {
    local image_tag="${IMAGE_NAME}:${VERSION}-${TARGET}"
    
    if [ -n "$REGISTRY" ]; then
        image_tag="${REGISTRY}/${image_tag}"
    fi

    log_info "Building multi-platform image: $image_tag"
    log_info "Platforms: linux/amd64,linux/arm64"
    
    # Create and use buildx builder
    docker buildx create --name multiarch-builder --use 2>/dev/null || true
    
    # Build and push multi-platform image
    docker buildx build \
        --platform linux/amd64,linux/arm64 \
        --target="$TARGET" \
        --tag="$image_tag" \
        --build-arg BUILDKIT_INLINE_CACHE=1 \
        --build-arg VERSION="$VERSION" \
        --build-arg BUILD_DATE="$(date -u +'%Y-%m-%dT%H:%M:%SZ')" \
        --build-arg VCS_REF="$(git rev-parse --short HEAD 2>/dev/null || echo 'unknown')" \
        $BUILD_ARGS \
        ${PUSH:+--push} \
        .

    log_info "Successfully built multi-platform image: $image_tag"
}

# Cleanup
cleanup() {
    log_info "Cleaning up build cache..."
    docker builder prune -f || true
}

# Show usage
usage() {
    echo "Usage: $0 [TARGET] [PUSH]"
    echo ""
    echo "TARGET:"
    echo "  production  - Minimal production image (default)"
    echo "  development - Development image with tools"
    echo "  testing     - Testing image with test execution"
    echo "  cli         - CLI-only image"
    echo ""
    echo "PUSH:"
    echo "  true   - Push image to registry"
    echo "  false  - Build only (default)"
    echo ""
    echo "Environment Variables:"
    echo "  DOCKER_REGISTRY - Docker registry URL"
    echo "  VERSION         - Image version tag (default: latest)"
    echo "  PLATFORM        - Build platform (default: linux/amd64)"
    echo "  BUILD_ARGS      - Additional docker build arguments"
    echo "  MULTIPLATFORM   - Build for multiple platforms (linux/amd64,linux/arm64)"
    echo ""
    echo "Examples:"
    echo "  $0 production"
    echo "  $0 development true"
    echo "  VERSION=v1.0.0 $0 production true"
    echo "  MULTIPLATFORM=true $0 production true"
}

# Main execution
main() {
    if [ "$1" = "--help" ] || [ "$1" = "-h" ]; then
        usage
        exit 0
    fi

    log_info "Starting Docker build process..."
    log_info "Target: $TARGET, Version: $VERSION, Push: $PUSH"
    
    validate_target
    
    if [ "${MULTIPLATFORM:-false}" = "true" ]; then
        build_multiplatform
    else
        build_image
        test_image
        security_scan
        push_image
    fi
    
    cleanup
    
    log_info "Docker build process completed successfully!"
}

# Run main function with all arguments
main "$@" 
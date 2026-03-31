#!/bin/bash
set -e

SCRIPT_DIR="$(cd "$(dirname "$0")" && pwd)"
PROJECT_DIR="$(dirname "$SCRIPT_DIR")"

usage() {
    echo "Usage: $0 <command>"
    echo ""
    echo "Commands:"
    echo "  build-entrypoint   Build kilo-entrypoint binary"
    echo "  build-host         Build kilo-docker host binary"
    echo "  build-all          Build both binaries"
    echo "  test               Run Go tests"
    echo "  tidy               Run go mod tidy"
    echo "  docker-build       Build Docker image"
    echo "  clean              Remove built binaries"
    exit 1
}

if [ $# -eq 0 ]; then
    usage
fi

COMMAND="$1"

run_go() {
    docker run --rm \
        -u "$(id -u):$(id -g)" \
        -v "${PROJECT_DIR}:/build" \
        -w /build \
        -e GOCACHE=/build/.cache/go-build \
        -e GOMODCACHE=/build/.cache/mod \
        golang:1.26-alpine \
        "$@"
}

run_go_env() {
    docker run --rm \
        -u "$(id -u):$(id -g)" \
        -v "${PROJECT_DIR}:/build" \
        -w /build \
        -e GOCACHE=/build/.cache/go-build \
        -e GOMODCACHE=/build/.cache/mod \
        -e CGO_ENABLED=0 \
        golang:1.26-alpine \
        "$@"
}

case "$COMMAND" in
    build-entrypoint)
        echo "Building kilo-entrypoint..."
        run_go_env go build -ldflags="-s -w" -o /build/bin/kilo-entrypoint ./cmd/kilo-entrypoint
        echo "Binary: bin/kilo-entrypoint"
        ;;
    build-host)
        echo "Building kilo-docker host binary..."
        run_go_env go build -ldflags="-s -w" -o /build/bin/kilo-docker ./cmd/kilo-docker
        echo "Binary: bin/kilo-docker"
        ;;
    build-all)
        echo "Building all binaries..."
        mkdir -p "${PROJECT_DIR}/bin"
        run_go_env go build -ldflags="-s -w" -o /build/bin/kilo-entrypoint ./cmd/kilo-entrypoint
        run_go_env go build -ldflags="-s -w" -o /build/bin/kilo-docker ./cmd/kilo-docker
        echo "Binaries: bin/kilo-entrypoint, bin/kilo-docker"
        ;;
    test)
        echo "Running tests..."
        run_go go test -v ./cmd/...
        ;;
    tidy)
        echo "Running go mod tidy..."
        run_go go mod tidy
        ;;
    docker-build)
        echo "Building Docker image..."
        docker build -t ghcr.io/mbabic84/kilo-docker:latest "${PROJECT_DIR}"
        echo "Image: ghcr.io/mbabic84/kilo-docker:latest"
        ;;
    clean)
        echo "Cleaning..."
        rm -rf "${PROJECT_DIR}/bin"
        echo "Done."
        ;;
    *)
        echo "Unknown command: $COMMAND"
        usage
        ;;
esac

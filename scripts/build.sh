#!/bin/sh
set -e

SCRIPT_DIR="$(cd "$(dirname "$0")" && pwd)"
PROJECT_DIR="$(dirname "$SCRIPT_DIR}")"

NO_CACHE=""
IMAGE_TAG="ghcr.io/mbabic84/kilo-docker:latest"

usage() {
    echo "Usage: $0 <command> [--no-cache]"
    echo ""
    echo "Commands:"
    echo "  entrypoint   Build kilo-entrypoint binary"
    echo "  host         Build kilo-docker host binary"
    echo "  all          Build both binaries and Docker image"
    echo "  test         Run Go tests"
    echo "  tidy         Run go mod tidy"
    echo "  docker       Build Docker image"
    echo "  clean        Remove built binaries"
    echo ""
    echo "Options:"
    echo "  --no-cache   Build Docker image without using cache"
    exit 1
}

if [ $# -eq 0 ]; then
    usage
fi

COMMAND=""
while [ $# -gt 0 ]; do
    case "$1" in
        --no-cache)
            NO_CACHE="--no-cache"
            ;;
        *)
            COMMAND="$1"
            ;;
    esac
    shift
done

if [ -z "$COMMAND" ]; then
    usage
fi

run_go() {
    docker run --rm \
        -u "$(id -u):$(id -g)" \
        -v "${PROJECT_DIR}:/build" \
        -w /build \
        -e GOCACHE=/build/.cache/go-build \
        -e GOMODCACHE=/build/.cache/mod \
golang:1.26-bookworm \
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
         golang:1.26-bookworm \
        "$@"
}

build_image() {
    echo "Building Docker image..."
    docker build $NO_CACHE -t "${IMAGE_TAG}" "${PROJECT_DIR}"
    echo "Image: ${IMAGE_TAG}"
}

case "$COMMAND" in
    entrypoint)
        echo "Building kilo-entrypoint..."
        run_go_env go build -ldflags="-s -w" -o /build/bin/kilo-entrypoint ./cmd/kilo-entrypoint
        echo "Binary: bin/kilo-entrypoint"
        ;;
    host)
        echo "Building kilo-docker host binary..."
        run_go_env go build -ldflags="-s -w" -o /build/bin/kilo-docker ./cmd/kilo-docker
        echo "Binary: bin/kilo-docker"
        ;;
    all)
        echo "Building all binaries..."
        mkdir -p "${PROJECT_DIR}/bin"
        run_go_env go build -ldflags="-s -w" -o /build/bin/kilo-entrypoint ./cmd/kilo-entrypoint
        run_go_env go build -ldflags="-s -w" -o /build/bin/kilo-docker ./cmd/kilo-docker
        echo "Binaries: bin/kilo-entrypoint, bin/kilo-docker"
        echo ""
        build_image
        ;;
    test)
        echo "Running tests..."
        run_go go test -v ./cmd/...
        ;;
    tidy)
        echo "Running go mod tidy..."
        run_go go mod tidy
        ;;
    docker)
        build_image
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

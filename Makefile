.PHONY: build build-backend clean dev test frontend frontend-dev lint

VERSION ?= $(shell git describe --tags --always --dirty 2>/dev/null || echo dev)

build: frontend-safe embed-frontend
	cd go/src && go build -ldflags "-X main.version=$(VERSION)" -o ../bin/flux ./cmd/flux

# Build only the Go binary without rebuilding the frontend.
# Used when frontend hasn't changed or when npm/node is unavailable.
build-backend:
	cd go/src && go build -ldflags "-X main.version=$(VERSION)" -o ../bin/flux ./cmd/flux

embed-frontend:
	@if [ -d frontend/dist ]; then \
		rm -rf go/src/web/dist; \
		cp -r frontend/dist go/src/web/dist; \
	else \
		echo "WARNING: frontend/dist not found, using existing embedded frontend"; \
	fi

clean:
	rm -rf go/bin/*
	rm -rf frontend/dist
	rm -rf go/src/web/dist
	mkdir -p go/src/web/dist && echo ".gitkeep" > go/src/web/dist/.gitkeep

dev:
	cd go/src && go run -ldflags "-X main.version=$(VERSION)" ./cmd/flux --config ../../config.yaml

test:
	cd go/src && go test ./...

frontend:
	cd frontend && npm run build

# frontend-safe builds the frontend but does not fail the overall build if npm/node is missing.
# This is critical for auto-deploy via launchd where PATH may not include nvm-managed node.
frontend-safe:
	@if command -v npm >/dev/null 2>&1; then \
		echo "Building frontend..."; \
		cd frontend && npm run build; \
	else \
		echo "WARNING: npm not found in PATH, skipping frontend build"; \
		echo "  The existing embedded frontend will be used."; \
		echo "  To build frontend manually: make frontend"; \
	fi

frontend-dev:
	cd frontend && npm run dev

lint:
	cd go/src && go vet ./...

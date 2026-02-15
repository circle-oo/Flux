.PHONY: build clean dev test frontend frontend-dev lint

VERSION ?= $(shell git describe --tags --always --dirty 2>/dev/null || echo dev)

build: frontend embed-frontend
	cd go/src && go build -ldflags "-X main.version=$(VERSION)" -o ../bin/flux ./cmd/flux

embed-frontend:
	rm -rf go/src/web/dist
	cp -r react/flux-ui/dist go/src/web/dist

clean:
	rm -rf go/bin/*
	rm -rf react/flux-ui/dist
	rm -rf go/src/web/dist
	mkdir -p go/src/web/dist && echo ".gitkeep" > go/src/web/dist/.gitkeep

dev:
	cd go/src && go run -ldflags "-X main.version=$(VERSION)" ./cmd/flux --config ../../config.yaml

test:
	cd go/src && go test ./...

frontend:
	cd react/flux-ui && npm run build

frontend-dev:
	cd react/flux-ui && npm run dev

lint:
	cd go/src && go vet ./...

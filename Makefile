.PHONY: build clean dev test frontend frontend-dev lint

VERSION ?= $(shell git describe --tags --always --dirty 2>/dev/null || echo dev)

build: frontend embed-frontend
	cd go && go build -ldflags "-X main.version=$(VERSION)" -o ../bin/flux ./cmd/flux

embed-frontend:
	rm -rf go/web/dist
	cp -r typescript/dist go/web/dist

clean:
	rm -rf bin/
	rm -rf typescript/dist
	rm -rf go/web/dist
	mkdir -p go/web/dist && echo ".gitkeep" > go/web/dist/.gitkeep

dev:
	cd go && go run -ldflags "-X main.version=$(VERSION)" ./cmd/flux --config ../config.yaml

test:
	cd go && go test ./...

frontend:
	cd typescript && npm run build

frontend-dev:
	cd typescript && npm run dev

lint:
	cd go && go vet ./...

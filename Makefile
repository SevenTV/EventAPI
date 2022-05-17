all: linux

BUILDER := "unknown"
VERSION := "unknown"

ifeq ($(origin EVENTS_BUILDER),undefined)
	BUILDER = $(shell git config --get user.name);
else
	BUILDER = ${EVENTS_BUILDER};
endif

ifeq ($(origin EVENTS_VERSION),undefined)
	VERSION = $(shell git rev-parse HEAD);
else
	VERSION = ${EVENTS_VERSION};
endif

linux:
	GOOS=linux GOARCH=amd64 go build -v -ldflags "-X 'main.Version=${VERSION}' -X 'main.Unix=$(shell date +%s)' -X 'main.User=${BUILDER}'" -o out/events cmd/*.go

lint:
	staticcheck ./...
	go vet ./...
	golangci-lint run --go=1.18
	yarn prettier --check .

format:
	gofmt -s -w .
	yarn prettier --write .

deps:
	go mod download
	go install honnef.co/go/tools/cmd/staticcheck@2022.1
	go install github.com/golangci/golangci-lint/cmd/golangci-lint@latest
	yarn

test:
	go test -count=1 -cover ./...

work:
	echo "go 1.18\n\nuse (\n\t.\n\t../Common\n)" > go.work
	go mod tidy

dev:
	go run cmd/main.go

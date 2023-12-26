all: build

.PHONY: build
build:
	CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -v -ldflags '-w -s' -o hanprom-linux-amd64 ./cmd/hanprom
	CGO_ENABLED=0 GOOS=linux GOARCH=arm64 go build -v -ldflags '-w -s' -o hanprom-linux-arm64 ./cmd/hanprom

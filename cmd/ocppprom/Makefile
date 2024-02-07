.PHONY: build
build:
	CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -v -ldflags '-w -s' -o ../../bin/ocppprom-linux-amd64
	CGO_ENABLED=0 GOOS=linux GOARCH=arm64 go build -v -ldflags '-w -s' -o ../../bin/ocppprom-linux-arm64

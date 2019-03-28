IMAGE   ?= hub.global.cloud.sap/monsoon/whale
VERSION = $(shell git rev-parse --verify HEAD | head -c 8)

GOOS    ?= $(shell go env | grep GOOS | cut -d'"' -f2)
BINARY  := whale

LDFLAGS := -X github.com/sapcc/whale/pkg/whale.VERSION=$(VERSION)
GOFLAGS := -ldflags "$(LDFLAGS)"

all: bin/$(GOOS)/$(BINARY)

bin/%/$(BINARY): $(GOFILES) Makefile
	GOOS=$* GOARCH=amd64 go build $(GOFLAGS) -v -i -o bin/$*/$(BINARY) ./cmd

build: 
	docker build -t $(IMAGE):$(VERSION) .

push: build
	docker push $(IMAGE):$(VERSION)

clean:
	rm -rf bin/*

vendor:
	go get -u ./... && go mod tidy && go mod vendor

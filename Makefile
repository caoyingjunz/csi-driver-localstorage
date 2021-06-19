TARGET_DIR ?= ./dist
GOPROXY ?= https://goproxy.cn,direct
ARCH ?= amd64
OS ?= linux
apps ?= $(shell ls cmd)

.PHONY: build client-gen vendor

build: client-gen
	for app in $(apps); do \
		CGO_ENABLED=0 GOOS=$(OS) GOARCH=$(ARCH) GOPROXY=$(GOPROXY) go build -o $(TARGET_DIR)/$(ARCH)/ ./cmd/$$app ;\
	done

client-gen: vendor
	./hack/update-codegen.sh

vendor:
	go mod vendor

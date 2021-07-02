TARGET_DIR ?= ./dist
GOPROXY ?= https://goproxy.cn,direct
ARCH ?= amd64
OS ?= linux
apps ?= $(shell ls cmd)
BUILDX ?= false
PLATFORM ?= linux/amd64,linux/arm64
ORG ?= jacky06
TAG ?= v0.0.1
WHAT ?= all # selected all apps
ifneq ($(filter $(WHAT),$(apps)),)
	apps := $(WHAT)
endif

.PHONY: build client-gen vendor image

build:
	@$(foreach app, $(apps), CGO_ENABLED=0 GOOS=$(OS) GOARCH=$(ARCH) GOPROXY=$(GOPROXY) go build -o $(TARGET_DIR)/$(ARCH)/ ./cmd/$(app);)

client-gen: vendor
	./hack/update-codegen.sh

vendor:
	go mod vendor

image:
ifeq ($(BUILDX), false)
	@$(foreach app, $(apps), \
	docker build \
		--build-arg GOPROXY=$(GOPROXY) \
		--build-arg APP=$(app) \
		--force-rm \
		--no-cache \
		-t $(ORG)/$(app):$(TAG) \
		.;)
else
	@$(foreach app, $(apps), \
	docker buildx build \
			--build-arg GOPROXY=$(GOPROXY) \
			--build-arg APP=$(app) \
			--force-rm \
			--no-cache \
			--platform $(PLATFORM) \
			--push \
			-t $(ORG)/$(app):$(TAG) \
			.;)
endif

push:
	@$(foreach app, $(apps), docker push $(ORG)/$(app):$(TAG);)

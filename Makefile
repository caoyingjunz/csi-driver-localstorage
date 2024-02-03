.PHONY: run build image push vendor client-gen clean

dockerhubUser = harbor.cloud.pixiuio.com/pixiuio
tag = latest
app ?=
ifeq ($(app),)
apps = $(shell ls cmd)
else
apps = $(app)
endif
targetDir ?= dist
buildx ?= false
dualPlatform ?= linux/amd64,linux/arm64

# check if app name is valid
check:
	@$(foreach app, $(apps), \
		if [ ! -d "cmd/$(app)" ]; then \
			echo "cmd/$(app) does not exist"; \
			echo "Please check your app name"; \
			exit 1; \
		fi;)

# build all apps
build: check
	@$(foreach app, $(apps), \
		echo "Building $(app)"; \
		CGO_ENABLED=0 go build -ldflags "-s -w" -o $(targetDir)/$(app) ./cmd/$(app);)

# build all images
image: check
ifeq ($(buildx), false)
	@$(foreach app, $(apps), \
		echo "Building $(app) image"; \
		docker build -t $(dockerhubUser)/$(app):$(tag) --no-cache --build-arg APP=$(app) \
		.;)
else ifeq ($(buildx), true)
	@$(foreach app, $(apps), \
		echo "Building $(app) multi-arch image"; \
		docker buildx build -t $(dockerhubUser)/$(app):$(tag) --no-cache --platform $(dualPlatform) --push --build-arg APP=$(app) \
		.;)
endif

# push all images
push: image
	@$(foreach app, $(apps), \
		echo "Pushing $(app) image"; \
		docker push $(dockerhubUser)/$(app):$(tag);)

# install vendor
vendor:
	go mod vendor

# generate client
client-gen: vendor
	./hack/update-codegen.sh

# rm vendor and github.com
clean:
	rm -rf vendor && rm -rf github.com

# show help
help:
	@echo ''
	@echo 'Usage:'
	@echo ' make [target]'
	@echo ''
	@echo 'Targets:'
	@awk '/^[a-zA-Z\-\_0-9]+:/ { \
	helpMessage = match(lastLine, /^# (.*)/); \
		if (helpMessage) { \
			helpCommand = substr($$1, 0, index($$1, ":")-1); \
			helpMessage = substr(lastLine, RSTART + 2, RLENGTH); \
			printf "\033[36m%-22s\033[0m %s\n", helpCommand,helpMessage; \
		} \
	} \
	{ lastLine = $$0 }' $(MAKEFILE_LIST)

.DEFAULT_GOAL := help

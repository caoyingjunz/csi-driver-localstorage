.PHONY: run image push vendor client-gen clean

dockerhubUser = harbor.powerlaw.club/pixiuio
tag = latest
apps ?= $(shell ls cmd)
appName ?= $(app)

# check if app name is valid
check:
	@if [ ! -d "cmd/$(appName)" ]; then \
		echo "cmd/$(appName) not exist"; \
		echo "Please check your app name"; \
		for app in $(apps); do \
			echo "Example: make xxx app=$$app"; \
			break; \
		done; \
		exit 1; \
	fi

# build all images
image: check
	@if [ -z "$(appName)" ]; then \
		for app in $(apps); do \
          	echo "Building $$app"; \
        	docker build -t $(dockerhubUser)/$$app:$(tag) --no-cache --build-arg APP=$$app .; \
    	done \
    else \
		echo "Building $(appName)"; \
		docker build -t $(dockerhubUser)/$(appName):$(tag) --no-cache --build-arg APP=$(appName) .; \
	fi

# push all images
push: image
	@if [ -z "$(appName)" ]; then \
		for app in $(apps); do \
			echo "Pushing $$app"; \
			docker push $(dockerhubUser)/$$app:$(tag); \
		done \
    else \
		echo "Pushing $(appName)"; \
		docker push $(dockerhubUser)/$(appName):$(tag); \
    fi

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

.PHONY: run image push vendor client-gen clean

dockerhubUser = pixiuio
tag = latest
apps ?= $(shell ls cmd)

# build all images
image:
	for app in $(apps); do \
  		echo "Building $$app"; \
		docker build -t $(dockerhubUser)/$$app:$(tag) --build-arg APP=$$app .; \
	done

# push all images
push: image
	for app in $(apps); do \
  		echo "Pushing $$app"; \
		docker push $(dockerhubUser)/$$app:$(tag); \
	done

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

.PHONY: run image push vendor client-gen clean

dockerhubUser = pixiuio
releaseName = lsplugin
tag = latest

image:
	docker build -t $(dockerhubUser)/$(releaseName):$(tag) .

push: image
	docker push $(dockerhubUser)/$(releaseName):$(tag)

vendor:
	go mod vendor

client-gen: vendor
	./hack/update-codegen.sh

clean:
	rm -rf vendor && rm -rf github.com

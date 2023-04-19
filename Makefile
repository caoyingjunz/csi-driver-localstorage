.PHONY: run image push

dockerhubUser = pixiuio
releaseName = lsplugin
tag = latest

image:
	docker build -t $(dockerhubUser)/$(releaseName):$(tag) .

push: image
	docker push $(dockerhubUser)/$(releaseName):$(tag)

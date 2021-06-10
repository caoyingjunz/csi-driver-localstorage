module github.com/caoyingjunz/pixiu

go 1.14

require (
	github.com/hashicorp/golang-lru v0.5.4 // indirect
	github.com/onsi/ginkgo v1.14.1 // indirect
	github.com/onsi/gomega v1.10.2 // indirect
	google.golang.org/appengine v1.6.6 // indirect
	k8s.io/api v0.21.1
	k8s.io/apimachinery v0.21.1
	k8s.io/client-go v0.21.1
	k8s.io/code-generator v0.21.1
	k8s.io/component-base v0.21.1
	k8s.io/klog/v2 v2.9.0
	k8s.io/utils v0.0.0-20201110183641-67b214c5f920
)

replace k8s.io/code-generator => k8s.io/code-generator v0.0.0-20210604114248-ed0f8d04eed3

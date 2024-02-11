package main

import (
	"flag"
	"time"

	"github.com/caoyingjunz/csi-driver-localstorage/pkg/controller/pvc"
	"github.com/caoyingjunz/csi-driver-localstorage/pkg/signals"
	"github.com/caoyingjunz/csi-driver-localstorage/pkg/util"
	"k8s.io/client-go/informers"
	"k8s.io/client-go/kubernetes"
	"k8s.io/klog/v2"
)

const (
	workers = 5
)

var (
	kubeconfig   = flag.String("kubeconfig", "", "paths to a kubeconfig. Only required if out-of-cluster.")
	kubeAPIQPS   = flag.Int("kube-api-qps", 5, "QPS to use while communicating with the kubernetes apiserver. Defaults to 5")
	kubeAPIBurst = flag.Int("kube-api-burst", 10, "Burst to use while communicating with the kubernetes apiserver. Defaults to 10.")
)

func init() {
	_ = flag.Set("logtostderr", "true")
}

func main() {
	klog.InitFlags(nil)
	flag.Parse()

	// set up signals so we handle the shutdown signal gracefully
	ctx := signals.SetupSignalHandler()

	kubeConfig, err := util.BuildClientConfig(*kubeconfig)
	if err != nil {
		klog.Fatalf("Failed to build kube config: %v", err)
	}
	kubeConfig.QPS = float32(*kubeAPIQPS)
	kubeConfig.Burst = *kubeAPIBurst

	kubeClient, err := kubernetes.NewForConfig(kubeConfig)
	if err != nil {
		klog.Fatalf("Failed to build kube clientSet: %v", err)
	}

	factory := informers.NewSharedInformerFactory(kubeClient, 300*time.Second)
	pvcInformer := factory.Core().V1().PersistentVolumeClaims()

	pc := pvc.NewPVCController(ctx, kubeClient, pvcInformer)

	factory.Start(ctx.Done())
	factory.WaitForCacheSync(ctx.Done())

	klog.Info("Starting pvc controller")
	go pc.Run(ctx, workers)

	select {}
}

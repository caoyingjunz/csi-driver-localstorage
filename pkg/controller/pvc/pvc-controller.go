package pvc

import (
	"context"
	"fmt"
	"strconv"
	"time"

	"github.com/caoyingjunz/csi-driver-localstorage/pkg/client/clientset/versioned/scheme"
	"github.com/caoyingjunz/csi-driver-localstorage/pkg/util"
	"github.com/houwenchen/libs/iolimit"
	v1core "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	"k8s.io/apimachinery/pkg/util/wait"
	v1 "k8s.io/client-go/informers/core/v1"
	"k8s.io/client-go/kubernetes"
	typedv1 "k8s.io/client-go/kubernetes/typed/core/v1"
	corev1 "k8s.io/client-go/listers/core/v1"
	"k8s.io/client-go/tools/cache"
	"k8s.io/client-go/tools/record"
	"k8s.io/client-go/util/workqueue"
	"k8s.io/klog/v2"
)

const (
	maxRetries = 15
)

type PVCController struct {
	kubeClient kubernetes.Interface

	eventBroadcaster record.EventBroadcaster
	eventRecorder    record.EventRecorder

	syncHandler func(ctx context.Context, dKey string) error
	handleError func(ctx context.Context, err error, dKey interface{})
	enqueuePVC  func(pvc *v1core.PersistentVolumeClaim)

	pvcLister       corev1.PersistentVolumeClaimLister
	pvcListerSynced cache.InformerSynced

	queue workqueue.RateLimitingInterface
}

func NewPVCController(ctx context.Context, kubeClientSet kubernetes.Interface, pvcInformer v1.PersistentVolumeClaimInformer) *PVCController {
	eventBroadcaster := record.NewBroadcaster()
	eventBroadcaster.StartLogging(klog.Infof)
	eventBroadcaster.StartRecordingToSink(&typedv1.EventSinkImpl{Interface: kubeClientSet.CoreV1().Events("")})

	pc := &PVCController{
		kubeClient:       kubeClientSet,
		eventBroadcaster: eventBroadcaster,
		eventRecorder:    eventBroadcaster.NewRecorder(scheme.Scheme, v1core.EventSource{Component: util.LocalstorageManagerUserAgent}),
		pvcLister:        pvcInformer.Lister(),
		queue:            workqueue.NewNamedRateLimitingQueue(workqueue.DefaultControllerRateLimiter(), "pvc-controller"),
	}

	pvcInformer.Informer().AddEventHandler(cache.ResourceEventHandlerFuncs{
		AddFunc:    pc.addPVC,
		UpdateFunc: pc.updatePVC,
		DeleteFunc: pc.deletePVC,
	})

	pc.syncHandler = pc.syncPVC
	pc.handleError = pc.handleErr
	pc.enqueuePVC = pc.enqueue

	pc.pvcListerSynced = pvcInformer.Informer().HasSynced

	return pc
}

func (pc *PVCController) addPVC(obj interface{}) {
	pvc, ok := obj.(*v1core.PersistentVolumeClaim)
	if !ok {
		utilruntime.HandleError(fmt.Errorf("expected pvc in addPVC, but got %#v", obj))
		return
	}

	klog.V(2).Info("Adding pvc", "pvc", klog.KObj(pvc))
	pc.enqueuePVC(pvc)
}

func (pc *PVCController) updatePVC(oldObj interface{}, newObj interface{}) {
	oldPVC := oldObj.(*v1core.PersistentVolumeClaim)
	curPVC := newObj.(*v1core.PersistentVolumeClaim)

	klog.V(2).Info("Updating pvc", "pvc", klog.KObj(oldPVC))
	pc.enqueuePVC(curPVC)
}

func (pc *PVCController) deletePVC(obj interface{}) {
	pvc, ok := obj.(*v1core.PersistentVolumeClaim)
	if !ok {
		tombstone, ok := obj.(cache.DeletedFinalStateUnknown)
		if !ok {
			utilruntime.HandleError(fmt.Errorf("couldn't get object from tombstone %#v", obj))
			return
		}
		pvc, ok = tombstone.Obj.(*v1core.PersistentVolumeClaim)
		if !ok {
			utilruntime.HandleError(fmt.Errorf("tombstone contained object that is not a pvc %#v", obj))
			return
		}
	}

	klog.V(2).Info("Deleting pvc", "pvc", klog.KObj(pvc))
	pc.enqueuePVC(pvc)
}

func (pc *PVCController) enqueue(pvc *v1core.PersistentVolumeClaim) {
	key, err := cache.MetaNamespaceKeyFunc(pvc)
	if err != nil {
		utilruntime.HandleError(fmt.Errorf("couldn't get key for object %#v: %v", pvc, err))
		return
	}

	pc.queue.Add(key)
}

func (pc *PVCController) handleErr(ctx context.Context, err error, key interface{}) {
	if err == nil || errors.HasStatusCause(err, v1core.NamespaceTerminatingCause) {
		pc.queue.Forget(key)
		return
	}
	ns, name, keyErr := cache.SplitMetaNamespaceKey(key.(string))
	if keyErr != nil {
		klog.Error(err, "Failed to split meta namespace cache key", "cacheKey", key)
	}

	if pc.queue.NumRequeues(key) < maxRetries {
		klog.V(2).Info("Error syncing pvc", "pvc", klog.KRef(ns, name), "err", err)
		pc.queue.AddRateLimited(key)
		return
	}

	utilruntime.HandleError(err)
	klog.V(2).Info("Dropping pvc out of the queue", "pvc", klog.KRef(ns, name), "err", err)
	pc.queue.Forget(key)
}

func (pc *PVCController) Run(ctx context.Context, workers int) {
	defer utilruntime.HandleCrash()
	defer pc.eventBroadcaster.Shutdown()
	defer pc.queue.ShutDown()

	klog.Infof("Starting PVC Manager")
	defer klog.Infof("Shutting down PVC Manager")

	if !cache.WaitForNamedCacheSync("pvc-manager", ctx.Done(), pc.pvcListerSynced) {
		return
	}

	for i := 0; i < workers; i++ {
		go wait.UntilWithContext(ctx, pc.worker, time.Second)
	}

	<-ctx.Done()
}

func (pc *PVCController) worker(ctx context.Context) {
	for pc.processNextWorkItem(ctx) {
	}
}

func (pc *PVCController) processNextWorkItem(ctx context.Context) bool {
	key, quit := pc.queue.Get()
	if quit {
		return false
	}
	defer pc.queue.Done(key)

	err := pc.syncHandler(ctx, key.(string))
	pc.handleError(ctx, err, key)

	return true
}

// 根据 pvc 的 annotations 下发 iolimit 配置
func (pc *PVCController) syncPVC(ctx context.Context, dKey string) error {
	startTime := time.Now()
	klog.V(2).InfoS("Started syncing pvc manager", "pvc", "startTime", startTime)
	defer func() {
		klog.V(2).InfoS("Finished syncing pvc manager", "pvc", "duration", time.Since(startTime))
	}()

	namespace, name, err := cache.SplitMetaNamespaceKey(dKey)
	if err != nil {
		klog.Errorf("failed to split meta namespace cache key: %s, err: %s", dKey, err)
		return err
	}

	pvc, err := pc.pvcLister.PersistentVolumeClaims(namespace).Get(name)
	if err != nil {
		if errors.IsNotFound(err) {
			klog.V(2).Infof("pvc has been deleted", dKey)
			return nil
		}
		return err
	}

	// 等待 pvc 的状态为 bound
	if pvc.Status.Phase != v1core.ClaimBound {
		return fmt.Errorf("pvc's status isn't bound")
	}

	// 获取 使用此 pvc 的 pods
	pods, err := pc.kubeClient.CoreV1().Pods(namespace).List(ctx, metav1.ListOptions{
		FieldSelector: fmt.Sprintf("spec.volumes.persistentVolumeClaim.claimName=%s", name),
	})
	if err != nil {
		return err
	}
	if len(pods.Items) == 0 {
		return fmt.Errorf("pvc isn't used by pod, needn't set iolimit")
	}
	klog.Infof("succeed to get pvc: %s, and it's ref pods: %#v", name, pods)

	// 获取 annotations
	deviceName, existDeviceName := pvc.Annotations["lvm/devicename"]
	rbps, existRbps := pvc.Annotations["cgroup.iolimit.rbps"]
	riops, existRiops := pvc.Annotations["cgroup.iolimit.riops"]
	wbps, existWbps := pvc.Annotations["cgroup.iolimit.wbps"]
	wiops, existWiops := pvc.Annotations["cgroup.iolimit.wiops"]

	// TODO: 后续 lvm 的逻辑合入后，使用新的方式获取对应的 device
	if !existDeviceName {
		return nil
	}
	if !existRbps && !existRiops && !existWbps && !existWiops {
		return fmt.Errorf("miss iolimit params")
	}

	// 遍历 pod，设置 iolimit
	for _, pod := range pods.Items {
		puid := pod.GetUID()

		ioInfo := &iolimit.IOInfo{
			Rbps:  parseUint(rbps),
			Riops: parseUint(riops),
			Wbps:  parseUint(wbps),
			Wiops: parseUint(wiops),
		}

		cgroupVersion, err := iolimit.GetCGroupVersion()
		if err != nil {
			return err
		}

		switch cgroupVersion {
		case iolimit.CGroupV1:
			iolimit, err := iolimit.NewIOLimitV1(iolimit.CGroupV1, string(puid), ioInfo, deviceName)
			if err != nil {
				return err
			}
			if err := iolimit.SetIOLimit(); err != nil {
				return err
			}
		case iolimit.CGroupV2:
			iolimit, err := iolimit.NewIOLimitV2(iolimit.CGroupV2, string(puid), ioInfo, deviceName)
			if err != nil {
				return err
			}
			if err := iolimit.SetIOLimit(); err != nil {
				return err
			}
		default:
			return fmt.Errorf("unsupport cgroup version")
		}
	}

	return nil
}

func parseUint(para string) uint64 {
	uint64Data, err := strconv.ParseUint(para, 10, 64)
	if err != nil {
		klog.Infof("parse iolimit data to uint format failed, %s", para)
		return 0
	}
	return uint64Data
}

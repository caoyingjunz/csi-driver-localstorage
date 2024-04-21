package metrics

import (
	"context"
	"net/http"
	"time"

	metaV1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/klog/v2"

	"github.com/caoyingjunz/csi-driver-localstorage/pkg/client/clientset/versioned"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
)

var (
	// volumesTotalGauge is the total number of localstorage volumes
	volumesTotalGauge = prometheus.NewGaugeVec(prometheus.GaugeOpts{
		Namespace: "localstorage_controller",
		Subsystem: "volumes",
		Name:      "total",
		Help:      "The total number of volumes in localstorage",
	}, []string{})

	// volumeSizeGauge is the size of localstorage volumes
	volumeSizeGauge = prometheus.NewGaugeVec(prometheus.GaugeOpts{
		Namespace: "localstorage_controller",
		Subsystem: "volume",
		Name:      "size",
		Help:      "The size of each volume in localstorage",
	}, []string{"volId", "volName", "volPath"})
)

func init() {
	prometheus.MustRegister(volumesTotalGauge, volumeSizeGauge)
}

// InstallHandler registers the prometheus handler
func InstallHandler(mux *http.ServeMux, path string) {
	mux.Handle(path, promhttp.Handler())
}

// RegisterVolumeSize registers the size of localstorage volumes
func RegisterVolumeSize(volId, volName, volPath string, size float64) {
	volumeSizeGauge.WithLabelValues(volId, volName, volPath).Set(size)
}

// RegisterVolumesGauge registers the total number of localstorage volumes
func RegisterVolumesGauge(total float64) {
	volumesTotalGauge.WithLabelValues().Set(total)
}

// TimingAcquisition is used to periodically obtain the volume indicator data
func TimingAcquisition(ctx context.Context, lsClientSet *versioned.Clientset, t time.Duration) {
	// The first execution
	if err := handleLocalStoragesList(ctx, lsClientSet); err != nil {
		klog.ErrorS(err, "Failed to get localstorage list")
	}

	go func() {
		ticker := time.NewTicker(t)
		defer ticker.Stop()
		for range ticker.C {
			if err := handleLocalStoragesList(ctx, lsClientSet); err != nil {
				klog.ErrorS(err, "Failed to get localstorage list")
			}
		}
	}()
}

// handleLocalStoragesList is used to obtain the volume indicator data
func handleLocalStoragesList(ctx context.Context, lsClientSet *versioned.Clientset) error {
	object, err := lsClientSet.StorageV1().LocalStorages().List(ctx, metaV1.ListOptions{})
	if err != nil {
		return err
	}

	volumeItems := object.Items
	var total float64
	for _, volumeItem := range volumeItems {
		volumes := volumeItem.Status.Volumes
		total += float64(len(volumes))
		for _, volume := range volumes {
			RegisterVolumeSize(volume.VolID, volume.VolName, volume.VolPath, float64(volume.VolSize))
		}
	}

	RegisterVolumesGauge(total)

	return nil
}

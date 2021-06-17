/*
Copyright 2021 The Pixiu Authors.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package options

import (
	"time"

	"github.com/spf13/cobra"
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	clientset "k8s.io/client-go/kubernetes"
	clientgokubescheme "k8s.io/client-go/kubernetes/scheme"
	v1core "k8s.io/client-go/kubernetes/typed/core/v1"
	"k8s.io/client-go/tools/record"
	componentbaseconfig "k8s.io/component-base/config"
	"k8s.io/klog/v2"

	"github.com/caoyingjunz/pixiu/cmd/pixiu-controller-manager/app/config"
	"github.com/caoyingjunz/pixiu/pkg/controller"
)

const (
	// AdvanceddeploymentControllerManagerUserAgent is the userAgent name when starting kubez-autoscaler managers.
	AdvanceddeploymentControllerManagerUserAgent = "advanceddeployment-manager"
)

// Options has all the params needed to run a Autoscaler
type Options struct {
	ComponentConfig config.PixiuConfiguration

	// ConfigFile is the location of the pixiu's configuration file.
	ConfigFile string

	Master string
}

func NewOptions() (*Options, error) {

	cfg := config.PixiuConfiguration{}
	o := &Options{
		ComponentConfig: cfg,
	}

	return o, nil
}

var (
	leaderElect       bool
	leaseDuration     int
	renewDeadline     int
	retryPeriod       int
	resourceLock      string
	resourceName      string
	resourceNamespace string

	// pprof vars
	startPprof bool
	pprofPort  string

	// log parameter
	verbosity string

	// healthz vars
	healthzHost string
	healthzPort string
)

const (
	LeaseDuration = 15
	RenewDeadline = 10
	RetryPeriod   = 2

	ResourceLock      = "endpointsleases"
	ResourceName      = "advanceddeployment-manager"
	ResourceNamespace = "kube-system"

	HealthzHost = "127.0.0.1"
	HealthzPort = "10256"
	PPort       = "8848"
)

// BindFlags binds the PixiuConfiguration struct fields
func (o *Options) BindFlags(cmd *cobra.Command) {
	// LeaderElection configuration
	cmd.Flags().BoolVarP(&leaderElect, "leader-elect", "l", true, ""+
		"Start a leader election client and gain leadership before "+
		"executing the main loop. Enable this when running replicated "+
		"components for high availability.")
	cmd.Flags().IntVarP(&leaseDuration, "leader-elect-lease-duration", "", LeaseDuration, ""+
		"The duration that non-leader candidates will wait after observing a leadership "+
		"renewal until attempting to acquire leadership of a led but unrenewed leader "+
		"slot. This is effectively the maximum duration that a leader can be stopped "+
		"before it is replaced by another candidate. This is only applicable if leader "+
		"election is enabled.")
	cmd.Flags().IntVarP(&renewDeadline, "leader-elect-renew-deadline", "", RenewDeadline, ""+
		"The interval between attempts by the acting master to renew a leadership slot "+
		"before it stops leading. This must be less than or equal to the lease duration. "+
		"This is only applicable if leader election is enabled.")
	cmd.Flags().IntVarP(&retryPeriod, "leader-elect-retry-period", "", RetryPeriod, ""+
		"The duration the clients should wait between attempting acquisition and renewal "+
		"of a leadership. This is only applicable if leader election is enabled.")
	cmd.Flags().StringVarP(&resourceLock, "leader-elect-resource-lock", "", ResourceLock, ""+
		"The type of resource object that is used for locking during "+
		"leader election. Supported options are `endpoints` (default) and `configmaps`.")
	cmd.Flags().StringVarP(&resourceName, "leader-elect-resource-name", "", ResourceName, ""+
		"The name of resource object that is used for locking during "+
		"leader election.")
	cmd.Flags().StringVarP(&resourceNamespace, "leader-elect-resource-namespace", "", ResourceNamespace, ""+
		"The namespace of resource object that is used for locking during "+
		"leader election.")

	// Log configuration
	cmd.Flags().StringVarP(&verbosity, "verbosity", "v", "0", "number for the log level verbosity")

	// ppof configuration
	cmd.Flags().BoolVarP(&startPprof, "start-pprof", "", true, ""+
		"Start pprof and gain leadership before executing the main loop")
	cmd.Flags().StringVarP(&pprofPort, "pprof-port", "", PPort, "The port of pprof to listen on")

	// Healthz configuration
	cmd.Flags().StringVarP(&healthzHost, "healthz-host", "", HealthzHost, "The host of Healthz")
	cmd.Flags().StringVarP(&healthzPort, "healthz-port", "", HealthzPort, "The port of Healthz to listen on")
}

func createRecorder(kubeClient clientset.Interface, userAgent string) record.EventRecorder {
	eventBroadcaster := record.NewBroadcaster()
	eventBroadcaster.StartLogging(klog.Infof)
	eventBroadcaster.StartRecordingToSink(&v1core.EventSinkImpl{Interface: kubeClient.CoreV1().Events("")})
	return eventBroadcaster.NewRecorder(clientgokubescheme.Scheme, v1.EventSource{Component: userAgent})
}

// Config return a kubez controller manager config objective
func (o *Options) Config() (*config.PixiuConfiguration, error) {
	kubeConfig, err := config.BuildKubeConfig()
	if err != nil {
		return nil, err
	}
	kubeConfig.QPS = 30000
	kubeConfig.Burst = 30000

	clientBuilder := controller.SimpleControllerClientBuilder{
		ClientConfig: kubeConfig,
	}

	client := clientBuilder.ClientOrDie("leader-client")
	eventRecorder := createRecorder(client, AdvanceddeploymentControllerManagerUserAgent)

	le := config.PixiuLeaderElectionConfiguration{
		componentbaseconfig.LeaderElectionConfiguration{
			LeaderElect:       leaderElect,
			LeaseDuration:     metav1.Duration{time.Duration(leaseDuration) * time.Second},
			RenewDeadline:     metav1.Duration{time.Duration(renewDeadline) * time.Second},
			RetryPeriod:       metav1.Duration{time.Duration(retryPeriod) * time.Second},
			ResourceLock:      resourceLock,
			ResourceName:      resourceName,
			ResourceNamespace: resourceNamespace,
		},
	}

	pp := config.PixiuPprof{
		Start: startPprof,
		Port:  pprofPort,
	}

	return &config.PixiuConfiguration{
		LeaderClient:   client,
		EventRecorder:  eventRecorder,
		LeaderElection: le,
		PixiuPprof:     pp,
		Healthz: config.HealthzConfiguration{
			HealthzHost: healthzHost,
			HealthzPort: healthzPort,
		},
	}, nil
}


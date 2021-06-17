package app
import (
	"context"
	"fmt"
	clientset "github.com/caoyingjunz/pixiu/pkg/client/clientset/versioned"
	adClientset "github.com/caoyingjunz/pixiu/pkg/generated/clientset/versioned"
	adInformers "github.com/caoyingjunz/pixiu/pkg/generated/informers/externalversions/advanceddeployment/v1alpha1"
	"net/http"
	"os"

	// import pprof for performance diagnosed
	_ "net/http/pprof"

	"github.com/spf13/cobra"
	"k8s.io/apimachinery/pkg/util/uuid"
	"k8s.io/client-go/tools/leaderelection"
	"k8s.io/client-go/tools/leaderelection/resourcelock"
	"k8s.io/klog/v2"

	"github.com/caoyingjunz/pixiu/cmd/pixiu-controller-manager/app/config"
	"github.com/caoyingjunz/pixiu/cmd/pixiu-controller-manager/app/options"
	"github.com/caoyingjunz/pixiu/pkg/controller"
	"github.com/caoyingjunz/pixiu/pkg/controller/advanceddeployment"
)

const (
	workers = 5
)

// NewPixiuCommand creates a *cobra.Command object with default parameters
func NewPixiuCommand() *cobra.Command {
	s, err := options.NewOptions()
	if err != nil {
		klog.Fatalf("unable to initialize command options: %v", err)
	}

	cmd := &cobra.Command{
		Use: "pixiu",
		Long: `The pixiu aims to supplement and enhance the native functions of kubernetes`,
		Run: func(cmd *cobra.Command, args []string) {
			c, err := s.Config()
			if err != nil {
				fmt.Fprintf(os.Stderr, "%v\n", err)
				os.Exit(1)
			}
			if err := Run(c); err != nil {
				fmt.Fprintf(os.Stderr, "%v\n", err)
				os.Exit(1)
			}
		},
		Args: func(cmd *cobra.Command, args []string) error {
			for _, arg := range args {
				if len(arg) > 0 {
					return fmt.Errorf("%q does not take any arguments, got %q", cmd.CommandPath(), args)
				}
			}
			return nil
		},
	}

	// BindFlags binds the Configuration struct fields to a cmd
	s.BindFlags(cmd)

	return cmd
}

// Run runs the kubez-autoscaler process. This should never exit.
func Run(c *config.PixiuConfiguration) error {
	go func() {
		if !c.PixiuPprof.Start {
			return
		}
		klog.Fatalf("pprof starting failed: %v", http.ListenAndServe(":"+c.PixiuPprof.Port, nil))
	}()

	stopCh := make(chan struct{})
	defer close(stopCh)

	kubeConfig, err := config.BuildKubeConfig()
	if err != nil {
		return err
	}

	run := func(ctx context.Context) {
		clientBuilder := controller.SimpleControllerClientBuilder{
			ClientConfig: kubeConfig,
		}

		pixiuCtx, err := CreateControllerContext(clientBuilder, clientBuilder, ctx.Done())
		if err != nil {
			klog.Fatal("create kubez context failed: %v", err)
		}

		// 后续优化控制器的初始化方式
		adc, err := advanceddeployment.NewPixiuController(
			pixiuCtx.InformerFactory.Apps().V1().Deployments(),
			pixiuCtx.InformerFactory.Apps().V1().Pods(),
			pixiuCtx.InformerFactory.Autoscaling().V2beta2().HorizontalPodAutoscalers(),
			clientBuilder.ClientOrDie("shared-informers"),
		)
		if err != nil {
			klog.Fatalf("error new adc controller: %v", err)
		}
		go adc.Run(workers, stopCh)

		pixiuCtx.InformerFactory.Start(stopCh)
		pixiuCtx.ObjectOrMetadataInformerFactory.Start(stopCh)

		// Heathz Check
		go StartHealthzServer(c.Healthz.HealthzHost, c.Healthz.HealthzPort)
		// always wait
		select {}
	}

	run(context.TODO())

	// LeaderElection TODO

	panic("unreachable")
	return nil
}

package main

import (
	"context"
	"flag"
	"runtime"

	stub "qiniu-ava/checkpoint-operator/pkg/stub"

	sdk "github.com/operator-framework/operator-sdk/pkg/sdk"
	k8sutil "github.com/operator-framework/operator-sdk/pkg/util/k8sutil"
	sdkVersion "github.com/operator-framework/operator-sdk/version"

	"github.com/sirupsen/logrus"
)

const (
	workerImage = "reg.qiniu.com/ava-os/checkpoint-worker:latest"
)

func printVersion() {
	logrus.Infof("Go Version: %s", runtime.Version())
	logrus.Infof("Go OS/Arch: %s/%s", runtime.GOOS, runtime.GOARCH)
	logrus.Infof("operator-sdk Version: %v", sdkVersion.Version)
}

func main() {
	printVersion()
	cfg := loadConfig()
	if cfg.verbose {
		logrus.SetLevel(logrus.DebugLevel)
	} else {
		logrus.SetLevel(logrus.InfoLevel)
	}

	sdk.ExposeMetricsPort()
	namespace, err := k8sutil.GetWatchNamespace()
	if err != nil {
		logrus.Fatalf("Failed to get watch namespace: %v", err)
	}
	resyncPeriod := 5
	{
		// watch on checkpoints to create worker jobs
		resource := "ava.qiniu.com/v1alpha1"
		kind := "Checkpoint"
		logrus.Infof("Watching %s, %s, %s, %d", resource, kind, namespace, resyncPeriod)
		sdk.Watch(resource, kind, namespace, resyncPeriod)
	}
	{
		// watch on jobs to update checkpoint status
		resource := "batch/v1"
		kind := "Job"
		logrus.Infof("Watching %s, %s, %s, %d", resource, kind, namespace, resyncPeriod)
		sdk.Watch(resource, kind, namespace, resyncPeriod)
	}
	sdk.Handle(stub.NewHandler(cfg.Config))
	sdk.Run(context.TODO())
}

type config struct {
	verbose bool
	*stub.Config
}

func loadConfig() *config {
	var verbose bool
	flag.BoolVar(&verbose, "verbose", false, "print debug log")

	cfg := &stub.Config{}
	flag.StringVar(&(cfg.CheckpointWorkerImage), "worker-image", workerImage, "checkpoint worker image, default to "+workerImage)
	flag.StringVar(&(cfg.ImagePullSecret), "pull-secret", "", "registry secret used to pull worker image")
	flag.Parse()

	cfg.Verbose = verbose
	return &config{verbose: verbose, Config: cfg}
}

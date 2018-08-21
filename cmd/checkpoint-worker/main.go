package main

import (
	"context"
	"flag"
	"runtime"

	"qiniu-ava/checkpoint-operator/pkg/worker"

	"github.com/sirupsen/logrus"
)

func printVersion() {
	logrus.Infof("Go Version: %s", runtime.Version())
	logrus.Infof("Go OS/Arch: %s/%s", runtime.GOOS, runtime.GOARCH)
}

func main() {
	printVersion()
	conf, opt, e := loadOptions()
	if e != nil {
		logrus.Fatal("load checkpoint options failed: ", e)
	}
	c, e := worker.NewDockerClient(conf.Version)
	if e != nil {
		logrus.Fatal("create docker client failed: ", e)
	}

	ctx := context.Background()
	go waitForInterruption(ctx)

	if e := c.Checkpoint(ctx, opt); e != nil {
		logrus.WithField("error", e).Fatal("commit and push checkpoint failed")
	}
	logrus.Info("commit and push checkpoint succeed")
}

type config struct {
	Version string
}

func loadOptions() (*config, *worker.CheckpointOptions, error) {
	o := &worker.CheckpointOptions{}
	flag.StringVar(&(o.Container), "container", "", "container name going to have a checkpoint")
	flag.StringVar(&(o.Image), "image", "", "full image name of the checkpoint")
	flag.StringVar(&(o.Auth), "auth", "", "registry authentication header, base64 encoded")
	flag.StringVar(&(o.Author), "author", "", "checkpoint author")
	flag.StringVar(&(o.Comment), "comment", "", "image comment")

	conf := &config{}
	flag.StringVar(&(conf.Version), "version", "1.38", "docker client api version")

	flag.Parse()

	if e := o.Validate(); e != nil {
		return nil, nil, e
	}
	return conf, o, nil
}

func waitForInterruption(ctx context.Context) {

}

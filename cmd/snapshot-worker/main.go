package main

import (
	"context"
	"flag"
	"runtime"

	"github.com/qiniu-ava/snapshot-operator/pkg/worker"

	"github.com/sirupsen/logrus"
)

const (
	defaultDockerVersion = "1.38"
	defaultDockerConfig  = "/config"
)

func printVersion() {
	logrus.Infof("Go Version: %s", runtime.Version())
	logrus.Infof("Go OS/Arch: %s/%s", runtime.GOOS, runtime.GOARCH)
}

func main() {
	printVersion()
	conf, e := loadConfig()
	if e != nil {
		logrus.WithField("error", e).Fatal("load snapshot options failed: ", e)
	}
	if conf.Verbose {
		logrus.SetLevel(logrus.DebugLevel)
	} else {
		logrus.SetLevel(logrus.InfoLevel)
	}

	c, e := worker.NewDockerClient(conf.DockerConfigDir, conf.Version)
	if e != nil {
		logrus.WithField("error", e).Fatal("create docker client failed")
	}

	ctx := context.Background()

	if e := c.Snapshot(ctx, conf.Options); e != nil {
		logrus.WithField("error", e).Fatal("commit and push snapshot failed")
	}
	logrus.Info("commit and push snapshot succeed")
}

type config struct {
	Version         string
	Verbose         bool
	Options         *worker.SnapshotOptions
	DockerConfigDir string
}

func loadConfig() (*config, error) {
	o := &worker.SnapshotOptions{}
	flag.StringVar(&(o.Container), "container", "", "container name going to have a snapshot")
	flag.StringVar(&(o.Image), "image", "", "full image name of the snapshot")
	flag.StringVar(&(o.Author), "author", "", "snapshot author")
	flag.StringVar(&(o.Comment), "comment", "", "image comment")

	conf := &config{}
	flag.StringVar(&(conf.Version), "version", defaultDockerVersion, "docker client api version, default to "+defaultDockerVersion)
	flag.BoolVar(&(conf.Verbose), "verbose", false, "verbose")
	flag.StringVar(&(conf.DockerConfigDir), "config-dir", defaultDockerConfig, "docker config directory, default to "+defaultDockerConfig)

	flag.Parse()

	if e := o.Validate(); e != nil {
		return nil, e
	}
	conf.Options = o

	return conf, nil
}

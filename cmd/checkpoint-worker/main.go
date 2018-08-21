package main

import (
	"context"
	"encoding/json"
	"flag"
	"os"
	"runtime"

	"qiniu-ava/checkpoint-operator/pkg/worker"

	"github.com/pkg/errors"
	"github.com/sirupsen/logrus"
)

const (
	defaultDockerVersion = "1.38"
	defaultDockerConfig  = "/config/.dockercfg"
)

func printVersion() {
	logrus.Infof("Go Version: %s", runtime.Version())
	logrus.Infof("Go OS/Arch: %s/%s", runtime.GOOS, runtime.GOARCH)
}

func main() {
	printVersion()
	conf, e := loadConfig()
	if e != nil {
		logrus.WithField("error", e).Fatal("load checkpoint options failed: ", e)
	}
	if conf.Verbose {
		logrus.SetLevel(logrus.DebugLevel)
	} else {
		logrus.SetLevel(logrus.InfoLevel)
	}

	c, e := worker.NewDockerClient(conf.DockerConfig, conf.Version)
	if e != nil {
		logrus.WithField("error", e).Fatal("create docker client failed")
	}

	ctx := context.Background()
	go waitForInterruption(ctx)

	if e := c.Checkpoint(ctx, conf.Options); e != nil {
		logrus.WithField("error", e).Fatal("commit and push checkpoint failed")
	}
	logrus.Info("commit and push checkpoint succeed")
}

type config struct {
	Version      string
	Verbose      bool
	DockerConfig worker.DockerConfig
	Options      *worker.CheckpointOptions
}

func loadConfig() (*config, error) {
	o := &worker.CheckpointOptions{}
	flag.StringVar(&(o.Container), "container", "", "container name going to have a checkpoint")
	flag.StringVar(&(o.Image), "image", "", "full image name of the checkpoint")
	flag.StringVar(&(o.Author), "author", "", "checkpoint author")
	flag.StringVar(&(o.Comment), "comment", "", "image comment")

	conf := &config{}
	flag.StringVar(&(conf.Version), "version", defaultDockerVersion, "docker client api version, default to "+defaultDockerVersion)
	flag.BoolVar(&(conf.Verbose), "verbose", false, "verbose")

	var configPath string
	flag.StringVar(&(configPath), "config", defaultDockerConfig, "docker config file path, default to "+defaultDockerConfig)

	flag.Parse()

	if e := o.Validate(); e != nil {
		return nil, e
	}
	df, e := loadDockerConfig(configPath)
	if e != nil {
		return nil, e
	}
	conf.DockerConfig = df
	conf.Options = o

	return conf, nil
}

func loadDockerConfig(path string) (worker.DockerConfig, error) {
	f, e := os.Open(path)
	if e != nil {
		return nil, errors.Wrap(e, "open docker config file failed")
	}
	conf := make(worker.DockerConfig)
	if e := json.NewDecoder(f).Decode(&conf); e != nil {
		return nil, errors.Wrap(e, "unmarshal docker config failed")
	}
	return conf, nil
}

func waitForInterruption(ctx context.Context) {

}

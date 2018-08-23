package main

import (
	"context"
	"encoding/json"
	"flag"
	"os"
	"runtime"

	"qiniu-ava/snapshot-operator/pkg/worker"

	"github.com/pkg/errors"
	"github.com/sirupsen/logrus"
)

const (
	defaultDockerVersion = "1.38"
	defaultDockerConfig  = "/config/.dockerconfigjson"
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

	c, e := worker.NewDockerClient(conf.DockerAuth, conf.Version)
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
	Version    string
	Verbose    bool
	DockerAuth worker.DockerAuth
	Options    *worker.SnapshotOptions
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

	var configPath string
	flag.StringVar(&(configPath), "config", defaultDockerConfig, "docker config file path, default to "+defaultDockerConfig)

	flag.Parse()

	if e := o.Validate(); e != nil {
		return nil, e
	}
	auth, e := loadDockerConfig(configPath)
	if e != nil {
		return nil, e
	}
	conf.DockerAuth = auth
	conf.Options = o

	return conf, nil
}

func loadDockerConfig(path string) (worker.DockerAuth, error) {
	f, e := os.Open(path)
	if e != nil {
		return nil, errors.Wrap(e, "open docker config file failed")
	}
	conf := struct {
		Auths worker.DockerAuth `json:"auths"`
	}{}
	if e := json.NewDecoder(f).Decode(&conf); e != nil {
		return nil, errors.Wrap(e, "unmarshal docker config failed")
	}
	return conf.Auths, nil
}

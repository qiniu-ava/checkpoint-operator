package worker

import (
	"context"
	"encoding/base64"
	"encoding/json"
	stderr "errors"
	"io"
	"io/ioutil"
	"os"
	"path/filepath"

	"github.com/docker/distribution/reference"
	"github.com/docker/docker/api/types"
	"github.com/docker/docker/api/types/container"
	"github.com/docker/docker/client"
	"github.com/docker/docker/pkg/jsonmessage"
	"github.com/pkg/errors"
	"github.com/sirupsen/logrus"
	"k8s.io/api/core/v1"
)

type DockerClient struct {
	c     client.APIClient
	auths DockerAuths
}

func NewDockerClient(dockerConfigDir string, version string) (*DockerClient, error) {
	auths, e := loadDockerAuths(dockerConfigDir)
	if e != nil {
		return nil, errors.Wrap(e, "load imagePushSecret failed")
	}
	c, e := client.NewClientWithOpts(client.FromEnv, client.WithVersion(version))
	if e != nil {
		return nil, errors.Wrap(e, "create docker client failed")
	}

	return &DockerClient{c: c, auths: auths}, nil
}

func (dc *DockerClient) Snapshot(ctx context.Context, opt *SnapshotOptions) error {
	l := logrus.WithFields(logrus.Fields{"container": opt.Container, "image": opt.Image})
	l.Info("creating snapshot")
	l.WithField("options", *opt).Debug("snapshot options")
	l.WithField("auths", dc.auths).Debug("docker auths")

	ref, e := reference.ParseNormalizedNamed(opt.Image)
	if e != nil {
		return errors.Wrap(e, "parse image name failed")
	}
	l.WithFields(logrus.Fields{"reference": ref, "familiar": reference.FamiliarString(ref)}).Debug("got reference")

	idRes, e := dc.c.ContainerCommit(ctx, opt.Container, types.ContainerCommitOptions{
		Reference: ref.String(),
		Author:    opt.Author,
		Comment:   opt.Comment,
		Config: &container.Config{
			Image: opt.Image,
		},
	})
	if e != nil {
		return errors.Wrap(e, "commit container failed")
	}
	l = l.WithField("id", idRes.ID)
	l.Info("snapshot committed")

	auth, e := dc.getAuth(ref)
	if e != nil {
		return errors.Wrap(e, "encode auth failed")
	}
	l.WithField("auth", auth).Debug()
	resp, e := dc.c.ImagePush(ctx, reference.FamiliarString(ref), types.ImagePushOptions{
		RegistryAuth: auth,
		PrivilegeFunc: func() (string, error) {
			return auth, nil
		},
	})
	defer func() {
		io.Copy(ioutil.Discard, resp)
		resp.Close()
	}()
	if e != nil {
		return errors.Wrap(e, "push image failed")
	}

	if e := jsonmessage.DisplayJSONMessagesStream(resp, l.Writer(), 0, false, nil); e != nil {
		return errors.Wrap(e, "push image failed")
	}
	l.Info("snapshot pushed")
	return nil
}

func (dc *DockerClient) getAuth(ref reference.Named) (string, error) {
	auth, exists := dc.auths[reference.Domain(ref)]
	if !exists {
		return "", stderr.New("no registry authentication found")
	}
	buf, e := json.Marshal(auth)
	if e != nil {
		return "", errors.Wrap(e, "marshal auth failed")
	}

	return base64.URLEncoding.EncodeToString(buf), nil
}

func loadDockerAuths(dir string) (DockerAuths, error) {
	logrus.WithField("dir", dir).Debug("looking for docker config file")
	if _, e := os.Stat(filepath.Join(dir, v1.DockerConfigKey)); e == nil {
		return loadAuthsFromDockerCfg(filepath.Join(dir, v1.DockerConfigKey))
	} else if os.IsNotExist(e) {
		if _, e = os.Stat(filepath.Join(dir, v1.DockerConfigJsonKey)); e == nil {
			return loadAuthsFromDockerConfigJSON(filepath.Join(dir, v1.DockerConfigJsonKey))
		}
	}

	return nil, errors.New("image push secret not found")
}

func loadAuthsFromDockerCfg(path string) (DockerAuths, error) {
	logrus.WithField("path", path).Debug("loading .dockercfg")
	f, e := os.Open(path)
	if e != nil {
		return nil, errors.Wrap(e, "open DockerCfg file failed")
	}
	auths := make(DockerAuths)
	if e := json.NewDecoder(f).Decode(&auths); e != nil {
		return nil, errors.Wrap(e, "unmarshal DockerCfg failed")
	}
	return auths, nil
}

func loadAuthsFromDockerConfigJSON(path string) (DockerAuths, error) {
	logrus.WithField("path", path).Debug("loading .dockerconfigjson")
	f, e := os.Open(path)
	if e != nil {
		return nil, errors.Wrap(e, "open DockerConfigJSON file failed")
	}
	config := struct {
		Auths DockerAuths `json:"auths"`
	}{}
	if e := json.NewDecoder(f).Decode(&config); e != nil {
		return nil, errors.Wrap(e, "unmarshal DockerConfigJSON failed")
	}
	return config.Auths, nil
}

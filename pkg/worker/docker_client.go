package worker

import (
	"context"
	"encoding/base64"
	"encoding/json"
	stderr "errors"
	"io"
	"io/ioutil"

	"github.com/docker/distribution/reference"
	"github.com/docker/docker/api/types"
	"github.com/docker/docker/api/types/container"
	"github.com/docker/docker/client"
	"github.com/docker/docker/pkg/jsonmessage"
	"github.com/pkg/errors"
	"github.com/sirupsen/logrus"
)

type DockerClient struct {
	c    client.APIClient
	conf DockerConfig
}

func NewDockerClient(conf DockerConfig, version string) (*DockerClient, error) {
	c, e := client.NewClientWithOpts(client.FromEnv, client.WithVersion(version))
	if e != nil {
		return nil, e
	}
	return &DockerClient{c: c, conf: conf}, nil
}

func (dc *DockerClient) Checkpoint(ctx context.Context, opt *CheckpointOptions) error {
	l := logrus.WithFields(logrus.Fields{"container": opt.Container, "image": opt.Image})
	l.Info("creating checkpoint")
	l.WithField("options", opt).Debug("checkpoint options")

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
	l.Info("checkpoint committed")

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
		io.CopyN(ioutil.Discard, resp, 512)
		resp.Close()
	}()
	if e != nil {
		return errors.Wrap(e, "push image failed")
	}

	defer resp.Close()
	if e := jsonmessage.DisplayJSONMessagesStream(resp, l.Writer(), 0, false, nil); e != nil {
		return errors.Wrap(e, "push image failed")
	}
	l.Info("checkpoint pushed")
	return nil
}

func (dc *DockerClient) getAuth(ref reference.Named) (string, error) {
	auth, exists := dc.conf[reference.Domain(ref)]
	if !exists {
		return "", stderr.New("no registry authentication found")
	}
	buf, e := json.Marshal(auth)
	if e != nil {
		return "", errors.Wrap(e, "marshal auth config failed")
	}

	return base64.URLEncoding.EncodeToString(buf), nil
}

package worker

import (
	"context"
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
	c *client.Client
}

func NewDockerClient(version string) (*DockerClient, error) {
	c, e := client.NewClientWithOpts(client.FromEnv, client.WithVersion(version))
	if e != nil {
		return nil, e
	}
	return &DockerClient{c: c}, nil
}

func (dc *DockerClient) Checkpoint(ctx context.Context, opt *CheckpointOptions) error {
	l := logrus.WithFields(logrus.Fields{"container": opt.Container, "image": opt.Image})
	l.Info("creating checkpoint")

	ref, e := reference.ParseNormalizedNamed(opt.Image)
	if e != nil {
		return errors.Wrap(e, "parse image name failed")
	}

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
	l.WithField("commitID", idRes.ID).Info("checkpoint committed")

	resp, e := dc.c.ImagePush(ctx, reference.FamiliarString(ref), types.ImagePushOptions{
		RegistryAuth: opt.Auth,
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
		return errors.Wrap(e, "push image filed")
	}
	l.Info("checkpoint pushed")
	return nil
}

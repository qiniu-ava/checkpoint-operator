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
	auth DockerAuth
}

func NewDockerClient(auth DockerAuth, version string) (*DockerClient, error) {
	c, e := client.NewClientWithOpts(client.FromEnv, client.WithVersion(version))
	if e != nil {
		return nil, e
	}
	return &DockerClient{c: c, auth: auth}, nil
}

func (dc *DockerClient) Snapshot(ctx context.Context, opt *SnapshotOptions) error {
	l := logrus.WithFields(logrus.Fields{"container": opt.Container, "image": opt.Image})
	l.Info("creating snapshot")
	l.WithField("options", *opt).Debug("snapshot options")
	l.WithField("auth", dc.auth).Debug("docker auth")

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
	l.Info("snapshot pushed")
	return nil
}

func (dc *DockerClient) getAuth(ref reference.Named) (string, error) {
	auth, exists := dc.auth[reference.Domain(ref)]
	if !exists {
		return "", stderr.New("no registry authentication found")
	}
	buf, e := json.Marshal(auth)
	if e != nil {
		return "", errors.Wrap(e, "marshal auth failed")
	}

	return base64.URLEncoding.EncodeToString(buf), nil
}

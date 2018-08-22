package worker

import (
	"errors"

	"github.com/docker/docker/api/types"
)

type CheckpointOptions struct {
	// docker registry auth header, from imagePushSecret
	// see: https://kubernetes.io/docs/tasks/configure-pod-container/pull-image-private-registry/#inspecting-the-secret-regcred
	Container string `json:"container,omitempty"`
	Image     string `json:"image,omitempty"` // image full name: host/path/image:tag
	Author    string `json:"author,omitempty"`
	Comment   string `json:"comment,omitempty"`
}

func (o *CheckpointOptions) Validate() error {
	if o.Container == "" {
		return errors.New("empty container")
	}
	if o.Image == "" {
		return errors.New("empty image")
	}
	return nil
}

type DockerConfig map[string]types.AuthConfig

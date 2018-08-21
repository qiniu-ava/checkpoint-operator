package worker

import "errors"

type CheckpointOptions struct {
	// docker registry auth header, from imagePushSecret
	// see: https://kubernetes.io/docs/tasks/configure-pod-container/pull-image-private-registry/#inspecting-the-secret-regcred
	Auth      string `json:"auth,omitempty"`
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
	if o.Auth == "" {
		return errors.New("empty auth")
	}
	return nil
}

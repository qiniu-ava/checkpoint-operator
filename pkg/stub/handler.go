package stub

import (
	"context"
	stderr "errors"

	"qiniu-ava/checkpoint-operator/pkg/apis/ava/v1alpha1"

	"github.com/operator-framework/operator-sdk/pkg/sdk"
	gerr "github.com/pkg/errors"
	"github.com/sirupsen/logrus"
	batchv1 "k8s.io/api/batch/v1"
	"k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	kubeletapis "k8s.io/kubernetes/pkg/kubelet/apis"
)

var (
	truevalue       = true
	one       int32 = 1
)

func NewHandler() sdk.Handler {
	return &Handler{}
}

type Handler struct {
	cfg *Config
}

type Config struct {
	ImagePullSecret string `json:"imagePullSecret"`
}

func (h *Handler) Handle(ctx context.Context, event sdk.Event) error {
	switch o := event.Object.(type) {
	case *v1alpha1.Checkpoint:
		cp := event.Object.(*v1alpha1.Checkpoint)
		logrus.Infof("handling checkpoint %s/%s", cp.Namespace, cp.Name)
		if err := h.createCheckpointJob(cp); err != nil {
			logrus.Errorf("Failed to new checkpoint job: %v", err)
			return err
		}
	default:
		logrus.Warningf("got unexpected object: %v", o)
	}
	return nil
}

func (h *Handler) createCheckpointJob(cp *v1alpha1.Checkpoint) error {
	pod := &v1.Pod{
		TypeMeta:   metav1.TypeMeta{Kind: "Pod", APIVersion: v1.SchemeGroupVersion.String()},
		ObjectMeta: metav1.ObjectMeta{Name: cp.Spec.PodName, Namespace: cp.Namespace},
	}
	if err := sdk.Get(pod); err != nil {
		return gerr.Wrap(err, "get pod info failed")
	}
	if !podIsReady(pod) {
		return stderr.New("pod is not ready for checkpoint")
	}
	cp.Spec.NodeName = pod.Spec.NodeName

	job := &batchv1.Job{
		TypeMeta: metav1.TypeMeta{Kind: "Job", APIVersion: batchv1.SchemeGroupVersion.String()},
		ObjectMeta: metav1.ObjectMeta{
			// Name: schema.
			Namespace: pod.Namespace,
			Labels: map[string]string{
				"pod":       cp.Spec.PodName,
				"container": cp.Spec.ContainerName,
				"node":      cp.Spec.NodeName,
			},
			OwnerReferences: []metav1.OwnerReference{{
				APIVersion:         v1alpha1.SchemeGroupVersion.String(),
				Kind:               "Checkpoint",
				Name:               cp.Name,
				UID:                cp.UID,
				Controller:         &truevalue,
				BlockOwnerDeletion: &truevalue,
			}},
		},
		Spec: batchv1.JobSpec{
			Parallelism: &one,
			Completions: &one,
			Template: v1.PodTemplateSpec{
				Spec: v1.PodSpec{
					// todo: mount /var/docker.sock
					Volumes: []v1.Volume{},
					// todo: commit and push
					Containers: []v1.Container{{
						Name:  "runner",
						Image: "", // todo...
					}},
					NodeSelector:     map[string]string{string(kubeletapis.LabelHostname): cp.Spec.NodeName},
					NodeName:         cp.Spec.NodeName,
					ImagePullSecrets: []v1.LocalObjectReference{{Name: h.cfg.ImagePullSecret}},
				},
			},
		},
	}

	if err := sdk.Create(job); err != nil {
		return gerr.Wrap(err, "create checkpoint job failed")
	}

	logrus.Infof("checkpoint job created for %s/%s ", cp.Namespace, cp.Name)
	return nil
}

func podIsReady(pod *v1.Pod) bool { return false }

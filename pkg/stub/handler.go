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
	switch event.Object.(type) {
	case *v1alpha1.Checkpoint:
		cp := event.Object.(*v1alpha1.Checkpoint)
		if event.Deleted {
			logrus.Infof("deleting checkpoint %s/%s", cp.Namespace, cp.Name)
			if err := h.deleteCheckpointJob(cp); err != nil {
				logrus.Errorf("failed to delete checkpoint job: %v", err)
				return err
			}
		} else {
			logrus.Infof("creating checkpoint %s/%s", cp.Namespace, cp.Name)
			if err := h.createCheckpointJob(cp); err != nil {
				logrus.Errorf("failed to create checkpoint job: %v", err)
				return err
			}
		}

	default:
		logrus.Warningf("got unexpected event: %v", event)
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
	cp.Status.NodeName = pod.Spec.NodeName

	job := &batchv1.Job{
		TypeMeta: metav1.TypeMeta{
			Kind:       "Job",
			APIVersion: batchv1.SchemeGroupVersion.String(),
		},
		ObjectMeta: metav1.ObjectMeta{
			GenerateName:    cp.Name,
			Namespace:       pod.Namespace,
			OwnerReferences: []metav1.OwnerReference{*metav1.NewControllerRef(cp, v1alpha1.SchemeGVK)},
			Annotations: map[string]string{
				v1alpha1.SchemeGroupVersion.String() + "/pod-name":       cp.Spec.PodName,
				v1alpha1.SchemeGroupVersion.String() + "/container-name": cp.Spec.ContainerName,
				v1alpha1.SchemeGroupVersion.String() + "/node-name":      cp.Status.NodeName,
			},
		},
		Spec: batchv1.JobSpec{
			Parallelism: &one,
			Completions: &one,
			Template: v1.PodTemplateSpec{
				Spec: v1.PodSpec{
					// todo: mount /var/docker.sock
					Volumes: []v1.Volume{},
					Containers: []v1.Container{{
						Name:  "runner",
						Image: "", // todo...
					}},
					NodeSelector:     map[string]string{string(kubeletapis.LabelHostname): cp.Status.NodeName},
					NodeName:         cp.Status.NodeName,
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

func (h *Handler) deleteCheckpointJob(cp *v1alpha1.Checkpoint) error { return nil }

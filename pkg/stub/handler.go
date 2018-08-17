package stub

import (
	"context"
	stderr "errors"
	"strings"

	"qiniu-ava/checkpoint-operator/pkg/apis/ava/v1alpha1"

	"github.com/operator-framework/operator-sdk/pkg/sdk"
	gerr "github.com/pkg/errors"
	"github.com/sirupsen/logrus"
	batchv1 "k8s.io/api/batch/v1"
	"k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	kubeletapis "k8s.io/kubernetes/pkg/kubelet/apis"
)

const listLimit = 16

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
			logrus.Infof("checkpoint %s/%s deleted", cp.Namespace, cp.Name)
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
	if len(cp.Labels) == 0 {
		return gerr.Wrap(err, "checkpoint object has no label")
	}
	if cp.Spec.Selector == nil {
		cp.Spec.Selector = &metav1.LabelSelector{}
	}
	cp.Spec.Selector.MatchLabels = cp.Labels

	// check if checkpoint job created earlier
	if cp.Status.JobRef.Name != "" {
		logrus.Infof("checkpoint job %s already exists, do nothing", cp.Status.JobRef.Name)
		return nil
	}
	if job, err := queryCheckpointJob(cp); err != nil {
		return gerr.Wrap(err, "query job for checkpoint failed")
	} else if job != nil {
		logrus.Infof("found existing checkpoint job %s, do nothing", job.Name)
		referenceJob(cp, job)
		return nil
	}

	// find the pod going to have a checkpoint
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

	// create checkpoint job
	job := &batchv1.Job{
		TypeMeta: metav1.TypeMeta{
			Kind:       "Job",
			APIVersion: batchv1.SchemeGroupVersion.String(),
		},
		ObjectMeta: metav1.ObjectMeta{
			GenerateName:    cp.Name,
			Namespace:       pod.Namespace,
			OwnerReferences: []metav1.OwnerReference{*metav1.NewControllerRef(cp, v1alpha1.SchemeGVK)},
			Labels:          cp.Labels,
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

	// query created job
	job, err := queryCheckpointJob(cp)
	if err != nil || job == nil {
		return gerr.Wrap(err, "query created checkpoint job failed")
	}
	logrus.Infof("checkpoint job created for %s/%s ", cp.Namespace, cp.Name)
	referenceJob(cp, job)

	return nil
}

func queryCheckpointJob(cp *v1alpha1.Checkpoint) (*batchv1.Job, error) {
	// list jobs by labels.
	selectors := make([]string, 0, len(cp.Spec.LabelSelector.MatchLabels))
	for k, v := range cp.cp.Spec.LabelSelector.MatchLabels {
		selectors = append(selectors, k+"="+v)
	}
	jobs := &batchv1.JobList{
		TypeMeta: metav1.TypeMeta{Kind: "Job", APIVersion: batchv1.SchemeGroupVersion.String()},
	}
	ops := &metav1.ListOptions{
		TypeMeta:             metav1.TypeMeta{Kind: "Job", APIVersion: batchv1.SchemeGroupVersion.String()},
		LabelSelector:        strings.Join(selectors, ","),
		IncludeUninitialized: true,
		Limit:                listLimit,
	}
	if err := sdk.List(cp.Namespace, jobs, sdk.WithListOptions(opts)); err != nil && !errors.IsNotFound(err) {
		return nil, gerr.Wrap(err, "list checkpoint jobs failed")
	}

	for _, job := range jobs.Items {
		for _, owner := range job.GetObjectMeta().GetOwnerReferences() {
			if owner.Kind == "Checkpoint" &&
				owner.APIVersion == v1alpha1.SchemeGroupVersion.String() &&
				owner.Namespace == cp.Namespace &&
				owner.Name == cp.Name &&
				owner.UID == cp.UID {
				return &job, nil
			}
		}
	}
	return nil, nil
}

func referenceJob(cp *v1alpha1.Checkpoint, job *batchv1.Job) {
	cp.Status.JobRef = v1.ObjectReference{
		Kind:            "Job",
		Namespace:       job.Namespace,
		Name:            job.Name,
		UID:             job.UID,
		APIVersion:      batchv1.SchemeGroupVersion.String(),
		ResourceVersion: job.ResourceVersion,
	}
}

func podIsReady(pod *v1.Pod) bool {
	switch pod.Status.Phase {
	case v1.PodRunning, v1.PodSucceeded:
		return true
	}
	return false
}

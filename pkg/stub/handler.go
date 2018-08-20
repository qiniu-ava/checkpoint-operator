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
	ImagePullSecret       string `json:"imagePullSecret"`
	CheckpointWorkerImage string `json:"checkpointWorkerImage"`
}

func (h *Handler) Handle(ctx context.Context, event sdk.Event) error {
	switch event.Object.(type) {
	case *v1alpha1.Checkpoint:
		cp := event.Object.(*v1alpha1.Checkpoint)
		if event.Deleted {
			logger(cp).Info("checkpoint deleted")
		} else {
			logger(cp).Info("creating checkpoint")
			if err := h.createCheckpointJob(cp); err != nil {
				logger(cp).WithField("error", err).Error("failed to create checkpoint job")
				return err
			}
			logger(cp).Info("checkpoint created")
		}
	case *batchv1.Job:
		job := event.Object.(*batchv1.Job)
		if event.Deleted {
			logger(job).Info("checkpoint job deleted")
		} else {
			logger(job).Info("updating checkpoint job")
			if err := updateCheckpointByJob(job); err != nil {
				logger(job).WithField("error", err).Error("failed to update checkpoint on job updated")
				return err
			}
			logger(job).Info("checkpoint updated")
		}
	default:
		logrus.WithField("event", event).Warning("got unexpected event")
	}
	return nil
}

func (h *Handler) createCheckpointJob(cp *v1alpha1.Checkpoint) error {
	if cp.Labels == nil {
		cp.Labels = make(map[string]string, 4)
	}
	cp.Labels[v1alpha1.SchemeGroupVersion.String()+"/pod-name"] = cp.Spec.PodName
	cp.Labels[v1alpha1.SchemeGroupVersion.String()+"/container-name"] = cp.Spec.ContainerName
	cp.Labels[v1alpha1.SchemeGroupVersion.String()+"/node-name"] = cp.Status.NodeName

	if cp.Spec.Selector == nil {
		cp.Spec.Selector = &metav1.LabelSelector{}
	}
	cp.Spec.Selector.MatchLabels = cp.Labels

	// check if checkpoint job created earlier
	if cp.Status.JobRef.Name != "" {
		logger(cp).WithField("job", cp.Status.JobRef.Name).Info("checkpoint job already exists, do nothing")
		return nil
	}
	if job, err := queryCheckpointJob(cp); err != nil {
		return gerr.Wrap(err, "query job for checkpoint failed")
	} else if job != nil {
		logger(cp).WithField("job", job.Name).Info("found existing checkpoint job, reference to it")
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
				v1alpha1.SchemeGroupVersion.String() + "/controller": v1alpha1.OperatorName,
			},
		},
		Spec: batchv1.JobSpec{
			Parallelism: &one,
			Completions: &one,
			Template: v1.PodTemplateSpec{
				Spec: v1.PodSpec{
					Volumes: []v1.Volume{}, // todo
					Containers: []v1.Container{{
						Name:  "runner",
						Image: h.cfg.CheckpointWorkerImage,
					}},
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
	logger(cp).WithField("job", job.Name).Info("created checkpoint job")
	referenceJob(cp, job)

	return nil
}

func queryCheckpointJob(cp *v1alpha1.Checkpoint) (*batchv1.Job, error) {
	// list jobs by labels.
	selectors := make([]string, 0, len(cp.Spec.Selector.MatchLabels))
	for k, v := range cp.Spec.Selector.MatchLabels {
		selectors = append(selectors, k+"="+v)
	}
	jobs := &batchv1.JobList{
		TypeMeta: metav1.TypeMeta{Kind: "Job", APIVersion: batchv1.SchemeGroupVersion.String()},
	}
	opts := &metav1.ListOptions{
		TypeMeta:             metav1.TypeMeta{Kind: "Job", APIVersion: batchv1.SchemeGroupVersion.String()},
		LabelSelector:        strings.Join(selectors, ","),
		IncludeUninitialized: true,
		Limit:                listLimit,
	}
	if err := sdk.List(cp.Namespace, jobs, sdk.WithListOptions(opts)); err != nil && !errors.IsNotFound(err) {
		return nil, gerr.Wrap(err, "list checkpoint jobs failed")
	}

	for _, job := range jobs.Items {
		if metav1.IsControlledBy(&job, cp) {
			return &job, nil
		}
	}
	return nil, nil
}

func referenceJob(cp *v1alpha1.Checkpoint, job *batchv1.Job) {
	cp.Status.JobRef = v1.LocalObjectReference{
		Name: job.Name,
	}
}

func podIsReady(pod *v1.Pod) bool {
	switch pod.Status.Phase {
	case v1.PodRunning, v1.PodSucceeded:
		return true
	}
	return false
}

func updateCheckpointByJob(job *batchv1.Job) error {
	if !isCheckpointJob(job) {
		logger(job).Debug("not a checkpoint job")
		return nil
	}

	// query checkpoint
	if len(job.OwnerReferences) == 0 {
		return stderr.New("checkpoint job has no owner")
	}
	if len(job.OwnerReferences) > 1 {
		return stderr.New("checkpoint job has too much owners")
	}
	owner := job.OwnerReferences[0]

	cp := &v1alpha1.Checkpoint{
		TypeMeta:   metav1.TypeMeta{Kind: "Checkpoint", APIVersion: v1alpha1.SchemeGroupVersion.String()},
		ObjectMeta: metav1.ObjectMeta{Name: owner.Name, Namespace: job.Namespace, UID: owner.UID},
	}
	if err := sdk.Get(cp); err != nil {
		return gerr.Wrap(err, "query checkpoint failed")
	}

	// update checkpoint conditions
	cp.Status.Conditions = job.Status.Conditions
	logger(cp).WithField("job", job.Name).Info("checkpoint conditions updated by job")
	return nil
}

func isCheckpointJob(job *batchv1.Job) bool {
	owner := metav1.GetControllerOf(job)
	return owner.Kind == v1alpha1.SchemeGroupVersion.String()
}

func logger(obj metav1.Object) *logrus.Entry {
	return logrus.WithFields(logrus.Fields{
		"name":      obj.GetName(),
		"namespace": obj.GetNamespace(),
	})
}

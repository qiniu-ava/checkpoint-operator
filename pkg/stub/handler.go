package stub

import (
	"context"
	stderr "errors"
	"strings"

	"github.com/qiniu-ava/snapshot-operator/pkg/apis/ava/v1alpha1"

	"github.com/operator-framework/operator-sdk/pkg/sdk"
	gerr "github.com/pkg/errors"
	"github.com/sirupsen/logrus"
	batchv1 "k8s.io/api/batch/v1"
	"k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

const (
	listLimit = 16

	labelPrefix      = "snapshot-operator.ava.qiniu.com"
	dockerSocketPath = "/var/run/docker.sock"
	dockerConfigDir  = "/config"
	containerPrefix  = "docker://"
)

var (
	truevalue                             = true
	one                   int32           = 1
	readOnly              int32           = 0400
	hostPathSocket        v1.HostPathType = v1.HostPathSocket
	workerDeadlineSeconds int64           = 30 * 60
)

func NewHandler(cfg *Config) sdk.Handler {
	return &Handler{cfg: cfg}
}

type Handler struct {
	cfg *Config
}

type Config struct {
	SnapshotWorkerImage string `json:"snapshotWorkerImage,omitempty"`
	ImagePullSecret     string `json:"imagePullSecret,omitempty"`
	Verbose             bool   `json:"verbose,omitempty"`
}

func (h *Handler) Handle(ctx context.Context, event sdk.Event) error {
	switch event.Object.(type) {
	case *v1alpha1.Snapshot:
		ss := event.Object.(*v1alpha1.Snapshot)
		if event.Deleted {
			logger(ss).Debug("deleting snapshot")
		} else {
			logger(ss).Info("updating snapshot")
			if err := h.onSnapshotUpdating(ss); err != nil {
				logger(ss).WithField("error", err).Error("failed to update snapshot job")
				return err
			}
			logger(ss).Info("snapshot updated")
		}
	case *batchv1.Job:
		job := event.Object.(*batchv1.Job)
		if event.Deleted {
			logger(job).Debug("got job deletion event")
		} else {
			logger(job).Debug("got job updating event")
			if err := h.onUpdatingJob(job); err != nil {
				logger(job).WithField("error", err).Error("failed to update checkpoint on job updated")
				return err
			}
		}
	default:
		logrus.WithField("event", event).Warning("got unexpected event")
	}
	return nil
}

func (h *Handler) onSnapshotUpdating(ss *v1alpha1.Snapshot) (e error) {
	// check if checkpoint created earlier
	var stale bool
	defer func() {
		if stale {
			logger(ss).Info("updating stale snapshot")
			if err := sdk.Update(ss); err != nil {
				e = gerr.Wrap(err, "update snapshot failed")
			}
		}
	}()

	if job, err := querySnapshotJob(ss); err != nil {
		return gerr.Wrap(err, "query job for snapshot failed")
	} else if job != nil {
		stale = updateCondition(ss, job)
		logger(ss).WithField("job", job.Name).Debug("found existing snapshot job")
		return nil
	}

	// newly created snapshot, create a job for it
	// 1. find the container going to have a snapshot
	pod := &v1.Pod{
		TypeMeta:   metav1.TypeMeta{Kind: "Pod", APIVersion: v1.SchemeGroupVersion.String()},
		ObjectMeta: metav1.ObjectMeta{Name: ss.Spec.PodName, Namespace: ss.Namespace},
	}
	if err := sdk.Get(pod); err != nil {
		return gerr.Wrap(err, "get pod info failed")
	}
	logger(ss).Debug("found snapshoting pod")
	if pod.Spec.NodeName == "" || (pod.Status.Phase != v1.PodRunning && pod.Status.Phase != v1.PodSucceeded) {
		ss.Status.Conditions = []v1alpha1.SnapshotCondition{
			*newCondition(v1alpha1.SnapshotFailed, "PodUnavailable", "pod is unavailable to have a snapshot"),
		}
		stale = true
		return stderr.New("pod is not ready for snapshoting")
	}
	ss.Status.NodeName = pod.Spec.NodeName
	container := getContainerID(pod, ss.Spec.ContainerName)
	if container == "" {
		return stderr.New("container id not found")
	}

	// 2. complete the snapshot spec
	if ss.Labels == nil {
		ss.Labels = make(map[string]string, 4)
	}
	ss.Labels[labelPrefix+"_pod-name"] = ss.Spec.PodName
	ss.Labels[labelPrefix+"_container-name"] = ss.Spec.ContainerName
	ss.Labels[labelPrefix+"_node-name"] = ss.Status.NodeName

	if ss.Spec.Selector == nil {
		ss.Spec.Selector = &metav1.LabelSelector{}
	}
	ss.Spec.Selector.MatchLabels = ss.Labels
	stale = true
	logger(ss).Debug("snapshot updated with labels")

	// 3. create snapshot job
	job := &batchv1.Job{
		TypeMeta: metav1.TypeMeta{
			Kind:       "Job",
			APIVersion: batchv1.SchemeGroupVersion.String(),
		},
		ObjectMeta: metav1.ObjectMeta{
			GenerateName:    ss.Name + "-",
			Namespace:       pod.Namespace,
			OwnerReferences: []metav1.OwnerReference{*metav1.NewControllerRef(ss, v1alpha1.SchemeGVK)},
			Labels:          ss.Labels,
			Annotations: map[string]string{
				labelPrefix + "_controller": v1alpha1.OperatorName,
			},
		},
		Spec: batchv1.JobSpec{
			Parallelism:           &one,
			Completions:           &one,
			ActiveDeadlineSeconds: &workerDeadlineSeconds,
			Template: v1.PodTemplateSpec{
				ObjectMeta: metav1.ObjectMeta{
					Labels: ss.Labels,
					Annotations: map[string]string{
						labelPrefix + "_controller": v1alpha1.OperatorName,
					},
				},
				Spec: v1.PodSpec{
					RestartPolicy:         v1.RestartPolicyOnFailure,
					ActiveDeadlineSeconds: &workerDeadlineSeconds,
					NodeName:              ss.Status.NodeName,
					ImagePullSecrets: func() []v1.LocalObjectReference {
						if h.cfg.ImagePullSecret != "" {
							return []v1.LocalObjectReference{{Name: h.cfg.ImagePullSecret}}
						}
						return nil
					}(),

					Containers: []v1.Container{{
						Name:  "worker",
						Image: h.cfg.SnapshotWorkerImage,
						Args: func() []string {
							args := []string{
								"--container=" + container,
								"--image=" + ss.Spec.ImageName,
							}
							if h.cfg.Verbose {
								args = append(args, "--verbose")
							}
							return args
						}(),
						VolumeMounts: []v1.VolumeMount{{
							Name:      "docker-socket",
							MountPath: dockerSocketPath,
						}, {
							Name:      "registry-secret",
							MountPath: dockerConfigDir,
						}},
					}},

					Volumes: []v1.Volume{{
						Name: "docker-socket",
						VolumeSource: v1.VolumeSource{HostPath: &v1.HostPathVolumeSource{
							Path: dockerSocketPath,
							Type: &hostPathSocket,
						}},
					}, {
						Name: "registry-secret",
						VolumeSource: v1.VolumeSource{Secret: &v1.SecretVolumeSource{
							SecretName:  ss.Spec.ImagePushSecret.Name,
							DefaultMode: &readOnly,
						}},
					}},
				},
			},
		},
	}

	// update snapshot before creating job, to elimate concurrently updating to snapshot
	if stale {
		stale = false
		if err := sdk.Update(ss); err != nil {
			return gerr.Wrap(err, "update snapshot failed")
		}
	}

	logger(ss).Info("creating snapshot job")
	return sdk.Create(job)
}

func querySnapshotJob(ss *v1alpha1.Snapshot) (*batchv1.Job, error) {
	// check jobRef
	if ss.Status.JobRef.Name != "" {
		job := &batchv1.Job{
			ObjectMeta: metav1.ObjectMeta{Name: ss.Status.JobRef.Name, Namespace: ss.GetNamespace()},
			TypeMeta:   metav1.TypeMeta{Kind: "Job", APIVersion: batchv1.SchemeGroupVersion.String()},
		}
		if err := sdk.Get(job); err != nil && !errors.IsNotFound(err) {
			return nil, gerr.Wrap(err, "get snapshot job failed")
		}
		if job.GetUID() != "" {
			return job, nil
		}
	}

	// list jobs by labels.
	selectors := make([]string, 0)
	if ss.Spec.Selector != nil {
		for k, v := range ss.Spec.Selector.MatchLabels {
			selectors = append(selectors, k+"="+v)
		}
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
	if err := sdk.List(ss.Namespace, jobs, sdk.WithListOptions(opts)); err != nil && !errors.IsNotFound(err) {
		return nil, gerr.Wrap(err, "list snapshot jobs failed")
	}

	// find job created by snapshot
	for _, job := range jobs.Items {
		if metav1.IsControlledBy(&job, ss) {
			ss.Status.JobRef.Name = job.Name
			logger(ss).WithField("job", job.Name).Debug("found job for snapshot with selector")
			return &job, nil
		}
	}
	return nil, nil
}

func updateCondition(ss *v1alpha1.Snapshot, job *batchv1.Job) (stale bool) {
	ss.Status.JobRef = v1.LocalObjectReference{Name: job.Name}
	var cond *v1alpha1.SnapshotCondition
	for _, c := range job.Status.Conditions {
		if c.Status == v1.ConditionTrue {
			switch c.Type {
			case batchv1.JobComplete:
				cond = newCondition(v1alpha1.SnapshotComplete, "JobCompleted", "snapshot job completed")
			case batchv1.JobFailed:
				cond = newCondition(v1alpha1.SnapshotFailed, "JobFailed", "snapshot job failed")
			}
			break
		}
	}

	if cond == nil {
		return false
	}

	newCondition := true
	for i, c := range ss.Status.Conditions {
		if c.Type == cond.Type {
			if c.Status == cond.Status {
				return false
			}
			ss.Status.Conditions[i] = *cond
			newCondition = false
			break
		}
	}
	if newCondition {
		ss.Status.Conditions = append(ss.Status.Conditions, *cond)
	}
	return true
}

func newCondition(cond v1alpha1.SnapshotConditionType, reason, message string) *v1alpha1.SnapshotCondition {
	return &v1alpha1.SnapshotCondition{
		Type:               cond,
		Status:             v1.ConditionTrue,
		LastProbeTime:      metav1.Now(),
		LastTransitionTime: metav1.Now(),
		Reason:             reason,
		Message:            message,
	}
}

func getContainerID(pod *v1.Pod, name string) string {
	for _, cs := range pod.Status.ContainerStatuses {
		if cs.Name == name {
			return strings.TrimPrefix(cs.ContainerID, containerPrefix)
		}
	}
	return ""
}

func (h *Handler) onUpdatingJob(job *batchv1.Job) error {
	owner := metav1.GetControllerOf(job)
	if owner == nil || owner.APIVersion != v1alpha1.SchemeGroupVersion.String() || owner.Kind != v1alpha1.Kind {
		logger(job).Debug("not a snapshot job")
		return nil
	}
	// query snapshot
	ss := &v1alpha1.Snapshot{
		TypeMeta:   metav1.TypeMeta{Kind: v1alpha1.Kind, APIVersion: v1alpha1.SchemeGroupVersion.String()},
		ObjectMeta: metav1.ObjectMeta{Name: owner.Name, Namespace: job.Namespace, UID: owner.UID},
	}
	if err := sdk.Get(ss); err != nil {
		return gerr.Wrap(err, "query snapshot failed")
	}

	// update snapshot conditions
	stale := updateCondition(ss, job)
	if stale {
		logger(ss).WithField("job", job.Name).Info("updating snapshot conditions")
		if err := sdk.Update(ss); err != nil {
			return gerr.Wrap(err, "update snapshot with condition change failed")
		}
	}
	return nil
}

func logger(obj metav1.Object) *logrus.Entry {
	return logrus.WithFields(logrus.Fields{
		"name":      obj.GetName(),
		"namespace": obj.GetNamespace(),
	})
}

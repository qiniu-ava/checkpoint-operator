package v1alpha1

import (
	batchv1 "k8s.io/api/batch/v1"
	"k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

type CheckpointList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata"`
	Items           []Checkpoint `json:"items"`
}

// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

type Checkpoint struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata"`
	Spec              CheckpointSpec   `json:"spec"`
	Status            CheckpointStatus `json:"status,omitempty"`
}

type CheckpointSpec struct {
	// PodName+ContainerName is the name of the running container going be have a checkpoint
	PodName       string `json:"podName"`
	ContainerName string `json:"containerName"`

	// ImageName is the checkpoint image name in the form of 'my-checkpoint:v0.0.1'
	// The full name for the pushed image will be my-docker-registry-server/my-checkpoint:v0.0.1
	// 'my-docker-registry-server' will be retrieved from ImagePushSecret
	ImageName string `json:"imageName"`

	// A label query over jobs.
	// Normally, the system sets this field for you.
	// More info: https://kubernetes.io/docs/concepts/overview/working-with-objects/labels/#label-selectors
	// +optional
	Selector *metav1.LabelSelector `json:"selector,omitempty" protobuf:"bytes,4,opt,name=selector"`

	// ImagePushSecret is a reference to a docker-registry secret in the same namespace to use for pushing checkout image,
	// same as an ImagePullSecret.
	// More info: https://kubernetes.io/docs/concepts/containers/images#specifying-imagepullsecrets-on-a-pod
	ImagePushSecret v1.LocalObjectReference `json:"imagePushSecret"`
}

type CheckpointStatus struct {
	// JobRef is a reference to the internal checkpoint job which does the real commit/push works
	JobRef v1.LocalObjectReference `json:"jobRef"`

	// NodeName is the name of the node the pod/container running on, the checkpoint job must run on this node
	NodeName string `json:"nodeName"`

	// The latest available observations of the underlying job
	// More info: https://kubernetes.io/docs/concepts/workloads/controllers/jobs-run-to-completion/
	// +optional
	// +patchMergeKey=type
	// +patchStrategy=merge
	Conditions []batchv1.JobCondition `json:"conditions,omitempty" patchStrategy:"merge" patchMergeKey:"type" protobuf:"bytes,1,rep,name=conditions"`
}

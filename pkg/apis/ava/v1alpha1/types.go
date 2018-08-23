package v1alpha1

import (
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

	// The latest available observations of the checkpoint
	// +optional
	// +patchMergeKey=type
	// +patchStrategy=merge
	Conditions []CheckpointCondition `json:"conditions,omitempty" patchStrategy:"merge" patchMergeKey:"type" protobuf:"bytes,1,rep,name=conditions"`
}

type CheckpointCondition struct {
	// Type of job condition, Complete or Failed.
	Type CheckpointConditionType `json:"type" protobuf:"bytes,1,opt,name=type,casttype=JobConditionType"`
	// Status of the condition, one of True, False, Unknown.
	Status v1.ConditionStatus `json:"status" protobuf:"bytes,2,opt,name=status,casttype=k8s.io/api/core/v1.ConditionStatus"`
	// Last time the condition was checked.
	// +optional
	LastProbeTime metav1.Time `json:"lastProbeTime,omitempty" protobuf:"bytes,3,opt,name=lastProbeTime"`
	// Last time the condition transit from one status to another.
	// +optional
	LastTransitionTime metav1.Time `json:"lastTransitionTime,omitempty" protobuf:"bytes,4,opt,name=lastTransitionTime"`
	// (brief) reason for the condition's last transition.
	// +optional
	Reason string `json:"reason,omitempty" protobuf:"bytes,5,opt,name=reason"`
	// Human readable message indicating details about last transition.
	// +optional
	Message string `json:"message,omitempty" protobuf:"bytes,6,opt,name=message"`
}

type CheckpointConditionType string

const (
	CheckpointComplete CheckpointConditionType = "Complete"
	CheckpointFailed   CheckpointConditionType = "Failed"
)

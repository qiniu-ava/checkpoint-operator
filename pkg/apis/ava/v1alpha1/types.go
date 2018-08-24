package v1alpha1

import (
	"k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

type SnapshotList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata"`
	Items           []Snapshot `json:"items"`
}

// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

type Snapshot struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata"`
	Spec              SnapshotSpec   `json:"spec"`
	Status            SnapshotStatus `json:"status,omitempty"`
}

type SnapshotSpec struct {
	// PodName+ContainerName is the name of the running container going be have a snapshot
	PodName       string `json:"podName"`
	ContainerName string `json:"containerName"`

	// ImageName is the snapshot image name in the form of 'my-snapshot:v0.0.1'
	// The full name for the pushed image will be my-docker-registry-server/my-snapshot:v0.0.1
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

type SnapshotStatus struct {
	// JobRef is a reference to the internal snapshot job which does the real commit/push works
	JobRef v1.LocalObjectReference `json:"jobRef"`

	// NodeName is the name of the node the pod/container running on, the snapshot job must run on this node
	NodeName string `json:"nodeName"`

	// The latest available observations of the snapshot
	// +optional
	// +patchMergeKey=type
	// +patchStrategy=merge
	Conditions []SnapshotCondition `json:"conditions,omitempty" patchStrategy:"merge" patchMergeKey:"type" protobuf:"bytes,1,rep,name=conditions"`
}

type SnapshotCondition struct {
	// Type of job condition, Complete or Failed.
	Type SnapshotConditionType `json:"type" protobuf:"bytes,1,opt,name=type,casttype=JobConditionType"`
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

type SnapshotConditionType string

const (
	SnapshotComplete SnapshotConditionType = "Complete"
	SnapshotFailed   SnapshotConditionType = "Failed"
)

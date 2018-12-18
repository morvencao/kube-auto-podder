package v1

import (
	corev1 "k8s.io/api/core/v1"
	meta_v1 "k8s.io/apimachinery/pkg/apis/meta/v1"

)

// +genclient
// +genclient:noStatus
// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// AutoPodder describes AutoPodder resource
type AutoPodder struct {
	// TypeMeta is the metadata for the resource, like kind and apiVersion
	meta_v1.TypeMeta		`json:",inline"`
	// ObjectMeta contains the metadata for the particular object, including:
	// - name
	// - namespace
	// - self link
	// - labels
	// - ...
	meta_v1.ObjectMeta		`json:"metadata,omitempty"`
	Spec AutoPodderSpec		`json:"spec"`
	Status AutoPodderSatus	`json:"status"`
}

// AutoPodderSpec is spec for AutoPodder resource
type AutoPodderSpec struct {
	PodName string	`json:"podName"`
	Image	string	`json:"image"`
	Tag		string	`json:"tag"`
}

// AutoPodderSatus is status for AutoPodder resource
type AutoPodderSatus struct {
	Phase	corev1.PodPhase	`json:"phase,omitempty" protobuf:"bytes,1,opt,name=phase,casttype=PodPhase"`
}

// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// AutoPodderList is a list of AutoPodder resources
type AutoPodderList struct {
	meta_v1.TypeMeta		`json:",inline"`
	meta_v1.ListMeta		`json:"metadata"`
	Items	[]AutoPodder	`json:"items"`
}

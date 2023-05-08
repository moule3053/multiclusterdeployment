package v1

import (
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// EDIT THIS FILE!  THIS IS SCAFFOLDING FOR YOU TO OWN!

// MultiClusterDeploymentSpec defines the desired state of MultiClusterDeployment
type MultiClusterDeploymentSpec struct {
	// INSERT ADDITIONAL SPEC FIELDS - desired state of cluster
	// Important: Run "make" to regenerate code after modifying this file

	// Name of the Deployment
	Name string `json:"name"`

	// Replicas is the desired number of replicas for the Deployment
	Replicas int32 `json:"replicas,omitempty"`

	// Clusters is the list of target clusters for the Deployment
	Clusters []string `json:"clusters"`

	// Image
	// +kubebuilder:validation:Required
	Image string `json:"image"`

	// CPURequest is the requested amount of CPU for the container
	CPURequest corev1.ResourceList `json:"cpuRequest"`

	// MemoryRequest is the requested amount of memory for the container
	MemoryRequest corev1.ResourceList `json:"memoryRequest"`

	// CPULimit is the CPU limit for the container
	CPULimit corev1.ResourceList `json:"cpuLimit"`

	// MemoryLimit is the memory limit for the container
	MemoryLimit corev1.ResourceList `json:"memoryLimit"`
}

// MultiClusterDeploymentStatus defines the observed state of MultiClusterDeployment
type MultiClusterDeploymentStatus struct {
	// INSERT ADDITIONAL STATUS FIELD - define observed state of cluster
	// Important: Run "make" to regenerate code after modifying this file
}

// +kubebuilder:object:root=true
// +kubebuilder:subresource:status

// MultiClusterDeployment is the Schema for the multiclusterdeployments API
type MultiClusterDeployment struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   MultiClusterDeploymentSpec   `json:"spec,omitempty"`
	Status MultiClusterDeploymentStatus `json:"status,omitempty"`
}

// +kubebuilder:object:root=true

// MultiClusterDeploymentList contains a list of MultiClusterDeployment
type MultiClusterDeploymentList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []MultiClusterDeployment `json:"items"`
}

func init() {
	SchemeBuilder.Register(&MultiClusterDeployment{}, &MultiClusterDeploymentList{})
}

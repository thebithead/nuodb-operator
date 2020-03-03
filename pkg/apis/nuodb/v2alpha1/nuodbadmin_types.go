package v2alpha1

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// NuodbAdminSpec defines the desired state of NuodbAdmin
// +k8s:openapi-gen=true
type NuodbAdminSpec struct {
	// INSERT ADDITIONAL SPEC FIELDS - desired state of cluster
	// Important: Run "operator-sdk generate k8s" to regenerate code after modifying this file
	// Add custom validation using kubebuilder tags: https://book-v1.book.kubebuilder.io/beyond_basics/generating_crd.html

	// adminCount
	// Number of admin service pods. Requires 1 server available for each
	// Admin Service
	// example: adminCount: 1
	AdminCount int32 `json:"adminCount"`

	// adminStorageClass
	// Admin persistent storage class name
	// example: adminStorageClass: glusterfs-storage
	AdminStorageClass string `json:"adminStorageClass"`

	// adminStorageSize
	// Admin service log volume size
	// example: adminStorageSize: 5Gi
	AdminStorageSize string `json:"adminStorageSize"`

	// storageMode
	// Run NuoDB using a persistent, local, disk volume "persistent"
	// or volatile storage "ephemeral".  Must be set to one of those values.
	// example: storageMode: persistent
	StorageMode string `json:"storageMode"`

	// insightsEnabled
	// Use to control Insights Opt In.  Insights provides database
	// monitoring.  Set to "true" to activate or "false" to deactivate
	// example: insightsEnabled: false
	InsightsEnabled bool `json:"insightsEnabled"`

	// apiServer
	// Load balancer service URL.  hostname:port (or LB address) for
	// nuoadmin process to connect to.
	// Example: apiServer: https://domain:8888
	ApiServer string `json:"apiServer"`

	// container
	// NuoDB fully qualified image name (FQIN) for the Docker image to use
	// container: "registry.connect.redhat.com/nuodb/nuodb-ce:latest"
	// Example: container: nuodb/nuodb-ce:latest
	Container string `json:"container"`
}

// NuodbAdminStatus defines the observed state of NuodbAdmin
// +k8s:openapi-gen=true
// +kubebuilder:subresource:status
type NuodbAdminStatus struct {
	// Admin Node Ready Count
	AdminReadyCount int32 `json:"adminReadyCount,omitempty"`
	// AdminHealth of the NuoDB Domain
	AdminHealth NuodbHealth `json:"adminHealth,omitempty"`
	// DomainHealth of the NuoDB Domain
	DomainHealth NuodbHealth `json:"domainHealth,omitempty"`
	// Orchestration phase of the NuoDB Domain
	Phase NuodbOrchestrationPhase `json:"phase,omitempty"`
	// ControllerVersion is the version of the controller that last updated the NuoDB Domain
	ControllerVersion string `json:"controllerVersion,omitempty"`
}

// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// NuodbAdmin is the Schema for the nuodbadmins API
// +k8s:openapi-gen=true
// +kubebuilder:resource:shortName=nuodbadmin
// +kubebuilder:categories=nuodbadmin
// +kubebuilder:printcolumn:name="Admin",type="string",JSONPath=".status.adminHealth"
// +kubebuilder:printcolumn:name="Domain",type="string",JSONPath=".status.domainHealth"
// +kubebuilder:printcolumn:name="Phase",type="string",JSONPath=".status.phase"
// +kubebuilder:printcolumn:name="ControllerVersion",type="string",JSONPath=".status.controllerVersion"
// +kubebuilder:printcolumn:name="Age",type="date",JSONPath=".metadata.creationTimestamp"
type NuodbAdmin struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   NuodbAdminSpec   `json:"spec,omitempty"`
	Status NuodbAdminStatus `json:"status,omitempty"`
}

// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// NuodbAdminList contains a list of NuodbAdmin
type NuodbAdminList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []NuodbAdmin `json:"items"`
}

func init() {
	SchemeBuilder.Register(&NuodbAdmin{}, &NuodbAdminList{})
}

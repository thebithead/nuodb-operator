package v2alpha1

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// NuodbInsightsServerSpec defines the desired state of NuodbInsightsServer
// +k8s:openapi-gen=true
type NuodbInsightsServerSpec struct {
	// INSERT ADDITIONAL SPEC FIELDS - desired state of cluster
	// Important: Run "operator-sdk generate k8s" to regenerate code after modifying this file
	// Add custom validation using kubebuilder tags: https://book-v1.book.kubebuilder.io/beyond_basics/generating_crd.html

	// ElasticSearch Version
	ElasticVersion string `json:"elasticVersion"`

	// ElasticSearch Node Count
	ElasticNodeCount int32 `json:"elasticNodeCount"`

	// Kibana Version
	KibanaVersion string `json:"kibanaVersion"`

	// Kibana Node Count
	KibanaNodeCount int32 `json:"kibanaNodeCount"`

	// Persistent Storage Class for internal components.
	StorageClass string `json:"storageClass"`
}

// NuodbInsightsServerStatus defines the observed state of NuodbInsightsServer
// +k8s:openapi-gen=true
type NuodbInsightsServerStatus struct {
	// INSERT ADDITIONAL STATUS FIELD - define observed state of cluster
	// Important: Run "operator-sdk generate k8s" to regenerate code after modifying this file
	// Add custom validation using kubebuilder tags: https://book-v1.book.kubebuilder.io/beyond_basics/generating_crd.html
}

// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// NuodbInsightsServer is the Schema for the nuodbinsightsservers API
// +k8s:openapi-gen=true
// +kubebuilder:subresource:status
type NuodbInsightsServer struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   NuodbInsightsServerSpec   `json:"spec,omitempty"`
	Status NuodbInsightsServerStatus `json:"status,omitempty"`
}

// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// NuodbInsightsServerList contains a list of NuodbInsightsServer
type NuodbInsightsServerList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []NuodbInsightsServer `json:"items"`
}

func init() {
	SchemeBuilder.Register(&NuodbInsightsServer{}, &NuodbInsightsServerList{})
}

package v2alpha1

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// NuodbYcsbWlSpec defines the desired state of NuodbYcsbWl
// +k8s:openapi-gen=true
type NuodbYcsbWlSpec struct {
	// INSERT ADDITIONAL SPEC FIELDS - desired state of cluster
	// Important: Run "operator-sdk generate k8s" to regenerate code after modifying this file
	// Add custom validation using kubebuilder tags: https://book-v1.book.kubebuilder.io/beyond_basics/generating_crd.html

	// dbName
	// NuoDB Database name.  must consist of lowercase alphanumeric
	// characters '[a-z0-9]+'
	// example: dbName: test
	DbName string `json:"dbName"`

	YcsbWorkloadCount int32 `json:"ycsbWorkloadCount"`

	// ycsbLoadName
	YcsbLoadName string `json:"ycsbLoadName"`

	YcsbWorkload string `json:"ycsbWorkload"`

	YcsbLbPolicy string `json:"ycsbLbPolicy"`

	YcsbNoOfProcesses int32 `json:"ycsbNoOfProcesses"`

	YcsbNoOfRows int32 `json:"ycsbNoOfRows"`

	YcsbNoOfIterations int32 `json:"ycsbNoOfIterations"`

	YcsbOpsPerIteration int32 `json:"ycsbOpsPerIteration"`

	YcsbMaxDelay int32 `json:"ycsbMaxDelay"`

	YcsbDbSchema string `json:"ycsbDbSchema"`

	// container
	// NuoDB YCSB fully qualified image name (FQIN) for the Docker image to use.
	// Example: container: nuodb/ycsb:latest
	YcsbContainer string `json:"ycsbContainer"`

}

// NuodbYcsbWlStatus defines the observed state of NuodbYcsbWl
// +k8s:openapi-gen=true
type NuodbYcsbWlStatus struct {
	// INSERT ADDITIONAL STATUS FIELD - define observed state of cluster
	// Important: Run "operator-sdk generate k8s" to regenerate code after modifying this file
	// Add custom validation using kubebuilder tags: https://book-v1.book.kubebuilder.io/beyond_basics/generating_crd.html
}

// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// NuodbYcsbWl is the Schema for the nuodbycsbwls API
// +k8s:openapi-gen=true
// +kubebuilder:subresource:status
type NuodbYcsbWl struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   NuodbYcsbWlSpec   `json:"spec,omitempty"`
	Status NuodbYcsbWlStatus `json:"status,omitempty"`
}

// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// NuodbYcsbWlList contains a list of NuodbYcsbWl
type NuodbYcsbWlList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []NuodbYcsbWl `json:"items"`
}

func init() {
	SchemeBuilder.Register(&NuodbYcsbWl{}, &NuodbYcsbWlList{})
}

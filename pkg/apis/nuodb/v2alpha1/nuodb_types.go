package v2alpha1

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// NuodbSpec defines the desired state of Nuodb
// +k8s:openapi-gen=true
type NuodbSpec struct {
	// INSERT ADDITIONAL SPEC FIELDS - desired state of cluster
	// Important: Run "operator-sdk generate k8s" to regenerate code after modifying this file
	// Add custom validation using kubebuilder tags: https://book-v1.book.kubebuilder.io/beyond_basics/generating_crd.html

	// container
	// NuoDB fully qualified image name (FQIN) for the Docker image to use
	// container: "registry.connect.redhat.com/nuodb/nuodb-ce:latest"
	// Example: container: nuodb/nuodb-ce:latest
	Container string `json:"container"`

	// dbAvailability
	//
    // Availability requirement for this database.  Values are:
	//
	// single-instance - The operator manages a single instance of each NuoDB
	//                   component (Admin, SM, TE).  In the event that one
	//                   component goes down or becomes unavailable, the
	//                   operator will automatically replace the failed component.
	//                   This is the most resource friendly option for applications
	//                   that can tolerate a brief outage.
	//
	// multiple-instance - The operator manages multiple instances of each
	//                     NuoDB component (Admin, SM, TE).
	//
	// high-availability - The operator will maximize performance and reliability.
	//
	// manual - The operator will enforce custom provided Admin, SM, TE instance counts.
	//
	// The default is: "high-availability"
	DbAvailability string `json:"dbAvailability"`

	// dbName
	// NuoDB Database name.  must consist of lowercase alphanumeric
	// characters '[a-z0-9]+'
	// example: dbName: test
	DbName string `json:"dbName"`
	
	// dbPassword
	// Database password
	// Example: dbPassword: secret
	DbPassword string `json:"dbPassword"`

    // dbUser
	// Name of Database user
	// example: dbUser: dba
	DbUser string `json:"dbUser"`

	// engineOptions
	// Additional "nuodb" engine options
	// Format: <option> <value> <option> <value> ...
	// Example: engineOptions: ""
	EngineOptions string `json:"engineOptions"`

	// smCount
	// Number of SM service pods. Requires 1 SM available for each
	// NuoDB Database
	// example: smCount: 1
	SmCount int32 `json:"smCount"`

	// smCpu
	// SM CPU cores to request
	// example: smCpu: 1
	SmCpu string `json:"smCpu"`

	// smMemory
	// SM memory
	// example: smMemory: 2Gi
	SmMemory string `json:"smMemory"`

	// smStorageClass
	// SM persistent storage class name
	// Example: smStorageClass: local-disk
	SmStorageClass string `json:"smStorageClass"`

	// smStorageSize
	// Storage manager (SM) volume size
	// Example: smStorageSize: 20Gi
	SmStorageSize string `json:"smStorageSize"`

	// storageMode
	// Run NuoDB using a persistent, local, disk volume "persistent"
	// or volatile storage "ephemeral".  Must be set to one of those values.
	// example: storageMode: persistent
	StorageMode string `json:"storageMode"`

	// teCount
	// Number of transaction engines (TE) nodes.
	// Limit is 3 in CE version of NuoDB
	// Example: teCount: 1
	TeCount int32 `json:"teCount"`

	// teCpu
	// TE CPU cores to request
	// Example: teCpu: 1
	TeCpu string `json:"teCpu"`

        // teMemory
	// TE memory
	// Example: teMemory: 2Gi
	TeMemory string `json:"teMemory"`
}


// IsDegraded returns true if the current status is worse than the previous.
//noinspection ALL,GoReceiverNames
func (nuodbStatus NuodbStatus) IsDegraded(prev NuodbStatus) bool {
	return nuodbStatus.DatabaseHealth.Less(prev.DatabaseHealth)
}


// NuodbStatus defines the observed state of Nuodb
// +k8s:openapi-gen=true
// +kubebuilder:subresource:status
type NuodbStatus struct {
	// SM Node Ready Count
	SmReadyCount int32 `json:"smReadyCount,omitempty"`
	// TE Node Ready Count
	TeReadyCount int32 `json:"teReadyCount,omitempty"`
	// AdminHealth of the NuoDB Database
	SmHealth NuodbHealth `json:"smHealth,omitempty"`
	// TE Health of the NuoDB Database
	TeHealth NuodbHealth `json:"teHealth,omitempty"`
	// Health of the NuoDB Database
	DatabaseHealth NuodbHealth `json:"databaseHealth,omitempty"`
	// Orchestration phase of the NuoDB Database
	Phase           NuodbOrchestrationPhase `json:"phase,omitempty"`
	// ControllerVersion is the version of the controller that last updated the NuoDB Domain
	ControllerVersion string `json:"controllerVersion,omitempty"`
}

// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// Nuodb is the Schema for the nuodbs API
// +k8s:openapi-gen=true
// +kubebuilder:resource:shortName=nuodb
// +kubebuilder:categories=nuodb
// +kubebuilder:printcolumn:name="SM",type="string",JSONPath=".status.smHealth"
// +kubebuilder:printcolumn:name="TE",type="string",JSONPath=".status.teHealth"
// +kubebuilder:printcolumn:name="Database",type="string",JSONPath=".status.databaseHealth"
// +kubebuilder:printcolumn:name="Phase",type="string",JSONPath=".status.phase"
// +kubebuilder:printcolumn:name="ControllerVersion",type="string",JSONPath=".status.controllerVersion"
// +kubebuilder:printcolumn:name="Age",type="date",JSONPath=".metadata.creationTimestamp"
type Nuodb struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   NuodbSpec   `json:"spec,omitempty"`
	Status NuodbStatus `json:"status,omitempty"`
}

// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// NuodbList contains a list of Nuodb
type NuodbList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []Nuodb `json:"items"`
}

func init() {
	SchemeBuilder.Register(&Nuodb{}, &NuodbList{})
}

/*


Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package v1

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// EDIT THIS FILE!  THIS IS SCAFFOLDING FOR YOU TO OWN!
// NOTE: json tags are required.  Any new fields you add must have json tags for the fields to be serialized.

// SubnetSpec defines the desired state of Subnet
type SubnetSpec struct {
	// ID represents the subnet id
	ID string `json:"ID,omitempty"`

	// Type represents whether it is an IPv4 or IPv6
	Type string `json:"type,omitempty"`

	// CIDR represents the Ip Adress Range
	CIDR string `json:"cidr,omitempty"`

	// NetworkGlobal represents the network which belongs to the subnet
	NetworkGlobalID string `json:"networkGlobalID,omitempty"`

	// PartiionID represents the location of the physical servers
	PartitionID string `json:"partitionID,omitempty"`

	// SubnetParentID represents the parent of the subnet if present
	SubnetParentID string `json:"subnetParentID,omitempty"`
}

// SubnetStatus defines the observed state of Subnet
type SubnetStatus struct {
	// Capacity represents the capacity of the subnet
	Capacity int `json:"capacity,omitempty"`

	// CapacityLeft represents the available capacity of the subnet
	CapacityLeft int `json:"capacityLeft,omitempty"`
}

// +kubebuilder:object:root=true

// Subnet is the Schema for the subnets API
type Subnet struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   SubnetSpec   `json:"spec,omitempty"`
	Status SubnetStatus `json:"status,omitempty"`
}

// +kubebuilder:object:root=true

// SubnetList contains a list of Subnet
type SubnetList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []Subnet `json:"items"`
}

func init() {
	SchemeBuilder.Register(&Subnet{}, &SubnetList{})
}

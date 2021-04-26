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

package v1alpha1

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// EDIT THIS FILE!  THIS IS SCAFFOLDING FOR YOU TO OWN!
// NOTE: json tags are required.  Any new fields you add must have json tags for the fields to be serialized.

// LogicSpec defines the desired state of Logic
type LogicSpec struct {
	// INSERT ADDITIONAL SPEC FIELDS - desired state of cluster
	// Important: Run "make" to regenerate code after modifying this file
	// Replica count of the DatabaseLogic.
	// +kubebuilder:validation:Minimum=1
	// +kubebuilder:validation:Maximum=10
	Size int32 `json:"size,omitempty"`
	// EPF application version.
	// Default value "<empty>".
	AppVersion string `json:"appVersion,omitempty"`
	// EPF Database version.
	// Default value "<empty>".
	DatabaseVersion string `json:"databaseVersion,omitempty"`
	// Uses the EPF image for the deployment.
	// Default value "<empty>".
	AppImage string `json:"appimage,omitempty"`
	//Use the App name Image identyfy the app
	// Default value "<empty>".
	AppName string `json:"appname,omitempty"`
	// Uses the proxy deployment.(Go proxy)
	// Default value "<empty>".
	SideCarImage string `json:"sideCarImage,omitempty"`
	// Uses the proxy deploment for proxy name
	// Default value "<empty>".
	SideCarName string `json:"sideCarName,omitempty"`
}

// LogicStatus defines the observed state of Logic
type LogicStatus struct {
	// INSERT ADDITIONAL STATUS FIELD - define observed state of cluster
	// Important: Run "make" to regenerate code after modifying this file
	PodNames          []string `json:"podNames"`
	AvailableReplicas int32    `json:"availableReplicas"`
}

// +kubebuilder:object:root=true
// +kubebuilder:subresource:status

// Logic is the Schema for the logics API
type Logic struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   LogicSpec   `json:"spec,omitempty"`
	Status LogicStatus `json:"status,omitempty"`
}

// +kubebuilder:object:root=true

// LogicList contains a list of Logic
type LogicList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []Logic `json:"items"`
}

func init() {
	SchemeBuilder.Register(&Logic{}, &LogicList{})
}

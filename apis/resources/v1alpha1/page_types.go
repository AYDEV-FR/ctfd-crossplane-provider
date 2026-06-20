/*
Copyright 2025 The Crossplane Authors.

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
	"reflect"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"

	xpv1 "github.com/crossplane/crossplane-runtime/v2/apis/common/v1"
	xpv2 "github.com/crossplane/crossplane-runtime/v2/apis/common/v2"
)

// PageParameters are the configurable fields of a Page.
type PageParameters struct {
	// Title of the page.
	Title string `json:"title"`

	// Route is the URL path the page is served at, e.g. "rules" or
	// "sponsors/acme".
	Route string `json:"route"`

	// Content of the page, in the configured Format.
	Content string `json:"content"`

	// Format of the Content, either "markdown" or "html".
	// +optional
	// +kubebuilder:default=markdown
	// +kubebuilder:validation:Enum=markdown;html
	Format string `json:"format,omitempty"`

	// Draft keeps the page unpublished and only visible to admins.
	// +optional
	// +kubebuilder:default=false
	Draft bool `json:"draft,omitempty"`

	// Hidden removes the page from the navigation bar while keeping it
	// reachable by its route.
	// +optional
	// +kubebuilder:default=false
	Hidden bool `json:"hidden,omitempty"`

	// AuthRequired restricts the page to authenticated users.
	// +optional
	// +kubebuilder:default=false
	AuthRequired bool `json:"authRequired,omitempty"`
}

// PageObservation are the observable fields of a Page.
type PageObservation struct {
	// ID is the CTFd-assigned numeric identifier of the page.
	ID int `json:"id,omitempty"`

	// Route the page is served at, as reported by CTFd.
	Route string `json:"route,omitempty"`
}

// A PageSpec defines the desired state of a Page.
type PageSpec struct {
	xpv2.ManagedResourceSpec `json:",inline"`
	ForProvider              PageParameters `json:"forProvider"`
}

// A PageStatus represents the observed state of a Page.
type PageStatus struct {
	xpv1.ResourceStatus `json:",inline"`
	AtProvider          PageObservation `json:"atProvider,omitempty"`
}

// +kubebuilder:object:root=true

// A Page is a CTFd content page.
// +kubebuilder:printcolumn:name="READY",type="string",JSONPath=".status.conditions[?(@.type=='Ready')].status"
// +kubebuilder:printcolumn:name="SYNCED",type="string",JSONPath=".status.conditions[?(@.type=='Synced')].status"
// +kubebuilder:printcolumn:name="ROUTE",type="string",JSONPath=".spec.forProvider.route"
// +kubebuilder:printcolumn:name="EXTERNAL-NAME",type="string",JSONPath=".metadata.annotations.crossplane\\.io/external-name"
// +kubebuilder:printcolumn:name="AGE",type="date",JSONPath=".metadata.creationTimestamp"
// +kubebuilder:subresource:status
// +kubebuilder:resource:scope=Namespaced,categories={crossplane,managed,ctfd}
type Page struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   PageSpec   `json:"spec"`
	Status PageStatus `json:"status,omitempty"`
}

// +kubebuilder:object:root=true

// PageList contains a list of Page
type PageList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []Page `json:"items"`
}

// Page type metadata.
var (
	PageKind             = reflect.TypeOf(Page{}).Name()
	PageGroupKind        = schema.GroupKind{Group: Group, Kind: PageKind}.String()
	PageKindAPIVersion   = PageKind + "." + SchemeGroupVersion.String()
	PageGroupVersionKind = SchemeGroupVersion.WithKind(PageKind)
)

func init() {
	SchemeBuilder.Register(&Page{}, &PageList{})
}

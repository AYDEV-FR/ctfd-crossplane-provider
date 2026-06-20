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

// ThemeParameters are the configurable fields of a Theme. A Theme is an
// instance-wide singleton: it writes the theme-related entries of the CTFd
// configuration (the "ctf_theme", "theme_header", "theme_footer" and
// "theme_settings" keys).
type ThemeParameters struct {
	// Name is the active CTFd theme, e.g. "core-beta". It maps to the
	// "ctf_theme" configuration key.
	Name string `json:"name"`

	// Header is custom HTML injected into the <head> of every themed page. It
	// maps to the "theme_header" configuration key.
	// +optional
	Header *string `json:"header,omitempty"`

	// Footer is custom HTML injected into the footer of every themed page. It
	// maps to the "theme_footer" configuration key.
	// +optional
	Footer *string `json:"footer,omitempty"`

	// Settings is a theme-specific JSON settings document. It maps to the
	// "theme_settings" configuration key.
	// +optional
	Settings *string `json:"settings,omitempty"`
}

// ThemeObservation are the observable fields of a Theme.
type ThemeObservation struct {
	// Name is the active theme as reported by CTFd.
	Name string `json:"name,omitempty"`
}

// A ThemeSpec defines the desired state of a Theme.
type ThemeSpec struct {
	xpv2.ManagedResourceSpec `json:",inline"`
	ForProvider              ThemeParameters `json:"forProvider"`
}

// A ThemeStatus represents the observed state of a Theme.
type ThemeStatus struct {
	xpv1.ResourceStatus `json:",inline"`
	AtProvider          ThemeObservation `json:"atProvider,omitempty"`
}

// +kubebuilder:object:root=true

// A Theme configures the instance-wide CTFd theme settings.
// +kubebuilder:printcolumn:name="READY",type="string",JSONPath=".status.conditions[?(@.type=='Ready')].status"
// +kubebuilder:printcolumn:name="SYNCED",type="string",JSONPath=".status.conditions[?(@.type=='Synced')].status"
// +kubebuilder:printcolumn:name="THEME",type="string",JSONPath=".spec.forProvider.name"
// +kubebuilder:printcolumn:name="AGE",type="date",JSONPath=".metadata.creationTimestamp"
// +kubebuilder:subresource:status
// +kubebuilder:resource:scope=Namespaced,categories={crossplane,managed,ctfd}
type Theme struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   ThemeSpec   `json:"spec"`
	Status ThemeStatus `json:"status,omitempty"`
}

// +kubebuilder:object:root=true

// ThemeList contains a list of Theme
type ThemeList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []Theme `json:"items"`
}

// Theme type metadata.
var (
	ThemeKind             = reflect.TypeOf(Theme{}).Name()
	ThemeGroupKind        = schema.GroupKind{Group: Group, Kind: ThemeKind}.String()
	ThemeKindAPIVersion   = ThemeKind + "." + SchemeGroupVersion.String()
	ThemeGroupVersionKind = SchemeGroupVersion.WithKind(ThemeKind)
)

func init() {
	SchemeBuilder.Register(&Theme{}, &ThemeList{})
}

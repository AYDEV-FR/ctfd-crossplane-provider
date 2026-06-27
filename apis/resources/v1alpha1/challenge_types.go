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

// Requirements describes the prerequisites a player must meet before a
// challenge (or hint) becomes available.
type Requirements struct {
	// Anonymize controls the behavior when the prerequisites are not met:
	//   - "false" (or unset): the resource is hidden until validated.
	//   - "true": the resource is anonymized (visible, but with little info).
	//   - "preview": the resource is visible with preview-only data.
	// +optional
	// +kubebuilder:validation:Enum=true;false;preview
	Anonymize *string `json:"anonymize,omitempty"`

	// Prerequisites is the list of challenge IDs that must be solved before
	// this resource is unlocked.
	// +optional
	Prerequisites []int `json:"prerequisites,omitempty"`
}

// ChallengeParameters are the configurable fields of a Challenge.
type ChallengeParameters struct {
	// Name of the challenge.
	Name string `json:"name"`

	// Category the challenge belongs to.
	Category string `json:"category"`

	// Description of the challenge, displayed to players.
	Description string `json:"description"`

	// Type of the challenge. CTFd ships with "standard" and "dynamic".
	// +optional
	// +kubebuilder:default=standard
	Type string `json:"type,omitempty"`

	// State of the challenge, either "hidden" or "visible". When left unset the
	// state is not managed: it is not reconciled, and CTFd's own default applies
	// at creation.
	// +optional
	// +kubebuilder:validation:Enum=hidden;visible
	State string `json:"state,omitempty"`

	// Value is the number of points awarded for solving a standard challenge.
	// Ignored for dynamic challenges, which derive their value from Initial,
	// Decay and Minimum.
	// +optional
	Value *int `json:"value,omitempty"`

	// Initial is the starting value of a dynamic challenge.
	// +optional
	Initial *int `json:"initial,omitempty"`

	// Decay controls how quickly a dynamic challenge loses value as it is
	// solved. Its meaning depends on Function.
	// +optional
	Decay *int `json:"decay,omitempty"`

	// Minimum is the floor value of a dynamic challenge.
	// +optional
	Minimum *int `json:"minimum,omitempty"`

	// Function is the decay function of a dynamic challenge, either "linear"
	// or "logarithmic".
	// +optional
	// +kubebuilder:validation:Enum=linear;logarithmic
	Function *string `json:"function,omitempty"`

	// MaxAttempts is the maximum number of submissions allowed. 0 means
	// unlimited.
	// +optional
	MaxAttempts *int `json:"maxAttempts,omitempty"`

	// ConnectionInfo gives players the information required to reach the
	// challenge infrastructure (e.g. an SSH command or a URL).
	// +optional
	ConnectionInfo *string `json:"connectionInfo,omitempty"`

	// Attribution credits the author of the challenge.
	// +optional
	Attribution *string `json:"attribution,omitempty"`

	// NextID is the ID of the challenge proposed to the player after solving
	// this one.
	// +optional
	NextID *int `json:"nextID,omitempty"`

	// Requirements the player must meet before the challenge is unlocked.
	// +optional
	Requirements *Requirements `json:"requirements,omitempty"`

	// Flags are the flags that solve this challenge. They are managed together
	// with the challenge: adding, editing or removing an entry creates, replaces
	// or deletes the corresponding CTFd flag.
	// +optional
	Flags []ChallengeFlag `json:"flags,omitempty"`

	// Hints are the hints offered for this challenge. They are managed together
	// with the challenge: adding, editing or removing an entry creates, replaces
	// or deletes the corresponding CTFd hint.
	// +optional
	Hints []ChallengeHint `json:"hints,omitempty"`
}

// A ChallengeFlag is a flag that solves the challenge it is declared on.
type ChallengeFlag struct {
	// Content of the flag. For a "static" flag this is the exact (or
	// case-insensitive) string players must submit; for a "regex" flag this is
	// the regular expression their submission must match.
	Content string `json:"content"`

	// Type of the flag, either "static" or "regex".
	// +optional
	// +kubebuilder:default=static
	// +kubebuilder:validation:Enum=static;regex
	Type string `json:"type,omitempty"`

	// Data carries flag options. Set to "case_insensitive" to make a static
	// flag case-insensitive; leave empty for the default case-sensitive
	// behavior.
	// +optional
	// +kubebuilder:validation:Enum="";case_insensitive
	Data string `json:"data,omitempty"`
}

// A ChallengeHint is a hint offered for the challenge it is declared on.
type ChallengeHint struct {
	// Content of the hint, revealed to players once unlocked.
	Content string `json:"content"`

	// Title of the hint, shown before it is unlocked.
	// +optional
	Title *string `json:"title,omitempty"`

	// Cost in points to unlock the hint. 0 makes it free.
	// +optional
	// +kubebuilder:default=0
	Cost int `json:"cost,omitempty"`

	// Prerequisites are the hints a player must unlock before this hint
	// becomes available. CTFd assigns hint IDs only at creation time, so each
	// prerequisite is referenced by the 0-based index of the target hint in
	// this challenge's hints list. This lets hints be chained into a
	// progressive, ordered sequence. An index must point to another hint in
	// the same list (no self-references and no cycles).
	// +optional
	Prerequisites []int `json:"prerequisites,omitempty"`

	// Anonymize controls how the hint behaves while its prerequisites are not
	// met. It has no effect without Prerequisites. See Requirements.Anonymize
	// for the accepted values.
	// +optional
	// +kubebuilder:validation:Enum=true;false;preview
	Anonymize *string `json:"anonymize,omitempty"`
}

// ChallengeObservation are the observable fields of a Challenge.
type ChallengeObservation struct {
	// ID is the CTFd-assigned numeric identifier of the challenge.
	ID int `json:"id,omitempty"`

	// Value is the current point value of the challenge as computed by CTFd.
	Value int `json:"value,omitempty"`

	// State of the challenge as reported by CTFd.
	State string `json:"state,omitempty"`

	// Type of the challenge as reported by CTFd.
	Type string `json:"type,omitempty"`

	// Solves is the number of times the challenge has been solved.
	Solves int `json:"solves,omitempty"`

	// Flags is the number of flags currently attached to the challenge.
	Flags int `json:"flags,omitempty"`

	// Hints is the number of hints currently attached to the challenge.
	Hints int `json:"hints,omitempty"`
}

// A ChallengeSpec defines the desired state of a Challenge.
type ChallengeSpec struct {
	xpv2.ManagedResourceSpec `json:",inline"`
	ForProvider              ChallengeParameters `json:"forProvider"`
}

// A ChallengeStatus represents the observed state of a Challenge.
type ChallengeStatus struct {
	xpv1.ResourceStatus `json:",inline"`
	AtProvider          ChallengeObservation `json:"atProvider,omitempty"`
}

// +kubebuilder:object:root=true

// A Challenge is a CTFd challenge.
// +kubebuilder:printcolumn:name="READY",type="string",JSONPath=".status.conditions[?(@.type=='Ready')].status"
// +kubebuilder:printcolumn:name="SYNCED",type="string",JSONPath=".status.conditions[?(@.type=='Synced')].status"
// +kubebuilder:printcolumn:name="CATEGORY",type="string",JSONPath=".spec.forProvider.category"
// +kubebuilder:printcolumn:name="EXTERNAL-NAME",type="string",JSONPath=".metadata.annotations.crossplane\\.io/external-name"
// +kubebuilder:printcolumn:name="AGE",type="date",JSONPath=".metadata.creationTimestamp"
// +kubebuilder:subresource:status
// +kubebuilder:resource:scope=Namespaced,categories={crossplane,managed,ctfd}
type Challenge struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   ChallengeSpec   `json:"spec"`
	Status ChallengeStatus `json:"status,omitempty"`
}

// +kubebuilder:object:root=true

// ChallengeList contains a list of Challenge
type ChallengeList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []Challenge `json:"items"`
}

// Challenge type metadata.
var (
	ChallengeKind             = reflect.TypeOf(Challenge{}).Name()
	ChallengeGroupKind        = schema.GroupKind{Group: Group, Kind: ChallengeKind}.String()
	ChallengeKindAPIVersion   = ChallengeKind + "." + SchemeGroupVersion.String()
	ChallengeGroupVersionKind = SchemeGroupVersion.WithKind(ChallengeKind)
)

func init() {
	SchemeBuilder.Register(&Challenge{}, &ChallengeList{})
}

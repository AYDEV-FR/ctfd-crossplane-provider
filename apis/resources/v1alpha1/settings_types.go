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

// SettingsParameters configure the instance-wide CTFd settings (the admin
// "Config" page). Settings is a singleton: keep a single Settings resource per
// CTFd instance. Every field maps to a CTFd configuration key and is optional —
// only the fields you set are written, so unset fields keep CTFd's value and do
// not cause perpetual updates.
type SettingsParameters struct {
	// Name of the CTF ("ctf_name").
	// +optional
	Name *string `json:"name,omitempty"`

	// Description of the CTF ("ctf_description").
	// +optional
	Description *string `json:"description,omitempty"`

	// Theme is the active CTFd theme, e.g. "core-beta" ("ctf_theme").
	// +optional
	Theme *string `json:"theme,omitempty"`

	// ThemeHeader is custom HTML injected into the <head> ("theme_header").
	// +optional
	ThemeHeader *string `json:"themeHeader,omitempty"`

	// ThemeFooter is custom HTML injected into the footer ("theme_footer").
	// +optional
	ThemeFooter *string `json:"themeFooter,omitempty"`

	// ThemeSettings is a theme-specific JSON document ("theme_settings").
	// +optional
	ThemeSettings *string `json:"themeSettings,omitempty"`

	// DefaultLocale is the default UI language, e.g. "en" ("default_locale").
	// +optional
	DefaultLocale *string `json:"defaultLocale,omitempty"`

	// ChallengeVisibility: "public", "private", or "admins"
	// ("challenge_visibility").
	// +optional
	// +kubebuilder:validation:Enum=public;private;admins
	ChallengeVisibility *string `json:"challengeVisibility,omitempty"`

	// AccountVisibility: "public", "private", or "admins"
	// ("account_visibility").
	// +optional
	// +kubebuilder:validation:Enum=public;private;admins
	AccountVisibility *string `json:"accountVisibility,omitempty"`

	// ScoreVisibility: "public", "private", "hidden", or "admins"
	// ("score_visibility").
	// +optional
	// +kubebuilder:validation:Enum=public;private;hidden;admins
	ScoreVisibility *string `json:"scoreVisibility,omitempty"`

	// RegistrationVisibility: "public", "private", "mlc", or "admins"
	// ("registration_visibility").
	// +optional
	RegistrationVisibility *string `json:"registrationVisibility,omitempty"`

	// Start is the competition start time, as a Unix timestamp string ("start").
	// +optional
	Start *string `json:"start,omitempty"`

	// End is the competition end time, as a Unix timestamp string ("end").
	// +optional
	End *string `json:"end,omitempty"`

	// Freeze is the scoreboard freeze time, as a Unix timestamp string
	// ("freeze").
	// +optional
	Freeze *string `json:"freeze,omitempty"`

	// Paused pauses the CTF when true ("paused").
	// +optional
	Paused *bool `json:"paused,omitempty"`

	// UserMode is "users" or "teams" ("user_mode").
	// +optional
	// +kubebuilder:validation:Enum=users;teams
	UserMode *string `json:"userMode,omitempty"`

	// TeamSize caps the number of members per team ("team_size").
	// +optional
	TeamSize *int `json:"teamSize,omitempty"`

	// VerifyEmails requires email verification on registration
	// ("verify_emails").
	// +optional
	VerifyEmails *bool `json:"verifyEmails,omitempty"`

	// NameChanges allows players to change their name ("name_changes").
	// +optional
	NameChanges *bool `json:"nameChanges,omitempty"`

	// RegistrationCode required to register, if set ("registration_code").
	// +optional
	RegistrationCode *string `json:"registrationCode,omitempty"`

	// ChallengeRatings enables challenge ratings ("challenge_ratings").
	// +optional
	ChallengeRatings *string `json:"challengeRatings,omitempty"`

	// HintsFreePublicAccess lets anonymous users see free hints
	// ("hints_free_public_access").
	// +optional
	HintsFreePublicAccess *bool `json:"hintsFreePublicAccess,omitempty"`

	// HTMLSanitization enables HTML sanitization ("html_sanitization").
	// +optional
	HTMLSanitization *bool `json:"htmlSanitization,omitempty"`

	// RobotsTxt is the content served at /robots.txt ("robots_txt").
	// +optional
	RobotsTxt *string `json:"robotsTxt,omitempty"`

	// TOSURL links to the terms of service ("tos_url").
	// +optional
	TOSURL *string `json:"tosURL,omitempty"`

	// TOSText is the inline terms of service ("tos_text").
	// +optional
	TOSText *string `json:"tosText,omitempty"`

	// PrivacyURL links to the privacy policy ("privacy_url").
	// +optional
	PrivacyURL *string `json:"privacyURL,omitempty"`

	// PrivacyText is the inline privacy policy ("privacy_text").
	// +optional
	PrivacyText *string `json:"privacyText,omitempty"`

	// Mail configures outbound email (the Integrations > Email section).
	// +optional
	Mail *MailSettings `json:"mail,omitempty"`
}

// MailSettings configure CTFd's SMTP integration.
type MailSettings struct {
	// Server is the SMTP host ("mail_server").
	// +optional
	Server *string `json:"server,omitempty"`

	// Port is the SMTP port ("mail_port").
	// +optional
	Port *string `json:"port,omitempty"`

	// Username for SMTP auth ("mail_username").
	// +optional
	Username *string `json:"username,omitempty"`

	// PasswordSecretRef references a Secret key holding the SMTP password
	// ("mail_password"). Provide the password via a Secret rather than inline.
	// +optional
	PasswordSecretRef *xpv1.SecretKeySelector `json:"passwordSecretRef,omitempty"`

	// TLS enables STARTTLS ("mail_tls").
	// +optional
	TLS *bool `json:"tls,omitempty"`

	// SSL enables implicit TLS ("mail_ssl").
	// +optional
	SSL *bool `json:"ssl,omitempty"`

	// UseAuth enables SMTP authentication ("mail_useauth").
	// +optional
	UseAuth *bool `json:"useAuth,omitempty"`
}

// SettingsObservation are the observable fields of Settings.
type SettingsObservation struct {
	// Name of the CTF as reported by CTFd.
	Name string `json:"name,omitempty"`

	// Theme as reported by CTFd.
	Theme string `json:"theme,omitempty"`

	// UserMode as reported by CTFd.
	UserMode string `json:"userMode,omitempty"`
}

// A SettingsSpec defines the desired state of Settings.
type SettingsSpec struct {
	xpv2.ManagedResourceSpec `json:",inline"`
	ForProvider              SettingsParameters `json:"forProvider"`
}

// A SettingsStatus represents the observed state of Settings.
type SettingsStatus struct {
	xpv1.ResourceStatus `json:",inline"`
	AtProvider          SettingsObservation `json:"atProvider,omitempty"`
}

// +kubebuilder:object:root=true

// Settings configures the instance-wide CTFd settings (admin "Config" page).
// +kubebuilder:printcolumn:name="READY",type="string",JSONPath=".status.conditions[?(@.type=='Ready')].status"
// +kubebuilder:printcolumn:name="SYNCED",type="string",JSONPath=".status.conditions[?(@.type=='Synced')].status"
// +kubebuilder:printcolumn:name="CTF",type="string",JSONPath=".status.atProvider.name"
// +kubebuilder:printcolumn:name="THEME",type="string",JSONPath=".status.atProvider.theme"
// +kubebuilder:printcolumn:name="AGE",type="date",JSONPath=".metadata.creationTimestamp"
// +kubebuilder:subresource:status
// +kubebuilder:resource:scope=Namespaced,categories={crossplane,managed,ctfd}
type Settings struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   SettingsSpec   `json:"spec"`
	Status SettingsStatus `json:"status,omitempty"`
}

// +kubebuilder:object:root=true

// SettingsList contains a list of Settings
type SettingsList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []Settings `json:"items"`
}

// Settings type metadata.
var (
	SettingsKind             = reflect.TypeOf(Settings{}).Name()
	SettingsGroupKind        = schema.GroupKind{Group: Group, Kind: SettingsKind}.String()
	SettingsKindAPIVersion   = SettingsKind + "." + SchemeGroupVersion.String()
	SettingsGroupVersionKind = SchemeGroupVersion.WithKind(SettingsKind)
)

func init() {
	SchemeBuilder.Register(&Settings{}, &SettingsList{})
}

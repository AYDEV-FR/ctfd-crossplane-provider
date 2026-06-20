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

package theme

import (
	"testing"

	"github.com/AYDEV-FR/ctfd-crossplane-provider/apis/resources/v1alpha1"
)

func TestIsUpToDate(t *testing.T) {
	s := func(v string) *string { return &v }

	cases := map[string]struct {
		desired v1alpha1.ThemeParameters
		got     map[string]string
		want    bool
	}{
		"NameMatches": {
			desired: v1alpha1.ThemeParameters{Name: "core-beta"},
			got:     map[string]string{keyTheme: "core-beta"},
			want:    true,
		},
		"NameDrift": {
			desired: v1alpha1.ThemeParameters{Name: "core-beta"},
			got:     map[string]string{keyTheme: "core"},
			want:    false,
		},
		"OptionalUnsetIgnored": {
			desired: v1alpha1.ThemeParameters{Name: "core-beta"},
			got:     map[string]string{keyTheme: "core-beta", keyHeader: "<style></style>"},
			want:    true,
		},
		"HeaderMatches": {
			desired: v1alpha1.ThemeParameters{Name: "core-beta", Header: s("<b>hi</b>")},
			got:     map[string]string{keyTheme: "core-beta", keyHeader: "<b>hi</b>"},
			want:    true,
		},
		"HeaderDrift": {
			desired: v1alpha1.ThemeParameters{Name: "core-beta", Header: s("<b>hi</b>")},
			got:     map[string]string{keyTheme: "core-beta", keyHeader: "<b>bye</b>"},
			want:    false,
		},
		"SettingsMissingOnServer": {
			desired: v1alpha1.ThemeParameters{Name: "core-beta", Settings: s("{}")},
			got:     map[string]string{keyTheme: "core-beta"},
			want:    false,
		},
	}

	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			if got := isUpToDate(tc.desired, tc.got); got != tc.want {
				t.Fatalf("isUpToDate() = %v, want %v", got, tc.want)
			}
		})
	}
}

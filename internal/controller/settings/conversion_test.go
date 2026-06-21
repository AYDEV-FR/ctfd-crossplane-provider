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

package settings

import (
	"testing"

	"github.com/AYDEV-FR/ctfd-crossplane-provider/apis/resources/v1alpha1"
)

func sp(s string) *string { return &s }
func bp(b bool) *bool     { return &b }
func ip(i int) *int       { return &i }

func TestDesiredConfigOnlySetFields(t *testing.T) {
	p := v1alpha1.SettingsParameters{
		Name:     sp("Barbhack"),
		UserMode: sp("teams"),
		Paused:   bp(false),
		TeamSize: ip(4),
	}
	got := desiredConfig(p, "")

	if len(got) != 4 {
		t.Fatalf("expected 4 keys, got %d: %v", len(got), got)
	}
	if got["ctf_name"] != "Barbhack" || got["user_mode"] != "teams" {
		t.Fatalf("unexpected string values: %v", got)
	}
	if got["paused"] != false {
		t.Fatalf("paused should be bool false, got %T %v", got["paused"], got["paused"])
	}
	if got["team_size"] != 4 {
		t.Fatalf("team_size should be int 4, got %T %v", got["team_size"], got["team_size"])
	}
}

func TestMailPasswordOnlyWhenResolved(t *testing.T) {
	p := v1alpha1.SettingsParameters{Mail: &v1alpha1.MailSettings{Server: sp("smtp")}}
	if _, ok := desiredConfig(p, "")["mail_password"]; ok {
		t.Fatal("mail_password must not be set when password is empty")
	}
	if desiredConfig(p, "s3cret")["mail_password"] != "s3cret" {
		t.Fatal("mail_password should carry the resolved password")
	}
}

func TestIsUpToDate(t *testing.T) {
	desired := desiredConfig(v1alpha1.SettingsParameters{
		Name:     sp("Barbhack"),
		Paused:   bp(true),
		TeamSize: ip(4),
		Mail:     &v1alpha1.MailSettings{Server: sp("smtp")},
	}, "s3cret")

	base := map[string]string{
		"ctf_name":    "Barbhack",
		"paused":      "1", // CTFd stores booleans as "1"/"0"
		"team_size":   "4",
		"mail_server": "smtp",
		// mail_password intentionally absent (CTFd never returns it)
	}
	if !isUpToDate(desired, base) {
		t.Fatalf("expected up to date; desired=%v observed=%v", desired, base)
	}

	drift := map[string]string{"ctf_name": "Other", "paused": "1", "team_size": "4", "mail_server": "smtp"}
	if isUpToDate(desired, drift) {
		t.Fatal("expected drift on ctf_name")
	}

	boolDrift := map[string]string{"ctf_name": "Barbhack", "paused": "0", "team_size": "4", "mail_server": "smtp"}
	if isUpToDate(desired, boolDrift) {
		t.Fatal("expected drift on paused bool")
	}
}

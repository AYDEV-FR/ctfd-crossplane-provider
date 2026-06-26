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

package challenge

import (
	"testing"

	ctfd "github.com/ctfer-io/go-ctfd/api"

	"github.com/AYDEV-FR/ctfd-crossplane-provider/apis/resources/v1alpha1"
)

func TestFlagsUpToDate(t *testing.T) {
	desired := []v1alpha1.ChallengeFlag{
		{Content: "CTF{a}"}, // type defaults to "static"
		{Content: "^x$", Type: "regex", Data: "case_insensitive"},
	}

	cases := map[string]struct {
		existing []*ctfd.Flag
		want     bool
	}{
		"Match": {
			existing: []*ctfd.Flag{
				{Content: "^x$", Type: "regex", Data: "case_insensitive"},
				{Content: "CTF{a}", Type: "static"}, // order ignored
			},
			want: true,
		},
		"DefaultTypeMatches": {
			existing: []*ctfd.Flag{
				{Content: "CTF{a}", Type: "static"},
				{Content: "^x$", Type: "regex", Data: "case_insensitive"},
			},
			want: true,
		},
		"Missing": {
			existing: []*ctfd.Flag{{Content: "CTF{a}", Type: "static"}},
			want:     false,
		},
		"Extra": {
			existing: []*ctfd.Flag{
				{Content: "CTF{a}", Type: "static"},
				{Content: "^x$", Type: "regex", Data: "case_insensitive"},
				{Content: "CTF{extra}", Type: "static"},
			},
			want: false,
		},
		"ContentDrift": {
			existing: []*ctfd.Flag{
				{Content: "CTF{b}", Type: "static"},
				{Content: "^x$", Type: "regex", Data: "case_insensitive"},
			},
			want: false,
		},
	}

	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			if got := flagsUpToDate(desired, tc.existing); got != tc.want {
				t.Fatalf("flagsUpToDate() = %v, want %v", got, tc.want)
			}
		})
	}
}

func TestHintsUpToDate(t *testing.T) {
	title := "Stuck?"
	desired := []v1alpha1.ChallengeHint{
		{Content: "look here", Title: &title, Cost: 10},
		{Content: "and there", Cost: 25},
	}

	sp := func(s string) *string { return &s }

	cases := map[string]struct {
		existing []*ctfd.Hint
		want     bool
	}{
		"Match": {
			existing: []*ctfd.Hint{
				{Content: sp("and there"), Cost: 25},
				{Content: sp("look here"), Title: sp("Stuck?"), Cost: 10},
			},
			want: true,
		},
		"CostDrift": {
			existing: []*ctfd.Hint{
				{Content: sp("look here"), Title: sp("Stuck?"), Cost: 99},
				{Content: sp("and there"), Cost: 25},
			},
			want: false,
		},
		"NilContentTreatedEmpty": {
			existing: []*ctfd.Hint{
				{Content: nil, Title: sp("Stuck?"), Cost: 10},
				{Content: sp("and there"), Cost: 25},
			},
			want: false,
		},
	}

	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			if got := hintsUpToDate(desired, tc.existing); got != tc.want {
				t.Fatalf("hintsUpToDate() = %v, want %v", got, tc.want)
			}
		})
	}
}

func TestHintsUpToDatePrerequisites(t *testing.T) {
	sp := func(s string) *string { return &s }
	// Hint #1 ("and there") must be unlocked before hint #0 ("look here").
	desired := []v1alpha1.ChallengeHint{
		{Content: "look here", Cost: 10, Prerequisites: []int{1}},
		{Content: "and there", Cost: 25},
	}

	cases := map[string]struct {
		existing []*ctfd.Hint
		want     bool
	}{
		"PrereqWired": {
			existing: []*ctfd.Hint{
				{ID: 7, Content: sp("and there"), Cost: 25},
				{ID: 4, Content: sp("look here"), Cost: 10, Requirements: &ctfd.Requirements{Prerequisites: []int{7}}},
			},
			want: true,
		},
		"PrereqMissing": {
			existing: []*ctfd.Hint{
				{ID: 7, Content: sp("and there"), Cost: 25},
				{ID: 4, Content: sp("look here"), Cost: 10},
			},
			want: false,
		},
		"PrereqWrongTarget": {
			existing: []*ctfd.Hint{
				{ID: 7, Content: sp("and there"), Cost: 25},
				{ID: 4, Content: sp("look here"), Cost: 10, Requirements: &ctfd.Requirements{Prerequisites: []int{99}}},
			},
			want: false,
		},
	}

	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			if got := hintsUpToDate(desired, tc.existing); got != tc.want {
				t.Fatalf("hintsUpToDate() = %v, want %v", got, tc.want)
			}
		})
	}
}

func TestValidateHintPrereqs(t *testing.T) {
	cases := map[string]struct {
		desired []v1alpha1.ChallengeHint
		wantErr bool
	}{
		"Valid": {
			desired: []v1alpha1.ChallengeHint{
				{Content: "a"},
				{Content: "b", Prerequisites: []int{0}},
			},
		},
		"OutOfRange": {
			desired: []v1alpha1.ChallengeHint{
				{Content: "a", Prerequisites: []int{3}},
			},
			wantErr: true,
		},
		"SelfReference": {
			desired: []v1alpha1.ChallengeHint{
				{Content: "a", Prerequisites: []int{0}},
			},
			wantErr: true,
		},
		"Cycle": {
			desired: []v1alpha1.ChallengeHint{
				{Content: "a", Prerequisites: []int{1}},
				{Content: "b", Prerequisites: []int{0}},
			},
			wantErr: true,
		},
	}

	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			if err := validateHintPrereqs(tc.desired); (err != nil) != tc.wantErr {
				t.Fatalf("validateHintPrereqs() error = %v, wantErr %v", err, tc.wantErr)
			}
		})
	}
}

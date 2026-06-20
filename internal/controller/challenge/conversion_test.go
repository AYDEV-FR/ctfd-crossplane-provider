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

func ptr[T any](v T) *T { return &v }

func TestIsUpToDate(t *testing.T) {
	base := func() (v1alpha1.ChallengeParameters, *ctfd.Challenge) {
		p := v1alpha1.ChallengeParameters{
			Name:        "chall",
			Category:    "misc",
			Description: "desc",
			Type:        "standard",
			State:       "visible",
			Value:       ptr(100),
		}
		ch := &ctfd.Challenge{
			Name:        "chall",
			Category:    "misc",
			Description: "desc",
			Type:        "standard",
			State:       "visible",
			Value:       100,
		}
		return p, ch
	}

	cases := map[string]struct {
		mutate func(p *v1alpha1.ChallengeParameters, ch *ctfd.Challenge)
		want   bool
	}{
		"InSync":            {mutate: func(*v1alpha1.ChallengeParameters, *ctfd.Challenge) {}, want: true},
		"DefaultedType":     {mutate: func(p *v1alpha1.ChallengeParameters, _ *ctfd.Challenge) { p.Type = "" }, want: true},
		"DefaultedState":    {mutate: func(p *v1alpha1.ChallengeParameters, ch *ctfd.Challenge) { p.State = ""; ch.State = "hidden" }, want: true},
		"NameDrift":         {mutate: func(_ *v1alpha1.ChallengeParameters, ch *ctfd.Challenge) { ch.Name = "other" }, want: false},
		"ValueDrift":        {mutate: func(_ *v1alpha1.ChallengeParameters, ch *ctfd.Challenge) { ch.Value = 200 }, want: false},
		"UnsetValueIgnored": {mutate: func(p *v1alpha1.ChallengeParameters, ch *ctfd.Challenge) { p.Value = nil; ch.Value = 999 }, want: true},
		"ConnInfoDrift": {mutate: func(p *v1alpha1.ChallengeParameters, ch *ctfd.Challenge) {
			p.ConnectionInfo = ptr("ssh a")
			ch.ConnectionInfo = ptr("ssh b")
		}, want: false},
	}

	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			p, ch := base()
			tc.mutate(&p, ch)
			if got := isUpToDate(p, ch, nil); got != tc.want {
				t.Fatalf("isUpToDate() = %v, want %v", got, tc.want)
			}
		})
	}
}

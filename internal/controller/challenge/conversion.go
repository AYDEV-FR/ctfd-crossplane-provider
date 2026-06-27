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
	ctfd "github.com/ctfer-io/go-ctfd/api"

	"github.com/AYDEV-FR/ctfd-crossplane-provider/apis/resources/v1alpha1"
)

const defaultType = "standard"

func typeOrDefault(t string) string {
	if t == "" {
		return defaultType
	}
	return t
}

// postParams builds the CTFd creation payload from the desired parameters.
func postParams(p v1alpha1.ChallengeParameters) *ctfd.PostChallengesParams {
	out := &ctfd.PostChallengesParams{
		Name:           p.Name,
		Category:       p.Category,
		Description:    p.Description,
		Type:           typeOrDefault(p.Type),
		State:          p.State,
		Attribution:    p.Attribution,
		ConnectionInfo: p.ConnectionInfo,
		Function:       p.Function,
		MaxAttempts:    p.MaxAttempts,
		Initial:        p.Initial,
		Decay:          p.Decay,
		Minimum:        p.Minimum,
		NextID:         p.NextID,
	}
	if p.Value != nil {
		out.Value = *p.Value
	}
	if p.Requirements != nil {
		out.Requirements = toReq(p.Requirements)
	}
	return out
}

// patchParams builds the CTFd update payload from the desired parameters.
// current is the state observed in CTFd; it is preserved when the user leaves
// State unset, so an unmanaged state is never blanked by an update.
func patchParams(p v1alpha1.ChallengeParameters, current string) *ctfd.PatchChallengeParams {
	state := p.State
	if state == "" {
		state = current
	}
	out := &ctfd.PatchChallengeParams{
		Name:           p.Name,
		Category:       p.Category,
		Description:    p.Description,
		State:          state,
		Attribution:    p.Attribution,
		ConnectionInfo: p.ConnectionInfo,
		Function:       p.Function,
		MaxAttempts:    p.MaxAttempts,
		Value:          p.Value,
		Initial:        p.Initial,
		Decay:          p.Decay,
		Minimum:        p.Minimum,
		NextID:         p.NextID,
	}
	if p.Requirements != nil {
		out.Requirements = toReq(p.Requirements)
	}
	return out
}

func toReq(r *v1alpha1.Requirements) *ctfd.Requirements {
	return &ctfd.Requirements{
		Anonymize:     r.Anonymize,
		Prerequisites: r.Prerequisites,
	}
}

// ChallengeObservation maps a CTFd challenge and its flags and hints onto the
// managed resource status.
func ChallengeObservation(ch *ctfd.Challenge, flags []*ctfd.Flag, hints []*ctfd.Hint) v1alpha1.ChallengeObservation {
	return v1alpha1.ChallengeObservation{
		ID:     ch.ID,
		Value:  ch.Value,
		State:  ch.State,
		Type:   ch.Type,
		Solves: ch.Solves,
		Flags:  len(flags),
		Hints:  len(hints),
	}
}

// isUpToDate reports whether the observed challenge matches the desired
// parameters. Only fields the user explicitly set are enforced, so CTFd
// defaults for unset optional fields do not cause perpetual updates.
//
//nolint:gocyclo // a flat sequence of field comparisons; splitting hurts readability
func isUpToDate(p v1alpha1.ChallengeParameters, ch *ctfd.Challenge, req *ctfd.Requirements) bool {
	switch {
	case ch.Name != p.Name,
		ch.Category != p.Category,
		ch.Description != p.Description,
		p.State != "" && ch.State != p.State,
		ch.Type != typeOrDefault(p.Type):
		return false
	}

	if typeOrDefault(p.Type) == defaultType && p.Value != nil && ch.Value != *p.Value {
		return false
	}
	if !eqIntPtr(p.Initial, ch.Initial) ||
		!eqIntPtr(p.Decay, ch.Decay) ||
		!eqIntPtr(p.Minimum, ch.Minimum) ||
		!eqIntPtr(p.MaxAttempts, ch.MaxAttempts) ||
		!eqIntPtr(p.NextID, ch.NextID) {
		return false
	}
	if !eqStrPtr(p.Function, ch.Function) ||
		!eqStrPtr(p.ConnectionInfo, ch.ConnectionInfo) ||
		!eqStrPtr(p.Attribution, ch.Attribution) {
		return false
	}
	if p.Requirements != nil {
		var got []int
		if req != nil {
			got = req.Prerequisites
		}
		if !sameIntSet(p.Requirements.Prerequisites, got) {
			return false
		}
	}
	return true
}

// eqIntPtr returns true unless the desired pointer is set and differs from the
// observed value.
func eqIntPtr(want, got *int) bool {
	if want == nil {
		return true
	}
	return got != nil && *want == *got
}

func eqStrPtr(want, got *string) bool {
	if want == nil {
		return true
	}
	return got != nil && *want == *got
}

func sameIntSet(a, b []int) bool {
	if len(a) != len(b) {
		return false
	}
	seen := make(map[int]int, len(a))
	for _, v := range a {
		seen[v]++
	}
	for _, v := range b {
		seen[v]--
		if seen[v] < 0 {
			return false
		}
	}
	return true
}

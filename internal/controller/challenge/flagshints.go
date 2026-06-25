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
	"context"
	"strconv"

	"github.com/crossplane/crossplane-runtime/v2/pkg/errors"
	ctfd "github.com/ctfer-io/go-ctfd/api"

	"github.com/AYDEV-FR/ctfd-crossplane-provider/apis/resources/v1alpha1"
)

const defaultFlagType = "static"

func flagTypeOrDefault(t string) string {
	if t == "" {
		return defaultFlagType
	}
	return t
}

func derefString(s *string) string {
	if s == nil {
		return ""
	}
	return *s
}

// flagKey is the value-based identity of a flag. Two flags are considered the
// same iff their content, type and data match.
func flagKey(content, typ, data string) string {
	return content + "\x00" + flagTypeOrDefault(typ) + "\x00" + data
}

// hintKey is the value-based identity of a hint.
func hintKey(content string, title *string, cost int) string {
	return content + "\x00" + derefString(title) + "\x00" + strconv.Itoa(cost)
}

// multiset counts string keys.
func multiset(keys []string) map[string]int {
	m := make(map[string]int, len(keys))
	for _, k := range keys {
		m[k]++
	}
	return m
}

func sameMultiset(a, b map[string]int) bool {
	if len(a) != len(b) {
		return false
	}
	for k, n := range a {
		if b[k] != n {
			return false
		}
	}
	return true
}

// flagsUpToDate reports whether the existing flags exactly match the desired
// set, ignoring order.
func flagsUpToDate(desired []v1alpha1.ChallengeFlag, existing []*ctfd.Flag) bool {
	want := make([]string, 0, len(desired))
	for _, f := range desired {
		want = append(want, flagKey(f.Content, f.Type, f.Data))
	}
	got := make([]string, 0, len(existing))
	for _, f := range existing {
		got = append(got, flagKey(f.Content, f.Type, f.Data))
	}
	return sameMultiset(multiset(want), multiset(got))
}

// hintsUpToDate reports whether the existing hints exactly match the desired
// set, including each hint's prerequisite chain. Hint order in the list is not
// significant on its own; what matters is the value of every hint and which
// other hints it requires.
func hintsUpToDate(desired []v1alpha1.ChallengeHint, existing []*ctfd.Hint) bool {
	if err := validateHintPrereqs(desired); err != nil {
		// An invalid spec can never be up to date; reporting drift lets Update
		// run and surface the validation error to the user.
		return false
	}

	idByIndex, deletes := planHints(desired, existing)
	if len(deletes) > 0 {
		return false
	}

	byID := make(map[int]*ctfd.Hint, len(existing))
	for _, h := range existing {
		byID[h.ID] = h
	}

	for i, d := range desired {
		id := idByIndex[i]
		if id < 0 {
			return false // a value-equal hint is missing and must be created
		}
		h := byID[id]
		if !sameIntSet(mapPrereqs(d.Prerequisites, idByIndex), prereqsOf(h)) {
			return false
		}
		if len(d.Prerequisites) > 0 && !eqAnonymize(d.Anonymize, h) {
			return false
		}
	}
	return true
}

// planHints matches desired hints to existing CTFd hints by value (content,
// title, cost), preserving identity so prerequisites can be resolved to real
// hint IDs. It returns, for each desired hint, the existing hint ID it maps to
// (or -1 when the hint must be created), and the existing hint IDs that are no
// longer desired and must be deleted.
func planHints(desired []v1alpha1.ChallengeHint, existing []*ctfd.Hint) (idByIndex []int, deletes []int) {
	pool := map[string][]int{}
	for _, h := range existing {
		k := hintKey(derefString(h.Content), h.Title, h.Cost)
		pool[k] = append(pool[k], h.ID)
	}

	idByIndex = make([]int, len(desired))
	for i, d := range desired {
		k := hintKey(d.Content, d.Title, d.Cost)
		if ids := pool[k]; len(ids) > 0 {
			idByIndex[i] = ids[0]
			pool[k] = ids[1:]
			continue
		}
		idByIndex[i] = -1
	}

	for _, ids := range pool {
		deletes = append(deletes, ids...)
	}
	return idByIndex, deletes
}

// mapPrereqs resolves desired prerequisite indexes to the CTFd hint IDs they
// point at. validateHintPrereqs must have accepted the spec, so every index is
// in range.
func mapPrereqs(idx, idByIndex []int) []int {
	if len(idx) == 0 {
		return nil
	}
	out := make([]int, 0, len(idx))
	for _, i := range idx {
		out = append(out, idByIndex[i])
	}
	return out
}

// prereqsOf returns a hint's prerequisite IDs, or nil when it has none.
func prereqsOf(h *ctfd.Hint) []int {
	if h == nil || h.Requirements == nil {
		return nil
	}
	return h.Requirements.Prerequisites
}

// eqAnonymize reports whether the desired anonymize behavior matches the hint.
// An unset desired value is not enforced.
func eqAnonymize(want *string, h *ctfd.Hint) bool {
	if want == nil {
		return true
	}
	var got *string
	if h != nil && h.Requirements != nil {
		got = h.Requirements.Anonymize
	}
	return eqStrPtr(want, got)
}

// validateHintPrereqs rejects prerequisite indexes that point outside the list,
// reference the hint itself, or form a cycle (which would make the hints
// impossible to unlock).
func validateHintPrereqs(desired []v1alpha1.ChallengeHint) error {
	n := len(desired)
	for i, h := range desired {
		for _, p := range h.Prerequisites {
			if p < 0 || p >= n {
				return errors.Errorf("hint %d: prerequisite index %d is out of range [0,%d)", i, p, n)
			}
			if p == i {
				return errors.Errorf("hint %d cannot be its own prerequisite", i)
			}
		}
	}
	return detectHintCycle(desired)
}

// detectHintCycle reports an error if the prerequisite graph contains a cycle.
func detectHintCycle(desired []v1alpha1.ChallengeHint) error {
	const (
		white = iota // unvisited
		gray         // on the current DFS stack
		black        // fully explored
	)
	color := make([]int, len(desired))

	var visit func(i int) error
	visit = func(i int) error {
		color[i] = gray
		for _, p := range desired[i].Prerequisites {
			switch color[p] {
			case gray:
				return errors.Errorf("hint prerequisites form a cycle involving hint %d", i)
			case white:
				if err := visit(p); err != nil {
					return err
				}
			}
		}
		color[i] = black
		return nil
	}

	for i := range desired {
		if color[i] == white {
			if err := visit(i); err != nil {
				return err
			}
		}
	}
	return nil
}

// challengeHints lists a challenge's hints, resolving each one's full
// representation (the challenge hint listing omits the content and
// prerequisites) so the values can be compared against the desired state.
func (e *external) challengeHints(ctx context.Context, cid int) ([]*ctfd.Hint, error) {
	hints, _, err := e.client.GetChallengeHints(cid, ctfd.WithContext(ctx))
	if err != nil {
		return nil, err
	}
	preview := true
	for i, h := range hints {
		full, _, err := e.client.GetHint(strconv.Itoa(h.ID), &ctfd.GetHintParams{Preview: &preview}, ctfd.WithContext(ctx))
		if err != nil {
			return nil, err
		}
		hints[i] = full
	}
	return hints, nil
}

// syncFlags reconciles the challenge's flags towards the desired set. Flags are
// matched by value; unmatched existing flags are deleted and missing desired
// flags are created.
func (e *external) syncFlags(ctx context.Context, cid int, desired []v1alpha1.ChallengeFlag) error {
	existing, _, err := e.client.GetChallengeFlags(cid, ctfd.WithContext(ctx))
	if err != nil {
		return err
	}

	pool := map[string][]int{}
	for _, f := range existing {
		k := flagKey(f.Content, f.Type, f.Data)
		pool[k] = append(pool[k], f.ID)
	}

	for _, d := range desired {
		k := flagKey(d.Content, d.Type, d.Data)
		if ids := pool[k]; len(ids) > 0 {
			pool[k] = ids[1:] // an equivalent flag already exists; keep it
			continue
		}
		if _, _, err := e.client.PostFlags(&ctfd.PostFlagsParams{
			Challenge: cid,
			Content:   d.Content,
			Data:      d.Data,
			Type:      flagTypeOrDefault(d.Type),
		}, ctfd.WithContext(ctx)); err != nil {
			return err
		}
	}

	for _, ids := range pool {
		for _, id := range ids {
			if _, err := e.client.DeleteFlag(strconv.Itoa(id), ctfd.WithContext(ctx)); err != nil {
				return err
			}
		}
	}
	return nil
}

// syncHints reconciles the challenge's hints towards the desired set, matching
// by value while preserving each hint's identity so prerequisites can be
// chained. Hints are created (or reused), then their prerequisites are wired up
// in a second pass — once every hint, including freshly created ones, has a
// CTFd ID — and finally unwanted hints are deleted.
func (e *external) syncHints(ctx context.Context, cid int, desired []v1alpha1.ChallengeHint) error {
	if err := validateHintPrereqs(desired); err != nil {
		return err
	}

	existing, err := e.challengeHints(ctx, cid)
	if err != nil {
		return err
	}

	idByIndex, deletes := planHints(desired, existing)

	if err := e.createMissingHints(ctx, cid, desired, idByIndex); err != nil {
		return err
	}
	if err := e.reconcileHintRequirements(ctx, cid, desired, existing, idByIndex); err != nil {
		return err
	}

	for _, id := range deletes {
		if _, err := e.client.DeleteHint(strconv.Itoa(id), ctfd.WithContext(ctx)); err != nil {
			return err
		}
	}
	return nil
}

// createMissingHints creates every desired hint that has no value-equal
// counterpart yet, recording its new ID in idByIndex. Prerequisites are left
// for reconcileHintRequirements, since they may point at hints created here.
func (e *external) createMissingHints(ctx context.Context, cid int, desired []v1alpha1.ChallengeHint, idByIndex []int) error {
	for i, d := range desired {
		if idByIndex[i] >= 0 {
			continue
		}
		h, _, err := e.client.PostHints(&ctfd.PostHintsParams{
			ChallengeID: cid,
			Title:       d.Title,
			Content:     d.Content,
			Cost:        d.Cost,
		}, ctfd.WithContext(ctx))
		if err != nil {
			return err
		}
		idByIndex[i] = h.ID
	}
	return nil
}

// reconcileHintRequirements wires up each hint's prerequisites (resolved from
// desired indexes to CTFd hint IDs) and anonymize behavior, patching only the
// hints whose current requirements differ.
func (e *external) reconcileHintRequirements(ctx context.Context, cid int, desired []v1alpha1.ChallengeHint, existing []*ctfd.Hint, idByIndex []int) error {
	byID := make(map[int]*ctfd.Hint, len(existing))
	for _, h := range existing {
		byID[h.ID] = h
	}

	for i, d := range desired {
		id := idByIndex[i]
		want := mapPrereqs(d.Prerequisites, idByIndex)
		cur := byID[id] // nil for a hint just created in the first pass
		if sameIntSet(want, prereqsOf(cur)) &&
			(len(d.Prerequisites) == 0 || eqAnonymize(d.Anonymize, cur)) {
			continue
		}
		if _, _, err := e.client.PatchHint(strconv.Itoa(id), &ctfd.PatchHintsParams{
			ChallengeID: cid,
			Title:       d.Title,
			Content:     d.Content,
			Cost:        d.Cost,
			Requirements: ctfd.Requirements{
				Anonymize:     d.Anonymize,
				Prerequisites: want,
			},
		}, ctfd.WithContext(ctx)); err != nil {
			return err
		}
	}
	return nil
}

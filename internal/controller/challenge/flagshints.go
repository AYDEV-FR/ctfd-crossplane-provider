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
// set, ignoring order.
func hintsUpToDate(desired []v1alpha1.ChallengeHint, existing []*ctfd.Hint) bool {
	want := make([]string, 0, len(desired))
	for _, h := range desired {
		want = append(want, hintKey(h.Content, h.Title, h.Cost))
	}
	got := make([]string, 0, len(existing))
	for _, h := range existing {
		got = append(got, hintKey(derefString(h.Content), h.Title, h.Cost))
	}
	return sameMultiset(multiset(want), multiset(got))
}

// challengeHints lists a challenge's hints, resolving each hint's content
// (which CTFd omits from the challenge hint listing) so the values can be
// compared against the desired state.
func (e *external) challengeHints(ctx context.Context, cid int) ([]*ctfd.Hint, error) {
	hints, _, err := e.client.GetChallengeHints(cid, ctfd.WithContext(ctx))
	if err != nil {
		return nil, err
	}
	preview := true
	for i, h := range hints {
		if h.Content != nil {
			continue
		}
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
// by value.
func (e *external) syncHints(ctx context.Context, cid int, desired []v1alpha1.ChallengeHint) error {
	existing, err := e.challengeHints(ctx, cid)
	if err != nil {
		return err
	}

	pool := map[string][]int{}
	for _, h := range existing {
		k := hintKey(derefString(h.Content), h.Title, h.Cost)
		pool[k] = append(pool[k], h.ID)
	}

	for _, d := range desired {
		k := hintKey(d.Content, d.Title, d.Cost)
		if ids := pool[k]; len(ids) > 0 {
			pool[k] = ids[1:]
			continue
		}
		if _, _, err := e.client.PostHints(&ctfd.PostHintsParams{
			ChallengeID: cid,
			Title:       d.Title,
			Content:     d.Content,
			Cost:        d.Cost,
		}, ctfd.WithContext(ctx)); err != nil {
			return err
		}
	}

	for _, ids := range pool {
		for _, id := range ids {
			if _, err := e.client.DeleteHint(strconv.Itoa(id), ctfd.WithContext(ctx)); err != nil {
				return err
			}
		}
	}
	return nil
}

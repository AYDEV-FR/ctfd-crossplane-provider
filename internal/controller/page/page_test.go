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

package page

import (
	"testing"

	ctfd "github.com/ctfer-io/go-ctfd/api"

	"github.com/AYDEV-FR/ctfd-crossplane-provider/apis/resources/v1alpha1"
)

func TestIsUpToDate(t *testing.T) {
	content := "hello"
	base := func() (v1alpha1.PageParameters, *ctfd.Page) {
		d := v1alpha1.PageParameters{
			Title:   "Rules",
			Route:   "rules",
			Content: content,
			Format:  "markdown",
		}
		p := &ctfd.Page{
			Title:   "Rules",
			Route:   "rules",
			Content: &content,
			Format:  "markdown",
		}
		return d, p
	}

	cases := map[string]struct {
		mutate func(d *v1alpha1.PageParameters, p *ctfd.Page)
		want   bool
	}{
		"InSync":        {mutate: func(*v1alpha1.PageParameters, *ctfd.Page) {}, want: true},
		"DefaultedFmt":  {mutate: func(d *v1alpha1.PageParameters, _ *ctfd.Page) { d.Format = "" }, want: true},
		"TitleDrift":    {mutate: func(_ *v1alpha1.PageParameters, p *ctfd.Page) { p.Title = "Other" }, want: false},
		"ContentDrift":  {mutate: func(_ *v1alpha1.PageParameters, p *ctfd.Page) { other := "bye"; p.Content = &other }, want: false},
		"HiddenDrift":   {mutate: func(_ *v1alpha1.PageParameters, p *ctfd.Page) { p.Hidden = true }, want: false},
		"NilServerBody": {mutate: func(_ *v1alpha1.PageParameters, p *ctfd.Page) { p.Content = nil }, want: false},
	}

	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			d, p := base()
			tc.mutate(&d, p)
			if got := isUpToDate(d, p); got != tc.want {
				t.Fatalf("isUpToDate() = %v, want %v", got, tc.want)
			}
		})
	}
}

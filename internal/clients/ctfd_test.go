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

package clients

import "testing"

func TestNewClient(t *testing.T) {
	cases := map[string]struct {
		data    string
		wantErr bool
	}{
		"APIKey":          {data: `{"url":"https://ctf.example.com","api_key":"ctfd_abc"}`, wantErr: false},
		"SessionAndNonce": {data: `{"url":"https://ctf.example.com","session":"s","nonce":"n"}`, wantErr: false},
		"MissingURL":      {data: `{"api_key":"ctfd_abc"}`, wantErr: true},
		"MissingAuth":     {data: `{"url":"https://ctf.example.com"}`, wantErr: true},
		"OnlySession":     {data: `{"url":"https://ctf.example.com","session":"s"}`, wantErr: true},
		"InvalidJSON":     {data: `not-json`, wantErr: true},
	}

	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			cli, err := NewClient([]byte(tc.data))
			if tc.wantErr {
				if err == nil {
					t.Fatalf("NewClient(%q): expected error, got nil", tc.data)
				}
				return
			}
			if err != nil {
				t.Fatalf("NewClient(%q): unexpected error: %v", tc.data, err)
			}
			if cli == nil {
				t.Fatalf("NewClient(%q): expected a client, got nil", tc.data)
			}
		})
	}
}

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

// Package clients contains helpers to build a CTFd API client from a
// ProviderConfig's credentials.
package clients

import (
	"context"
	"encoding/json"

	xpv1 "github.com/crossplane/crossplane-runtime/v2/apis/common/v1"
	"github.com/crossplane/crossplane-runtime/v2/pkg/errors"
	"github.com/crossplane/crossplane-runtime/v2/pkg/resource"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"

	ctfd "github.com/ctfer-io/go-ctfd/api"

	apisv1alpha1 "github.com/AYDEV-FR/ctfd-crossplane-provider/apis/v1alpha1"
)

// Error strings.
const (
	errGetPC        = "cannot get ProviderConfig"
	errGetCPC       = "cannot get ClusterProviderConfig"
	errGetCreds     = "cannot get credentials"
	errUnmarshal    = "cannot unmarshal CTFd credentials as JSON"
	errMissingURL   = "CTFd credentials must contain a non-empty \"url\""
	errMissingAuth  = "CTFd credentials must contain an \"api_key\", or a \"session\" and \"nonce\" pair"
	errUnsupportedK = "unsupported provider config kind: %s"
)

// Credentials is the JSON structure expected in the ProviderConfig credentials
// source. An api_key is the recommended authentication method; a session and
// nonce pair is supported as a fallback.
type Credentials struct {
	// URL of the CTFd instance, e.g. "https://ctf.example.com".
	URL string `json:"url"`
	// APIKey is a CTFd admin API token ("ctfd_..."). Recommended.
	APIKey string `json:"api_key,omitempty"`
	// Session cookie value, used when no APIKey is provided.
	Session string `json:"session,omitempty"`
	// Nonce (CSRF token), used together with Session.
	Nonce string `json:"nonce,omitempty"`
}

// NewClient builds a CTFd API client from raw credential bytes.
func NewClient(data []byte) (*ctfd.Client, error) {
	creds := Credentials{}
	if err := json.Unmarshal(data, &creds); err != nil {
		return nil, errors.Wrap(err, errUnmarshal)
	}
	if creds.URL == "" {
		return nil, errors.New(errMissingURL)
	}
	if creds.APIKey == "" && (creds.Session == "" || creds.Nonce == "") {
		return nil, errors.New(errMissingAuth)
	}
	return ctfd.NewClient(creds.URL, creds.Nonce, creds.Session, creds.APIKey), nil
}

// ProviderConfigReferencer is implemented by every CTFd managed resource: it
// exposes the referenced ProviderConfig and the resource's namespace.
type ProviderConfigReferencer interface {
	GetProviderConfigReference() *xpv1.ProviderConfigReference
	GetNamespace() string
}

// FromProviderConfig resolves the ProviderConfig (namespaced) or
// ClusterProviderConfig referenced by a managed resource, extracts its
// credentials and returns a ready-to-use CTFd client.
func FromProviderConfig(ctx context.Context, kube client.Client, mg ProviderConfigReferencer) (*ctfd.Client, error) {
	var cd apisv1alpha1.ProviderCredentials

	ref := mg.GetProviderConfigReference()

	switch ref.Kind {
	case "ProviderConfig":
		pc := &apisv1alpha1.ProviderConfig{}
		if err := kube.Get(ctx, types.NamespacedName{Name: ref.Name, Namespace: mg.GetNamespace()}, pc); err != nil {
			return nil, errors.Wrap(err, errGetPC)
		}
		cd = pc.Spec.Credentials
	case "ClusterProviderConfig":
		cpc := &apisv1alpha1.ClusterProviderConfig{}
		if err := kube.Get(ctx, types.NamespacedName{Name: ref.Name}, cpc); err != nil {
			return nil, errors.Wrap(err, errGetCPC)
		}
		cd = cpc.Spec.Credentials
	default:
		return nil, errors.Errorf(errUnsupportedK, ref.Kind)
	}

	data, err := resource.CommonCredentialExtractor(ctx, cd.Source, kube, cd.CommonCredentialSelectors)
	if err != nil {
		return nil, errors.Wrap(err, errGetCreds)
	}

	return NewClient(data)
}

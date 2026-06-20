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
	"context"

	xpv1 "github.com/crossplane/crossplane-runtime/v2/apis/common/v1"
	"github.com/crossplane/crossplane-runtime/v2/pkg/controller"
	"github.com/crossplane/crossplane-runtime/v2/pkg/errors"
	"github.com/crossplane/crossplane-runtime/v2/pkg/event"
	"github.com/crossplane/crossplane-runtime/v2/pkg/feature"
	"github.com/crossplane/crossplane-runtime/v2/pkg/meta"
	"github.com/crossplane/crossplane-runtime/v2/pkg/ratelimiter"
	"github.com/crossplane/crossplane-runtime/v2/pkg/reconciler/managed"
	"github.com/crossplane/crossplane-runtime/v2/pkg/resource"
	"github.com/crossplane/crossplane-runtime/v2/pkg/statemetrics"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"

	ctfd "github.com/ctfer-io/go-ctfd/api"

	"github.com/AYDEV-FR/ctfd-crossplane-provider/apis/resources/v1alpha1"
	apisv1alpha1 "github.com/AYDEV-FR/ctfd-crossplane-provider/apis/v1alpha1"
	"github.com/AYDEV-FR/ctfd-crossplane-provider/internal/clients"
)

const (
	errTrackPCUsage = "cannot track ProviderConfig usage"
	errNewClient    = "cannot create new CTFd client"
	errGet          = "cannot read CTFd configuration"
	errApply        = "cannot apply theme configuration to CTFd"

	// externalName is the fixed external name of the singleton Theme resource.
	externalName = "theme"

	keyTheme    = "ctf_theme"
	keyHeader   = "theme_header"
	keyFooter   = "theme_footer"
	keySettings = "theme_settings"
)

// SetupGated adds a controller that reconciles Theme managed resources with
// safe-start support.
func SetupGated(mgr ctrl.Manager, o controller.Options) error {
	o.Gate.Register(func() {
		if err := Setup(mgr, o); err != nil {
			panic(errors.Wrap(err, "cannot setup Theme controller"))
		}
	}, v1alpha1.ThemeGroupVersionKind)
	return nil
}

// Setup adds a controller that reconciles Theme managed resources.
func Setup(mgr ctrl.Manager, o controller.Options) error {
	name := managed.ControllerName(v1alpha1.ThemeGroupKind)

	opts := []managed.ReconcilerOption{
		managed.WithTypedExternalConnector[*v1alpha1.Theme](&connector{
			kube:  mgr.GetClient(),
			usage: resource.NewProviderConfigUsageTracker(mgr.GetClient(), &apisv1alpha1.ProviderConfigUsage{}),
		}),
		managed.WithLogger(o.Logger.WithValues("controller", name)),
		managed.WithPollInterval(o.PollInterval),
		managed.WithRecorder(event.NewAPIRecorder(mgr.GetEventRecorderFor(name))),
	}

	if o.Features.Enabled(feature.EnableBetaManagementPolicies) {
		opts = append(opts, managed.WithManagementPolicies())
	}

	if o.Features.Enabled(feature.EnableAlphaChangeLogs) {
		opts = append(opts, managed.WithChangeLogger(o.ChangeLogOptions.ChangeLogger))
	}

	if o.MetricOptions != nil {
		opts = append(opts, managed.WithMetricRecorder(o.MetricOptions.MRMetrics))
	}

	if o.MetricOptions != nil && o.MetricOptions.MRStateMetrics != nil {
		stateMetricsRecorder := statemetrics.NewMRStateRecorder(
			mgr.GetClient(), o.Logger, o.MetricOptions.MRStateMetrics, &v1alpha1.ThemeList{}, o.MetricOptions.PollStateMetricInterval,
		)
		if err := mgr.Add(stateMetricsRecorder); err != nil {
			return errors.Wrap(err, "cannot register MR state metrics recorder for kind v1alpha1.ThemeList")
		}
	}

	r := managed.NewReconciler(mgr, resource.ManagedKind(v1alpha1.ThemeGroupVersionKind), opts...)

	return ctrl.NewControllerManagedBy(mgr).
		Named(name).
		WithOptions(o.ForControllerRuntime()).
		WithEventFilter(resource.DesiredStateChanged()).
		For(&v1alpha1.Theme{}).
		Complete(ratelimiter.NewReconciler(name, r, o.GlobalRateLimiter))
}

type connector struct {
	kube  client.Client
	usage *resource.ProviderConfigUsageTracker
}

func (c *connector) Connect(ctx context.Context, cr *v1alpha1.Theme) (managed.TypedExternalClient[*v1alpha1.Theme], error) {
	if err := c.usage.Track(ctx, cr); err != nil {
		return nil, errors.Wrap(err, errTrackPCUsage)
	}

	cli, err := clients.FromProviderConfig(ctx, c.kube, cr)
	if err != nil {
		return nil, errors.Wrap(err, errNewClient)
	}

	return &external{client: cli}, nil
}

type external struct {
	client *ctfd.Client
}

// currentConfig reads the CTFd configuration and returns the theme-related
// values keyed by their config key.
func (e *external) currentConfig(ctx context.Context) (map[string]string, error) {
	configs, _, err := e.client.GetConfigs(nil, ctfd.WithContext(ctx))
	if err != nil {
		return nil, err
	}
	values := make(map[string]string, len(configs))
	for _, c := range configs {
		values[c.Key] = c.Value
	}
	return values, nil
}

func (e *external) Observe(ctx context.Context, cr *v1alpha1.Theme) (managed.ExternalObservation, error) {
	if meta.GetExternalName(cr) == "" {
		return managed.ExternalObservation{ResourceExists: false}, nil
	}

	values, err := e.currentConfig(ctx)
	if err != nil {
		return managed.ExternalObservation{}, errors.Wrap(err, errGet)
	}

	cr.Status.AtProvider = v1alpha1.ThemeObservation{Name: values[keyTheme]}
	cr.Status.SetConditions(xpv1.Available())

	return managed.ExternalObservation{
		ResourceExists:   true,
		ResourceUpToDate: isUpToDate(cr.Spec.ForProvider, values),
	}, nil
}

// isUpToDate reports whether the observed configuration matches the desired
// theme. Optional fields are only enforced when set.
func isUpToDate(d v1alpha1.ThemeParameters, got map[string]string) bool {
	if got[keyTheme] != d.Name {
		return false
	}
	if d.Header != nil && got[keyHeader] != *d.Header {
		return false
	}
	if d.Footer != nil && got[keyFooter] != *d.Footer {
		return false
	}
	if d.Settings != nil && got[keySettings] != *d.Settings {
		return false
	}
	return true
}

// apply PATCHes only the theme-related configuration keys, leaving every other
// CTFd setting untouched.
func (e *external) apply(ctx context.Context, d v1alpha1.ThemeParameters) error {
	body := map[string]any{keyTheme: d.Name}
	if d.Header != nil {
		body[keyHeader] = *d.Header
	}
	if d.Footer != nil {
		body[keyFooter] = *d.Footer
	}
	if d.Settings != nil {
		body[keySettings] = *d.Settings
	}

	_, err := e.client.Patch("/configs", body, nil, ctfd.WithContext(ctx))
	return errors.Wrap(err, errApply)
}

func (e *external) Create(ctx context.Context, cr *v1alpha1.Theme) (managed.ExternalCreation, error) {
	cr.Status.SetConditions(xpv1.Creating())

	if err := e.apply(ctx, cr.Spec.ForProvider); err != nil {
		return managed.ExternalCreation{}, err
	}

	meta.SetExternalName(cr, externalName)
	return managed.ExternalCreation{}, nil
}

func (e *external) Update(ctx context.Context, cr *v1alpha1.Theme) (managed.ExternalUpdate, error) {
	return managed.ExternalUpdate{}, e.apply(ctx, cr.Spec.ForProvider)
}

// Delete is a no-op: a CTFd instance always has an active theme, so deleting
// the Theme resource only stops Crossplane from managing it; it does not revert
// CTFd to a previous theme.
func (e *external) Delete(_ context.Context, cr *v1alpha1.Theme) (managed.ExternalDelete, error) {
	cr.Status.SetConditions(xpv1.Deleting())
	return managed.ExternalDelete{}, nil
}

func (e *external) Disconnect(_ context.Context) error {
	return nil
}

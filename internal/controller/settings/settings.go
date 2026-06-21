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
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/types"
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
	errApply        = "cannot apply configuration to CTFd"
	errMailSecret   = "cannot read mail password secret"

	// externalName is the fixed external name of the singleton Settings resource.
	externalName = "settings"
)

// SetupGated adds a controller that reconciles Settings managed resources with
// safe-start support.
func SetupGated(mgr ctrl.Manager, o controller.Options) error {
	o.Gate.Register(func() {
		if err := Setup(mgr, o); err != nil {
			panic(errors.Wrap(err, "cannot setup Settings controller"))
		}
	}, v1alpha1.SettingsGroupVersionKind)
	return nil
}

// Setup adds a controller that reconciles Settings managed resources.
func Setup(mgr ctrl.Manager, o controller.Options) error {
	name := managed.ControllerName(v1alpha1.SettingsGroupKind)

	opts := []managed.ReconcilerOption{
		managed.WithTypedExternalConnector[*v1alpha1.Settings](&connector{
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
			mgr.GetClient(), o.Logger, o.MetricOptions.MRStateMetrics, &v1alpha1.SettingsList{}, o.MetricOptions.PollStateMetricInterval,
		)
		if err := mgr.Add(stateMetricsRecorder); err != nil {
			return errors.Wrap(err, "cannot register MR state metrics recorder for kind v1alpha1.SettingsList")
		}
	}

	r := managed.NewReconciler(mgr, resource.ManagedKind(v1alpha1.SettingsGroupVersionKind), opts...)

	return ctrl.NewControllerManagedBy(mgr).
		Named(name).
		WithOptions(o.ForControllerRuntime()).
		WithEventFilter(resource.DesiredStateChanged()).
		For(&v1alpha1.Settings{}).
		Complete(ratelimiter.NewReconciler(name, r, o.GlobalRateLimiter))
}

type connector struct {
	kube  client.Client
	usage *resource.ProviderConfigUsageTracker
}

func (c *connector) Connect(ctx context.Context, cr *v1alpha1.Settings) (managed.TypedExternalClient[*v1alpha1.Settings], error) {
	if err := c.usage.Track(ctx, cr); err != nil {
		return nil, errors.Wrap(err, errTrackPCUsage)
	}

	cli, err := clients.FromProviderConfig(ctx, c.kube, cr)
	if err != nil {
		return nil, errors.Wrap(err, errNewClient)
	}

	return &external{kube: c.kube, client: cli}, nil
}

type external struct {
	kube   client.Client
	client *ctfd.Client
}

// currentConfig reads the CTFd configuration into a key/value map.
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

// mailPassword resolves the SMTP password from its Secret, if configured.
func (e *external) mailPassword(ctx context.Context, cr *v1alpha1.Settings) (string, error) {
	mail := cr.Spec.ForProvider.Mail
	if mail == nil || mail.PasswordSecretRef == nil {
		return "", nil
	}
	ref := mail.PasswordSecretRef
	s := &corev1.Secret{}
	if err := e.kube.Get(ctx, types.NamespacedName{Namespace: ref.Namespace, Name: ref.Name}, s); err != nil {
		return "", errors.Wrap(err, errMailSecret)
	}
	return string(s.Data[ref.Key]), nil
}

func (e *external) Observe(ctx context.Context, cr *v1alpha1.Settings) (managed.ExternalObservation, error) {
	if meta.GetExternalName(cr) == "" {
		return managed.ExternalObservation{ResourceExists: false}, nil
	}

	observed, err := e.currentConfig(ctx)
	if err != nil {
		return managed.ExternalObservation{}, errors.Wrap(err, errGet)
	}

	cr.Status.AtProvider = v1alpha1.SettingsObservation{
		Name:     observed["ctf_name"],
		Theme:    observed["ctf_theme"],
		UserMode: observed["user_mode"],
	}
	cr.Status.SetConditions(xpv1.Available())

	pw, err := e.mailPassword(ctx, cr)
	if err != nil {
		return managed.ExternalObservation{}, err
	}

	return managed.ExternalObservation{
		ResourceExists:   true,
		ResourceUpToDate: isUpToDate(desiredConfig(cr.Spec.ForProvider, pw), observed),
	}, nil
}

func (e *external) apply(ctx context.Context, cr *v1alpha1.Settings) error {
	pw, err := e.mailPassword(ctx, cr)
	if err != nil {
		return err
	}
	body := desiredConfig(cr.Spec.ForProvider, pw)
	if len(body) == 0 {
		return nil
	}
	if _, err := e.client.Patch("/configs", body, nil, ctfd.WithContext(ctx)); err != nil {
		return errors.Wrap(err, errApply)
	}
	return nil
}

func (e *external) Create(ctx context.Context, cr *v1alpha1.Settings) (managed.ExternalCreation, error) {
	cr.Status.SetConditions(xpv1.Creating())
	if err := e.apply(ctx, cr); err != nil {
		return managed.ExternalCreation{}, err
	}
	meta.SetExternalName(cr, externalName)
	return managed.ExternalCreation{}, nil
}

func (e *external) Update(ctx context.Context, cr *v1alpha1.Settings) (managed.ExternalUpdate, error) {
	return managed.ExternalUpdate{}, e.apply(ctx, cr)
}

// Delete is a no-op: a CTFd instance always has a configuration. Deleting the
// Settings resource only stops Crossplane from managing it.
func (e *external) Delete(_ context.Context, cr *v1alpha1.Settings) (managed.ExternalDelete, error) {
	cr.Status.SetConditions(xpv1.Deleting())
	return managed.ExternalDelete{}, nil
}

func (e *external) Disconnect(_ context.Context) error {
	return nil
}

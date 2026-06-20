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
	"context"
	"strconv"

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
	errCreate       = "cannot create page in CTFd"
	errUpdate       = "cannot update page in CTFd"
	errDelete       = "cannot delete page in CTFd"
	errBadID        = "cannot parse external name as a page ID"

	defaultFormat = "markdown"
)

// SetupGated adds a controller that reconciles Page managed resources with
// safe-start support.
func SetupGated(mgr ctrl.Manager, o controller.Options) error {
	o.Gate.Register(func() {
		if err := Setup(mgr, o); err != nil {
			panic(errors.Wrap(err, "cannot setup Page controller"))
		}
	}, v1alpha1.PageGroupVersionKind)
	return nil
}

// Setup adds a controller that reconciles Page managed resources.
func Setup(mgr ctrl.Manager, o controller.Options) error {
	name := managed.ControllerName(v1alpha1.PageGroupKind)

	opts := []managed.ReconcilerOption{
		managed.WithTypedExternalConnector[*v1alpha1.Page](&connector{
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
			mgr.GetClient(), o.Logger, o.MetricOptions.MRStateMetrics, &v1alpha1.PageList{}, o.MetricOptions.PollStateMetricInterval,
		)
		if err := mgr.Add(stateMetricsRecorder); err != nil {
			return errors.Wrap(err, "cannot register MR state metrics recorder for kind v1alpha1.PageList")
		}
	}

	r := managed.NewReconciler(mgr, resource.ManagedKind(v1alpha1.PageGroupVersionKind), opts...)

	return ctrl.NewControllerManagedBy(mgr).
		Named(name).
		WithOptions(o.ForControllerRuntime()).
		WithEventFilter(resource.DesiredStateChanged()).
		For(&v1alpha1.Page{}).
		Complete(ratelimiter.NewReconciler(name, r, o.GlobalRateLimiter))
}

type connector struct {
	kube  client.Client
	usage *resource.ProviderConfigUsageTracker
}

func (c *connector) Connect(ctx context.Context, cr *v1alpha1.Page) (managed.TypedExternalClient[*v1alpha1.Page], error) {
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

func formatOrDefault(f string) string {
	if f == "" {
		return defaultFormat
	}
	return f
}

func (e *external) Observe(ctx context.Context, cr *v1alpha1.Page) (managed.ExternalObservation, error) {
	id := meta.GetExternalName(cr)
	if id == "" {
		return managed.ExternalObservation{ResourceExists: false}, nil
	}

	p, _, err := e.client.GetPage(id, ctfd.WithContext(ctx))
	if err != nil {
		return managed.ExternalObservation{ResourceExists: false}, nil
	}

	cr.Status.AtProvider = v1alpha1.PageObservation{ID: p.ID, Route: p.Route}
	cr.Status.SetConditions(xpv1.Available())

	return managed.ExternalObservation{
		ResourceExists:   true,
		ResourceUpToDate: isUpToDate(cr.Spec.ForProvider, p),
	}, nil
}

func isUpToDate(d v1alpha1.PageParameters, p *ctfd.Page) bool {
	content := ""
	if p.Content != nil {
		content = *p.Content
	}
	switch {
	case p.Title != d.Title,
		p.Route != d.Route,
		content != d.Content,
		p.Format != formatOrDefault(d.Format),
		p.Draft != d.Draft,
		p.Hidden != d.Hidden,
		p.AuthRequired != d.AuthRequired:
		return false
	}
	return true
}

func (e *external) Create(ctx context.Context, cr *v1alpha1.Page) (managed.ExternalCreation, error) {
	cr.Status.SetConditions(xpv1.Creating())

	d := cr.Spec.ForProvider
	p, _, err := e.client.PostPages(&ctfd.PostPagesParams{
		Title:        d.Title,
		Route:        d.Route,
		Content:      d.Content,
		Format:       formatOrDefault(d.Format),
		Draft:        d.Draft,
		Hidden:       d.Hidden,
		AuthRequired: d.AuthRequired,
	}, ctfd.WithContext(ctx))
	if err != nil {
		return managed.ExternalCreation{}, errors.Wrap(err, errCreate)
	}

	meta.SetExternalName(cr, strconv.Itoa(p.ID))
	return managed.ExternalCreation{}, nil
}

func (e *external) Update(ctx context.Context, cr *v1alpha1.Page) (managed.ExternalUpdate, error) {
	id := meta.GetExternalName(cr)

	d := cr.Spec.ForProvider
	if _, _, err := e.client.PatchPage(id, &ctfd.PatchPageParams{
		Title:        d.Title,
		Route:        d.Route,
		Content:      d.Content,
		Format:       formatOrDefault(d.Format),
		Draft:        d.Draft,
		Hidden:       d.Hidden,
		AuthRequired: d.AuthRequired,
	}, ctfd.WithContext(ctx)); err != nil {
		return managed.ExternalUpdate{}, errors.Wrap(err, errUpdate)
	}

	return managed.ExternalUpdate{}, nil
}

func (e *external) Delete(ctx context.Context, cr *v1alpha1.Page) (managed.ExternalDelete, error) {
	cr.Status.SetConditions(xpv1.Deleting())

	id := meta.GetExternalName(cr)
	if id == "" {
		return managed.ExternalDelete{}, nil
	}

	if _, err := e.client.DeletePage(id, ctfd.WithContext(ctx)); err != nil {
		return managed.ExternalDelete{}, errors.Wrap(err, errDelete)
	}
	return managed.ExternalDelete{}, nil
}

func (e *external) Disconnect(_ context.Context) error {
	return nil
}

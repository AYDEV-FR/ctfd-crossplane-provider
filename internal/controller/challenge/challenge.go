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
	errCreate       = "cannot create challenge in CTFd"
	errUpdate       = "cannot update challenge in CTFd"
	errDelete       = "cannot delete challenge in CTFd"
	errBadID        = "cannot parse external name as a challenge ID"
	errListFlags    = "cannot list challenge flags in CTFd"
	errSyncFlags    = "cannot reconcile challenge flags in CTFd"
	errListHints    = "cannot list challenge hints in CTFd"
	errSyncHints    = "cannot reconcile challenge hints in CTFd"
)

// SetupGated adds a controller that reconciles Challenge managed resources with
// safe-start support.
func SetupGated(mgr ctrl.Manager, o controller.Options) error {
	o.Gate.Register(func() {
		if err := Setup(mgr, o); err != nil {
			panic(errors.Wrap(err, "cannot setup Challenge controller"))
		}
	}, v1alpha1.ChallengeGroupVersionKind)
	return nil
}

// Setup adds a controller that reconciles Challenge managed resources.
func Setup(mgr ctrl.Manager, o controller.Options) error {
	name := managed.ControllerName(v1alpha1.ChallengeGroupKind)

	opts := []managed.ReconcilerOption{
		managed.WithTypedExternalConnector[*v1alpha1.Challenge](&connector{
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
			mgr.GetClient(), o.Logger, o.MetricOptions.MRStateMetrics, &v1alpha1.ChallengeList{}, o.MetricOptions.PollStateMetricInterval,
		)
		if err := mgr.Add(stateMetricsRecorder); err != nil {
			return errors.Wrap(err, "cannot register MR state metrics recorder for kind v1alpha1.ChallengeList")
		}
	}

	r := managed.NewReconciler(mgr, resource.ManagedKind(v1alpha1.ChallengeGroupVersionKind), opts...)

	return ctrl.NewControllerManagedBy(mgr).
		Named(name).
		WithOptions(o.ForControllerRuntime()).
		WithEventFilter(resource.DesiredStateChanged()).
		For(&v1alpha1.Challenge{}).
		Complete(ratelimiter.NewReconciler(name, r, o.GlobalRateLimiter))
}

// A connector produces an ExternalClient when its Connect method is called.
type connector struct {
	kube  client.Client
	usage *resource.ProviderConfigUsageTracker
}

// Connect tracks ProviderConfig usage and builds a CTFd client.
func (c *connector) Connect(ctx context.Context, cr *v1alpha1.Challenge) (managed.TypedExternalClient[*v1alpha1.Challenge], error) {
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

func (e *external) Observe(ctx context.Context, cr *v1alpha1.Challenge) (managed.ExternalObservation, error) {
	id := meta.GetExternalName(cr)
	if id == "" {
		return managed.ExternalObservation{ResourceExists: false}, nil
	}

	cid, err := strconv.Atoi(id)
	if err != nil {
		// Crossplane defaults the external-name annotation to the resource's
		// metadata.name. Until Create assigns the numeric CTFd ID, a non-numeric
		// external-name means the challenge does not exist yet, so Create runs.
		return managed.ExternalObservation{ResourceExists: false}, nil //nolint:nilerr // a non-numeric external-name means "not created yet"
	}

	ch, _, err := e.client.GetChallenge(cid, ctfd.WithContext(ctx))
	if err != nil {
		// The challenge could not be retrieved: treat it as absent so the
		// managed reconciler re-creates it.
		return managed.ExternalObservation{ResourceExists: false}, nil //nolint:nilerr // a failed GET means the challenge is gone; recreate it
	}

	var req *ctfd.Requirements
	if cr.Spec.ForProvider.Requirements != nil {
		req, _, _ = e.client.GetChallengeRequirements(cid, ctfd.WithContext(ctx))
	}

	flags, _, err := e.client.GetChallengeFlags(cid, ctfd.WithContext(ctx))
	if err != nil {
		return managed.ExternalObservation{}, errors.Wrap(err, errListFlags)
	}
	hints, err := e.challengeHints(ctx, cid)
	if err != nil {
		return managed.ExternalObservation{}, errors.Wrap(err, errListHints)
	}

	cr.Status.AtProvider = ChallengeObservation(ch, flags, hints)
	cr.Status.SetConditions(xpv1.Available())

	upToDate := isUpToDate(cr.Spec.ForProvider, ch, req) &&
		flagsUpToDate(cr.Spec.ForProvider.Flags, flags) &&
		hintsUpToDate(cr.Spec.ForProvider.Hints, hints)

	return managed.ExternalObservation{
		ResourceExists:   true,
		ResourceUpToDate: upToDate,
	}, nil
}

func (e *external) Create(ctx context.Context, cr *v1alpha1.Challenge) (managed.ExternalCreation, error) {
	cr.Status.SetConditions(xpv1.Creating())

	ch, _, err := e.client.PostChallenges(postParams(cr.Spec.ForProvider), ctfd.WithContext(ctx))
	if err != nil {
		return managed.ExternalCreation{}, errors.Wrap(err, errCreate)
	}

	meta.SetExternalName(cr, strconv.Itoa(ch.ID))
	return managed.ExternalCreation{}, nil
}

func (e *external) Update(ctx context.Context, cr *v1alpha1.Challenge) (managed.ExternalUpdate, error) {
	cid, err := strconv.Atoi(meta.GetExternalName(cr))
	if err != nil {
		return managed.ExternalUpdate{}, errors.Wrap(err, errBadID)
	}

	if _, _, err := e.client.PatchChallenge(cid, patchParams(cr.Spec.ForProvider, cr.Status.AtProvider.State), ctfd.WithContext(ctx)); err != nil {
		return managed.ExternalUpdate{}, errors.Wrap(err, errUpdate)
	}

	if err := e.syncFlags(ctx, cid, cr.Spec.ForProvider.Flags); err != nil {
		return managed.ExternalUpdate{}, errors.Wrap(err, errSyncFlags)
	}
	if err := e.syncHints(ctx, cid, cr.Spec.ForProvider.Hints); err != nil {
		return managed.ExternalUpdate{}, errors.Wrap(err, errSyncHints)
	}

	return managed.ExternalUpdate{}, nil
}

func (e *external) Delete(ctx context.Context, cr *v1alpha1.Challenge) (managed.ExternalDelete, error) {
	cr.Status.SetConditions(xpv1.Deleting())

	id := meta.GetExternalName(cr)
	cid, err := strconv.Atoi(id)
	if err != nil {
		// Never created (external-name still defaults to the resource name).
		return managed.ExternalDelete{}, nil //nolint:nilerr // nothing to delete if it was never created
	}

	if _, err := e.client.DeleteChallenge(cid, ctfd.WithContext(ctx)); err != nil {
		return managed.ExternalDelete{}, errors.Wrap(err, errDelete)
	}
	return managed.ExternalDelete{}, nil
}

func (e *external) Disconnect(_ context.Context) error {
	return nil
}

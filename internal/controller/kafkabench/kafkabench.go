/*
Copyright 2020 The Crossplane Authors.

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

package kafkabench

import (
	"context"
	"fmt"
	"strconv"

	xpv1 "github.com/crossplane/crossplane-runtime/apis/common/v1"
	"github.com/mitchellh/mapstructure"
	"github.com/pkg/errors"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/util/workqueue"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller"

	"github.com/crossplane/crossplane-runtime/pkg/event"
	"github.com/crossplane/crossplane-runtime/pkg/logging"
	"github.com/crossplane/crossplane-runtime/pkg/ratelimiter"
	"github.com/crossplane/crossplane-runtime/pkg/reconciler/managed"
	"github.com/crossplane/crossplane-runtime/pkg/resource"

	"github.com/crossplane/provider-template/apis/tarasque/v1alpha1"
	apisv1alpha1 "github.com/crossplane/provider-template/apis/v1alpha1"
)

const (
	roundTripWorkload = "org.apache.kafka.trogdor.workload.RoundTripWorkloadSpec"
	producerWorkload  = "org.apache.kafka.trogdor.workload.ProduceBenchSpec"
	consumerWorkload  = "org.apache.kafka.trogdor.workload.ConsumeBenchSpec"
	errNotKafkaBench  = "managed resource is not a KafkaBench custom resource"
	errTrackPCUsage   = "cannot track ProviderConfig usage"
	errGetPC          = "cannot get ProviderConfig"
	errGetCreds       = "cannot get credentials"

	errNewClient = "cannot create new Service"
	errNewTask   = "cannot create new Task"
)

// A NoOpService does nothing.
type NoOpService struct{}

var (
	newNoOpService = func(_ []byte) (*TrogdorAgentService, error) { return NewTrogdorService(), nil }
)

// Setup adds a controller that reconciles KafkaBench managed resources.
func Setup(mgr ctrl.Manager, l logging.Logger, rl workqueue.RateLimiter) error {
	name := managed.ControllerName(v1alpha1.KafkaBenchGroupKind)

	o := controller.Options{
		RateLimiter: ratelimiter.NewDefaultManagedRateLimiter(rl),
	}

	r := managed.NewReconciler(mgr,
		resource.ManagedKind(v1alpha1.KafkaBenchGroupVersionKind),
		managed.WithExternalConnecter(&connector{
			kube:         mgr.GetClient(),
			usage:        resource.NewProviderConfigUsageTracker(mgr.GetClient(), &apisv1alpha1.ProviderConfigUsage{}),
			newServiceFn: newNoOpService}),
		managed.WithLogger(l.WithValues("controller", name)),
		managed.WithRecorder(event.NewAPIRecorder(mgr.GetEventRecorderFor(name))))

	return ctrl.NewControllerManagedBy(mgr).
		Named(name).
		WithOptions(o).
		For(&v1alpha1.KafkaBench{}).
		Complete(r)
}

type WorkerTaskSpec struct {
	v1alpha1.KafkaBenchSpec
	StartMs int64 `json:"startMs,omitempty"`
}

// KafkaTopics are part of the desired state fields
type WorkerTask struct {
	TaskId   string         `json:"taskId,omitempty"`
	WorkerId int64          `json:"workerId,omitempty"`
	Spec     WorkerTaskSpec `json:"spec,omitempty"`
}

// A connector is expected to produce an ExternalClient when its Connect method
// is called.
type connector struct {
	kube         client.Client
	usage        resource.Tracker
	newServiceFn func(creds []byte) (*TrogdorAgentService, error)
}

// Connect typically produces an ExternalClient by:
// 1. Tracking that the managed resource is using a ProviderConfig.
// 2. Getting the managed resource's ProviderConfig.
// 3. Getting the credentials specified by the ProviderConfig.
// 4. Using the credentials to form a client.
func (c *connector) Connect(ctx context.Context, mg resource.Managed) (managed.ExternalClient, error) {
	cr, ok := mg.(*v1alpha1.KafkaBench)
	if !ok {
		return nil, errors.New(errNotKafkaBench)
	}

	if err := c.usage.Track(ctx, mg); err != nil {
		return nil, errors.Wrap(err, errTrackPCUsage)
	}

	pc := &apisv1alpha1.ProviderConfig{}
	if err := c.kube.Get(ctx, types.NamespacedName{Name: cr.GetProviderConfigReference().Name}, pc); err != nil {
		return nil, errors.Wrap(err, errGetPC)
	}

	cd := pc.Spec.Credentials
	data, err := resource.CommonCredentialExtractor(ctx, cd.Source, c.kube, cd.CommonCredentialSelectors)
	if err != nil {
		return nil, errors.Wrap(err, errGetCreds)
	}

	svc, err := c.newServiceFn(data)
	if err != nil {
		return nil, errors.Wrap(err, errNewClient)
	}

	return &external{service: svc}, nil
}

// An ExternalClient observes, then either creates, updates, or deletes an
// external resource to ensure it reflects the managed resource's desired state.
type external struct {
	// A 'client' used to connect to the external resource API. In practice this
	// would be something like an AWS SDK client.
	service *TrogdorAgentService
}

func (c *external) Observe(ctx context.Context, mg resource.Managed) (managed.ExternalObservation, error) {
	cr, ok := mg.(*v1alpha1.KafkaBench)
	if !ok {
		return managed.ExternalObservation{}, errors.New(errNotKafkaBench)
	}

	// These fmt statements should be removed in the real implementation.
	fmt.Printf("Observing: %+v \n", cr)

	return managed.ExternalObservation{
		// Return false when the external resource does not exist. This lets
		// the managed resource reconciler know that it needs to call Create to
		// (re)create the resource, or that it has successfully been deleted.
		ResourceExists: cr.Status.AtProvider.TaskId != "",

		// Return false when the external resource exists, but it not up to date
		// with the desired managed resource state. This lets the managed
		// resource reconciler know that it needs to call Update.
		ResourceUpToDate: cr.Status.AtProvider.TaskStatus == "DONE",

		// Return any details that may be required to connect to the external
		// resource. These will be stored as the connection secret.
		ConnectionDetails: managed.ConnectionDetails{},
	}, nil
}

func (c *external) Create(ctx context.Context, mg resource.Managed) (managed.ExternalCreation, error) {
	cr, ok := mg.(*v1alpha1.KafkaBench)
	if !ok {
		return managed.ExternalCreation{}, errors.New(errNotKafkaBench)
	}
	cr.SetConditions(xpv1.Creating())

	workerTask, err := c.service.CreateWorkerTask(cr.Spec)
	if err != nil {
		return managed.ExternalCreation{}, err
	}
	cr.Status.AtProvider.TaskStatus = "CREATED"
	cr.Status.AtProvider.TaskId = workerTask.TaskId
	cr.Status.AtProvider.WorkerId = workerTask.WorkerId

	return managed.ExternalCreation{
		// Optionally return any details that may be required to connect to the
		// external resource. These will be stored as the connection secret.
		ConnectionDetails: managed.ConnectionDetails{
			"taskId":    []byte(workerTask.TaskId),
			"name":      []byte(cr.Name),
			"namespace": []byte(cr.Namespace),
		},
	}, nil
}

func (c *external) Update(ctx context.Context, mg resource.Managed) (managed.ExternalUpdate, error) {
	cr, ok := mg.(*v1alpha1.KafkaBench)
	if !ok {
		return managed.ExternalUpdate{}, errors.New(errNotKafkaBench)
	}
	fmt.Printf("Updating: %+v \n", cr)

	workerId := strconv.FormatInt(cr.Status.AtProvider.WorkerId, 10)
	statusResponse, err := c.service.CollectWorkerTaskResult(workerId)
	if err != nil {
		return managed.ExternalUpdate{}, err
	}

	cr.Status.AtProvider.TaskStatus = statusResponse.State
	// status could be a string like "creating topics..."
	if _, ok := statusResponse.Status.(map[string]interface{}); !ok {
		fmt.Printf("Tasks running but waiting for a condition: %v \n", statusResponse.Status)
		return managed.ExternalUpdate{}, nil
	}
	switch cr.Spec.Class {
	case producerWorkload:
		if err := mapstructure.Decode(statusResponse.Status, &cr.Status.AtProvider.ProducerStats); err != nil {
			return managed.ExternalUpdate{}, err
		}
	case roundTripWorkload:
		if err := mapstructure.Decode(statusResponse.Status, &cr.Status.AtProvider.RoundTripStats); err != nil {
			return managed.ExternalUpdate{}, err
		}
	case consumerWorkload:
		if err := mapstructure.Decode(statusResponse.Status, &cr.Status.AtProvider.ConsumerStats); err != nil {
			return managed.ExternalUpdate{}, err
		}
	}
	fmt.Printf("Got from collect %v for workerId %s\n", statusResponse, workerId)
	cr.SetConditions(xpv1.Available())
	return managed.ExternalUpdate{
		// Optionally return any details that may be required to connect to the
		// external resource. These will be stored as the connection secret.
		ConnectionDetails: managed.ConnectionDetails{},
	}, nil
}

func (c *external) Delete(ctx context.Context, mg resource.Managed) error {
	cr, ok := mg.(*v1alpha1.KafkaBench)
	if !ok {
		return errors.New(errNotKafkaBench)
	}

	fmt.Printf("Deleting: %+v", cr)
	cr.SetConditions(xpv1.Deleting())
	workerId := strconv.FormatInt(cr.Status.AtProvider.WorkerId, 10)
	if err := c.service.DeleteWorkerTask(workerId); err != nil {
		return err
	}

	cr.Status.AtProvider.TaskId = ""
	return nil
}

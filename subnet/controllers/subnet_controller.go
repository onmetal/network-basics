/*


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

package controllers

import (
	"context"

	"github.com/go-logr/logr"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/util/workqueue"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/event"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
	"sigs.k8s.io/controller-runtime/pkg/source"

	corev1 "gardener/subnet/api/v1"

	netGlo "gardener/networkGlobal/api/v1"
)

const (
	SubnetFinalizerName = "core.gardener.cloud/networkglobal"
)

// SubnetReconciler reconciles a Subnet object
type SubnetReconciler struct {
	client.Client
	Log           logr.Logger
	Scheme        *runtime.Scheme
	Manager       ctrl.Manager
	Subnet        *corev1.Subnet
	Request       ctrl.Request
	NetworkGlobal netGlo.NetworkGlobal
}

// +kubebuilder:rbac:groups=core.gardener.cloud,resources=subnets,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=core.gardener.cloud,resources=subnets/status,verbs=get;update;patch

func (r *SubnetReconciler) Reconcile(req ctrl.Request) (ctrl.Result, error) {
	ctx := context.Background()
	r.Request = req
	log := r.Log.WithValues("subnet", req.NamespacedName)

	reqLogger := r.Log.WithValues("Request.Name", req.Name)
	reqLogger.Info("Reconciling Subnet")

	r.Subnet = &corev1.Subnet{}

	if err := r.Get(ctx, req.NamespacedName, r.Subnet); err != nil {
		log.Info("unable to fetch Subnet", "Subnet", req, "Error", err)
		// we'll ignore not-found errors, since they can't be fixed by an immediate
		// requeue (we'll need to wait for a new notification), and we can get them
		// on deleted requests.
		return ctrl.Result{}, client.IgnoreNotFound(err)
	}

	// Add finalizer on the Subnet object if not added already.
	if r.Subnet.DeletionTimestamp.IsZero() {
		err := r.addSubnetFinalizer()
		if err != nil {
			log.Error(err, "Can't add the finalizer", "Subnet", r.Subnet.Name)
			return ctrl.Result{}, err
		}
	}

	// Deletion Flow
	if !r.Subnet.DeletionTimestamp.IsZero() {
		log.Info("Deleting the Subnet", "Name", r.Subnet.Name)

		// Remove the finalizer
		err := r.deleteSubnetFinalizers()
		if err != nil {
			log.Error(err, "Couldn't delete the finalizer", "Subnet", r.Subnet.Name)
			return ctrl.Result{}, err
		}
		log.V(0).Info("Successfully deleted the Subnet", "Name", r.Subnet.Name)
		return ctrl.Result{}, nil
	}

	return ctrl.Result{}, nil
}

func (r *SubnetReconciler) SetupWithManager(mgr ctrl.Manager) error {

	return ctrl.NewControllerManagedBy(mgr).
		For(&corev1.Subnet{}).
		Watches(&source.Kind{Type: &corev1.Subnet{}}, handler.Funcs{
			CreateFunc: func(e event.CreateEvent, q workqueue.RateLimitingInterface) {
				ctx := context.Background()

				netGloList := &netGlo.NetworkGlobalList{}

				if err := r.List(ctx, netGloList.DeepCopyObject(), &client.ListOptions{}); err != nil {
					r.Log.Info("unable to find NetworkGlobalID", "Error", err)
				}
				r.Log.Info("NetGloList danach: ", "Name", *netGloList)

				q.Add(reconcile.Request{NamespacedName: types.NamespacedName{
					Name:      e.Meta.GetName(),
					Namespace: e.Meta.GetNamespace(),
				}})
			},
			DeleteFunc: func(e event.DeleteEvent, q workqueue.RateLimitingInterface) {
				q.Add(reconcile.Request{NamespacedName: types.NamespacedName{
					Name:      e.Meta.GetName(),
					Namespace: e.Meta.GetNamespace(),
				}})
			},
		}).
		Complete(r)
}

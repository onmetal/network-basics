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

	corev1 "gardener/networkGlobal/api/v1"
)

const (
	NetworkGlobalFinalizerName = "core.core.gardener.cloud/networkglobal"
)

// NetworkGlobalReconciler reconciles a NetworkGlobal object
type NetworkGlobalReconciler struct {
	client.Client
	Log           logr.Logger
	Scheme        *runtime.Scheme
	Manager       ctrl.Manager
	NetworkGlobal *corev1.NetworkGlobal
	Request       ctrl.Request
}

// +kubebuilder:rbac:groups=core.core.gardener.cloud,resources=networkglobals,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=core.core.gardener.cloud,resources=networkglobals/status,verbs=get;update;patch

func (r *NetworkGlobalReconciler) Reconcile(req ctrl.Request) (ctrl.Result, error) {
	ctx := context.Background()
	log := r.Log.WithValues("networkglobal", req.NamespacedName)

	reqLogger := r.Log.WithValues("Request.Name", req.Name)
	reqLogger.Info("Reconciling NetworkGlobal")

	r.NetworkGlobal = &corev1.NetworkGlobal{}

	if err := r.Get(ctx, req.NamespacedName, r.NetworkGlobal); err != nil {
		log.Info("unable to fetch NetworkGlobal", "NetworkGlobal", req, "Error", err)
		// we'll ignore not-found errors, since they can't be fixed by an immediate
		// requeue (we'll need to wait for a new notification), and we can get them
		// on deleted requests.
		return ctrl.Result{}, client.IgnoreNotFound(err)
	}

	// Add finalizer on the NetworkGlobal object if not added already.
	err := r.addNetworkGlobalFinalizer()
	if err != nil {
		log.Error(err, "Can't add the finalizer", "NetworkGlobal", r.NetworkGlobal.Name)
		return ctrl.Result{}, err
	}

	// Deletion Flow
	if !r.NetworkGlobal.DeletionTimestamp.IsZero() {
		log.Info("Deleting the NetworkGlobal", "Name", r.NetworkGlobal.Name)
	}

	return ctrl.Result{}, nil
}

func (r *NetworkGlobalReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&corev1.NetworkGlobal{}).
		Watches(&source.Kind{Type: &corev1.NetworkGlobal{}}, handler.Funcs{
			CreateFunc: func(e event.CreateEvent, q workqueue.RateLimitingInterface) {
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

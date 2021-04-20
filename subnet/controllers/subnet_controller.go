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
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/event"
	"sigs.k8s.io/controller-runtime/pkg/predicate"

	corev1 "gardener/subnet/api/v1"
)

const (
	SubnetFinalizerName  = "core.gardener.cloud/subnet"
	LabelTreeDepthSuffix = ".tree/depth"
)

// SubnetReconciler reconciles a Subnet object
type SubnetReconciler struct {
	client.Client
	Log     logr.Logger
	Scheme  *runtime.Scheme
	Manager ctrl.Manager
	Subnet  *corev1.Subnet
	Request ctrl.Request
}

// +kubebuilder:rbac:groups=core.gardener.cloud,resources=subnets,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=core.gardener.cloud,resources=subnets/status,verbs=get;update;patch

func (r *SubnetReconciler) Reconcile(req ctrl.Request) (ctrl.Result, error) {
	ctx := context.Background()
	r.Request = req
	log := r.Log.WithValues("subnet", req.NamespacedName)

	reqLogger := r.Log.WithValues("Request.Name", req.Name)
	reqLogger.Info("Reconciling Subnet Test")

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

		if ok, err := r.IsSubnetLeafNode(r.Subnet); !ok {
			//r.Log.Error(err, "Subnet can't be deleted because it has child subnets", "Subnet", r.Subnet.Name)
			r.Subnet.Status.Messages = append(r.Subnet.Status.Messages, "Subnet can't be deleted because it has child subnets")
			r.updateSubnetStatusMessages(r.Subnet.Status.Messages)
			return ctrl.Result{}, err
		} else {
			// Remove the finalizer
			err := r.deleteSubnetFinalizers()
			if err != nil {
				log.Error(err, "Couldn't delete the finalizer", "Subnet", r.Subnet.Name)
				return ctrl.Result{}, err
			}
			log.V(0).Info("Successfully deleted the Subnet", "Name", r.Subnet.Name)
			return ctrl.Result{}, nil
		}
	}

	return ctrl.Result{}, nil
}

func (r *SubnetReconciler) SetupWithManager(mgr ctrl.Manager) error {

	predicateFunctions := predicate.Funcs{
		CreateFunc: func(e event.CreateEvent) bool {
			subnet := e.Object.(*corev1.Subnet)

			r.Log.Info("SetupWithManager - IsNetworkGlobalIDValid")
			if ok, err := r.IsNetworkGlobalIDValid(subnet); !ok {
				r.Log.Error(err, "NetworkGlobalID is invalid, resource doesn't exist", "Subnet", subnet.Spec.NetworkGlobalID)
				// TODO: take some action when this is invalid
				return false
			}
			r.Log.Info("SetupWithManager - addSubnetToTree")
			if ok, err := r.addSubnetToTree(subnet); !ok {
				r.Log.Error(err, "Integrity of the subnet is invalid", "Subnet", subnet.ObjectMeta.Name)
				// TODO: take some action when this is invalid
				return false
			}
			r.Log.Info("SetupWithManager - checkIPIntegrity")
			if ok, err := r.checkIPIntegrity(subnet); !ok {
				r.Log.Error(err, "IP Integrity of the subnet is invalid", "Subnet", subnet.ObjectMeta.Name)
				// TODO: take some action when this is invalid
				return false
			}
			return true
		},
		DeleteFunc: func(e event.DeleteEvent) bool {
			return false
		},
	}

	return ctrl.NewControllerManagedBy(mgr).
		For(&corev1.Subnet{}).
		WithEventFilter(predicateFunctions).
		Complete(r)
}

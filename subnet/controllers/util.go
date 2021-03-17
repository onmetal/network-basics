package controllers

import (
	"context"

	v1 "gardener/subnet/api/v1"

	"k8s.io/apimachinery/pkg/util/sets"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

// addSubnetFinalizer adds the finalizer on the Subnet
func (r SubnetReconciler) addSubnetFinalizer() error {
	ctx := context.Background()
	subnet := &v1.Subnet{}

	err := r.Get(ctx, r.Request.NamespacedName, subnet)
	if err != nil {
		return client.IgnoreNotFound(err)
	}

	clone := subnet.DeepCopy()

	if finalizers := sets.NewString(clone.Finalizers...); !finalizers.Has(SubnetFinalizerName) {
		r.Log.Info("Adding finalizer")
		finalizers.Insert(SubnetFinalizerName)
		err := r.updateSubnetFinalizers(finalizers.List())
		if err != nil {
			return client.IgnoreNotFound(err)
		}
	}
	return nil
}

// updateSubnetFinalizers updates the finalizer on the Subnet.
func (r *SubnetReconciler) updateSubnetFinalizers(finalizers []string) error {
	ctx := context.Background()
	subnet := &v1.Subnet{}
	err := r.Get(ctx, r.Request.NamespacedName, subnet)
	if err != nil {
		return client.IgnoreNotFound(err)
	}

	clone := subnet.DeepCopy()
	clone.Finalizers = finalizers

	err = r.Patch(ctx, clone, client.MergeFrom(subnet))
	if err != nil {
		return client.IgnoreNotFound(err)
	}

	r.Subnet = clone
	return nil
}

// deleteSubnetFinalizers deletes the finalizer from the Subnet.
func (r *SubnetReconciler) deleteSubnetFinalizers() error {
	ctx := context.Background()
	subnet := &v1.Subnet{}
	err := r.Get(ctx, r.Request.NamespacedName, subnet)
	if err != nil {
		return client.IgnoreNotFound(err)
	}

	clone := subnet.DeepCopy()

	if finalizers := sets.NewString(clone.Finalizers...); finalizers.Has(SubnetFinalizerName) {
		finalizers.Delete(SubnetFinalizerName)
		err := r.updateSubnetFinalizers(finalizers.List())
		if err != nil {
			return client.IgnoreNotFound(err)
		}
	}
	return nil
}

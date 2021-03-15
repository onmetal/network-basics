package controllers

import (
	"context"

	v1 "gardener/networkGlobal/api/v1"

	"k8s.io/apimachinery/pkg/util/sets"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

// addNetworkGlobalFinalizer adds the finalizer on the NetworkGlobal.
func (r NetworkGlobalReconciler) addNetworkGlobalFinalizer() error {
	ctx := context.Background()
	nGlobal := &v1.NetworkGlobal{}

	err := r.Get(ctx, r.Request.NamespacedName, nGlobal)
	if err != nil {
		return client.IgnoreNotFound(err)
	}

	clone := nGlobal.DeepCopy()

	if finalizers := sets.NewString(clone.Finalizers...); !finalizers.Has(NetworkGlobalFinalizerName) {
		r.Log.Info("Adding finalizer")
		finalizers.Insert(NetworkGlobalFinalizerName)
		err := r.updateNetworkGlobalFinalizers(finalizers.List())
		if err != nil {
			return client.IgnoreNotFound(err)
		}
	}
	return nil
}

// updateNetworkGlobalFinalizers updates the finalizer on the NetworkGlobal.
func (r *NetworkGlobalReconciler) updateNetworkGlobalFinalizers(finalizers []string) error {
	ctx := context.Background()
	nGlobal := &v1.NetworkGlobal{}
	err := r.Get(ctx, r.Request.NamespacedName, nGlobal)
	if err != nil {
		return client.IgnoreNotFound(err)
	}

	clone := nGlobal.DeepCopy()
	clone.Finalizers = finalizers

	err = r.Patch(ctx, clone, client.MergeFrom(nGlobal))
	if err != nil {
		return client.IgnoreNotFound(err)
	}

	r.NetworkGlobal = clone
	return nil
}

// deleteNetworkGlobalFinalizers deletes the finalizer from the NetworkGlobal.
func (r *NetworkGlobalReconciler) deleteNetworkGlobalFinalizers() error {
	ctx := context.Background()
	nGlobal := &v1.NetworkGlobal{}
	err := r.Get(ctx, r.Request.NamespacedName, nGlobal)
	if err != nil {
		return client.IgnoreNotFound(err)
	}

	clone := nGlobal.DeepCopy()

	if finalizers := sets.NewString(clone.Finalizers...); finalizers.Has(NetworkGlobalFinalizerName) {
		finalizers.Delete(NetworkGlobalFinalizerName)
		err := r.updateNetworkGlobalFinalizers(finalizers.List())
		if err != nil {
			return client.IgnoreNotFound(err)
		}
	}
	return nil
}

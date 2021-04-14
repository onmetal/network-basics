package controllers

import (
	"context"
	"errors"
	"strconv"

	netGlo "gardener/networkGlobal/api/v1"
	corev1 "gardener/subnet/api/v1"
	v1 "gardener/subnet/api/v1"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
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

// validates if the entered networkglobalID is a existing NetworkGlobal Object
func (r *SubnetReconciler) IsNetworkGlobalIDValid(obj metav1.Object) (bool, error) {
	ctx := context.Background()
	subnet, ok := obj.(*corev1.Subnet)
	if !ok {
		return false, errors.New("not a subnet object")
	}

	netGloID := subnet.Spec.NetworkGlobalID
	netGlobalObject := &netGlo.NetworkGlobal{}

	if err := r.Get(ctx, types.NamespacedName{Name: netGloID, Namespace: subnet.Namespace}, netGlobalObject); err != nil {
		err = errors.New("Subnet is not valid because resource with networkGlobalID doesn't exist")
		subnet.Status.Messages = append(subnet.Status.Messages, err.Error())
		return false, err
	}
	r.Log.Info("Succesfully got the NetWorkGlobal Object", "Name", netGlobalObject.Name)
	return true, nil
}

func (r *SubnetReconciler) addSubnetToTree(obj metav1.Object) (bool, error) {
	ctx := context.Background()
	subnet, ok := obj.(*corev1.Subnet)
	if !ok {
		return false, errors.New("not a subnet object")
	}

	clone := subnet.DeepCopy()

	subnetParent := &corev1.Subnet{}
	newLabels := make(map[string]string)
	parentID := subnet.Spec.SubnetParentID

	if parentID != "" {

		if err := r.Get(ctx, types.NamespacedName{Name: parentID, Namespace: subnet.Namespace}, subnetParent); err != nil {
			return false, errors.New("ParentID not valid because parent resource does not exist")
		}
		if ok, err := r.IsPartitionIDValid(*subnet, *subnetParent); !ok {
			r.Log.Error(err, "PartitionID not valid", "Subnet", subnet.Spec.PartitionID)
			return false, err
		}
		r.Log.Info("PartitionID valid", "Subnet", subnet.Spec.PartitionID)

		newLabels[subnet.Name+LabelTreeDepthSuffix] = "0"
		oldLables := subnetParent.GetLabels()
		for key, element := range oldLables {
			element, _ := strconv.Atoi(element)
			element = element + 1
			newLabels[key] = strconv.Itoa(element)
		}
	} else {
		newLabels[subnet.Name+LabelTreeDepthSuffix] = "0"
		newLabels[subnet.Spec.NetworkGlobalID+LabelTreeDepthSuffix] = "1"
	}
	clone.SetLabels(newLabels)

	subnet.ObjectMeta = clone.ObjectMeta

	err := r.Update(ctx, subnet)
	if err != nil {
		r.Log.Error(err, "update error", "Subnet", subnet.Name)
		return false, err
	}

	r.Log.Info("Succesfully added the Subnet to the tree", "Name", subnet.Name)
	return true, nil
}

func (r *SubnetReconciler) IsPartitionIDValid(subnet corev1.Subnet, subnetParent corev1.Subnet) (bool, error) {
	if subnet.Spec.PartitionID != subnetParent.Spec.PartitionID {
		return false, errors.New("PartitionID not valid because it doesn't matches the parent subnet PartitionID")
	}
	return true, nil
}

func (r *SubnetReconciler) IsSubnetLeafNode(obj metav1.Object) (bool, error) {
	ctx := context.Background()
	subnet, ok := obj.(*corev1.Subnet)
	if !ok {
		return false, errors.New("not a subnet object")
	}

	subnetList := &corev1.SubnetList{}
	opts := []client.ListOption{
		client.InNamespace(subnet.Namespace),
		client.MatchingLabels{subnet.Name + LabelTreeDepthSuffix: "1"},
	}
	r.List(ctx, subnetList, opts...)
	if subnetList != nil {
		err := errors.New("not valid because subnet has childs")
		subnet.Status.Messages = append(subnet.Status.Messages, err.Error())
		return false, err
	}
	r.Log.Info("Deletion accapted", "Name", subnetList)
	return true, nil
}

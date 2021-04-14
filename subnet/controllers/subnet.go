package controllers

import (
	"context"
	"errors"
	"fmt"
	corev1 "gardener/subnet/api/v1"
	"net"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
)

func (r SubnetReconciler) checkIPIntegrity(obj metav1.Object) error {
	//log := r.Log.WithValues("subnet getIPRange", r.Subnet.Name)
	cidr := "10.12.0.0/16"
	ip, ipnet, _ := net.ParseCIDR(cidr)

	r.Log.Info(fmt.Sprintf("cidr: %s", cidr))
	r.Log.Info(fmt.Sprintf("Ip Adress: %s", ip))
	r.Log.Info(fmt.Sprintf("network: %s", ipnet))

	// just do it once here, so you don't have to do it in each method
	// you should also apply it to the util
	ctx := context.Background()
	subnet, _ := obj.(*corev1.Subnet)

	subnetParent := &corev1.Subnet{}
	parentID := subnet.Spec.SubnetParentID
	if err := r.Get(ctx, types.NamespacedName{Name: parentID, Namespace: subnet.Namespace}, subnetParent); err != nil {
		return errors.New("ParentID not valid because parent resource does not exist")
	}

	// TODO
	// checkParentCidrRange
	// check if there is even a parent
	// if nono just check the subnet level integrity
	// if yes checkParentCidrRange + subnet level integrity
	if err := r.checkParentCidrRange(subnet, subnetParent); err != nil {
		return err
	}

	return nil
}

func (r SubnetReconciler) checkParentCidrRange(subnet *corev1.Subnet, subnetParent *corev1.Subnet) error {
	// TODO
	// add error management
	ipSubnet, _, _ := net.ParseCIDR(subnet.Spec.CIDR)
	_, ipnetSupnetParent, _ := net.ParseCIDR(subnetParent.Spec.CIDR)

	if ipnetSupnetParent.Contains(ipSubnet) {
		return errors.New("violated ip integrity, subnet ")
	}

	return nil
}

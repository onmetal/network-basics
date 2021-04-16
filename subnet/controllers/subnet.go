package controllers

import (
	"context"
	//"errors"
	"cidr"
	"fmt"
	corev1 "gardener/subnet/api/v1"
	"net"

	"github.com/pkg/errors"
	"sigs.k8s.io/controller-runtime/pkg/client"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
)

func (r SubnetReconciler) checkIPIntegrity(obj metav1.Object) error {
	//log := r.Log.WithValues("subnet getIPRange", r.Subnet.Name)

	// just do it once here, so you don't have to do it in each method
	// you should also apply it to the util
	ctx := context.Background()
	subnet, _ := obj.(*corev1.Subnet)

	// TODO
	// add logs
	// check if there is even a parent
	// if nono just check the subnet level integrity
	// if yes checkParentCidrRange + subnet level integrity
	if subnet.Spec.SubnetParentID != "" {

		// get parent resource
		subnetParent := &corev1.Subnet{}
		parentID := subnet.Spec.SubnetParentID
		if err := r.Get(ctx, types.NamespacedName{Name: parentID, Namespace: subnet.Namespace}, subnetParent); err != nil {
			return errors.New("ParentID not valid because parent resource does not exist")
		}

		// check if subnet cidr range fits in parent cidr range
		ipNetSubnet, err := checkParentCidrRange(subnet, subnetParent)
		if err != nil {
			return err
		}

		// get the subnet cidr ranges from the same level of the subtree
		cidrnets, err := r.getSubtreeLevelCidrRanges(subnet)
		if err != nil {
			return err
		}

		// verifies that none of cidr blocks overlap
		err = validateCidrOverlap(ipNetSubnet, cidrnets)
		if err != nil {
			return err
		}
	}

	return nil
}

func checkParentCidrRange(subnet *corev1.Subnet, subnetParent *corev1.Subnet) (*net.IPNet, error) {
	ipSubnet, ipnetSubnet, _ := validateCidr(subnet.Spec.CIDR)
	_, ipnetSubnetParent, _ := validateCidr(subnetParent.Spec.CIDR)

	if ipnetSubnetParent.Contains(ipSubnet) {
		return ipnetSubnet, errors.New("Subnet CIDR range does not fit in parent subnet CIDR range")
	}

	return ipnetSubnet, nil
}

func validateCidr(cidr string) (net.IP, *net.IPNet, error) {
	ip, ipnet, err := net.ParseCIDR(cidr)
	if err != nil {
		return nil, nil, errors.Wrap(err, "Error parsing CIDR")
	}
	return ip, ipnet, nil
}

func (r SubnetReconciler) getSubtreeLevelCidrRanges(subnet *corev1.Subnet) ([]*net.IPNet, error) {

	var cidrnets []*net.IPNet

	// get the path of the subnet to root and delete own entry
	labels := subnet.GetLabels()
	delete(labels, subnet.Name+LabelTreeDepthSuffix)

	// find each subnet that has the same path, that are the siblings
	subnetList := &corev1.SubnetList{}
	opts := []client.ListOption{}
	for key, value := range labels {
		opts = []client.ListOption{
			client.InNamespace(subnet.Namespace),
			client.MatchingLabels{key: value},
		}
	}
	// TODO
	// add error Managegement
	r.List(context.Background(), subnetList, opts...)

	// get each cidr range from the siblings and transform them to IPNet
	for _, sn := range subnetList.Items {

		_, ipnet, err := validateCidr(sn.Spec.CIDR)
		if err != nil {
			return cidrnets, err
		}
		cidrnets = append(cidrnets, ipnet)
	}
	return cidrnets, nil
}

func validateCidrOverlap(ipNetSubnet *net.IPNet, cidrnets []*net.IPNet) error {

	// get the first and the last IP Adress from the new Subnet Cidr range
	firstIp, lastIp := cidr.AddressRange(ipNetSubnet)

	for _, cidrnet := range cidrnets {
		if cidrnet.Contains(firstIp) || cidrnet.Contains(lastIp) {
			return errors.Wrap(fmt.Errorf("Subnets CIDR overlaps with CIDR %s", cidrnet), "CIDR overlap validation")
		}
	}
	return nil
}

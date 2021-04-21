package controllers

import (
	"context"
	"math"

	//"errors"
	"fmt"
	corev1 "gardener/subnet/api/v1"
	v1 "gardener/subnet/api/v1"
	"go-cidr/cidr"
	"net"

	"github.com/pkg/errors"
	"sigs.k8s.io/controller-runtime/pkg/client"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
)

func (r SubnetReconciler) checkIPIntegrity(obj metav1.Object) (bool, error) {
	// just do it once here, so you don't have to do it in each method
	// you should also apply it to the util
	ctx := context.Background()
	subnet, ok := obj.(*corev1.Subnet)
	if !ok {
		return false, errors.New("not a subnet object")
	}
	log := r.Log.WithValues("checkIPIntegrity", fmt.Sprintf("Subnet: %s ,cidr: %s", subnet.Spec.ID, subnet.Spec.CIDR))

	// TODO
	// add logs
	// check if there is even a parent
	// if nono just check the subnet level integrity
	// if yes checkParentCidrRange + subnet level integrity
	if subnet.Spec.SubnetParentID != "" {
		log.Info("Subnet has Parent")
		// get parent resource
		subnetParent := &corev1.Subnet{}
		parentID := subnet.Spec.SubnetParentID
		if err := r.Get(ctx, types.NamespacedName{Name: parentID, Namespace: subnet.Namespace}, subnetParent); err != nil {
			return false, errors.New("ParentID not valid because parent resource does not exist")
		}

		log.Info("validate cidr")
		ipSubnet, ipnetSubnet, _ := validateCidr(subnet.Spec.CIDR)
		_, ipnetSubnetParent, _ := validateCidr(subnetParent.Spec.CIDR)

		// check if subnet cidr range fits in parent cidr range
		log.Info("check parent cidr range")
		if err := checkParentCidrRange(ipSubnet, ipnetSubnetParent); err != nil {
			return false, err
		}

		// get the subnet cidr ranges from the same level of the subtree
		log.Info("get subtree level cidr ranges")
		cidrnets, err := r.getSubtreeLevelCidrRanges(subnet)
		if err != nil {
			return false, err
		}

		// verifies that none of cidr blocks overlap
		log.Info("validate cidr overlap")
		err = validateCidrOverlap(ipnetSubnet, cidrnets)
		if err != nil {
			return false, err
		}

		// update subnet status
		log.Info("update subnet status")
		prefixSize, _ := ipnetSubnet.Mask.Size()
		if err := r.updateSubnetStatusCapacity(subnet, prefixSize); err != nil {
			return false, err
		}

	} else {
		log.Info("Subnet does not has Parent")
		log.Info("validate cidr")
		_, ipnetSubnet, _ := validateCidr(subnet.Spec.CIDR)

		// get the subnet cidr ranges from the same level of the subtree
		log.Info("get subtree level cidr ranges")
		cidrnets, err := r.getSubtreeLevelCidrRanges(subnet)
		if err != nil {
			return false, err
		}

		// check if cidrnets is empty, than you dont have to validate overlap
		if cidrnets != nil {
			// verifies that none of cidr blocks overlap
			log.Info("validate cidr overlap")
			err = validateCidrOverlap(ipnetSubnet, cidrnets)
			if err != nil {
				return false, err
			}
		}

		// update subnet status
		log.Info("update subnet status")
		prefixSize, _ := ipnetSubnet.Mask.Size()
		if err := r.updateSubnetStatusCapacity(subnet, prefixSize); err != nil {
			return false, err
		}
	}

	return true, nil
}

func checkParentCidrRange(ipSubnet net.IP, ipnetSubnetParent *net.IPNet) error {
	if !ipnetSubnetParent.Contains(ipSubnet) {
		return errors.New("Subnet CIDR range does not fit in parent subnet CIDR range")
	}
	return nil
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
	opts := []client.ListOption{
		client.InNamespace(subnet.Namespace),
	}
	for key, value := range labels {
		opts = []client.ListOption{
			client.MatchingLabels{key: value},
		}
	}
	// TODO
	// What happens when list is empty?
	if err := r.List(context.Background(), subnetList, opts...); err != nil {
		r.Log.Info("List is empty")
		return cidrnets, err
	}

	// TODO
	// when subnet list only has one elment, than there are no siblings
	if len(subnetList.Items) == 1 {
		r.Log.Info("Subnet does not has siblings")
		return cidrnets, nil
	}

	// get each cidr range from the siblings and transform them to IPNet
	for _, sn := range subnetList.Items {

		// don't add yourself
		if sn.Name != subnet.Name {
			_, ipnet, err := validateCidr(sn.Spec.CIDR)
			if err != nil {
				return cidrnets, err
			}
			cidrnets = append(cidrnets, ipnet)
		}

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

func (r *SubnetReconciler) updateSubnetStatusCapacity(subnet *corev1.Subnet, prefixSize int) error {
	ctx := context.Background()
	//subnet := &v1.Subnet{}
	//err := r.Get(ctx, r.Request.NamespacedName, subnet)
	//if err != nil {
	//	r.Log.Error(err, "not found")
	//	return client.IgnoreNotFound(err)
	//}

	clone := subnet.DeepCopy()

	// TODO
	// differentiate between ipv4 and ipv6
	// remove magic numbers
	var capacity int
	if subnet.Spec.Type == "IPv4" {
		capacity = 32 - prefixSize
		r.Log.Info(fmt.Sprintf("Capacity IPv4: %d", capacity))
	} else {
		capacity = 128 - prefixSize
	}

	capacity = powInt(2, capacity)

	clone.Status.Capacity = capacity

	err := r.Patch(ctx, clone, client.MergeFrom(subnet))
	if err != nil {
		return client.IgnoreNotFound(err)
	}

	r.Subnet = clone
	return nil
}

func (r *SubnetReconciler) updateSubnetStatusCapacityLeft(msgs []string) error {
	ctx := context.Background()
	subnet := &v1.Subnet{}
	err := r.Get(ctx, r.Request.NamespacedName, subnet)
	if err != nil {
		return client.IgnoreNotFound(err)
	}

	clone := subnet.DeepCopy()
	clone.Status.Messages = msgs

	err = r.Patch(ctx, clone, client.MergeFrom(subnet))
	if err != nil {
		return client.IgnoreNotFound(err)
	}

	r.Subnet = clone
	return nil
}

func powInt(x, y int) int {
	return int(math.Pow(float64(x), float64(y)))
}

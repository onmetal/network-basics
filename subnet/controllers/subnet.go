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
	ctx := context.Background()
	subnet, ok := obj.(*corev1.Subnet)
	if !ok {
		return false, errors.New("not a subnet object")
	}
	log := r.Log.WithValues("checkIPIntegrity", fmt.Sprintf("Subnet: %s ,cidr: %s", subnet.Spec.ID, subnet.Spec.CIDR))

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
	clone := subnet.DeepCopy()
	var cidrnets []*net.IPNet

	// get the path of the subnet to root and delete own entry
	labels := clone.GetLabels()
	r.Log.Info(fmt.Sprint("Labels: ", labels))
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

	if err := r.List(context.Background(), subnetList, opts...); err != nil {
		r.Log.Info("List is empty")
		return cidrnets, err
	}

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

func (r *SubnetReconciler) updateSubnetStatusCapacity() error {
	ctx := context.Background()
	subnet := &v1.Subnet{}
	err := r.Get(ctx, r.Request.NamespacedName, subnet)
	if err != nil {
		return client.IgnoreNotFound(err)
	}
	clone := subnet.DeepCopy()
	prefixSize := getMaskSize(subnet.Spec.CIDR)

	// TODO
	// remove magic numbers
	var capacity int
	if subnet.Spec.Type == "IPv4" {
		capacity = 32 - prefixSize
	} else if subnet.Spec.Type == "IPv6" {
		// 2^63 is the highest value for integer
		if prefixSize < 63 {
			capacity = 128 - prefixSize
		} else {
			return nil
		}

	} else {
		return errors.New("the spec field type is invalid")
	}

	capacity = powInt(2, capacity)
	clone.Status.Capacity = capacity

	err = r.Patch(ctx, clone, client.MergeFrom(subnet))
	if err != nil {
		return client.IgnoreNotFound(err)
	}

	r.Subnet = clone
	return nil
}

func (r *SubnetReconciler) updateSubnetStatusCapacityLeft() error {
	ctx := context.Background()
	subnet := &v1.Subnet{}
	err := r.Get(ctx, r.Request.NamespacedName, subnet)
	if err != nil {
		return client.IgnoreNotFound(err)
	}
	clone := subnet.DeepCopy()

	subnetParent := &corev1.Subnet{}
	labels := subnet.GetLabels()
	delete(labels, subnet.Spec.NetworkGlobalID+LabelTreeDepthSuffix)
	r.Log.Info(fmt.Sprint("Labels: ", labels))

	for key, _ := range labels {
		subnetName := getName(key)

		if subnetName == subnet.Name || len(labels) == 1 {
			clone.Status.CapacityLeft = subnet.Status.Capacity
			r.Log.Info(fmt.Sprintf("Capacity from %s: %d", subnetName, clone.Status.CapacityLeft))

			err := r.Patch(ctx, clone, client.MergeFrom(subnet))
			if err != nil {
				return client.IgnoreNotFound(err)
			}

		} else {
			if err := r.Get(ctx, types.NamespacedName{Name: subnetName, Namespace: subnet.Namespace}, subnetParent); err != nil {
				err = errors.New(fmt.Sprintf("Subnet CapacityLeft cannot be updated because parent Subnet %s not found", subnetName))
				return err
			}

			cloneParent := subnetParent.DeepCopy()
			cloneParent.Status.CapacityLeft = subnetParent.Status.Capacity - subnet.Status.Capacity

			err := r.Patch(ctx, cloneParent, client.MergeFrom(subnetParent))
			if err != nil {
				return client.IgnoreNotFound(err)
			}
		}
	}
	return nil
}

func powInt(x, y int) int {
	return int(math.Pow(float64(x), float64(y)))
}

func getName(name string) string {
	return name[0 : len(name)-len(LabelTreeDepthSuffix)]
}

func getMaskSize(ip string) int {
	var size int
	_, ipnet, _ := net.ParseCIDR(ip)
	size, _ = ipnet.Mask.Size()
	return size
}

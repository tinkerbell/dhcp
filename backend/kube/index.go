package kube

import (
	"github.com/tinkerbell/tink/api/v1alpha1"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

// HardwareByMACAddr is an index used with a controller-runtime client to lookup hardware by MAC.
const HardwareByMACAddr = ".Spec.NetworkInterfaces.MAC"

// HardwareByMACAddrFunc returns a list of MAC addresses for a Hardware object.
func HardwareByMACAddrFunc(obj client.Object) []string {
	hw, ok := obj.(*v1alpha1.Hardware)
	if !ok {
		return nil
	}
	return GetMACs(hw)
}

// GetMACs retrieves all MACs associated with h.
func GetMACs(h *v1alpha1.Hardware) []string {
	var macs []string
	for _, i := range h.Spec.Interfaces {
		for _, m := range i.DHCP.MAC {
			macs = append(macs, string(m))
		}
	}
	return macs
}

// HardwareByIPAddr is an index used with a controller-runtime client to lookup hardware by IP.
const HardwareByIPAddr = ".Spec.NetworkInterfaces.DHCP.IP"

// HardwareByIPAddrFunc returns a list of IP addresses for a Hardware object.
func HardwareByIPAddrFunc(obj client.Object) []string {
	hw, ok := obj.(*v1alpha1.Hardware)
	if !ok {
		return nil
	}
	return GetIPs(hw)
}

// GetIPs retrieves all IP addresses.
func GetIPs(h *v1alpha1.Hardware) []string {
	var ips []string
	for _, i := range h.Spec.Interfaces {
		if i.DHCP != nil {
			ips = append(ips, i.DHCP.IP.Address)
		}
	}
	return ips
}

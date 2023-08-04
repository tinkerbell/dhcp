package kube

import (
	"testing"

	"github.com/google/go-cmp/cmp"
	"github.com/tinkerbell/tink/api/v1alpha1"
)

func TestMACAddrs(t *testing.T) {
	hw := &v1alpha1.Hardware{
		Spec: v1alpha1.HardwareSpec{
			Interfaces: []v1alpha1.Interface{
				{
					DHCP: &v1alpha1.DHCP{
						MAC: "00:00:00:00:00:00",
					},
				},
				{
					DHCP: &v1alpha1.DHCP{
						MAC: "00:00:00:00:00:01",
					},
				},
				{
					DHCP: &v1alpha1.DHCP{},
				},
			},
		},
	}
	macs := MACAddrs(hw)
	if diff := cmp.Diff([]string{"00:00:00:00:00:00", "00:00:00:00:00:01"}, macs); diff != "" {
		t.Errorf("unexpected MACs (-want +got):\n%s", diff)
	}
}

func TestIPAddrs(t *testing.T) {
	hw := &v1alpha1.Hardware{
		Spec: v1alpha1.HardwareSpec{
			Interfaces: []v1alpha1.Interface{
				{
					DHCP: &v1alpha1.DHCP{
						IP: &v1alpha1.IP{
							Address: "192.168.2.1",
						},
					},
				},
				{
					DHCP: &v1alpha1.DHCP{
						IP: &v1alpha1.IP{
							Address: "192.168.2.2",
						},
					},
				},
				{
					DHCP: &v1alpha1.DHCP{},
				},
				{
					DHCP: &v1alpha1.DHCP{
						IP: &v1alpha1.IP{},
					},
				},
			},
		},
	}
	ips := IPAddrs(hw)
	if diff := cmp.Diff([]string{"192.168.2.1", "192.168.2.2"}, ips); diff != "" {
		t.Errorf("unexpected IPs (-want +got):\n%s", diff)
	}
}

package tink

import (
	"context"
	"errors"
	"fmt"
	"net"
	"net/url"
	"testing"

	"github.com/go-logr/logr"
	"github.com/google/go-cmp/cmp"
	"github.com/tinkerbell/dhcp/data"
	"github.com/tinkerbell/tink/protos/hardware"
	"google.golang.org/grpc"
	"inet.af/netaddr"
)

var errMacNotFound = fmt.Errorf("mac not found")

type hardwareServerMock struct {
	mac      string
	ipAddr   string
	subnet   string
	allowPXE bool
	ipxeURL  string
	err      error
}

func (h *hardwareServerMock) getMockedHardwareServiceClient() *hardware.HardwareServiceClientMock {
	hardwareSvcClient := &hardware.HardwareServiceClientMock{
		ByMACFunc: func(ctx context.Context, in *hardware.GetRequest, opts ...grpc.CallOption) (*hardware.Hardware, error) {
			if h.err != nil {
				fmt.Printf("in mock: %T\n", h.err)
				return nil, h.err
			}
			return &hardware.Hardware{Network: &hardware.Hardware_Network{
				Interfaces: []*hardware.Hardware_Network_Interface{
					{
						Dhcp:    &hardware.Hardware_DHCP{Mac: h.mac, Ip: &hardware.Hardware_DHCP_IP{Address: h.ipAddr, Netmask: h.subnet}},
						Netboot: &hardware.Hardware_Netboot{AllowPxe: h.allowPXE, Ipxe: &hardware.Hardware_Netboot_IPXE{Url: h.ipxeURL}},
					},
				},
			}}, nil
		},
	}

	return hardwareSvcClient
}

func TestRead(t *testing.T) {
	tests := map[string]struct {
		input       net.HardwareAddr
		mock        hardwareServerMock
		wantDHCP    *data.DHCP
		wantNetboot *data.Netboot
		err         error
	}{
		"success": {
			input: net.HardwareAddr{0x00, 0x00, 0x00, 0x00, 0x00, 0x00},
			mock: hardwareServerMock{
				mac:      "00:00:00:00:00:00",
				ipAddr:   "192.168.2.190",
				subnet:   "255.255.255.0",
				allowPXE: true,
				ipxeURL:  "http://net.boot.xyz",
			},
			wantDHCP:    &data.DHCP{MACAddress: net.HardwareAddr{0x00, 0x00, 0x00, 0x00, 0x00, 0x00}, IPAddress: netaddr.IPv4(192, 168, 2, 190), SubnetMask: net.IPv4Mask(255, 255, 255, 0)},
			wantNetboot: &data.Netboot{AllowNetboot: true, IPXEScriptURL: &url.URL{Scheme: "http", Host: "net.boot.xyz"}},
		},
		"error mac not found": {
			mock: hardwareServerMock{err: errMacNotFound},
			err:  errMacNotFound,
		},
		"error mac from tink does match input": {
			input: net.HardwareAddr{0x00, 0x00, 0x00, 0x00, 0x00, 0x01},
			mock:  hardwareServerMock{mac: "00:00:00:00:00:00"},
			err:   errRecordNotFound,
		},
		"error with translate": {
			mock: hardwareServerMock{},
			err:  errParseIP,
		},
	}

	for name, tt := range tests {
		t.Run(name, func(t *testing.T) {
			s := &Config{Log: logr.Discard(), Client: tt.mock.getMockedHardwareServiceClient()}
			d, n, err := s.Read(context.Background(), tt.input)
			if !errors.Is(err, tt.err) {
				t.Fatalf("want: %v, type: %[1]T, got: %v, type: %[1]T", tt.err, err)
			}
			if diff := cmp.Diff(d, tt.wantDHCP, comparers()...); diff != "" {
				t.Errorf(diff)
			}
			if diff := cmp.Diff(n, tt.wantNetboot); diff != "" {
				t.Errorf(diff)
			}
		})
	}
}

func comparers() []cmp.Option {
	return []cmp.Option{
		cmp.Comparer(func(x, y netaddr.IP) bool { return x.Compare(y) == 0 }),
	}
}

func TestTranslate(t *testing.T) {
	tests := map[string]struct {
		inputDHCP    *hardware.Hardware_DHCP
		inputNetboot *hardware.Hardware_Netboot
		wantDHCP     *data.DHCP
		wantNetboot  *data.Netboot
		err          error
	}{
		"failure bad IP address":  {err: errParseIP},
		"failure bad mac address": {inputDHCP: &hardware.Hardware_DHCP{Ip: &hardware.Hardware_DHCP_IP{Address: "192.168.3.8"}}, err: &net.ParseError{}},
		"failure bad subnet mask": {inputDHCP: &hardware.Hardware_DHCP{Mac: "00:00:00:00:00:00", Ip: &hardware.Hardware_DHCP_IP{Address: "192.168.3.8"}}, err: errParseSubnet},
		"success, but bad input gateway": {
			inputDHCP:   &hardware.Hardware_DHCP{Mac: "00:00:00:00:00:00", Ip: &hardware.Hardware_DHCP_IP{Address: "192.168.3.8", Netmask: "255.255.255.0", Gateway: "bad"}},
			wantDHCP:    &data.DHCP{MACAddress: net.HardwareAddr{0x00, 0x00, 0x00, 0x00, 0x00, 0x00}, IPAddress: netaddr.IPv4(192, 168, 3, 8), SubnetMask: net.IPv4Mask(255, 255, 255, 0)},
			wantNetboot: &data.Netboot{},
		},
		"success, good gateway": {
			inputDHCP:   &hardware.Hardware_DHCP{Mac: "00:00:00:00:00:00", Ip: &hardware.Hardware_DHCP_IP{Address: "192.168.3.8", Netmask: "255.255.255.0", Gateway: "192.168.3.1"}},
			wantDHCP:    &data.DHCP{MACAddress: net.HardwareAddr{0x00, 0x00, 0x00, 0x00, 0x00, 0x00}, IPAddress: netaddr.IPv4(192, 168, 3, 8), SubnetMask: net.IPv4Mask(255, 255, 255, 0), DefaultGateway: netaddr.IPv4(192, 168, 3, 1)},
			wantNetboot: &data.Netboot{},
		},
		"success, with ntp and dns servers": {
			inputDHCP: &hardware.Hardware_DHCP{
				Mac:         "00:00:00:00:00:00",
				NameServers: []string{"1.1.1.1"},
				TimeServers: []string{"39.38.37.36"},
				Ip:          &hardware.Hardware_DHCP_IP{Address: "192.168.3.8", Netmask: "255.255.255.0", Gateway: "192.168.3.1"},
			},
			wantDHCP: &data.DHCP{
				MACAddress:     net.HardwareAddr{0x00, 0x00, 0x00, 0x00, 0x00, 0x00},
				IPAddress:      netaddr.IPv4(192, 168, 3, 8),
				SubnetMask:     net.IPv4Mask(255, 255, 255, 0),
				DefaultGateway: netaddr.IPv4(192, 168, 3, 1),
				NameServers:    []net.IP{{1, 1, 1, 1}},
				NTPServers:     []net.IP{{39, 38, 37, 36}},
			},
			wantNetboot: &data.Netboot{},
		},
		"success, with one bad ntp and dns server": {
			inputDHCP: &hardware.Hardware_DHCP{
				Mac:         "00:00:00:00:00:00",
				NameServers: []string{"1.1.1.1", "bad"},
				TimeServers: []string{"39.38.37.36", "bad"},
				Ip:          &hardware.Hardware_DHCP_IP{Address: "192.168.3.8", Netmask: "255.255.255.0", Gateway: "192.168.3.1"},
			},
			wantDHCP: &data.DHCP{
				MACAddress:     net.HardwareAddr{0x00, 0x00, 0x00, 0x00, 0x00, 0x00},
				IPAddress:      netaddr.IPv4(192, 168, 3, 8),
				SubnetMask:     net.IPv4Mask(255, 255, 255, 0),
				DefaultGateway: netaddr.IPv4(192, 168, 3, 1),
				NameServers:    []net.IP{{1, 1, 1, 1}},
				NTPServers:     []net.IP{{39, 38, 37, 36}},
			},
			wantNetboot: &data.Netboot{},
		},
		"success, with ipxe URL": {
			inputDHCP:    &hardware.Hardware_DHCP{Mac: "00:00:00:00:00:00", Ip: &hardware.Hardware_DHCP_IP{Address: "192.168.3.8", Netmask: "255.255.255.0"}},
			inputNetboot: &hardware.Hardware_Netboot{Ipxe: &hardware.Hardware_Netboot_IPXE{Url: "http://net.boot.xyz"}},
			wantDHCP:     &data.DHCP{MACAddress: net.HardwareAddr{0x00, 0x00, 0x00, 0x00, 0x00, 0x00}, IPAddress: netaddr.IPv4(192, 168, 3, 8), SubnetMask: net.IPv4Mask(255, 255, 255, 0)},
			wantNetboot:  &data.Netboot{IPXEScriptURL: &url.URL{Scheme: "http", Host: "net.boot.xyz"}},
		},
		"failure, bad ipxe URL": {
			inputDHCP:    &hardware.Hardware_DHCP{Mac: "00:00:00:00:00:00", Ip: &hardware.Hardware_DHCP_IP{Address: "192.168.3.8", Netmask: "255.255.255.0"}},
			inputNetboot: &hardware.Hardware_Netboot{Ipxe: &hardware.Hardware_Netboot_IPXE{Url: ":bad"}},
			err:          errParseURL,
		},
	}
	for name, tt := range tests {
		t.Run(name, func(t *testing.T) {
			s := &Config{Log: logr.Discard()}
			gotDHCP, gotNetboot, err := s.translate(tt.inputDHCP, tt.inputNetboot)
			if !errors.Is(err, tt.err) && !errors.As(err, &tt.err) {
				t.Fatalf("want: %v, type: %[1]T, got: %v, type: %[1]T", tt.err, err)
			}
			if diff := cmp.Diff(gotDHCP, tt.wantDHCP, comparers()...); diff != "" {
				t.Errorf(diff)
			}
			if diff := cmp.Diff(gotNetboot, tt.wantNetboot); diff != "" {
				t.Errorf(diff)
			}
		})
	}
}

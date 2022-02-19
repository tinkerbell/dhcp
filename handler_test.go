package dhcp

import (
	"bytes"
	"context"
	"fmt"
	"log"
	"net"
	"net/url"
	"testing"
	"time"

	"github.com/go-logr/logr"
	"github.com/go-logr/stdr"
	"github.com/google/go-cmp/cmp"
	"github.com/insomniacslk/dhcp/dhcpv4"
	"github.com/insomniacslk/dhcp/iana"
	"github.com/insomniacslk/dhcp/rfc1035label"
	"github.com/tinkerbell/dhcp/data"
	"go.opentelemetry.io/otel"
	"golang.org/x/net/nettest"
	"inet.af/netaddr"
)

type mockBackend struct {
	err          error
	allowNetboot bool
	ipxeScript   *url.URL
}

func (m *mockBackend) Read(context.Context, net.HardwareAddr) (*data.Dhcp, *data.Netboot, error) {
	d := &data.Dhcp{
		MacAddress:     []byte{0x01, 0x02, 0x03, 0x04, 0x05, 0x06},
		IPAddress:      netaddr.IPv4(192, 168, 1, 100),
		SubnetMask:     []byte{255, 255, 255, 0},
		DefaultGateway: netaddr.IPv4(192, 168, 1, 1),
		NameServers: []net.IP{
			{1, 1, 1, 1},
		},
		Hostname:         "test-host",
		DomainName:       "mydomain.com",
		BroadcastAddress: netaddr.IPv4(192, 168, 1, 255),
		NTPServers: []net.IP{
			{132, 163, 96, 2},
		},
		LeaseTime: 60,
		DomainSearch: []string{
			"mydomain.com",
		},
	}
	n := &data.Netboot{
		AllowNetboot:  m.allowNetboot,
		IpxeScriptURL: m.ipxeScript,
	}
	return d, n, m.err
}

func TestHandleDiscover(t *testing.T) {
	type args struct {
		m *dhcpv4.DHCPv4
	}
	tests := map[string]struct {
		server Server
		args   args
		want   *dhcpv4.DHCPv4
	}{
		"success": {
			server: Server{
				Log:      logr.Discard(),
				Backend:  &mockBackend{},
				Listener: netaddr.IPPortFrom(netaddr.IPv4(192, 168, 1, 1), 67),
				IPAddr:   netaddr.IPv4(192, 168, 1, 1),
			},
			args: args{
				m: &dhcpv4.DHCPv4{
					ClientHWAddr: []byte{0x01, 0x02, 0x03, 0x04, 0x05, 0x06},
				},
			},
			want: &dhcpv4.DHCPv4{
				OpCode:       dhcpv4.OpcodeBootRequest,
				ClientHWAddr: []byte{0x01, 0x02, 0x03, 0x04, 0x05, 0x06},
				YourIPAddr:   []byte{192, 168, 1, 100},
				ServerIPAddr: []byte{192, 168, 1, 1},
				ClientIPAddr: []byte{0, 0, 0, 0},
				Options: dhcpv4.OptionsFromList(
					dhcpv4.OptSubnetMask(net.IPMask{255, 255, 255, 0}),
					dhcpv4.OptDomainName("mydomain.com"),
					dhcpv4.OptHostName("test-host"),
					dhcpv4.OptRouter(net.IP{192, 168, 1, 1}),
					dhcpv4.OptDNS([]net.IP{
						{1, 1, 1, 1},
					}...),
					dhcpv4.OptBroadcastAddress(net.IP{192, 168, 1, 255}),
					dhcpv4.OptMessageType(dhcpv4.MessageTypeOffer),
					dhcpv4.OptServerIdentifier(net.IP{192, 168, 1, 1}),
					dhcpv4.OptNTPServers([]net.IP{
						{132, 163, 96, 2},
					}...),
					dhcpv4.OptIPAddressLeaseTime(time.Minute),
					dhcpv4.OptDomainSearch(&rfc1035label.Labels{
						Labels: []string{"mydomain.com"},
					}),
				),
			},
		},
		"success with netboot options": {
			server: Server{
				Log: logr.Discard(),
				Backend: &mockBackend{
					allowNetboot: true,
					ipxeScript:   &url.URL{Scheme: "http", Host: "localhost:8181", Path: "/01:02:03:04:05:06/auto.ipxe"},
				},
				Listener:       netaddr.IPPortFrom(netaddr.IPv4(192, 168, 1, 1), 67),
				IPAddr:         netaddr.IPv4(192, 168, 1, 1),
				IPXEScriptURL:  &url.URL{Scheme: "http", Host: "localhost:8181", Path: "/01:02:03:04:05:06/auto.ipxe"},
				NetbootEnabled: true,
			},
			args: args{
				m: &dhcpv4.DHCPv4{
					ClientHWAddr: []byte{0x01, 0x02, 0x03, 0x04, 0x05, 0x06},
					Options: dhcpv4.OptionsFromList(
						dhcpv4.OptUserClass("Tinkerbell"),
						dhcpv4.OptClassIdentifier("HTTPClient:Arch:xxxxx:UNDI:yyyzzz"),
						dhcpv4.OptClientArch(iana.EFI_ARM64_HTTP),
						dhcpv4.OptGeneric(dhcpv4.OptionClientNetworkInterfaceIdentifier, []byte{0x01, 0x02, 0x03, 0x04, 0x05, 0x06}),
						dhcpv4.OptGeneric(dhcpv4.OptionClientMachineIdentifier, []byte{0x00, 0x02, 0x03, 0x04, 0x05, 0x06, 0x00, 0x02, 0x03, 0x04, 0x05, 0x06, 0x00, 0x02, 0x03, 0x04, 0x05}),
						dhcpv4.OptMessageType(dhcpv4.MessageTypeDiscover),
					),
				},
			},
			want: &dhcpv4.DHCPv4{
				OpCode:       dhcpv4.OpcodeBootRequest,
				ClientHWAddr: []byte{0x01, 0x02, 0x03, 0x04, 0x05, 0x06},
				YourIPAddr:   []byte{192, 168, 1, 100},
				ClientIPAddr: []byte{0, 0, 0, 0},
				BootFileName: "http://localhost:8181/01:02:03:04:05:06/auto.ipxe",
				Options: dhcpv4.OptionsFromList(
					dhcpv4.OptSubnetMask(net.IPMask{255, 255, 255, 0}),
					dhcpv4.OptDomainName("mydomain.com"),
					dhcpv4.OptHostName("test-host"),
					dhcpv4.OptRouter(net.IP{192, 168, 1, 1}),
					dhcpv4.OptDNS([]net.IP{
						{1, 1, 1, 1},
					}...),
					dhcpv4.OptBroadcastAddress(net.IP{192, 168, 1, 255}),
					dhcpv4.OptMessageType(dhcpv4.MessageTypeOffer),
					dhcpv4.OptServerIdentifier(net.IP{192, 168, 1, 1}),
					dhcpv4.OptNTPServers([]net.IP{
						{132, 163, 96, 2},
					}...),
					dhcpv4.OptIPAddressLeaseTime(time.Minute),
					dhcpv4.OptDomainSearch(&rfc1035label.Labels{
						Labels: []string{"mydomain.com"},
					}),
					dhcpv4.OptClassIdentifier("HTTPClient"),
					dhcpv4.OptGeneric(dhcpv4.OptionVendorSpecificInformation, dhcpv4.Options{
						6:  []byte{8},
						69: binaryTpFromContext(context.Background()),
					}.ToBytes()),
				),
			},
		},
		"fail backend error": {
			server: Server{
				Log:     logr.Discard(),
				Backend: &mockBackend{err: fmt.Errorf("test error")},
			},
			args: args{
				m: &dhcpv4.DHCPv4{},
			},
			want: nil,
		},
	}
	for name, tt := range tests {
		t.Run(name, func(t *testing.T) {
			s := tt.server
			got := s.handleDiscover(context.Background(), otel.Tracer("DHCP"), tt.args.m)
			if diff := cmp.Diff(got, tt.want); diff != "" {
				t.Fatal(diff)
			}
		})
	}
}

func TestHandleRequest(t *testing.T) {
	type args struct {
		m *dhcpv4.DHCPv4
	}
	tests := map[string]struct {
		server Server
		args   args
		want   *dhcpv4.DHCPv4
	}{
		"success": {
			server: Server{
				Log:      logr.Discard(),
				Backend:  &mockBackend{},
				Listener: netaddr.IPPortFrom(netaddr.IPv4(192, 168, 1, 1), 67),
				IPAddr:   netaddr.IPv4(192, 168, 1, 1),
			},
			args: args{
				m: &dhcpv4.DHCPv4{
					ClientHWAddr: []byte{0x01, 0x02, 0x03, 0x04, 0x05, 0x06},
				},
			},
			want: &dhcpv4.DHCPv4{
				OpCode:       dhcpv4.OpcodeBootRequest,
				ClientHWAddr: []byte{0x01, 0x02, 0x03, 0x04, 0x05, 0x06},
				YourIPAddr:   []byte{192, 168, 1, 100},
				ServerIPAddr: []byte{192, 168, 1, 1},
				ClientIPAddr: []byte{0, 0, 0, 0},
				Options: dhcpv4.OptionsFromList(
					dhcpv4.OptSubnetMask(net.IPMask{255, 255, 255, 0}),
					dhcpv4.OptDomainName("mydomain.com"),
					dhcpv4.OptHostName("test-host"),
					dhcpv4.OptRouter(net.IP{192, 168, 1, 1}),
					dhcpv4.OptDNS([]net.IP{
						{1, 1, 1, 1},
					}...),
					dhcpv4.OptBroadcastAddress(net.IP{192, 168, 1, 255}),
					dhcpv4.OptMessageType(dhcpv4.MessageTypeAck),
					dhcpv4.OptServerIdentifier(net.IP{192, 168, 1, 1}),
					dhcpv4.OptNTPServers([]net.IP{
						{132, 163, 96, 2},
					}...),
					dhcpv4.OptIPAddressLeaseTime(time.Minute),
					dhcpv4.OptDomainSearch(&rfc1035label.Labels{
						Labels: []string{"mydomain.com"},
					}),
				),
			},
		},
		"success with netboot options": {
			server: Server{
				Log: logr.Discard(),
				Backend: &mockBackend{
					allowNetboot: true,
					ipxeScript:   &url.URL{Scheme: "http", Host: "localhost:8181", Path: "/01:02:03:04:05:06/auto.ipxe"},
				},
				Listener:       netaddr.IPPortFrom(netaddr.IPv4(192, 168, 1, 1), 67),
				IPAddr:         netaddr.IPv4(192, 168, 1, 1),
				IPXEScriptURL:  &url.URL{Scheme: "http", Host: "localhost:8181", Path: "/01:02:03:04:05:06/auto.ipxe"},
				NetbootEnabled: true,
			},
			args: args{
				m: &dhcpv4.DHCPv4{
					ClientHWAddr: []byte{0x01, 0x02, 0x03, 0x04, 0x05, 0x06},
					Options: dhcpv4.OptionsFromList(
						dhcpv4.OptUserClass("Tinkerbell"),
						dhcpv4.OptClassIdentifier("HTTPClient"),
						dhcpv4.OptClientArch(iana.EFI_ARM64_HTTP),
						dhcpv4.OptGeneric(dhcpv4.OptionClientNetworkInterfaceIdentifier, []byte{0x01, 0x02, 0x03, 0x04, 0x05, 0x06}),
						dhcpv4.OptGeneric(dhcpv4.OptionClientMachineIdentifier, []byte{0x00, 0x02, 0x03, 0x04, 0x05, 0x06, 0x00, 0x02, 0x03, 0x04, 0x05, 0x06, 0x00, 0x02, 0x03, 0x04, 0x05}),
						dhcpv4.OptMessageType(dhcpv4.MessageTypeRequest),
					),
				},
			},
			want: &dhcpv4.DHCPv4{
				OpCode:       dhcpv4.OpcodeBootRequest,
				ClientHWAddr: []byte{0x01, 0x02, 0x03, 0x04, 0x05, 0x06},
				YourIPAddr:   []byte{192, 168, 1, 100},
				ClientIPAddr: []byte{0, 0, 0, 0},
				BootFileName: "http://localhost:8181/01:02:03:04:05:06/auto.ipxe",
				Options: dhcpv4.OptionsFromList(
					dhcpv4.OptSubnetMask(net.IPMask{255, 255, 255, 0}),
					dhcpv4.OptDomainName("mydomain.com"),
					dhcpv4.OptHostName("test-host"),
					dhcpv4.OptRouter(net.IP{192, 168, 1, 1}),
					dhcpv4.OptDNS([]net.IP{
						{1, 1, 1, 1},
					}...),
					dhcpv4.OptBroadcastAddress(net.IP{192, 168, 1, 255}),
					dhcpv4.OptMessageType(dhcpv4.MessageTypeAck),
					dhcpv4.OptServerIdentifier(net.IP{192, 168, 1, 1}),
					dhcpv4.OptNTPServers([]net.IP{
						{132, 163, 96, 2},
					}...),
					dhcpv4.OptIPAddressLeaseTime(time.Minute),
					dhcpv4.OptDomainSearch(&rfc1035label.Labels{
						Labels: []string{"mydomain.com"},
					}),
					dhcpv4.OptClassIdentifier("HTTPClient"),
					dhcpv4.OptGeneric(dhcpv4.OptionVendorSpecificInformation, dhcpv4.Options{
						6:  []byte{8},
						69: binaryTpFromContext(context.Background()),
					}.ToBytes()),
				),
			},
		},
		"fail backend error": {
			server: Server{
				Log:     logr.Discard(),
				Backend: &mockBackend{err: fmt.Errorf("test error")},
			},
			args: args{
				m: &dhcpv4.DHCPv4{},
			},
			want: nil,
		},
	}
	for name, tt := range tests {
		t.Run(name, func(t *testing.T) {
			s := tt.server
			got := s.handleRequest(context.Background(), otel.Tracer("DHCP"), tt.args.m)
			if diff := cmp.Diff(got, tt.want); diff != "" {
				t.Fatal(diff)
			}
		})
	}
}

func TestHandleRelease(t *testing.T) {
	out := &bytes.Buffer{}
	s := &Server{Log: stdr.New(log.New(out, "", 0))}
	expectedLog := `"level"=0 "msg"="received release, no response required" "mac"="01:02:03:04:05:06"`
	s.handleRelease(context.Background(), &dhcpv4.DHCPv4{ClientHWAddr: []byte{0x01, 0x02, 0x03, 0x04, 0x05, 0x06}})
	if diff := cmp.Diff(out.String(), expectedLog+"\n"); diff != "" {
		t.Fatal(diff)
	}
}

func TestHandleFunc(t *testing.T) {
	type args struct {
		peer net.Addr
		m    *dhcpv4.DHCPv4
	}
	tests := map[string]struct {
		server Server
		args   args
		out    *bytes.Buffer
		want   string
	}{
		"fail unknown DHCP message type": {
			server: Server{
				ctx:      context.Background(),
				Listener: netaddr.IPPortFrom(netaddr.IPv4(127, 0, 0, 1), 67),
				IPAddr:   netaddr.IPv4(127, 0, 0, 1),
			},
			args: args{
				peer: &net.UDPAddr{IP: net.IP{127, 0, 0, 1}, Port: 6767},
				m: &dhcpv4.DHCPv4{
					OpCode:       dhcpv4.OpcodeBootRequest,
					ClientHWAddr: []byte{0x01, 0x02, 0x03, 0x04, 0x05, 0x06},
					Options: dhcpv4.OptionsFromList(
						dhcpv4.OptMessageType(dhcpv4.MessageTypeInform),
					),
				},
			},
			out:  &bytes.Buffer{},
			want: `"level"=0 "msg"="received unknown message type" "mac"="01:02:03:04:05:06" "type"="INFORM"` + "\n",
		},
		"success discover message type": {
			server: Server{
				ctx:     context.Background(),
				Backend: &mockBackend{},
			},
			args: args{
				peer: &net.UDPAddr{IP: net.IP{127, 0, 0, 1}, Port: 6767},
				m: &dhcpv4.DHCPv4{
					OpCode:       dhcpv4.OpcodeBootRequest,
					ClientHWAddr: []byte{0x01, 0x02, 0x03, 0x04, 0x05, 0x06},
					Options: dhcpv4.OptionsFromList(
						dhcpv4.OptMessageType(dhcpv4.MessageTypeDiscover),
					),
				},
			},
			want: `"level"=0 "msg"="received discover packet" "mac"="01:02:03:04:05:06"` + "\n" + `"level"=0 "msg"="sending offer packet" "mac"="01:02:03:04:05:06"` + "\n" + `"level"=0 "msg"="sent DHCP response" "mac"="01:02:03:04:05:06"` + "\n",
			out:  &bytes.Buffer{},
		},
		"success request message type": {
			server: Server{
				ctx:     context.Background(),
				Backend: &mockBackend{},
			},
			args: args{
				peer: &net.UDPAddr{IP: net.IP{127, 0, 0, 1}, Port: 6767},
				m: &dhcpv4.DHCPv4{
					OpCode:       dhcpv4.OpcodeBootRequest,
					ClientHWAddr: []byte{0x01, 0x02, 0x03, 0x04, 0x05, 0x06},
					Options: dhcpv4.OptionsFromList(
						dhcpv4.OptMessageType(dhcpv4.MessageTypeRequest),
					),
				},
			},
			want: `"level"=0 "msg"="received request packet" "mac"="01:02:03:04:05:06"` + "\n" + `"level"=0 "msg"="sending ack packet" "mac"="01:02:03:04:05:06"` + "\n" + `"level"=0 "msg"="sent DHCP response" "mac"="01:02:03:04:05:06"` + "\n",
			out:  &bytes.Buffer{},
		},
		"success release message type": {
			server: Server{
				ctx:     context.Background(),
				Backend: &mockBackend{},
			},
			args: args{
				peer: &net.UDPAddr{IP: net.IP{127, 0, 0, 1}, Port: 6767},
				m: &dhcpv4.DHCPv4{
					OpCode:       dhcpv4.OpcodeBootRequest,
					ClientHWAddr: []byte{0x01, 0x02, 0x03, 0x04, 0x05, 0x06},
					Options: dhcpv4.OptionsFromList(
						dhcpv4.OptMessageType(dhcpv4.MessageTypeRelease),
					),
				},
			},
			want: `"level"=0 "msg"="received release, no response required" "mac"="01:02:03:04:05:06"` + "\n",
			out:  &bytes.Buffer{},
		},
		"fail replying": {
			server: Server{
				ctx:     context.Background(),
				Backend: &mockBackend{},
			},
			args: args{
				peer: nil,
				m: &dhcpv4.DHCPv4{
					OpCode:       dhcpv4.OpcodeBootRequest,
					ClientHWAddr: []byte{0x01, 0x02, 0x03, 0x04, 0x05, 0x06},
					Options: dhcpv4.OptionsFromList(
						dhcpv4.OptMessageType(dhcpv4.MessageTypeDiscover),
					),
				},
			},
			want: `"level"=0 "msg"="received discover packet" "mac"="01:02:03:04:05:06"` + "\n" + `"level"=0 "msg"="sending offer packet" "mac"="01:02:03:04:05:06"` + "\n" + `"msg"="failed to send DHCP" "error"="write udp4 127.0.0.1:%v: invalid argument" "mac"="01:02:03:04:05:06"` + "\n",
			out:  &bytes.Buffer{},
		},
	}
	for name, tt := range tests {
		t.Run(name, func(t *testing.T) {
			s := tt.server
			s.Log = stdr.New(log.New(tt.out, "", 0))
			conn, err := nettest.NewLocalPacketListener("udp")
			if err != nil {
				t.Fatal(err)
			}
			want := tt.want
			if tt.args.peer == nil {
				i, err := netaddr.ParseIPPort(conn.LocalAddr().String())
				if err != nil {
					t.Error(err)
				}
				want = fmt.Sprintf(tt.want, i.UDPAddr().Port)
			}
			s.handleFunc(conn, tt.args.peer, tt.args.m)
			if diff := cmp.Diff(tt.out.String(), want); diff != "" {
				t.Fatal(diff)
			}
		})
	}
}

func TestIsNetbootClient(t *testing.T) {
	tests := map[string]struct {
		input *dhcpv4.DHCPv4
		want  bool
	}{
		"fail invalid message type": {input: &dhcpv4.DHCPv4{Options: dhcpv4.OptionsFromList(dhcpv4.OptMessageType(dhcpv4.MessageTypeInform))}, want: false},
		"fail no opt60":             {input: &dhcpv4.DHCPv4{Options: dhcpv4.OptionsFromList(dhcpv4.OptMessageType(dhcpv4.MessageTypeDiscover))}, want: false},
		"fail bad opt60": {input: &dhcpv4.DHCPv4{Options: dhcpv4.OptionsFromList(
			dhcpv4.OptMessageType(dhcpv4.MessageTypeDiscover),
			dhcpv4.OptClassIdentifier("BadClient"),
		)}, want: false},
		"fail no opt93": {input: &dhcpv4.DHCPv4{Options: dhcpv4.OptionsFromList(
			dhcpv4.OptMessageType(dhcpv4.MessageTypeDiscover),
			dhcpv4.OptClassIdentifier("HTTPClient:Arch:xxxxx:UNDI:yyyzzz"),
		)}, want: false},
		"fail no opt94": {input: &dhcpv4.DHCPv4{Options: dhcpv4.OptionsFromList(
			dhcpv4.OptMessageType(dhcpv4.MessageTypeDiscover),
			dhcpv4.OptClassIdentifier("HTTPClient:Arch:xxxxx:UNDI:yyyzzz"),
			dhcpv4.OptClientArch(iana.EFI_ARM64_HTTP),
		)}, want: false},
		"fail invalid opt97[0] != 0": {input: &dhcpv4.DHCPv4{Options: dhcpv4.OptionsFromList(
			dhcpv4.OptMessageType(dhcpv4.MessageTypeDiscover),
			dhcpv4.OptClassIdentifier("HTTPClient:Arch:xxxxx:UNDI:yyyzzz"),
			dhcpv4.OptClientArch(iana.EFI_ARM64_HTTP),
			dhcpv4.OptGeneric(dhcpv4.OptionClientNetworkInterfaceIdentifier, []byte{0x01, 0x02, 0x03, 0x04, 0x05, 0x06}),
			dhcpv4.OptGeneric(dhcpv4.OptionClientMachineIdentifier, []byte{0x01, 0x02, 0x03, 0x04, 0x05, 0x06, 0x00, 0x02, 0x03, 0x04, 0x05, 0x06, 0x00, 0x02, 0x03, 0x04, 0x05}),
		)}, want: false},
		"fail invalid len(opt97)": {input: &dhcpv4.DHCPv4{Options: dhcpv4.OptionsFromList(
			dhcpv4.OptMessageType(dhcpv4.MessageTypeDiscover),
			dhcpv4.OptClassIdentifier("HTTPClient:Arch:xxxxx:UNDI:yyyzzz"),
			dhcpv4.OptClientArch(iana.EFI_ARM64_HTTP),
			dhcpv4.OptGeneric(dhcpv4.OptionClientNetworkInterfaceIdentifier, []byte{0x01, 0x02, 0x03, 0x04, 0x05, 0x06}),
			dhcpv4.OptGeneric(dhcpv4.OptionClientMachineIdentifier, []byte{0x01, 0x02}),
		)}, want: false},
		"success len(opt97) == 0": {input: &dhcpv4.DHCPv4{Options: dhcpv4.OptionsFromList(
			dhcpv4.OptMessageType(dhcpv4.MessageTypeDiscover),
			dhcpv4.OptClassIdentifier("HTTPClient:Arch:xxxxx:UNDI:yyyzzz"),
			dhcpv4.OptClientArch(iana.EFI_ARM64_HTTP),
			dhcpv4.OptGeneric(dhcpv4.OptionClientNetworkInterfaceIdentifier, []byte{0x01, 0x02, 0x03, 0x04, 0x05, 0x06}),
			dhcpv4.OptGeneric(dhcpv4.OptionClientMachineIdentifier, []byte{}),
		)}, want: true},
	}
	for name, tt := range tests {
		t.Run(name, func(t *testing.T) {
			s := &Server{}
			if s.isNetbootClient(tt.input) != tt.want {
				t.Errorf("isNetbootClient() = %v, want %v", !tt.want, tt.want)
			}
		})
	}
}

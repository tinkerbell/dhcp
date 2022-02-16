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
	ipxeScript   string
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
	type fields struct {
		ctx               context.Context
		Log               logr.Logger
		ListenAddr        netaddr.IPPort
		IPAddr            netaddr.IP
		IPXEBinServerTFTP netaddr.IPPort
		IPXEBinServerHTTP *url.URL
		IPXEScriptURL     *url.URL
		NetbootEnabled    bool
		UserClass         UserClass
		Backend           BackendReader
	}
	type args struct {
		m *dhcpv4.DHCPv4
	}
	tests := map[string]struct {
		fields fields
		args   args
		want   *dhcpv4.DHCPv4
	}{
		"success": {
			fields: fields{
				Log:        logr.Discard(),
				Backend:    &mockBackend{},
				ListenAddr: netaddr.IPPortFrom(netaddr.IPv4(192, 168, 1, 1), 67),
				IPAddr:     netaddr.IPv4(192, 168, 1, 1),
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
				// GatewayIPAddr: []byte{0, 0, 0, 0},
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
			fields: fields{
				Log: logr.Discard(),
				Backend: &mockBackend{
					allowNetboot: true,
					ipxeScript:   "http://localhost:8181/01:02:03:04:05:06/auto.ipxe",
				},
				ListenAddr:     netaddr.IPPortFrom(netaddr.IPv4(192, 168, 1, 1), 67),
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
					),
				},
			},
			want: &dhcpv4.DHCPv4{
				OpCode:       dhcpv4.OpcodeBootRequest,
				ClientHWAddr: []byte{0x01, 0x02, 0x03, 0x04, 0x05, 0x06},
				YourIPAddr:   []byte{192, 168, 1, 100},
				ClientIPAddr: []byte{0, 0, 0, 0},
				// GatewayIPAddr: []byte{0, 0, 0, 0},
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
			fields: fields{
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
			s := &Server{
				ctx:               tt.fields.ctx,
				Log:               tt.fields.Log,
				Listener:          tt.fields.ListenAddr,
				IPAddr:            tt.fields.IPAddr,
				IPXEBinServerTFTP: tt.fields.IPXEBinServerTFTP,
				IPXEBinServerHTTP: tt.fields.IPXEBinServerHTTP,
				IPXEScriptURL:     tt.fields.IPXEScriptURL,
				NetbootEnabled:    tt.fields.NetbootEnabled,
				UserClass:         tt.fields.UserClass,
				Backend:           tt.fields.Backend,
			}
			got := s.handleDiscover(context.Background(), otel.Tracer("DHCP"), tt.args.m)
			if diff := cmp.Diff(got, tt.want); diff != "" {
				t.Fatal(diff)
			}
		})
	}
}

func TestHandleRequest(t *testing.T) {
	type fields struct {
		ctx               context.Context
		Log               logr.Logger
		ListenAddr        netaddr.IPPort
		IPAddr            netaddr.IP
		IPXEBinServerTFTP netaddr.IPPort
		IPXEBinServerHTTP *url.URL
		IPXEScriptURL     *url.URL
		NetbootEnabled    bool
		UserClass         UserClass
		Backend           BackendReader
	}
	type args struct {
		m *dhcpv4.DHCPv4
	}
	tests := map[string]struct {
		fields fields
		args   args
		want   *dhcpv4.DHCPv4
	}{
		"success": {
			fields: fields{
				Log:        logr.Discard(),
				Backend:    &mockBackend{},
				ListenAddr: netaddr.IPPortFrom(netaddr.IPv4(192, 168, 1, 1), 67),
				IPAddr:     netaddr.IPv4(192, 168, 1, 1),
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
				// GatewayIPAddr: []byte{0, 0, 0, 0},
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
			fields: fields{
				Log: logr.Discard(),
				Backend: &mockBackend{
					allowNetboot: true,
					ipxeScript:   "http://localhost:8181/01:02:03:04:05:06/auto.ipxe",
				},
				ListenAddr:     netaddr.IPPortFrom(netaddr.IPv4(192, 168, 1, 1), 67),
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
					),
				},
			},
			want: &dhcpv4.DHCPv4{
				OpCode:       dhcpv4.OpcodeBootRequest,
				ClientHWAddr: []byte{0x01, 0x02, 0x03, 0x04, 0x05, 0x06},
				YourIPAddr:   []byte{192, 168, 1, 100},
				ClientIPAddr: []byte{0, 0, 0, 0},
				// GatewayIPAddr: []byte{0, 0, 0, 0},
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
			fields: fields{
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
			s := &Server{
				ctx:               tt.fields.ctx,
				Log:               tt.fields.Log,
				Listener:          tt.fields.ListenAddr,
				IPAddr:            tt.fields.IPAddr,
				IPXEBinServerTFTP: tt.fields.IPXEBinServerTFTP,
				IPXEBinServerHTTP: tt.fields.IPXEBinServerHTTP,
				IPXEScriptURL:     tt.fields.IPXEScriptURL,
				NetbootEnabled:    tt.fields.NetbootEnabled,
				UserClass:         tt.fields.UserClass,
				Backend:           tt.fields.Backend,
			}
			got := s.handleRequest(context.Background(), otel.Tracer("DHCP"), tt.args.m)
			if diff := cmp.Diff(got, tt.want); diff != "" {
				t.Fatal(diff)
			}
		})
	}
}

func TestHandleRelease(t *testing.T) {
	out := &bytes.Buffer{}
	s := &Server{Log: stdr.New(log.New(out, "", log.Lshortfile))}
	expectedLog := `handler.go:135: "level"=0 "msg"="received release, no response required"`
	s.handleRelease(context.Background(), &dhcpv4.DHCPv4{})
	if diff := cmp.Diff(out.String(), expectedLog+"\n"); diff != "" {
		t.Fatal(diff)
	}
}

func TestHandleFunc(t *testing.T) {
	type fields struct {
		ctx               context.Context
		Log               logr.Logger
		ListenAddr        netaddr.IPPort
		IPAddr            netaddr.IP
		IPXEBinServerTFTP netaddr.IPPort
		IPXEBinServerHTTP *url.URL
		IPXEScriptURL     *url.URL
		NetbootEnabled    bool
		UserClass         UserClass
		Backend           BackendReader
	}
	type args struct {
		peer net.Addr
		m    *dhcpv4.DHCPv4
	}
	tests := map[string]struct {
		fields fields
		args   args
		out    *bytes.Buffer
		want   string
	}{
		"fail unknown DHCP message type": {
			fields: fields{
				ctx:        context.Background(),
				ListenAddr: netaddr.IPPortFrom(netaddr.IPv4(127, 0, 0, 1), 67),
				IPAddr:     netaddr.IPv4(127, 0, 0, 1),
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
			want: `handler.go:32: "level"=0 "msg"="received unknown message type" "type"="INFORM"` + "\n",
		},
		"success discover message type": {
			fields: fields{
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
			want: `handler.go:52: "level"=0 "msg"="received discover packet"` + "\n" + `handler.go:86: "level"=0 "msg"="sending offer packet"` + "\n",
			out:  &bytes.Buffer{},
		},
		"success request message type": {
			fields: fields{
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
			want: `handler.go:93: "level"=0 "msg"="received request packet"` + "\n" + `handler.go:125: "level"=0 "msg"="sending ack packet"` + "\n",
			out:  &bytes.Buffer{},
		},
		"success release message type": {
			fields: fields{
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
			want: `handler.go:135: "level"=0 "msg"="received release, no response required"` + "\n",
			out:  &bytes.Buffer{},
		},
	}
	for name, tt := range tests {
		t.Run(name, func(t *testing.T) {
			l := stdr.New(log.New(tt.out, "", log.Lshortfile))
			s := &Server{
				ctx:               tt.fields.ctx,
				Log:               l,
				Listener:          tt.fields.ListenAddr,
				IPAddr:            tt.fields.IPAddr,
				IPXEBinServerTFTP: tt.fields.IPXEBinServerTFTP,
				IPXEBinServerHTTP: tt.fields.IPXEBinServerHTTP,
				IPXEScriptURL:     tt.fields.IPXEScriptURL,
				NetbootEnabled:    tt.fields.NetbootEnabled,
				UserClass:         tt.fields.UserClass,
				Backend:           tt.fields.Backend,
			}
			conn, err := nettest.NewLocalPacketListener("udp")
			if err != nil {
				t.Fatal(err)
			}
			s.handleFunc(conn, tt.args.peer, tt.args.m)
			if diff := cmp.Diff(tt.out.String(), tt.want); diff != "" {
				t.Log(tt.out.String())
				t.Fatal(diff)
			}
		})
	}
}

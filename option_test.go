package dhcp

import (
	"bytes"
	"context"
	"net"
	"net/url"
	"testing"
	"time"

	"github.com/go-logr/logr"
	"github.com/google/go-cmp/cmp"
	"github.com/google/go-cmp/cmp/cmpopts"
	"github.com/insomniacslk/dhcp/dhcpv4"
	"github.com/insomniacslk/dhcp/iana"
	"github.com/insomniacslk/dhcp/rfc1035label"
	"github.com/tinkerbell/dhcp/data"
	"go.opentelemetry.io/otel/trace"
	"inet.af/netaddr"
)

func TestSetDHCPOpts(t *testing.T) {
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
		in0 context.Context
		m   *dhcpv4.DHCPv4
		d   *data.Dhcp
	}
	tests := map[string]struct {
		fields fields
		args   args
		want   *dhcpv4.DHCPv4
	}{
		"success": {
			fields: fields{Log: logr.Discard()},
			args: args{
				in0: context.Background(),
				m:   &dhcpv4.DHCPv4{Options: dhcpv4.OptionsFromList(dhcpv4.OptParameterRequestList(dhcpv4.OptionSubnetMask))},
				d: &data.Dhcp{
					MacAddress:     net.HardwareAddr{0x00, 0x00, 0x00, 0x00, 0x00, 0x00},
					IPAddress:      netaddr.IPv4(192, 168, 4, 4),
					SubnetMask:     []byte{255, 255, 255, 0},
					DefaultGateway: netaddr.IPv4(192, 168, 4, 1),
					NameServers: []net.IP{
						{8, 8, 8, 8},
						{8, 8, 4, 4},
					},
					Hostname:         "test-server",
					DomainName:       "mynet.local",
					BroadcastAddress: netaddr.IPv4(192, 168, 4, 255),
					NTPServers: []net.IP{
						{132, 163, 96, 2},
						{132, 163, 96, 3},
					},
					LeaseTime: 84600,
					DomainSearch: []string{
						"mynet.local",
					},
				},
			},
			want: &dhcpv4.DHCPv4{
				OpCode:        dhcpv4.OpcodeBootRequest,
				HWType:        iana.HWTypeEthernet,
				ClientHWAddr:  net.HardwareAddr{0x00, 0x00, 0x00, 0x00, 0x00, 0x00},
				ClientIPAddr:  []byte{0, 0, 0, 0},
				YourIPAddr:    []byte{192, 168, 4, 4},
				ServerIPAddr:  []byte{0, 0, 0, 0},
				GatewayIPAddr: []byte{0, 0, 0, 0},
				Options: dhcpv4.OptionsFromList(
					dhcpv4.OptSubnetMask(net.IPMask{255, 255, 255, 0}),
					dhcpv4.OptBroadcastAddress(net.IP{192, 168, 4, 255}),
					dhcpv4.OptIPAddressLeaseTime(time.Duration(84600)*time.Second),
					dhcpv4.OptDomainName("mynet.local"),
					dhcpv4.OptHostName("test-server"),
					dhcpv4.OptRouter(net.IP{192, 168, 4, 1}),
					dhcpv4.OptDNS([]net.IP{
						{8, 8, 8, 8},
						{8, 8, 4, 4},
					}...),
					dhcpv4.OptNTPServers([]net.IP{
						{132, 163, 96, 2},
						{132, 163, 96, 3},
					}...),
					dhcpv4.OptDomainSearch(&rfc1035label.Labels{
						Labels: []string{"mynet.local"},
					}),
				),
			},
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
			mods := s.setDHCPOpts(tt.args.in0, tt.args.m, tt.args.d)
			finalPkt, err := dhcpv4.New(mods...)
			if err != nil {
				t.Fatalf("setDHCPOpts() error = %v, wantErr nil", err)
			}
			if diff := cmp.Diff(tt.want, finalPkt, cmpopts.IgnoreFields(dhcpv4.DHCPv4{}, "TransactionID")); diff != "" {
				t.Fatal(diff)
			}
		})
	}
}

func TestArch(t *testing.T) {
	tests := map[string]struct {
		pkt  *dhcpv4.DHCPv4
		want iana.Arch
	}{
		"found": {
			pkt:  &dhcpv4.DHCPv4{Options: dhcpv4.OptionsFromList(dhcpv4.OptClientArch(iana.INTEL_X86PC))},
			want: iana.INTEL_X86PC,
		},
		"unknown": {
			pkt:  &dhcpv4.DHCPv4{Options: dhcpv4.OptionsFromList(dhcpv4.OptClientArch(iana.Arch(255)))},
			want: iana.Arch(255),
		},
		"unknown: opt 93 len 0": {
			pkt:  &dhcpv4.DHCPv4{},
			want: iana.Arch(255),
		},
	}
	for name, tt := range tests {
		t.Run(name, func(t *testing.T) {
			got := arch(tt.pkt)
			if diff := cmp.Diff(tt.want, got); diff != "" {
				t.Fatal(diff)
			}
		})
	}
}

func TestBootfileAndNextServer(t *testing.T) {
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
		OTELEnabled       bool
	}
	type args struct {
		mac     net.HardwareAddr
		uClass  UserClass
		opt60   string
		bin     string
		tftp    netaddr.IPPort
		ipxe    *url.URL
		iscript *url.URL
	}
	tests := map[string]struct {
		fields       fields
		args         args
		wantBootFile string
		wantNextSrv  net.IP
	}{
		"success bootfile only": {
			fields: fields{Log: logr.Discard()},
			args: args{
				uClass:  Tinkerbell,
				iscript: &url.URL{Scheme: "http", Host: "localhost:8080", Path: "/auto.ipxe"},
			},
			wantBootFile: "http://localhost:8080/auto.ipxe",
			wantNextSrv:  nil,
		},
		"success httpClient": {
			fields: fields{Log: logr.Discard()},
			args: args{
				mac:   net.HardwareAddr{0x01, 0x02, 0x03, 0x04, 0x05, 0x06},
				opt60: httpClient.String(),
				bin:   "snp.ipxe",
				ipxe:  &url.URL{Scheme: "http", Host: "localhost:8181"},
			},
			wantBootFile: "http://localhost:8181/01:02:03:04:05:06/snp.ipxe",
			wantNextSrv:  net.IPv4(0, 0, 0, 0),
		},
		"success userclass iPXE": {
			fields: fields{Log: logr.Discard()},
			args: args{
				mac:    net.HardwareAddr{0x01, 0x02, 0x03, 0x04, 0x05, 0x07},
				uClass: IPXE,
				bin:    "unidonly.kpxe",
				tftp:   netaddr.IPPortFrom(netaddr.IPv4(192, 168, 6, 5), 69),
				ipxe:   &url.URL{Scheme: "tftp", Host: "192.168.6.5:69"},
			},
			wantBootFile: "tftp://192.168.6.5:69/01:02:03:04:05:07/unidonly.kpxe",
			wantNextSrv:  net.ParseIP("192.168.6.5"),
		},
		"success userclass iPXE with otel": {
			fields: fields{Log: logr.Discard(), OTELEnabled: true},
			args: args{
				mac:    net.HardwareAddr{0x01, 0x02, 0x03, 0x04, 0x05, 0x07},
				uClass: IPXE,
				bin:    "unidonly.kpxe",
				tftp:   netaddr.IPPortFrom(netaddr.IPv4(192, 168, 6, 5), 69),
				ipxe:   &url.URL{Scheme: "tftp", Host: "192.168.6.5:69"},
			},
			wantBootFile: "tftp://192.168.6.5:69/01:02:03:04:05:07/unidonly.kpxe-00-00000000000000000000000000000000-0000000000000000-00",
			wantNextSrv:  net.ParseIP("192.168.6.5"),
		},
		"success default": {
			fields: fields{Log: logr.Discard()},
			args: args{
				mac:  net.HardwareAddr{0x01, 0x02, 0x03, 0x04, 0x05, 0x07},
				bin:  "unidonly.kpxe",
				tftp: netaddr.IPPortFrom(netaddr.IPv4(192, 168, 6, 5), 69),
				ipxe: &url.URL{Scheme: "tftp", Host: "192.168.6.5:69"},
			},
			wantBootFile: "01:02:03:04:05:07/unidonly.kpxe",
			wantNextSrv:  net.ParseIP("192.168.6.5"),
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
				OTELEnabled:       tt.fields.OTELEnabled,
			}
			bootfile, nextServer := s.bootfileAndNextServer(tt.fields.ctx, tt.args.mac, tt.args.uClass, tt.args.opt60, tt.args.bin, tt.args.tftp, tt.args.ipxe, tt.args.iscript)
			if diff := cmp.Diff(tt.wantBootFile, bootfile); diff != "" {
				t.Fatal("bootfile", diff)
			}
			if diff := cmp.Diff(tt.wantNextSrv, nextServer); diff != "" {
				t.Fatal("nextServer", diff)
			}
		})
	}
}

func TestSetNetworkBootOpts(t *testing.T) {
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
		in0 context.Context
		m   *dhcpv4.DHCPv4
		n   *data.Netboot
	}
	tests := map[string]struct {
		fields fields
		args   args
		want   *dhcpv4.DHCPv4
	}{
		"netboot not allowed": {
			fields: fields{Log: logr.Discard()},
			args: args{
				in0: context.Background(),
				m:   &dhcpv4.DHCPv4{},
				n:   &data.Netboot{AllowNetboot: false},
			},
			want: &dhcpv4.DHCPv4{ServerIPAddr: net.IPv4(0, 0, 0, 0), BootFileName: "/netboot-not-allowed"},
		},
		"netboot allowed": {
			fields: fields{Log: logr.Discard(), IPXEScriptURL: &url.URL{Scheme: "http", Host: "localhost:8181", Path: "/01:02:03:04:05:06/auto.ipxe"}},
			args: args{
				in0: context.Background(),
				m: &dhcpv4.DHCPv4{
					ClientHWAddr: net.HardwareAddr{0x01, 0x02, 0x03, 0x04, 0x05, 0x06},
					Options: dhcpv4.OptionsFromList(
						dhcpv4.OptUserClass(Tinkerbell.String()),
						dhcpv4.OptClassIdentifier("HTTPClient:xxxxx"),
						dhcpv4.OptClientArch(iana.EFI_X86_64_HTTP),
					),
				},
				n: &data.Netboot{AllowNetboot: true},
			},
			want: &dhcpv4.DHCPv4{BootFileName: "http://localhost:8181/01:02:03:04:05:06/auto.ipxe", Options: dhcpv4.OptionsFromList(
				dhcpv4.OptGeneric(dhcpv4.OptionVendorSpecificInformation, dhcpv4.Options{
					6:  []byte{8},
					69: binaryTpFromContext(context.Background()),
				}.ToBytes()),
				dhcpv4.OptClassIdentifier("HTTPClient"),
			)},
		},
		"netboot not allowed, arch unknown": {
			fields: fields{Log: logr.Discard(), IPXEScriptURL: &url.URL{Scheme: "http", Host: "localhost:8181", Path: "/01:02:03:04:05:06/auto.ipxe"}},
			args: args{
				in0: context.Background(),
				m: &dhcpv4.DHCPv4{
					ClientHWAddr: net.HardwareAddr{0x01, 0x02, 0x03, 0x04, 0x05, 0x06},
					Options: dhcpv4.OptionsFromList(
						dhcpv4.OptUserClass(Tinkerbell.String()),
						dhcpv4.OptClientArch(iana.UBOOT_ARM64),
					),
				},
				n: &data.Netboot{AllowNetboot: true},
			},
			want: &dhcpv4.DHCPv4{ServerIPAddr: net.IPv4(0, 0, 0, 0), BootFileName: "/netboot-not-allowed"},
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
			gotFunc := s.setNetworkBootOpts(tt.args.in0, tt.args.m, tt.args.n)
			got := new(dhcpv4.DHCPv4)
			gotFunc(got)
			if diff := cmp.Diff(tt.want, got); diff != "" {
				t.Fatal(diff)
			}
		})
	}
}

func TestBinaryTpFromContext(t *testing.T) {
	want := []byte{0, 1, 2, 3, 4, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 5, 6, 7, 8, 0, 0, 0, 0, 1}
	sc := trace.NewSpanContext(trace.SpanContextConfig{
		TraceID:    trace.TraceID{0x01, 0x02, 0x03, 0x04},
		SpanID:     trace.SpanID{0x05, 0x06, 0x07, 0x08},
		TraceFlags: trace.TraceFlags(1),
	})
	rmSpan := trace.ContextWithRemoteSpanContext(context.Background(), sc)

	got := binaryTpFromContext(rmSpan)
	if !bytes.Equal(got, want) {
		t.Errorf("binaryTpFromContext() = %v, want %v", got, want)
	}
}

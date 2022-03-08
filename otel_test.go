package dhcp

import (
	"errors"
	"fmt"
	"log"
	"net"
	"os"
	"testing"
	"time"

	"github.com/go-logr/stdr"
	"github.com/google/go-cmp/cmp"
	"github.com/insomniacslk/dhcp/dhcpv4"
	"github.com/insomniacslk/dhcp/rfc1035label"
	"go.opentelemetry.io/otel/attribute"
)

func TestEncodeError(t *testing.T) {
	tests := map[string]struct {
		input *encodeError
		want  string
	}{
		"success":           {input: &encodeError{err: fmt.Errorf("test error")}, want: "test error"},
		"success nil error": {input: &encodeError{}, want: ""},
	}
	for name, tt := range tests {
		t.Run(name, func(t *testing.T) {
			got := tt.input.Error()
			if diff := cmp.Diff(got, tt.want); diff != "" {
				t.Fatal(diff)
			}
		})
	}
}

func TestSetOpt1(t *testing.T) {
	tests := map[string]struct {
		input   *dhcpv4.DHCPv4
		want    []attribute.KeyValue
		wantErr error
	}{
		"success": {
			input: &dhcpv4.DHCPv4{Options: dhcpv4.OptionsFromList(
				dhcpv4.OptSubnetMask(net.IP{255, 255, 255, 0}.DefaultMask()),
			)},
			want: []attribute.KeyValue{attribute.String("DHCP.testing.Opt1.SubnetMask", "255.255.255.0")},
		},
		"error": {wantErr: &encodeError{}},
	}
	for name, tt := range tests {
		t.Run(name, func(t *testing.T) {
			a := encoder{log: stdr.New(log.New(os.Stdout, "", log.Lshortfile))}
			stdr.SetVerbosity(1)
			err := a.encodeOpt1(tt.input, "testing")
			if !errors.Is(err, tt.wantErr) {
				t.Fatalf("setOpt1() error (type: %T) = %[1]v, wantErr (type: %T) %[2]v", err, tt.wantErr)
			}
			got := attribute.NewSet(a.attributes...)
			want := attribute.NewSet(tt.want...)
			enc := attribute.DefaultEncoder()
			if diff := cmp.Diff(got.Encoded(enc), want.Encoded(enc)); diff != "" {
				t.Log(got.Encoded(enc))
				t.Log(want.Encoded(enc))
				t.Fatal(diff)
			}
		})
	}
}

func TestSetOpt3(t *testing.T) {
	tests := map[string]struct {
		input   *dhcpv4.DHCPv4
		want    []attribute.KeyValue
		wantErr error
	}{
		"success": {
			input: &dhcpv4.DHCPv4{Options: dhcpv4.OptionsFromList(
				dhcpv4.OptRouter([]net.IP{{192, 168, 1, 1}}...),
			)},
			want: []attribute.KeyValue{attribute.String("DHCP.testing.Opt3.DefaultGateway", "192.168.1.1")},
		},
		"error": {wantErr: &encodeError{}},
	}
	for name, tt := range tests {
		t.Run(name, func(t *testing.T) {
			a := encoder{log: stdr.New(log.New(os.Stdout, "", log.Lshortfile))}
			stdr.SetVerbosity(1)
			err := a.encodeOpt3(tt.input, "testing")
			if !errors.Is(err, tt.wantErr) {
				t.Fatalf("setOpt13() error (type: %T) = %[1]v, wantErr (type: %T) %[2]v", err, tt.wantErr)
			}
			got := attribute.NewSet(a.attributes...)
			want := attribute.NewSet(tt.want...)
			enc := attribute.DefaultEncoder()
			if diff := cmp.Diff(got.Encoded(enc), want.Encoded(enc)); diff != "" {
				t.Log(got.Encoded(enc))
				t.Log(want.Encoded(enc))
				t.Fatal(diff)
			}
		})
	}
}

func TestSetOpt6(t *testing.T) {
	tests := map[string]struct {
		input   *dhcpv4.DHCPv4
		want    []attribute.KeyValue
		wantErr error
	}{
		"success": {
			input: &dhcpv4.DHCPv4{Options: dhcpv4.OptionsFromList(
				dhcpv4.OptDNS([]net.IP{{1, 1, 1, 1}}...),
			)},
			want: []attribute.KeyValue{attribute.String("DHCP.testing.Opt6.NameServers", "1.1.1.1")},
		},
		"error": {wantErr: &encodeError{}},
	}
	for name, tt := range tests {
		t.Run(name, func(t *testing.T) {
			a := encoder{log: stdr.New(log.New(os.Stdout, "", log.Lshortfile))}
			stdr.SetVerbosity(1)
			err := a.encodeOpt6(tt.input, "testing")
			if !errors.Is(err, tt.wantErr) {
				t.Fatalf("setOpt6() error (type: %T) = %[1]v, wantErr (type: %T) %[2]v", err, tt.wantErr)
			}
			got := attribute.NewSet(a.attributes...)
			want := attribute.NewSet(tt.want...)
			enc := attribute.DefaultEncoder()
			if diff := cmp.Diff(got.Encoded(enc), want.Encoded(enc)); diff != "" {
				t.Log(got.Encoded(enc))
				t.Log(want.Encoded(enc))
				t.Fatal(diff)
			}
		})
	}
}

func TestSetOpt12(t *testing.T) {
	tests := map[string]struct {
		input   *dhcpv4.DHCPv4
		want    []attribute.KeyValue
		wantErr error
	}{
		"success": {
			input: &dhcpv4.DHCPv4{Options: dhcpv4.OptionsFromList(
				dhcpv4.OptHostName("test-host"),
			)},
			want: []attribute.KeyValue{attribute.String("DHCP.testing.Opt12.Hostname", "test-host")},
		},
		"error": {wantErr: &encodeError{}},
	}
	for name, tt := range tests {
		t.Run(name, func(t *testing.T) {
			a := encoder{log: stdr.New(log.New(os.Stdout, "", log.Lshortfile))}
			stdr.SetVerbosity(1)
			err := a.encodeOpt12(tt.input, "testing")
			if !errors.Is(err, tt.wantErr) {
				t.Fatalf("setOpt12() error (type: %T) = %[1]v, wantErr (type: %T) %[2]v", err, tt.wantErr)
			}
			got := attribute.NewSet(a.attributes...)
			want := attribute.NewSet(tt.want...)
			enc := attribute.DefaultEncoder()
			if diff := cmp.Diff(got.Encoded(enc), want.Encoded(enc)); diff != "" {
				t.Log(got.Encoded(enc))
				t.Log(want.Encoded(enc))
				t.Fatal(diff)
			}
		})
	}
}

func TestSetOpt15(t *testing.T) {
	tests := map[string]struct {
		input   *dhcpv4.DHCPv4
		want    []attribute.KeyValue
		wantErr error
	}{
		"success": {
			input: &dhcpv4.DHCPv4{Options: dhcpv4.OptionsFromList(
				dhcpv4.OptDomainName("example.com"),
			)},
			want: []attribute.KeyValue{attribute.String("DHCP.testing.Opt15.DomainName", "example.com")},
		},
		"error": {wantErr: &encodeError{}},
	}
	for name, tt := range tests {
		t.Run(name, func(t *testing.T) {
			a := encoder{log: stdr.New(log.New(os.Stdout, "", log.Lshortfile))}
			stdr.SetVerbosity(1)
			err := a.encodeOpt15(tt.input, "testing")
			if !errors.Is(err, tt.wantErr) {
				t.Fatalf("setOpt15() error (type: %T) = %[1]v, wantErr (type: %T) %[2]v", err, tt.wantErr)
			}
			got := attribute.NewSet(a.attributes...)
			want := attribute.NewSet(tt.want...)
			enc := attribute.DefaultEncoder()
			if diff := cmp.Diff(got.Encoded(enc), want.Encoded(enc)); diff != "" {
				t.Log(got.Encoded(enc))
				t.Log(want.Encoded(enc))
				t.Fatal(diff)
			}
		})
	}
}

func TestSetOpt28(t *testing.T) {
	tests := map[string]struct {
		input   *dhcpv4.DHCPv4
		want    []attribute.KeyValue
		wantErr error
	}{
		"success": {
			input: &dhcpv4.DHCPv4{Options: dhcpv4.OptionsFromList(
				dhcpv4.OptBroadcastAddress(net.IP{192, 168, 1, 255}),
			)},
			want: []attribute.KeyValue{attribute.String("DHCP.testing.Opt28.BroadcastAddress", "192.168.1.255")},
		},
		"error": {wantErr: &encodeError{}},
	}
	for name, tt := range tests {
		t.Run(name, func(t *testing.T) {
			a := encoder{log: stdr.New(log.New(os.Stdout, "", log.Lshortfile))}
			stdr.SetVerbosity(1)
			err := a.setOpt28(tt.input, "testing")
			if !errors.Is(err, tt.wantErr) {
				t.Fatalf("setOpt28() error (type: %T) = %[1]v, wantErr (type: %T) %[2]v", err, tt.wantErr)
			}
			got := attribute.NewSet(a.attributes...)
			want := attribute.NewSet(tt.want...)
			enc := attribute.DefaultEncoder()
			if diff := cmp.Diff(got.Encoded(enc), want.Encoded(enc)); diff != "" {
				t.Log(got.Encoded(enc))
				t.Log(want.Encoded(enc))
				t.Fatal(diff)
			}
		})
	}
}

func TestSetOpt42(t *testing.T) {
	tests := map[string]struct {
		input   *dhcpv4.DHCPv4
		want    []attribute.KeyValue
		wantErr error
	}{
		"success": {
			input: &dhcpv4.DHCPv4{Options: dhcpv4.OptionsFromList(
				dhcpv4.OptNTPServers([]net.IP{{132, 163, 96, 2}}...),
			)},
			want: []attribute.KeyValue{attribute.String("DHCP.testing.Opt42.NTPServers", "132.163.96.2")},
		},
		"error": {wantErr: &encodeError{}},
	}
	for name, tt := range tests {
		t.Run(name, func(t *testing.T) {
			a := encoder{log: stdr.New(log.New(os.Stdout, "", log.Lshortfile))}
			stdr.SetVerbosity(1)
			err := a.encodeOpt42(tt.input, "testing")
			if !errors.Is(err, tt.wantErr) {
				t.Fatalf("setOpt42() error (type: %T) = %[1]v, wantErr (type: %T) %[2]v", err, tt.wantErr)
			}
			got := attribute.NewSet(a.attributes...)
			want := attribute.NewSet(tt.want...)
			enc := attribute.DefaultEncoder()
			if diff := cmp.Diff(got.Encoded(enc), want.Encoded(enc)); diff != "" {
				t.Log(got.Encoded(enc))
				t.Log(want.Encoded(enc))
				t.Fatal(diff)
			}
		})
	}
}

func TestSetOpt51(t *testing.T) {
	tests := map[string]struct {
		input   *dhcpv4.DHCPv4
		want    []attribute.KeyValue
		wantErr error
	}{
		"success": {
			input: &dhcpv4.DHCPv4{Options: dhcpv4.OptionsFromList(
				dhcpv4.OptIPAddressLeaseTime(time.Minute),
			)},
			want: []attribute.KeyValue{attribute.String("DHCP.testing.Opt51.LeaseTime", "60")},
		},
		"error": {wantErr: &encodeError{}},
	}
	for name, tt := range tests {
		t.Run(name, func(t *testing.T) {
			a := encoder{log: stdr.New(log.New(os.Stdout, "", log.Lshortfile))}
			stdr.SetVerbosity(1)
			err := a.encodeOpt51(tt.input, "testing")
			if !errors.Is(err, tt.wantErr) {
				t.Fatalf("setOpt51() error (type: %T) = %[1]v, wantErr (type: %T) %[2]v", err, tt.wantErr)
			}
			got := attribute.NewSet(a.attributes...)
			want := attribute.NewSet(tt.want...)
			enc := attribute.DefaultEncoder()
			if diff := cmp.Diff(got.Encoded(enc), want.Encoded(enc)); diff != "" {
				t.Log(got.Encoded(enc))
				t.Log(want.Encoded(enc))
				t.Fatal(diff)
			}
		})
	}
}

func TestSetOpt53(t *testing.T) {
	tests := map[string]struct {
		input   *dhcpv4.DHCPv4
		want    []attribute.KeyValue
		wantErr error
	}{
		"success": {
			input: &dhcpv4.DHCPv4{Options: dhcpv4.OptionsFromList(
				dhcpv4.OptMessageType(dhcpv4.MessageTypeOffer),
			)},
			want: []attribute.KeyValue{attribute.String("DHCP.testing.Opt53.MessageType", "OFFER")},
		},
		"error": {wantErr: &encodeError{}},
	}
	for name, tt := range tests {
		t.Run(name, func(t *testing.T) {
			a := encoder{log: stdr.New(log.New(os.Stdout, "", log.Lshortfile))}
			stdr.SetVerbosity(1)
			err := a.encodeOpt53(tt.input, "testing")
			if !errors.Is(err, tt.wantErr) {
				t.Fatalf("setOpt53() error (type: %T) = %[1]v, wantErr (type: %T) %[2]v", err, tt.wantErr)
			}
			got := attribute.NewSet(a.attributes...)
			want := attribute.NewSet(tt.want...)
			enc := attribute.DefaultEncoder()
			if diff := cmp.Diff(got.Encoded(enc), want.Encoded(enc)); diff != "" {
				t.Log(got.Encoded(enc))
				t.Log(want.Encoded(enc))
				t.Fatal(diff)
			}
		})
	}
}

func TestSetOpt54(t *testing.T) {
	tests := map[string]struct {
		input   *dhcpv4.DHCPv4
		want    []attribute.KeyValue
		wantErr error
	}{
		"success": {
			input: &dhcpv4.DHCPv4{Options: dhcpv4.OptionsFromList(
				dhcpv4.OptServerIdentifier(net.IP{127, 0, 0, 1}),
			)},
			want: []attribute.KeyValue{attribute.String("DHCP.testing.Opt54.ServerIdentifier", "127.0.0.1")},
		},
		"error": {wantErr: &encodeError{}},
	}
	for name, tt := range tests {
		t.Run(name, func(t *testing.T) {
			a := encoder{log: stdr.New(log.New(os.Stdout, "", log.Lshortfile))}
			stdr.SetVerbosity(1)
			err := a.encodeOpt54(tt.input, "testing")
			if !errors.Is(err, tt.wantErr) {
				t.Fatalf("setOpt54() error (type: %T) = %[1]v, wantErr (type: %T) %[2]v", err, tt.wantErr)
			}
			got := attribute.NewSet(a.attributes...)
			want := attribute.NewSet(tt.want...)
			enc := attribute.DefaultEncoder()
			if diff := cmp.Diff(got.Encoded(enc), want.Encoded(enc)); diff != "" {
				t.Log(got.Encoded(enc))
				t.Log(want.Encoded(enc))
				t.Fatal(diff)
			}
		})
	}
}

func TestSetOpt119(t *testing.T) {
	tests := map[string]struct {
		input   *dhcpv4.DHCPv4
		want    []attribute.KeyValue
		wantErr error
	}{
		"success": {
			input: &dhcpv4.DHCPv4{Options: dhcpv4.OptionsFromList(
				dhcpv4.OptDomainSearch(&rfc1035label.Labels{Labels: []string{"mydomain.com"}}),
			)},
			want: []attribute.KeyValue{attribute.String("DHCP.testing.Opt119.DomainSearch", "mydomain.com")},
		},
		"error": {wantErr: &encodeError{}},
	}
	for name, tt := range tests {
		t.Run(name, func(t *testing.T) {
			a := encoder{log: stdr.New(log.New(os.Stdout, "", log.Lshortfile))}
			stdr.SetVerbosity(1)
			err := a.encodeOpt119(tt.input, "testing")
			if !errors.Is(err, tt.wantErr) {
				t.Fatalf("setOpt119() error (type: %T) = %[1]v, wantErr (type: %T) %[2]v", err, tt.wantErr)
			}
			got := attribute.NewSet(a.attributes...)
			want := attribute.NewSet(tt.want...)
			enc := attribute.DefaultEncoder()
			if diff := cmp.Diff(got.Encoded(enc), want.Encoded(enc)); diff != "" {
				t.Log(got.Encoded(enc))
				t.Log(want.Encoded(enc))
				t.Fatal(diff)
			}
		})
	}
}

func TestSetHeaderYIADDR(t *testing.T) {
	tests := map[string]struct {
		input   *dhcpv4.DHCPv4
		want    []attribute.KeyValue
		wantErr error
	}{
		"success": {
			input: &dhcpv4.DHCPv4{YourIPAddr: []byte{192, 168, 2, 100}},
			want:  []attribute.KeyValue{attribute.String("DHCP.testing.Header.yiaddr", "192.168.2.100")},
		},
		"error": {wantErr: &encodeError{}},
	}
	for name, tt := range tests {
		t.Run(name, func(t *testing.T) {
			a := encoder{log: stdr.New(log.New(os.Stdout, "", log.Lshortfile))}
			stdr.SetVerbosity(1)
			err := a.encodeYIADDR(tt.input, "testing")
			if !errors.Is(err, tt.wantErr) {
				t.Fatalf("setHeaderYIADDR() error (type: %T) = %[1]v, wantErr (type: %T) %[2]v", err, tt.wantErr)
			}
			got := attribute.NewSet(a.attributes...)
			want := attribute.NewSet(tt.want...)
			enc := attribute.DefaultEncoder()
			if diff := cmp.Diff(got.Encoded(enc), want.Encoded(enc)); diff != "" {
				t.Log(got.Encoded(enc))
				t.Log(want.Encoded(enc))
				t.Fatal(diff)
			}
		})
	}
}

func TestSetHeaderSIADDR(t *testing.T) {
	tests := map[string]struct {
		input   *dhcpv4.DHCPv4
		want    []attribute.KeyValue
		wantErr error
	}{
		"success": {
			input: &dhcpv4.DHCPv4{ServerIPAddr: []byte{127, 0, 0, 1}},
			want:  []attribute.KeyValue{attribute.String("DHCP.testing.Header.siaddr", "127.0.0.1")},
		},
		"error": {wantErr: &encodeError{}},
	}
	for name, tt := range tests {
		t.Run(name, func(t *testing.T) {
			a := encoder{log: stdr.New(log.New(os.Stdout, "", log.Lshortfile))}
			stdr.SetVerbosity(1)
			err := a.encodeSIADDR(tt.input, "testing")
			if !errors.Is(err, tt.wantErr) {
				t.Fatalf("setHeaderSIADDR() error (type: %T) = %[1]v, wantErr (type: %T) %[2]v", err, tt.wantErr)
			}
			got := attribute.NewSet(a.attributes...)
			want := attribute.NewSet(tt.want...)
			enc := attribute.DefaultEncoder()
			if diff := cmp.Diff(got.Encoded(enc), want.Encoded(enc)); diff != "" {
				t.Log(got.Encoded(enc))
				t.Log(want.Encoded(enc))
				t.Fatal(diff)
			}
		})
	}
}

func TestSetHeaderCHADDR(t *testing.T) {
	tests := map[string]struct {
		input   *dhcpv4.DHCPv4
		want    []attribute.KeyValue
		wantErr error
	}{
		"success": {
			input: &dhcpv4.DHCPv4{ClientHWAddr: []byte{0x01, 0x02, 0x03, 0x04, 0x05, 0x06}},
			want:  []attribute.KeyValue{attribute.String("DHCP.testing.Header.chaddr", "01:02:03:04:05:06")},
		},
		"error": {wantErr: &encodeError{}},
	}
	for name, tt := range tests {
		t.Run(name, func(t *testing.T) {
			a := encoder{log: stdr.New(log.New(os.Stdout, "", log.Lshortfile))}
			stdr.SetVerbosity(1)
			err := a.encodeCHADDR(tt.input, "testing")
			if !errors.Is(err, tt.wantErr) {
				t.Fatalf("setHeaderCHADDR() error (type: %T) = %[1]v, wantErr (type: %T) %[2]v", err, tt.wantErr)
			}
			got := attribute.NewSet(a.attributes...)
			want := attribute.NewSet(tt.want...)
			enc := attribute.DefaultEncoder()
			if diff := cmp.Diff(got.Encoded(enc), want.Encoded(enc)); diff != "" {
				t.Log(got.Encoded(enc))
				t.Log(want.Encoded(enc))
				t.Fatal(diff)
			}
		})
	}
}

func TestSetHeaderFILE(t *testing.T) {
	tests := map[string]struct {
		input   *dhcpv4.DHCPv4
		want    []attribute.KeyValue
		wantErr error
	}{
		"success": {
			input: &dhcpv4.DHCPv4{BootFileName: "snp.efi"},
			want:  []attribute.KeyValue{attribute.String("DHCP.testing.Header.file", "snp.efi")},
		},
		"error": {wantErr: &encodeError{}},
	}
	for name, tt := range tests {
		t.Run(name, func(t *testing.T) {
			a := encoder{log: stdr.New(log.New(os.Stdout, "", log.Lshortfile))}
			stdr.SetVerbosity(1)
			err := a.encodeFILE(tt.input, "testing")
			if !errors.Is(err, tt.wantErr) {
				t.Fatalf("setHeaderFILE() error (type: %T) = %[1]v, wantErr (type: %T) %[2]v", err, tt.wantErr)
			}
			got := attribute.NewSet(a.attributes...)
			want := attribute.NewSet(tt.want...)
			enc := attribute.DefaultEncoder()
			if diff := cmp.Diff(got.Encoded(enc), want.Encoded(enc)); diff != "" {
				t.Log(got.Encoded(enc))
				t.Log(want.Encoded(enc))
				t.Fatal(diff)
			}
		})
	}
}

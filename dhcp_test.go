package dhcp

import (
	"context"
	"errors"
	"net"
	"net/netip"
	"testing"
	"time"

	"github.com/go-logr/logr"
	"github.com/google/go-cmp/cmp"
	"github.com/insomniacslk/dhcp/dhcpv4"
	"github.com/insomniacslk/dhcp/dhcpv4/nclient4"
	"github.com/tinkerbell/dhcp/handler/noop"
	"golang.org/x/net/nettest"
)

type mock struct {
	Log         logr.Logger
	ServerIP    net.IP
	LeaseTime   uint32
	YourIP      net.IP
	NameServers []net.IP
	SubnetMask  net.IPMask
	Router      net.IP
}

func (m *mock) Handle(conn net.PacketConn, peer net.Addr, pkt *dhcpv4.DHCPv4) {
	if m.Log.GetSink() == nil {
		m.Log = logr.Discard()
	}

	mods := m.setOpts()
	switch mt := pkt.MessageType(); mt {
	case dhcpv4.MessageTypeDiscover:
		mods = append(mods, dhcpv4.WithMessageType(dhcpv4.MessageTypeOffer))
	case dhcpv4.MessageTypeRequest:
		mods = append(mods, dhcpv4.WithMessageType(dhcpv4.MessageTypeAck))
	case dhcpv4.MessageTypeRelease:
		mods = append(mods, dhcpv4.WithMessageType(dhcpv4.MessageTypeAck))
	default:
		m.Log.Info("unsupported message type", "type", mt.String())
		return
	}
	reply, err := dhcpv4.NewReplyFromRequest(pkt, mods...)
	if err != nil {
		m.Log.Error(err, "error creating reply")
		return
	}
	if _, err := conn.WriteTo(reply.ToBytes(), peer); err != nil {
		m.Log.Error(err, "failed to send reply")
		return
	}
	m.Log.Info("sent reply")
}

func (m *mock) setOpts() []dhcpv4.Modifier {
	mods := []dhcpv4.Modifier{
		dhcpv4.WithGeneric(dhcpv4.OptionServerIdentifier, m.ServerIP),
		dhcpv4.WithServerIP(m.ServerIP),
		dhcpv4.WithLeaseTime(m.LeaseTime),
		dhcpv4.WithYourIP(m.YourIP),
		dhcpv4.WithDNS(m.NameServers...),
		dhcpv4.WithNetmask(m.SubnetMask),
		dhcpv4.WithRouter(m.Router),
	}

	return mods
}

func dhcp(ctx context.Context) (*dhcpv4.DHCPv4, error) {
	rifs, err := nettest.RoutedInterface("ip", net.FlagUp|net.FlagBroadcast)
	if err != nil {
		return nil, err
	}
	c, err := nclient4.New(rifs.Name,
		nclient4.WithServerAddr(&net.UDPAddr{IP: net.IPv4(127, 0, 0, 1), Port: 7676}),
		nclient4.WithUnicast(&net.UDPAddr{IP: net.IPv4(127, 0, 0, 1), Port: 7677}),
	)
	if err != nil {
		return nil, err
	}
	defer c.Close()

	return c.DiscoverOffer(ctx)
}

func TestListenAndServe(t *testing.T) {
	// test if the server is listening on the correct address and port
	tests := map[string]struct {
		h            Handler
		addr         netip.AddrPort
		wantListener *Listener
	}{
		"success": {addr: netip.MustParseAddrPort("127.0.0.1:7676"), h: &mock{}},
	}
	for name, tt := range tests {
		t.Run(name, func(t *testing.T) {
			s := &Listener{Addr: tt.addr}
			t.Logf("before: %+v", s)
			ctx, done := context.WithCancel(context.Background())
			defer done()
			go func() {
				<-ctx.Done()
				s.Shutdown()
			}()

			go func() {
				t.Log(s.ListenAndServe(tt.h))
			}()

			// make client calls
			offer, err := dhcp(ctx)
			if err != nil {
				t.Fatal(err)
			}
			t.Log(offer)

			done()
		})
	}
}

func TestListenerAndServe(t *testing.T) {
	tests := map[string]struct {
		h    Handler
		addr netip.AddrPort
		err  error
	}{
		"noop handler":             {h: &noop.Handler{}, addr: netip.MustParseAddrPort("0.0.0.0:7678")},
		"no handler":               {addr: netip.MustParseAddrPort("0.0.0.0:7678")},
		"mock handler":             {h: &mock{}, addr: netip.MustParseAddrPort("0.0.0.0:7678")},
		"success use default addr": {h: &mock{}},
	}
	for name, tt := range tests {
		t.Run(name, func(t *testing.T) {
			s := &Listener{
				Addr: tt.addr,
			}
			ctx, done := context.WithTimeout(context.Background(), time.Millisecond*100)
			defer done()
			go func() {
				<-ctx.Done()
				s.Shutdown()
			}()

			err := s.ListenAndServe(tt.h)
			switch err.(type) {
			case *net.OpError:
			default:
				if err.Error() != "failed to create udp connection: cannot bind to port 67: permission denied" && !errors.Is(err, ErrNoConn) {
					t.Log(err)
					t.Fatalf("got: %T, wanted: %T or ErrNoConn", err, &net.OpError{})
				}
			}
		})
	}
}

func TestServe(t *testing.T) {
	tests := map[string]struct {
		h    Handler
		addr netip.AddrPort
		err  error
	}{
		"noop handler": {addr: netip.MustParseAddrPort("0.0.0.0:7676"), h: &noop.Handler{}},
		"no handler":   {addr: netip.MustParseAddrPort("0.0.0.0:7678")},
	}
	for name, tt := range tests {
		t.Run(name, func(t *testing.T) {
			s := &Listener{
				Addr: tt.addr,
			}
			ctx, done := context.WithTimeout(context.Background(), time.Millisecond*100)
			defer done()
			go func() {
				<-ctx.Done()
				s.Shutdown()
			}()

			err := Serve(nil, tt.h)
			switch err.(type) {
			case *net.OpError:
			default:
				if !errors.Is(err, ErrNoConn) {
					t.Fatalf("got: %T, wanted: %T or ErrNoConn", err, &net.OpError{})
				}
			}
		})
	}
}

func TestNoConnError(t *testing.T) {
	want := "no connection specified"
	got := ErrNoConn
	if diff := cmp.Diff(got.Error(), want); diff != "" {
		t.Fatal(diff)
	}
}

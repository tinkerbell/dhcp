package dhcp

import (
	"context"
	"errors"
	"net"
	"testing"
	"time"

	"github.com/go-logr/logr"
	"github.com/insomniacslk/dhcp/dhcpv4"
	"github.com/insomniacslk/dhcp/dhcpv4/nclient4"
	"github.com/tinkerbell/dhcp/handler/noop"
	"inet.af/netaddr"
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

func (p *mock) setOpts() []dhcpv4.Modifier {
	mods := []dhcpv4.Modifier{
		dhcpv4.WithGeneric(dhcpv4.OptionServerIdentifier, p.ServerIP),
		dhcpv4.WithServerIP(p.ServerIP),
		dhcpv4.WithLeaseTime(p.LeaseTime),
		dhcpv4.WithYourIP(p.YourIP),
		dhcpv4.WithDNS(p.NameServers...),
		dhcpv4.WithNetmask(p.SubnetMask),
		dhcpv4.WithRouter(p.Router),
	}

	return mods
}

func dhcp(ctx context.Context, ifname string) (*dhcpv4.DHCPv4, error) {
	c, err := nclient4.New(ifname)
	if err != nil {
		return nil, err
	}
	defer c.Close()

	return c.DiscoverOffer(ctx)
}

func TestListenAndServe(t *testing.T) {
	t.Skip()
	// test if the server is listening on the correct address and port
	tests := map[string]struct {
		h            Handler
		addr         netaddr.IPPort
		wantListener *Listener
	}{
		"success": {addr: netaddr.IPPortFrom(netaddr.IPv4(0, 0, 0, 0), 7676), h: &mock{}},
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

			go s.ListenAndServe(tt.h)

			// make client calls
			d, err := dhcp(ctx, "eth0")
			t.Log(d)
			t.Log(err)

			done()
			t.Fatal()
		})
	}
}

func TestListenerAndServe(t *testing.T) {
	tests := map[string]struct {
		h    Handler
		addr netaddr.IPPort
		err  error
	}{
		"noop handler": {h: &noop.Handler{}, addr: netaddr.IPPortFrom(netaddr.IPv4(0, 0, 0, 0), 7676)},
		"no handler":   {addr: netaddr.IPPortFrom(netaddr.IPv4(0, 0, 0, 0), 7678)},
		"mock handler": {h: &mock{}, addr: netaddr.IPPortFrom(netaddr.IPv4(0, 0, 0, 0), 7678)},
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
				if !errors.Is(err, ErrNoConn) {
					t.Fatalf("got: %T, wanted: %T or ErrNoConn", err, &net.OpError{})
				}
			}
		})
	}
}

func TestServe(t *testing.T) {
	tests := map[string]struct {
		h    Handler
		addr netaddr.IPPort
		err  error
	}{
		"noop handler": {addr: netaddr.IPPortFrom(netaddr.IPv4(0, 0, 0, 0), 7676), h: &noop.Handler{}},
		"no handler":   {addr: netaddr.IPPortFrom(netaddr.IPv4(0, 0, 0, 0), 7678)},
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

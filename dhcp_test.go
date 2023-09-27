package dhcp

/*
type mock struct {
	Log         logr.Logger
	ServerIP    net.IP
	LeaseTime   uint32
	YourIP      net.IP
	NameServers []net.IP
	SubnetMask  net.IPMask
	Router      net.IP
}

func (m *mock) Handle(ctx context.Context, conn net.PacketConn, d data.Packet) {
	if m.Log.GetSink() == nil {
		m.Log = logr.Discard()
	}

	mods := m.setOpts()
	switch mt := d.Pkt.MessageType(); mt {
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
	reply, err := dhcpv4.NewReplyFromRequest(d.Pkt, mods...)
	if err != nil {
		m.Log.Error(err, "error creating reply")
		return
	}
	if _, err := conn.WriteTo(reply.ToBytes(), d.Peer); err != nil {
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
		"success": {addr: netip.MustParseAddrPort("127.0.0.1:7676"), h: func() Handler { m := &mock{}; return m.Handle }()},
	}
	for name, tt := range tests {
		t.Run(name, func(t *testing.T) {
			s := &Listener{Addr: tt.addr}
			t.Logf("before: %+v", s)
			ctx, done := context.WithCancel(context.Background())
			defer done()

			go s.ListenAndServe(ctx, tt.h)

			// make client calls
			d, err := dhcp(ctx)
			if err != nil {
				t.Fatal(err)
			}
			t.Log(d)

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
		"noop handler":             {h: func() Handler { m := &noop.Handler{}; return m.Handle }(), addr: netip.MustParseAddrPort("0.0.0.0:7678")},
		"no handler":               {addr: netip.MustParseAddrPort("0.0.0.0:7678")},
		"mock handler":             {h: func() Handler { m := &mock{}; return m.Handle }(), addr: netip.MustParseAddrPort("0.0.0.0:7678")},
		"success use default addr": {h: func() Handler { m := &mock{}; return m.Handle }()},
	}
	for name, tt := range tests {
		t.Run(name, func(t *testing.T) {
			s := &Listener{
				Addr: tt.addr,
			}
			ctx, done := context.WithTimeout(context.Background(), time.Millisecond*100)
			defer done()

			err := s.ListenAndServe(ctx, tt.h)
			if err != tt.err && err.Error() != "failed to create udp connection: cannot bind to port 67: permission denied" && !errors.Is(err, ErrNoConn) && !closeConnErr(err) { //nolint:errorlint // nil pointer dereference without this.
				t.Log(err)
				t.Fatalf("got: %T, wanted: %T or ErrNoConn", err, &net.OpError{})
			}
		})
	}
}

func closeConnErr(err error) bool {
	l, ok := err.(*net.OpError)
	if !ok {
		return false
	}

	return l.Op == "close"
}

func TestServe(t *testing.T) {
	tests := map[string]struct {
		h    Handler
		addr netip.AddrPort
		err  error
	}{
		"noop handler": {addr: netip.MustParseAddrPort("0.0.0.0:7676"), h: func() Handler { m := &noop.Handler{}; return m.Handle }()},
		"no handler":   {addr: netip.MustParseAddrPort("0.0.0.0:7678")},
	}
	for name, tt := range tests {
		t.Run(name, func(t *testing.T) {
			ctx, done := context.WithTimeout(context.Background(), time.Millisecond*100)
			defer done()
			go func() {
				<-ctx.Done()
			}()

			err := Serve(ctx, nil, tt.h)
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
*/

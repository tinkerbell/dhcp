package dhcp

import (
	"context"
	"net"

	"github.com/insomniacslk/dhcp/dhcpv4"
)

// handleFunc is the main handler for DHCPv4 packets.
func (s *Server) handleFunc(conn net.PacketConn, peer net.Addr, m *dhcpv4.DHCPv4) {
	var reply *dhcpv4.DHCPv4
	switch mt := m.MessageType(); mt {
	case dhcpv4.MessageTypeDiscover:
		reply = s.handleDiscover(s.ctx, m)
	case dhcpv4.MessageTypeRequest:
		reply = s.handleRequest(s.ctx, m)
	case dhcpv4.MessageTypeRelease:
		s.handleRelease(s.ctx, m)
	default:
		s.Log.Info("received unknown message type", "type", mt)
	}
	if reply != nil {
		if _, err := conn.WriteTo(reply.ToBytes(), peer); err != nil {
			s.Log.Error(err, "failed to send DHCP")
		}
	}
}

// handleDiscover handles DHCP packets message type of discover.
func (s *Server) handleDiscover(ctx context.Context, m *dhcpv4.DHCPv4) *dhcpv4.DHCPv4 {
	s.Log.Info("received discover packet")
	d, n, err := s.Backend.Read(ctx, m.ClientHWAddr)
	if err != nil {
		s.Log.Info("not sending DHCP OFFER", "mac", m.ClientHWAddr, "error", err)
		return nil
	}
	mods := []dhcpv4.Modifier{
		dhcpv4.WithMessageType(dhcpv4.MessageTypeOffer),
		dhcpv4.WithGeneric(dhcpv4.OptionServerIdentifier, s.IPAddr.IPAddr().IP),
		dhcpv4.WithServerIP(s.IPAddr.IPAddr().IP),
	}
	mods = append(mods, s.setDHCPOpts(ctx, m, d)...)
	if s.NetbootEnabled {
		mods = append(mods, s.setNetworkBootOpts(ctx, m, n))
	}
	reply, err := dhcpv4.NewReplyFromRequest(m, mods...)
	if err != nil {
		return nil
	}
	s.Log.Info("sending offer packet")
	return reply
}

// handleRequest handles DHCP packets message type of request.
func (s *Server) handleRequest(ctx context.Context, m *dhcpv4.DHCPv4) *dhcpv4.DHCPv4 {
	s.Log.Info("received request packet")
	d, n, err := s.Backend.Read(ctx, m.ClientHWAddr)
	if err != nil {
		s.Log.Info("not sending DHCP ACK", "mac", m.ClientHWAddr, "error", err)
		return nil
	}
	mods := []dhcpv4.Modifier{
		dhcpv4.WithMessageType(dhcpv4.MessageTypeAck),
		dhcpv4.WithGeneric(dhcpv4.OptionServerIdentifier, s.Listener.UDPAddr().IP),
		dhcpv4.WithServerIP(s.IPAddr.IPAddr().IP),
	}
	mods = append(mods, s.setDHCPOpts(ctx, m, d)...)
	if s.NetbootEnabled {
		mods = append(mods, s.setNetworkBootOpts(ctx, m, n))
	}
	reply, err := dhcpv4.NewReplyFromRequest(m, mods...)
	if err != nil {
		return nil
	}
	s.Log.Info("sending ack packet")
	return reply
}

// handleRelease handles DHCP packets message type of release.
// Since the design of this DHCP server is that all IP addresses are
// Host reservations, when a client releases the address, the server
// doesn't have anything to do. This method is included for clarity of this
// design decision.
func (s *Server) handleRelease(_ context.Context, _ *dhcpv4.DHCPv4) {
	s.Log.Info("received release, no response required")
}

package dhcp

import (
	"context"
	"fmt"
	"net"

	"github.com/insomniacslk/dhcp/dhcpv4"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"
	"go.opentelemetry.io/otel/trace"
)

// handleFunc is the main handler for DHCPv4 packets.
func (s *Server) handleFunc(conn net.PacketConn, peer net.Addr, m *dhcpv4.DHCPv4) {
	tracer := otel.Tracer("DHCP")
	ctx, span := tracer.Start(s.ctx, "DHCP Packet Received",
		trace.WithAttributes(attribute.String("MAC", m.ClientHWAddr.String())),
		trace.WithAttributes(attribute.String("MessageType", m.MessageType().String())),
	)
	defer span.End()
	var reply *dhcpv4.DHCPv4
	switch mt := m.MessageType(); mt {
	case dhcpv4.MessageTypeDiscover:
		reply = s.handleDiscover(ctx, tracer, m)
	case dhcpv4.MessageTypeRequest:
		reply = s.handleRequest(ctx, tracer, m)
	case dhcpv4.MessageTypeRelease:
		s.handleRelease(ctx, m)
	default:
		s.Log.Info("received unknown message type", "type", mt)
	}
	c := codes.Ok
	cmsg := fmt.Sprintf("no reply send DHCP: %v", m.MessageType())
	if reply != nil {
		span.SetAttributes(attribute.String("IP", reply.YourIPAddr.String()))
		if _, err := conn.WriteTo(reply.ToBytes(), peer); err != nil {
			s.Log.Error(err, "failed to send DHCP")
			c = codes.Error
			cmsg = fmt.Sprintf("failed to send DHCP: %v, err: %v", reply.MessageType().String(), err.Error())
		} else {
			cmsg = fmt.Sprintf("sent DHCP: %s", reply.MessageType().String())
		}
	}
	span.SetStatus(c, cmsg)
}

// handleDiscover handles DHCP packets message type of discover.
func (s *Server) handleDiscover(ctx context.Context, tracer trace.Tracer, m *dhcpv4.DHCPv4) *dhcpv4.DHCPv4 {
	ctx, span := tracer.Start(ctx, "DHCP Discover Message")
	s.Log.Info("received discover packet")
	d, n, err := s.Backend.Read(ctx, m.ClientHWAddr)
	if err != nil {
		s.Log.Info("not sending DHCP OFFER", "mac", m.ClientHWAddr, "error", err)
		span.SetStatus(codes.Error, fmt.Sprintf("not sending DHCP OFFER: %v", err))
		span.End()
		return nil
	}
	span.SetAttributes(
		attribute.String("Subnet", d.SubnetMask.String()),
		attribute.String("DefaultGateway", d.DefaultGateway.String()),
		attribute.String("Nameservers", fmt.Sprintf("%v", d.NameServers)),
		attribute.String("Hostname", d.Hostname),
		attribute.String("DomainName", d.DomainName),
		attribute.Int("LeaseTime", int(d.LeaseTime)),
		attribute.Bool("AllowNetboot", n.AllowNetboot),
		attribute.String("IpxeScriptURL", n.IpxeScriptURL),
	)
	span.SetStatus(codes.Ok, "done reading from backend")
	span.End()

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
func (s *Server) handleRequest(ctx context.Context, tracer trace.Tracer, m *dhcpv4.DHCPv4) *dhcpv4.DHCPv4 {
	ctx, span := tracer.Start(ctx, "DHCP Request Message")
	s.Log.Info("received request packet")
	d, n, err := s.Backend.Read(ctx, m.ClientHWAddr)
	if err != nil {
		s.Log.Info("not sending DHCP ACK", "mac", m.ClientHWAddr, "error", err)
		return nil
	}
	span.SetAttributes(
		attribute.String("Subnet", d.SubnetMask.String()),
		attribute.String("DefaultGateway", d.DefaultGateway.String()),
		attribute.String("Nameservers", fmt.Sprintf("%v", d.NameServers)),
		attribute.String("Hostname", d.Hostname),
		attribute.String("DomainName", d.DomainName),
		attribute.Int("LeaseTime", int(d.LeaseTime)),
		attribute.Bool("AllowNetboot", n.AllowNetboot),
		attribute.String("IpxeScriptURL", n.IpxeScriptURL),
	)
	span.SetStatus(codes.Ok, "reading from backend done")
	span.End()

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

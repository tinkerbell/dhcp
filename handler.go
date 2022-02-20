package dhcp

import (
	"context"
	"fmt"
	"net"
	"strings"

	"github.com/insomniacslk/dhcp/dhcpv4"
	"github.com/tinkerbell/dhcp/data"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"
	"go.opentelemetry.io/otel/trace"
)

// BackendReader is the interface that wraps the Read method.
//
// Backends implement this interface to provide DHCP data to the DHCP server.
type BackendReader interface {
	// Read data (from a backend) based on a mac address
	// and return DHCP headers and options, including netboot info.
	Read(context.Context, net.HardwareAddr) (*data.Dhcp, *data.Netboot, error)
}

// handleFunc is the main handler for DHCPv4 packets.
func (s *Server) handleFunc(conn net.PacketConn, peer net.Addr, m *dhcpv4.DHCPv4) {
	log := s.Log.WithValues("mac", m.ClientHWAddr.String())
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
		log.Info("received unknown message type", "type", mt)
	}
	c := codes.Ok
	cmsg := fmt.Sprintf("no reply send DHCP: %v", m.MessageType())
	if reply != nil {
		span.SetAttributes(attribute.String("IP", reply.YourIPAddr.String()))
		if _, err := conn.WriteTo(reply.ToBytes(), peer); err != nil {
			log.Error(err, "failed to send DHCP")
			c = codes.Error
			cmsg = fmt.Sprintf("failed to send DHCP: %v, err: %v", reply.MessageType().String(), err.Error())
		} else {
			log.Info("sent DHCP response")
			cmsg = fmt.Sprintf("sent DHCP: %s", reply.MessageType().String())
		}
	}
	span.SetStatus(c, cmsg)
}

// handleDiscover handles DHCP packets message type of discover.
func (s *Server) handleDiscover(ctx context.Context, tracer trace.Tracer, m *dhcpv4.DHCPv4) *dhcpv4.DHCPv4 {
	log := s.Log.WithValues("mac", m.ClientHWAddr.String())
	ctx, span := tracer.Start(ctx, "DHCP Discover Message")
	log.Info("received discover packet")
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
		attribute.String("IpxeScriptURL", fmt.Sprintf("%v", n.IpxeScriptURL)),
	)
	span.SetStatus(codes.Ok, "done reading from backend")
	span.End()

	mods := []dhcpv4.Modifier{
		dhcpv4.WithMessageType(dhcpv4.MessageTypeOffer),
		dhcpv4.WithGeneric(dhcpv4.OptionServerIdentifier, s.IPAddr.IPAddr().IP),
		dhcpv4.WithServerIP(s.IPAddr.IPAddr().IP),
	}
	mods = append(mods, s.setDHCPOpts(ctx, m, d)...)

	if s.NetbootEnabled && s.isNetbootClient(m) {
		mods = append(mods, s.setNetworkBootOpts(ctx, m, n))
	}
	reply, err := dhcpv4.NewReplyFromRequest(m, mods...)
	if err != nil {
		return nil
	}
	log.Info("sending offer packet")
	return reply
}

// handleRequest handles DHCP packets message type of request.
func (s *Server) handleRequest(ctx context.Context, tracer trace.Tracer, m *dhcpv4.DHCPv4) *dhcpv4.DHCPv4 {
	log := s.Log.WithValues("mac", m.ClientHWAddr.String())
	ctx, span := tracer.Start(ctx, "DHCP Request Message")
	log.Info("received request packet")
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
		attribute.String("IpxeScriptURL", fmt.Sprintf("%v", n.IpxeScriptURL)),
	)
	span.SetStatus(codes.Ok, "reading from backend done")
	span.End()

	mods := []dhcpv4.Modifier{
		dhcpv4.WithMessageType(dhcpv4.MessageTypeAck),
		dhcpv4.WithGeneric(dhcpv4.OptionServerIdentifier, s.Listener.UDPAddr().IP),
		dhcpv4.WithServerIP(s.IPAddr.IPAddr().IP),
	}
	mods = append(mods, s.setDHCPOpts(ctx, m, d)...)

	if s.NetbootEnabled && s.isNetbootClient(m) {
		log.Info("netboot client")
		mods = append(mods, s.setNetworkBootOpts(ctx, m, n))
	}
	reply, err := dhcpv4.NewReplyFromRequest(m, mods...)
	if err != nil {
		return nil
	}
	log.Info("sending ack packet")
	return reply
}

// handleRelease handles DHCP packets message type of release.
// Since the design of this DHCP server is that all IP addresses are
// Host reservations, when a client releases the address, the server
// doesn't have anything to do. This method is included for clarity of this
// design decision.
func (s *Server) handleRelease(_ context.Context, m *dhcpv4.DHCPv4) {
	s.Log.Info("received release, no response required", "mac", m.ClientHWAddr.String())
}

// isNetbootClient returns true if the client is a valid netboot client.
// A valid netboot client will have the following in its DHCP request:
// http://www.pix.net/software/pxeboot/archive/pxespec.pdf
//
// 1. is a DHCP discovery/request message type.
// 2. option 93 is set.
// 3. option 94 is set.
// 4. option 97 is correct length.
// 5. option 60 is set with this format: "PXEClient:Arch:xxxxx:UNDI:yyyzzz" or "HTTPClient:Arch:xxxxx:UNDI:yyyzzz".
func (s *Server) isNetbootClient(pkt *dhcpv4.DHCPv4) bool {
	// only response to DISCOVER and REQUEST packets
	if pkt.MessageType() != dhcpv4.MessageTypeDiscover && pkt.MessageType() != dhcpv4.MessageTypeRequest {
		s.Log.Info("not a netboot client", "reason", "message type must be either Discover or Request", "mac", pkt.ClientHWAddr.String(), "message type", pkt.MessageType())
		return false
	}
	// option 60 must be set
	if !pkt.Options.Has(dhcpv4.OptionClassIdentifier) {
		s.Log.Info("not a netboot client", "reason", "option 60 not set", "mac", pkt.ClientHWAddr.String())
		return false
	}
	// option 60 must start with PXEClient or HTTPClient
	opt60 := pkt.GetOneOption(dhcpv4.OptionClassIdentifier)
	if !strings.HasPrefix(string(opt60), string(pxeClient)) && !strings.HasPrefix(string(opt60), string(httpClient)) {
		s.Log.Info("not a netboot client", "reason", "option 60 not PXEClient or HTTPClient", "mac", pkt.ClientHWAddr.String(), "option 60", string(opt60))
		return false
	}

	// option 93 must be set
	if !pkt.Options.Has(dhcpv4.OptionClientSystemArchitectureType) {
		s.Log.Info("not a netboot client", "reason", "option 93 not set", "mac", pkt.ClientHWAddr.String())
		return false
	}

	// option 94 must be set
	if !pkt.Options.Has(dhcpv4.OptionClientNetworkInterfaceIdentifier) {
		s.Log.Info("not a netboot client", "reason", "option 94 not set", "mac", pkt.ClientHWAddr.String())
		return false
	}

	// option 97 must be have correct length or not be set
	guid := pkt.GetOneOption(dhcpv4.OptionClientMachineIdentifier)
	switch len(guid) {
	case 0:
		// A missing GUID is invalid according to the spec, however
		// there are PXE ROMs in the wild that omit the GUID and still
		// expect to boot. The only thing we do with the GUID is
		// mirror it back to the client if it's there, so we might as
		// well accept these buggy ROMs.
	case 17:
		if guid[0] != 0 {
			s.Log.Info("not a netboot client", "reason", "option 97 does not start with 0", "mac", pkt.ClientHWAddr.String(), "option 97", string(guid))
			return false
		}
	default:
		s.Log.Info("not a netboot client", "reason", "option 97 has invalid length (0 or 17)", "mac", pkt.ClientHWAddr.String(), "option 97", string(guid))
		return false
	}
	return true
}

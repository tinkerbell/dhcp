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
)

const tracerName = "github.com/tinkerbell/dhcp"

// BackendReader is the interface that wraps the Read method.
//
// Backends implement this interface to provide DHCP data to the DHCP server.
type BackendReader interface {
	// Read data (from a backend) based on a mac address
	// and return DHCP headers and options, including netboot info.
	Read(context.Context, net.HardwareAddr) (*data.DHCP, *data.Netboot, error)
}

// handleFunc is the main handler for DHCPv4 packets.
func (s *Server) handleFunc(conn net.PacketConn, peer net.Addr, m *dhcpv4.DHCPv4) {
	log := s.Log.WithValues("mac", m.ClientHWAddr.String())
	log.Info("received DHCP packet", "type", m.MessageType().String())
	tracer := otel.Tracer(tracerName)
	ctx, span := tracer.Start(s.ctx, fmt.Sprintf("DHCP Packet Received: %v", m.MessageType().String()))
	defer span.End()

	var reply []byte
	switch mt := m.MessageType(); mt {
	case dhcpv4.MessageTypeDiscover:
		d, n, err := s.readBackend(ctx, m)
		if err != nil {
			log.Error(err, "error reading from backend")
			span.SetStatus(codes.Error, err.Error())

			return
		}

		reply = s.updateMsg(ctx, m, d, n, dhcpv4.MessageTypeOffer)
		log = log.WithValues("type", dhcpv4.MessageTypeOffer.String())
	case dhcpv4.MessageTypeRequest:
		d, n, err := s.readBackend(ctx, m)
		if err != nil {
			log.Error(err, "error reading from backend")
			span.SetStatus(codes.Error, err.Error())

			return
		}
		reply = s.updateMsg(ctx, m, d, n, dhcpv4.MessageTypeAck)
		log = log.WithValues("type", dhcpv4.MessageTypeAck.String())
	case dhcpv4.MessageTypeRelease:
		// Since the design of this DHCP server is that all IP addresses are
		// Host reservations, when a client releases an address, the server
		// doesn't have anything to do. This case is included for clarity of this
		// design decision.
		log.Info("received release, no response required")
		span.SetStatus(codes.Ok, "received release, no response required")

		return
	default:
		log.Info("received unknown message type")
		span.SetStatus(codes.Error, "received unknown message type")

		return
	}

	if _, err := conn.WriteTo(reply, peer); err != nil {
		log.Error(err, "failed to send DHCP")
		span.SetStatus(codes.Error, err.Error())

		return
	}

	log.Info("sent DHCP response")
	span.SetAttributes(encodeToAttributes(reply)...)
	span.SetStatus(codes.Ok, "sent DHCP response")
}

// readBackend encapsulates the backend read and opentelemetry handling.
func (s *Server) readBackend(ctx context.Context, m *dhcpv4.DHCPv4) (*data.DHCP, *data.Netboot, error) {
	tracer := otel.Tracer(tracerName)
	ctx, span := tracer.Start(ctx, "Hardware data get")
	defer span.End()

	d, n, err := s.Backend.Read(ctx, m.ClientHWAddr)
	if err != nil {
		s.Log.Info("error getting DHCP data from backend", "mac", m.ClientHWAddr, "error", err)
		span.SetStatus(codes.Error, err.Error())

		return nil, nil, err
	}

	span.SetAttributes(
		attribute.String("messageType", m.MessageType().String()),
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

	return d, n, nil
}

// updateMsg handles updating DHCP packets with the data from the backend.
func (s *Server) updateMsg(ctx context.Context, m *dhcpv4.DHCPv4, d *data.DHCP, n *data.Netboot, msgType dhcpv4.MessageType) []byte {
	mods := []dhcpv4.Modifier{
		dhcpv4.WithMessageType(msgType),
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

	return reply.ToBytes()
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

// encodeToAttributes takes a DHCP packet in byte form and return opentelemetry key/value attributes.
func encodeToAttributes(pkt []byte) []attribute.KeyValue {
	d, err := dhcpv4.FromBytes(pkt)
	if err != nil {
		return []attribute.KeyValue{}
	}

	var ns []string
	for _, e := range d.DNS() {
		ns = append(ns, e.String())
	}

	var ntp []string
	for _, e := range d.NTPServers() {
		ntp = append(ntp, e.String())
	}

	var ds []string
	if l := d.DomainSearch(); l != nil {
		ds = append(ds, l.Labels...)
	}

	var routers []string
	for _, e := range d.Router() {
		routers = append(routers, e.String())
	}

	var sm string
	if d.SubnetMask() != nil {
		sm = net.IP(d.SubnetMask()).String()
	}

	// this is needed because dhcpv4.DHCPv4.Options don't get zero values like top level struct values do.
	var ba string
	if d.BroadcastAddress() != nil {
		ba = d.BroadcastAddress().String()
	}

	var si string
	if d.ServerIdentifier() != nil {
		si = d.ServerIdentifier().String()
	}

	return []attribute.KeyValue{
		attribute.String("DHCP.Header.yiaddr", d.YourIPAddr.String()),
		attribute.String("DHCP.Header.siaddr", d.ServerIPAddr.String()),
		attribute.String("DHCP.Header.chaddr", d.ClientHWAddr.String()),
		attribute.String("DHCP.Header.file", d.BootFileName),
		attribute.String("DHCP.Opt1.SubnetMask", sm),
		attribute.String("DHCP.Opt3.DefaultGateway", strings.Join(routers, ",")),
		attribute.String("DHCP.Opt6.NameServers", strings.Join(ns, ",")),
		attribute.String("DHCP.Opt12.Hostname", d.HostName()),
		attribute.String("DHCP.Opt15.DomainName", d.DomainName()),
		attribute.String("DHCP.Opt28.BroadcastAddress", ba),
		attribute.String("DHCP.Opt42.NTPServers", strings.Join(ntp, ",")),
		attribute.Float64("DHCP.Opt51.LeaseTime", d.IPAddressLeaseTime(0).Seconds()),
		attribute.String("DHCP.Opt53.MessageType", d.MessageType().String()),
		attribute.String("DHCP.Opt54.ServerIdentifier", si),
		attribute.String("DHCP.Opt119.DomainSearch", strings.Join(ds, ",")),
	}
}

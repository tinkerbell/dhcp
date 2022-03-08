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
	ctx, span := tracer.Start(s.ctx,
		fmt.Sprintf("DHCP Packet Received: %v", m.MessageType().String()),
		trace.WithAttributes(s.encodeToAttributes(m, "request")...),
	)
	defer span.End()

	var reply *dhcpv4.DHCPv4
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

	if _, err := conn.WriteTo(reply.ToBytes(), peer); err != nil {
		log.Error(err, "failed to send DHCP")
		span.SetStatus(codes.Error, err.Error())

		return
	}

	log.Info("sent DHCP response")
	span.SetAttributes(s.encodeToAttributes(reply, "reply")...)
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

	span.SetAttributes(d.EncodeToAttributes()...)
	span.SetAttributes(n.EncodeToAttributes()...)
	span.SetStatus(codes.Ok, "done reading from backend")

	return d, n, nil
}

// updateMsg handles updating DHCP packets with the data from the backend.
func (s *Server) updateMsg(ctx context.Context, m *dhcpv4.DHCPv4, d *data.DHCP, n *data.Netboot, msgType dhcpv4.MessageType) *dhcpv4.DHCPv4 {
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

	return reply
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

// encodeToAttributes takes a DHCP packet and returns opentelemetry key/value attributes.
func (s *Server) encodeToAttributes(d *dhcpv4.DHCPv4, namespace string) []attribute.KeyValue {
	a := &encoder{log: s.Log}
	all := []func(d *dhcpv4.DHCPv4, namespace string) error{
		a.encodeYIADDR, a.encodeSIADDR,
		a.encodeCHADDR, a.encodeFILE,
		a.encodeOpt1, a.encodeOpt3, a.encodeOpt6,
		a.encodeOpt12, a.encodeOpt15, a.setOpt28,
		a.encodeOpt42, a.encodeOpt51, a.encodeOpt53,
		a.encodeOpt54, a.encodeOpt119,
	}
	return a.encode(d, namespace, all...)
}

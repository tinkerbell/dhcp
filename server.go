package dhcp

import (
	"context"
	"net"

	"github.com/go-logr/logr"
	"github.com/insomniacslk/dhcp/dhcpv4"
	"github.com/insomniacslk/dhcp/dhcpv4/server4"
	"github.com/tinkerbell/dhcp/data"
	"golang.org/x/net/ipv4"
)

// Handler is a type that defines the handler function to be called every time a
// valid DHCPv4 message is received
// type Handler func(ctx context.Context, conn net.PacketConn, d data.Packet).
type Handler interface {
	Handle(ctx context.Context, conn *ipv4.PacketConn, d data.Packet)
}

// Server represents a DHCPv4 server object.
type Server struct {
	Conn     net.PacketConn
	Handlers []Handler
	Logger   logr.Logger
}

// Serve serves requests.
func (s *Server) Serve(ctx context.Context) error {
	go func() {
		<-ctx.Done()
		_ = s.Close()
	}()
	s.Logger.Info("Server listening on", "addr", s.Conn.LocalAddr())

	nConn := ipv4.NewPacketConn(s.Conn)
	if err := nConn.SetControlMessage(ipv4.FlagInterface, true); err != nil {
		s.Logger.Info("error setting control message", "err", err)
		return err
	}

	defer func() {
		_ = nConn.Close()
		_ = s.Close()
	}()
	for {
		rbuf := make([]byte, 4096)
		n, cm, peer, err := nConn.ReadFrom(rbuf)
		if err != nil {
			select {
			case <-ctx.Done():
				return nil
			default:
			}
			s.Logger.Info("error reading from packet conn", "err", err)
			return err
		}

		m, err := dhcpv4.FromBytes(rbuf[:n])
		if err != nil {
			s.Logger.Info("error parsing DHCPv4 request", "err", err)
			continue
		}

		upeer, ok := peer.(*net.UDPAddr)
		if !ok {
			s.Logger.Info("not a UDP connection? Peer is", "peer", peer)
			continue
		}
		// Set peer to broadcast if the client did not have an IP.
		if upeer.IP == nil || upeer.IP.To4().Equal(net.IPv4zero) {
			upeer = &net.UDPAddr{
				IP:   net.IPv4bcast,
				Port: upeer.Port,
			}
		}

		var ifName string
		if n, err := net.InterfaceByIndex(cm.IfIndex); err == nil {
			ifName = n.Name
		}

		for _, handler := range s.Handlers {
			go handler.Handle(ctx, nConn, data.Packet{Peer: upeer, Pkt: m, Md: &data.Metadata{IfName: ifName, IfIndex: cm.IfIndex}})
		}
	}
}

// Close sends a termination request to the server, and closes the UDP listener.
func (s *Server) Close() error {
	return s.Conn.Close()
}

// ServerOpt adds optional configuration to a server.
type ServerOpt func(s *Server)

// WithConn configures the server with the given connection.
func WithConn(c net.PacketConn) ServerOpt {
	return func(s *Server) {
		s.Conn = c
	}
}

// NewServer initializes and returns a new Server object.
func NewServer(ifname string, addr *net.UDPAddr, handler []Handler, opt ...ServerOpt) (*Server, error) {
	s := &Server{
		Handlers: handler,
		Logger:   logr.Discard(),
	}

	for _, o := range opt {
		o(s)
	}
	if s.Conn == nil {
		var err error
		conn, err := server4.NewIPv4UDPConn(ifname, addr)
		if err != nil {
			return nil, err
		}
		s.Conn = conn
	}
	return s, nil
}

// WithLogger set the logger (see interface Logger).
func WithLogger(newLogger logr.Logger) ServerOpt {
	return func(s *Server) {
		s.Logger = newLogger
	}
}

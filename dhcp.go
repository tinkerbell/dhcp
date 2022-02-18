// Package dhcp provides a DHCPv4 server implementation.
package dhcp

import (
	"context"
	"fmt"
	"net"
	"net/url"
	"reflect"

	"github.com/go-logr/logr"
	"github.com/imdario/mergo"
	"github.com/insomniacslk/dhcp/dhcpv4/server4"
	"github.com/tinkerbell/dhcp/data"
	"golang.org/x/sync/errgroup"
	"inet.af/netaddr"
)

// Server holds the configuration details for the running the DHCP server.
type Server struct {
	// ctx in a struct is not generally the best way to handle context (see https://pkg.go.dev/context),
	// but with the way handlers are written in github.com/insomniacslk/dhcp/dhcpv4
	// this is the only way to get the context into the handlers.
	ctx context.Context

	// Log is used to log messages.
	// `logr.Discard()` can be used if no logging is desired.
	Log logr.Logger

	// Listener collects an IP and port.
	// The port is combined with 0.0.0.0 to listen for broadcast traffic.
	// The IP is used to find the network interface to listen on for DHCP requests.
	Listener netaddr.IPPort

	// IPAddr is the IP address to use in DHCP responses.
	// Option 54 and the sname DHCP header.
	// This could be a load balancer IP address or an ingress IP address or a local IP address.
	IPAddr netaddr.IP

	// iPXE binary server IP:Port serving via TFTP.
	IPXEBinServerTFTP netaddr.IPPort

	// IPXEBinServerHTTP is the URL to the IPXE binary server serving via HTTP(s).
	IPXEBinServerHTTP *url.URL

	// IPXEScriptURL is the URL to the IPXE script to use.
	IPXEScriptURL *url.URL

	// NetbootEnabled is whether to enable sending netboot DHCP options.
	NetbootEnabled bool

	// UserClass (for network booting) allows a custom DHCP option 77 to be used to break out of an iPXE loop.
	UserClass UserClass

	// Backend is the backend to use for getting DHCP data.
	Backend BackendReader

	// OTELEnabled is used to determine if netboot options include otel naming.
	// When true, the netboot filename will be appended with otel information.
	// For example, the filename will be "snp.efi-00-23b1e307bb35484f535a1f772c06910e-d887dc3912240434-01".
	// <original filename>-00-<trace id>-<span id>-<trace flags>
	OTELEnabled bool
}

// ListenAndServe will listen and serve DHCP server.
//
// Default listen port is ":67".
//
// Override the defaults by setting the Server struct fields.
func (s *Server) ListenAndServe(ctx context.Context) error {
	defaults := &Server{
		ctx:            ctx,
		Log:            logr.Discard(),
		Listener:       netaddr.IPPortFrom(netaddr.IPv4(0, 0, 0, 0), 67),
		IPAddr:         defaultIP(),
		NetbootEnabled: true,
		Backend:        &alwaysError{},
	}

	err := mergo.Merge(s, defaults, mergo.WithTransformers(s))
	if err != nil {
		return err
	}
	// for broadcast traffic we need to listen on all IPs
	conn := &net.UDPAddr{
		IP:   net.ParseIP("0.0.0.0"),
		Port: s.Listener.UDPAddr().Port,
	}

	s.ctx = ctx
	// server4.NewServer() will isolate listening to the specific interface.
	srv, err := server4.NewServer(getInterfaceByIP(s.Listener.IP().String()), conn, s.handleFunc)
	if err != nil {
		return err
	}

	g, ctx := errgroup.WithContext(ctx)
	g.Go(func() error {
		s.Log.Info("starting DHCP server", "port", s.Listener.Port(), "interface", s.IPAddr.String())
		return srv.Serve()
	})

	<-ctx.Done()
	_ = srv.Close()

	return g.Wait()
}

// Serve run the DHCP server using the given PacketConn.
func (s *Server) Serve(ctx context.Context, conn net.PacketConn) error {
	defaults := &Server{
		ctx:            ctx,
		Log:            logr.Discard(),
		Listener:       netaddr.IPPortFrom(netaddr.IPv4(0, 0, 0, 0), 67),
		IPAddr:         defaultIP(),
		NetbootEnabled: true,
		Backend:        &alwaysError{},
	}

	err := mergo.Merge(s, defaults, mergo.WithTransformers(s))
	if err != nil {
		return err
	}

	s.ctx = ctx
	// server4.NewServer() will isolate listening to the specific interface.
	srv, err := server4.NewServer("", nil, s.handleFunc, server4.WithConn(conn))
	if err != nil {
		return err
	}

	g, ctx := errgroup.WithContext(ctx)
	g.Go(func() error {
		return srv.Serve()
	})

	<-ctx.Done()
	_ = srv.Close()

	return g.Wait()
}

// getInterfaceByIP returns the interface with the given IP address or an empty string.
func getInterfaceByIP(ip string) string {
	ifaces, err := net.Interfaces()
	if err != nil {
		return ""
	}
	for _, iface := range ifaces {
		addrs, err := iface.Addrs()
		if err != nil {
			continue
		}
		for _, addr := range addrs {
			if ipnet, ok := addr.(*net.IPNet); ok {
				if ipnet.IP.String() == ip {
					return iface.Name
				}
			}
		}
	}
	return ""
}

// Transformer for merging the netaddr.IPPort and logr.Logger structs.
func (s *Server) Transformer(typ reflect.Type) func(dst, src reflect.Value) error {
	switch typ {
	case reflect.TypeOf(logr.Logger{}):
		return func(dst, src reflect.Value) error {
			if dst.CanSet() {
				isZero := dst.MethodByName("GetSink")
				result := isZero.Call(nil)
				if result[0].IsNil() {
					dst.Set(src)
				}
			}
			return nil
		}
	case reflect.TypeOf(netaddr.IPPort{}):
		return func(dst, src reflect.Value) error {
			if dst.CanSet() {
				isZero := dst.MethodByName("IsZero")
				result := isZero.Call([]reflect.Value{})
				if result[0].Bool() {
					dst.Set(src)
				}
			}
			return nil
		}
	case reflect.TypeOf(netaddr.IP{}):
		return func(dst, src reflect.Value) error {
			if dst.CanSet() {
				isZero := dst.MethodByName("IsZero")
				result := isZero.Call([]reflect.Value{})
				if result[0].Bool() {
					dst.Set(src)
				}
			}
			return nil
		}
	}
	return nil
}

// defaultIP will return either the default IP associated with default route or 0.0.0.0.
func defaultIP() netaddr.IP {
	addrs, err := net.InterfaceAddrs()
	if err != nil {
		return netaddr.IPv4(0, 0, 0, 0)
	}
	for _, addr := range addrs {
		ip, ok := addr.(*net.IPNet)
		if !ok {
			continue
		}
		v4 := ip.IP.To4()
		if v4 == nil || !v4.IsGlobalUnicast() {
			continue
		}

		if i, ok := netaddr.FromStdIP(v4); ok {
			return i
		}
	}
	return netaddr.IPv4(0, 0, 0, 0)
}

type alwaysError struct{}

// ErrNilBackend is used when the backend is not specified.
var ErrNilBackend = fmt.Errorf("please specify a backend")

func (*alwaysError) Read(context.Context, net.HardwareAddr) (*data.DHCP, *data.Netboot, error) {
	return nil, nil, ErrNilBackend
}

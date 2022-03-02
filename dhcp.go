// Package dhcp provides a DHCPv4 server implementation.
package dhcp

import (
	"context"
	"net/url"

	"github.com/go-logr/logr"
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

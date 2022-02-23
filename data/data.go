// Package data is an interface between DHCP backend implementations and the DHCP server.
package data

import (
	"net"
	"net/url"

	"inet.af/netaddr"
)

// Dhcp holds the headers and options available to be set in a DHCP server response.
type Dhcp struct {
	MacAddress       net.HardwareAddr // chaddr DHCP header.
	IPAddress        netaddr.IP       // yiaddr DHCP header.
	SubnetMask       net.IPMask       // DHCP option 1.
	DefaultGateway   netaddr.IP       // DHCP option 3.
	NameServers      []net.IP         // DHCP option 6.
	Hostname         string           // DHCP option 12.
	DomainName       string           // DHCP option 15.
	BroadcastAddress netaddr.IP       // DHCP option 28.
	NTPServers       []net.IP         // DHCP option 42.
	LeaseTime        uint32           // DHCP option 51.
	DomainSearch     []string         // DHCP option 119.
}

// Netboot holds info used in netbooting a client.
type Netboot struct {
	AllowNetboot  bool     // If true, the client will be provided netboot options in the DHCP offer/ack.
	IpxeScriptURL *url.URL // Overrides a default value that is passed into DHCP on startup.
}

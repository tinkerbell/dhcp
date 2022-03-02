// Package data is an interface between DHCP backend implementations and the DHCP server.
package data

import (
	"net"
	"net/url"
	"strings"

	"go.opentelemetry.io/otel/attribute"
	"inet.af/netaddr"
)

// DHCP holds the headers and options available to be set in a DHCP server response.
// This is the API between the DHCP server and a backend.
type DHCP struct {
	MACAddress       net.HardwareAddr // chaddr DHCP header.
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
	IPXEScriptURL *url.URL // Overrides a default value that is passed into DHCP on startup.
}

// EncodeToAttributes returns a slice of opentelemetry attributes that can be used to set span.SetAttributes.
func (d *DHCP) EncodeToAttributes() []attribute.KeyValue {
	var ns []string
	for _, e := range d.NameServers {
		ns = append(ns, e.String())
	}

	var ntp []string
	for _, e := range d.NTPServers {
		ntp = append(ntp, e.String())
	}

	var ip string
	if !d.IPAddress.IsZero() {
		ip = d.IPAddress.String()
	}

	var sm string
	if d.SubnetMask != nil {
		sm = net.IP(d.SubnetMask).String()
	}

	var dfg string
	if !d.DefaultGateway.IsZero() {
		dfg = d.DefaultGateway.String()
	}

	var ba string
	if !d.BroadcastAddress.IsZero() {
		ba = d.BroadcastAddress.String()
	}

	return []attribute.KeyValue{
		attribute.String("DHCP.MACAddress", d.MACAddress.String()),
		attribute.String("DHCP.IPAddress", ip),
		attribute.String("DHCP.SubnetMask", sm),
		attribute.String("DHCP.DefaultGateway", dfg),
		attribute.String("DHCP.NameServers", strings.Join(ns, ",")),
		attribute.String("DHCP.Hostname", d.Hostname),
		attribute.String("DHCP.DomainName", d.DomainName),
		attribute.String("DHCP.BroadcastAddress", ba),
		attribute.String("DHCP.NTPServers", strings.Join(ntp, ",")),
		attribute.Int64("DHCP.LeaseTime", int64(d.LeaseTime)),
		attribute.String("DHCP.DomainSearch", strings.Join(d.DomainSearch, ",")),
	}
}

// EncodeToAttributes returns a slice of opentelemetry attributes that can be used to set span.SetAttributes.
func (n *Netboot) EncodeToAttributes() []attribute.KeyValue {
	var s string
	if n.IPXEScriptURL != nil {
		s = n.IPXEScriptURL.String()
	}
	return []attribute.KeyValue{
		attribute.Bool("Netboot.AllowNetboot", n.AllowNetboot),
		attribute.String("Netboot.IPXEScriptURL", s),
	}
}

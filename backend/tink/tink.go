// Package tink reads hardware data from a Tink server.
package tink

import (
	"context"
	"fmt"
	"net"
	"net/url"

	"github.com/go-logr/logr"
	"github.com/tinkerbell/dhcp/data"
	"github.com/tinkerbell/tink/protos/hardware"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/codes"
	"inet.af/netaddr"
)

const tracerName = "github.com/tinkerbell/dhcp"

var (
	errRecordNotFound = fmt.Errorf("record not found for mac address")
	errParseIP        = fmt.Errorf("failed to parse IP")
	errParseSubnet    = fmt.Errorf("failed to parse subnet mask")
	errParseURL       = fmt.Errorf("failed to parse URL")
)

// Config for tink backend.
type Config struct {
	Log    logr.Logger
	Client hardware.HardwareServiceClient
}

// Read is the Tink implementation of the Backend interface.
func (c *Config) Read(ctx context.Context, mac net.HardwareAddr) (*data.DHCP, *data.Netboot, error) {
	tracer := otel.Tracer(tracerName)
	_, span := tracer.Start(ctx, "backend.tink.Read")
	defer span.End()

	h, err := c.Client.ByMAC(ctx, &hardware.GetRequest{Mac: mac.String()})
	if err != nil {
		span.SetStatus(codes.Error, err.Error())

		return nil, nil, err
	}

	for _, e := range h.GetNetwork().Interfaces {
		if e.GetDhcp().Mac == mac.String() {
			d, n, err := c.translate(e.GetDhcp(), e.GetNetboot())
			if err != nil {
				span.SetStatus(codes.Error, err.Error())

				return nil, nil, err
			}
			span.SetStatus(codes.Ok, "")

			return d, n, nil
		}
	}

	err = fmt.Errorf("%w: %s", errRecordNotFound, mac.String())
	span.SetStatus(codes.Error, err.Error())

	return nil, nil, err
}

func (c *Config) translate(h *hardware.Hardware_DHCP, n *hardware.Hardware_Netboot) (*data.DHCP, *data.Netboot, error) {
	dhcp := new(data.DHCP)
	netboot := new(data.Netboot)

	// ip address, required
	ip, err := netaddr.ParseIP(h.GetIp().GetAddress())
	if err != nil {
		return nil, nil, fmt.Errorf("%v: %w", err, errParseIP)
	}
	dhcp.IPAddress = ip

	// subnet mask, required
	sm, err := netaddr.ParseIP(h.GetIp().Netmask)
	if err != nil {
		return nil, nil, fmt.Errorf("%v: %w", err, errParseSubnet)
	}
	dhcp.SubnetMask = sm.IPAddr().IP.DefaultMask()

	// default gateway, optional
	if dg, err := netaddr.ParseIP(h.GetIp().Gateway); err != nil {
		c.Log.Info("failed to parse default gateway", "defaultGateway", h.GetIp().Gateway, "err", err)
	} else {
		dhcp.DefaultGateway = dg
	}

	// name servers, optional
	for _, s := range h.GetNameServers() {
		ip := net.ParseIP(s)
		if ip == nil {
			c.Log.Info("failed to parse name server", "nameServer", s)
			break
		}
		dhcp.NameServers = append(dhcp.NameServers, ip)
	}

	// hostname, optional
	dhcp.Hostname = h.GetHostname()

	// ntp servers, optional
	for _, s := range h.GetTimeServers() {
		ip := net.ParseIP(s)
		if ip == nil {
			c.Log.Info("failed to parse ntp server", "ntpServer", s)
			break
		}
		dhcp.NTPServers = append(dhcp.NTPServers, ip)
	}

	// lease time
	dhcp.LeaseTime = uint32(h.GetLeaseTime())

	// allow machine to netboot
	netboot.AllowNetboot = n.GetAllowPxe()

	// ipxe script url is optional but if provided, it must be a valid url
	if n.GetIpxe().GetUrl() != "" {
		u, err := url.Parse(n.GetIpxe().GetUrl())
		if err != nil {
			return nil, nil, fmt.Errorf("%v: %w", err, errParseURL)
		}
		netboot.IPXEScriptURL = u
	}

	return dhcp, netboot, nil
}

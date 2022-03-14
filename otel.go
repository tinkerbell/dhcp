package dhcp

import (
	"fmt"
	"net"
	"strings"

	"github.com/go-logr/logr"
	"github.com/insomniacslk/dhcp/dhcpv4"
	"go.opentelemetry.io/otel/attribute"
)

type encoder struct {
	log        logr.Logger
	attributes []attribute.KeyValue
}

type encodeError struct {
	key string
	err error
}

func (e *encodeError) name() string {
	return e.key
}

func (e *encodeError) Error() string {
	if e.err != nil {
		return e.err.Error()
	}
	return ""
}

func (e *encodeError) Is(target error) bool {
	_, ok := target.(*encodeError)
	return ok
}

// encode runs a slice of encoders against a DHCPv4 packet turning the values into opentelemetry attribute key/value pairs.
func (e *encoder) encode(d *dhcpv4.DHCPv4, namespace string, encoders ...func(pkt *dhcpv4.DHCPv4, namespace string) error) []attribute.KeyValue {
	for _, elem := range encoders {
		if err := elem(d, namespace); err != nil {
			type attr interface {
				name() string
			}
			keysAndValues := []interface{}{"error", err}
			a, ok := err.(attr)
			if ok {
				keysAndValues = append(keysAndValues, "attributeKey", a.name())
			}
			e.log.V(1).Info("opentelemetry attribute not added", keysAndValues...)
		}
	}

	return e.attributes
}

func (e *encoder) encodeOpt1(d *dhcpv4.DHCPv4, namespace string) error {
	key := fmt.Sprintf("DHCP.%v.Opt1.SubnetMask", namespace)
	if d != nil && d.SubnetMask() != nil {
		sm := net.IP(d.SubnetMask()).String()
		e.attributes = append(e.attributes, attribute.String(key, sm))
		return nil
	}

	return &encodeError{err: fmt.Errorf("subnetMask not found in packet"), key: key}
}

func (e *encoder) encodeOpt3(d *dhcpv4.DHCPv4, namespace string) error {
	key := fmt.Sprintf("DHCP.%v.Opt3.DefaultGateway", namespace)
	if d != nil {
		var routers []string
		for _, e := range d.Router() {
			routers = append(routers, e.String())
		}
		if len(routers) > 0 {
			e.attributes = append(e.attributes, attribute.String(key, strings.Join(routers, ",")))
			return nil
		}
	}

	return &encodeError{err: fmt.Errorf("defaultGateway not found in packet"), key: key}
}

func (e *encoder) encodeOpt6(d *dhcpv4.DHCPv4, namespace string) error {
	key := fmt.Sprintf("DHCP.%v.Opt6.NameServers", namespace)
	if d != nil {
		var ns []string
		for _, e := range d.DNS() {
			ns = append(ns, e.String())
		}
		if len(ns) > 0 {
			e.attributes = append(e.attributes, attribute.String(key, strings.Join(ns, ",")))
			return nil
		}
	}

	return &encodeError{err: fmt.Errorf("nameServers not found in packet"), key: key}
}

func (e *encoder) encodeOpt12(d *dhcpv4.DHCPv4, namespace string) error {
	key := fmt.Sprintf("DHCP.%v.Opt12.Hostname", namespace)
	if d != nil && d.HostName() != "" {
		e.attributes = append(e.attributes, attribute.String(key, d.HostName()))
		return nil
	}

	return &encodeError{err: fmt.Errorf("hostname not found in packet"), key: key}
}

func (e *encoder) encodeOpt15(d *dhcpv4.DHCPv4, namespace string) error {
	key := fmt.Sprintf("DHCP.%v.Opt15.DomainName", namespace)
	if d != nil && d.DomainName() != "" {
		e.attributes = append(e.attributes, attribute.String(key, d.DomainName()))
		return nil
	}

	return &encodeError{err: fmt.Errorf("domainName not found in packet"), key: key}
}

func (e *encoder) setOpt28(d *dhcpv4.DHCPv4, namespace string) error {
	key := fmt.Sprintf("DHCP.%v.Opt28.BroadcastAddress", namespace)
	if d != nil && d.BroadcastAddress() != nil {
		e.attributes = append(e.attributes, attribute.String(key, d.BroadcastAddress().String()))
		return nil
	}

	return &encodeError{err: fmt.Errorf("broadcastAddress not found in packet"), key: key}
}

func (e *encoder) encodeOpt42(d *dhcpv4.DHCPv4, namespace string) error {
	key := fmt.Sprintf("DHCP.%v.Opt42.NTPServers", namespace)
	if d != nil {
		var ntp []string
		for _, e := range d.NTPServers() {
			ntp = append(ntp, e.String())
		}
		if len(ntp) > 0 {
			e.attributes = append(e.attributes, attribute.String(key, strings.Join(ntp, ",")))
			return nil
		}
	}

	return &encodeError{err: fmt.Errorf("ntpServers not found in packet"), key: key}
}

func (e *encoder) encodeOpt51(d *dhcpv4.DHCPv4, namespace string) error {
	key := fmt.Sprintf("DHCP.%v.Opt51.LeaseTime", namespace)
	if d != nil && d.IPAddressLeaseTime(0) != 0 {
		e.attributes = append(e.attributes, attribute.Float64(key, d.IPAddressLeaseTime(0).Seconds()))
		return nil
	}

	return &encodeError{err: fmt.Errorf("leaseTime not found in packet"), key: key}
}

func (e *encoder) encodeOpt53(d *dhcpv4.DHCPv4, namespace string) error {
	key := fmt.Sprintf("DHCP.%v.Opt53.MessageType", namespace)
	if d != nil && d.MessageType() != dhcpv4.MessageTypeNone {
		e.attributes = append(e.attributes, attribute.String(key, d.MessageType().String()))
		return nil
	}

	return &encodeError{err: fmt.Errorf("messageType not found in packet"), key: key}
}

func (e *encoder) encodeOpt54(d *dhcpv4.DHCPv4, namespace string) error {
	key := fmt.Sprintf("DHCP.%v.Opt54.ServerIdentifier", namespace)
	if d != nil && d.ServerIdentifier() != nil {
		e.attributes = append(e.attributes, attribute.String(key, d.ServerIdentifier().String()))
		return nil
	}

	return &encodeError{err: fmt.Errorf("serverIdentifier not found in packet"), key: key}
}

func (e *encoder) encodeOpt119(d *dhcpv4.DHCPv4, namespace string) error {
	key := fmt.Sprintf("DHCP.%v.Opt119.DomainSearch", namespace)
	if d != nil {
		if l := d.DomainSearch(); l != nil {
			e.attributes = append(e.attributes, attribute.String(key, strings.Join(l.Labels, ",")))
			return nil
		}
	}

	return &encodeError{err: fmt.Errorf("domainSearch not found in packet"), key: key}
}

func (e *encoder) encodeYIADDR(d *dhcpv4.DHCPv4, namespace string) error {
	key := fmt.Sprintf("DHCP.%v.Header.yiaddr", namespace)
	if d != nil && d.YourIPAddr != nil {
		e.attributes = append(e.attributes, attribute.String(key, d.YourIPAddr.String()))
		return nil
	}

	return &encodeError{err: fmt.Errorf("yiaddr not found in packet"), key: key}
}

func (e *encoder) encodeSIADDR(d *dhcpv4.DHCPv4, namespace string) error {
	key := fmt.Sprintf("DHCP.%v.Header.siaddr", namespace)
	if d != nil && d.ServerIPAddr != nil {
		e.attributes = append(e.attributes, attribute.String(key, d.ServerIPAddr.String()))
		return nil
	}

	return &encodeError{err: fmt.Errorf("siaddr not found in packet"), key: key}
}

func (e *encoder) encodeCHADDR(d *dhcpv4.DHCPv4, namespace string) error {
	key := fmt.Sprintf("DHCP.%v.Header.chaddr", namespace)
	if d != nil && d.ClientHWAddr != nil {
		e.attributes = append(e.attributes, attribute.String(key, d.ClientHWAddr.String()))
		return nil
	}

	return &encodeError{err: fmt.Errorf("chaddr not found in packet"), key: key}
}

func (e *encoder) encodeFILE(d *dhcpv4.DHCPv4, namespace string) error {
	key := fmt.Sprintf("DHCP.%v.Header.file", namespace)
	if d != nil && d.BootFileName != "" {
		e.attributes = append(e.attributes, attribute.String(key, d.BootFileName))
		return nil
	}

	return &encodeError{err: fmt.Errorf("bootfile not found in packet"), key: key}
}

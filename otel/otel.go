// Package otel handles translating DHCP headers and options to otel key/value attributes.
package otel

import (
	"context"
	"fmt"
	"net"
	"strings"

	"github.com/go-logr/logr"
	"github.com/insomniacslk/dhcp/dhcpv4"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/trace"
)

const keyNamespace = "DHCP"

// Encoder holds the otel key/value attributes.
type Encoder struct {
	Log        logr.Logger
	Attributes []attribute.KeyValue
}

/*type encodeError struct {
	key string
	err error
}*/

type optNotFoundError struct {
	optName string
}

func (e *optNotFoundError) Error() string {
	return fmt.Sprintf("%q not found in DHCP packet", e.optName)
}

func (e *optNotFoundError) found() bool {
	return true
}

type found interface {
	found() bool
}

// OptNotFound returns true if err is an option not found error.
func OptNotFound(err error) bool {
	te, ok := err.(found)
	return ok && te.found()
}

// Encode runs a slice of encoders against a DHCPv4 packet turning the values into opentelemetry attribute key/value pairs.
func (e *Encoder) Encode(d *dhcpv4.DHCPv4, namespace string, encoders ...func(pkt *dhcpv4.DHCPv4, namespace string) error) []attribute.KeyValue {
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
			e.Log.V(1).Info("opentelemetry attribute not added", keysAndValues...)
		}
	}

	return e.Attributes
}

// EncodeOpt1 takes DHCP Opt 1 from a DHCP packet and adds an OTEL
// key/value pair to the Encoder.Attributes. See https://www.iana.org/assignments/bootp-dhcp-parameters/bootp-dhcp-parameters.xhtml
func (e *Encoder) EncodeOpt1(d *dhcpv4.DHCPv4, namespace string) error {
	key := fmt.Sprintf("%v.%v.Opt1.SubnetMask", keyNamespace, namespace)
	if d != nil && d.SubnetMask() != nil {
		sm := net.IP(d.SubnetMask()).String()
		e.Attributes = append(e.Attributes, attribute.String(key, sm))
		return nil
	}

	return &optNotFoundError{optName: key}
}

// EncodeOpt3 takes DHCP Opt 3 from a DHCP packet and adds an OTEL
// key/value pair to the Encoder.Attributes. See https://www.iana.org/assignments/bootp-dhcp-parameters/bootp-dhcp-parameters.xhtml
func (e *Encoder) EncodeOpt3(d *dhcpv4.DHCPv4, namespace string) error {
	key := fmt.Sprintf("%v.%v.Opt3.DefaultGateway", keyNamespace, namespace)
	if d != nil {
		var routers []string
		for _, e := range d.Router() {
			routers = append(routers, e.String())
		}
		if len(routers) > 0 {
			e.Attributes = append(e.Attributes, attribute.String(key, strings.Join(routers, ",")))
			return nil
		}
	}

	return &optNotFoundError{optName: key}
}

// EncodeOpt6 takes DHCP Opt 6 from a DHCP packet and adds an OTEL
// key/value pair to the Encoder.Attributes. See https://www.iana.org/assignments/bootp-dhcp-parameters/bootp-dhcp-parameters.xhtml
func (e *Encoder) EncodeOpt6(d *dhcpv4.DHCPv4, namespace string) error {
	key := fmt.Sprintf("%v.%v.Opt6.NameServers", keyNamespace, namespace)
	if d != nil {
		var ns []string
		for _, e := range d.DNS() {
			ns = append(ns, e.String())
		}
		if len(ns) > 0 {
			e.Attributes = append(e.Attributes, attribute.String(key, strings.Join(ns, ",")))
			return nil
		}
	}

	return &optNotFoundError{optName: key}
}

// EncodeOpt12 takes DHCP Opt 12 from a DHCP packet and adds an OTEL
// key/value pair to the Encoder.Attributes. See https://www.iana.org/assignments/bootp-dhcp-parameters/bootp-dhcp-parameters.xhtml
func (e *Encoder) EncodeOpt12(d *dhcpv4.DHCPv4, namespace string) error {
	key := fmt.Sprintf("%v.%v.Opt12.Hostname", keyNamespace, namespace)
	if d != nil && d.HostName() != "" {
		e.Attributes = append(e.Attributes, attribute.String(key, d.HostName()))
		return nil
	}

	return &optNotFoundError{optName: key}
}

// EncodeOpt15 takes DHCP Opt 15 from a DHCP packet and adds an OTEL
// key/value pair to the Encoder.Attributes. See https://www.iana.org/assignments/bootp-dhcp-parameters/bootp-dhcp-parameters.xhtml
func (e *Encoder) EncodeOpt15(d *dhcpv4.DHCPv4, namespace string) error {
	key := fmt.Sprintf("%v.%v.Opt15.DomainName", keyNamespace, namespace)
	if d != nil && d.DomainName() != "" {
		e.Attributes = append(e.Attributes, attribute.String(key, d.DomainName()))
		return nil
	}

	return &optNotFoundError{optName: key}
}

// EncodeOpt28 takes DHCP Opt 28 from a DHCP packet and adds an OTEL
// key/value pair to the Encoder.Attributes. See https://www.iana.org/assignments/bootp-dhcp-parameters/bootp-dhcp-parameters.xhtml
func (e *Encoder) EncodeOpt28(d *dhcpv4.DHCPv4, namespace string) error {
	key := fmt.Sprintf("%v.%v.Opt28.BroadcastAddress", keyNamespace, namespace)
	if d != nil && d.BroadcastAddress() != nil {
		e.Attributes = append(e.Attributes, attribute.String(key, d.BroadcastAddress().String()))
		return nil
	}

	return &optNotFoundError{optName: key}
}

// EncodeOpt42 takes DHCP Opt 42 from a DHCP packet and adds an OTEL
// key/value pair to the Encoder.Attributes. See https://www.iana.org/assignments/bootp-dhcp-parameters/bootp-dhcp-parameters.xhtml
func (e *Encoder) EncodeOpt42(d *dhcpv4.DHCPv4, namespace string) error {
	key := fmt.Sprintf("%v.%v.Opt42.NTPServers", keyNamespace, namespace)
	if d != nil {
		var ntp []string
		for _, e := range d.NTPServers() {
			ntp = append(ntp, e.String())
		}
		if len(ntp) > 0 {
			e.Attributes = append(e.Attributes, attribute.String(key, strings.Join(ntp, ",")))
			return nil
		}
	}

	return &optNotFoundError{optName: key}
}

// EncodeOpt51 takes DHCP Opt 51 from a DHCP packet and adds an OTEL
// key/value pair to the Encoder.Attributes. See https://www.iana.org/assignments/bootp-dhcp-parameters/bootp-dhcp-parameters.xhtml
func (e *Encoder) EncodeOpt51(d *dhcpv4.DHCPv4, namespace string) error {
	key := fmt.Sprintf("%v.%v.Opt51.LeaseTime", keyNamespace, namespace)
	if d != nil && d.IPAddressLeaseTime(0) != 0 {
		e.Attributes = append(e.Attributes, attribute.Float64(key, d.IPAddressLeaseTime(0).Seconds()))
		return nil
	}

	return &optNotFoundError{optName: key}
}

// EncodeOpt53 takes DHCP Opt 53 from a DHCP packet and adds an OTEL
// key/value pair to the Encoder.Attributes. See https://www.iana.org/assignments/bootp-dhcp-parameters/bootp-dhcp-parameters.xhtml
func (e *Encoder) EncodeOpt53(d *dhcpv4.DHCPv4, namespace string) error {
	key := fmt.Sprintf("%v.%v.Opt53.MessageType", keyNamespace, namespace)
	if d != nil && d.MessageType() != dhcpv4.MessageTypeNone {
		e.Attributes = append(e.Attributes, attribute.String(key, d.MessageType().String()))
		return nil
	}

	return &optNotFoundError{optName: key}
}

// EncodeOpt54 takes DHCP Opt 54 from a DHCP packet and adds an OTEL
// key/value pair to the Encoder.Attributes. See https://www.iana.org/assignments/bootp-dhcp-parameters/bootp-dhcp-parameters.xhtml
func (e *Encoder) EncodeOpt54(d *dhcpv4.DHCPv4, namespace string) error {
	key := fmt.Sprintf("%v.%v.Opt54.ServerIdentifier", keyNamespace, namespace)
	if d != nil && d.ServerIdentifier() != nil {
		e.Attributes = append(e.Attributes, attribute.String(key, d.ServerIdentifier().String()))
		return nil
	}

	return &optNotFoundError{optName: key}
}

// EncodeOpt119 takes DHCP Opt 119 from a DHCP packet and adds an OTEL
// key/value pair to the Encoder.Attributes. See https://www.iana.org/assignments/bootp-dhcp-parameters/bootp-dhcp-parameters.xhtml
func (e *Encoder) EncodeOpt119(d *dhcpv4.DHCPv4, namespace string) error {
	key := fmt.Sprintf("%v.%v.Opt119.DomainSearch", keyNamespace, namespace)
	if d != nil {
		if l := d.DomainSearch(); l != nil {
			e.Attributes = append(e.Attributes, attribute.String(key, strings.Join(l.Labels, ",")))
			return nil
		}
	}

	return &optNotFoundError{optName: key}
}

// EncodeYIADDR takes the yiaddr header from a DHCP packet and adds an OTEL
// key/value pair to the Encoder.Attributes. See https://datatracker.ietf.org/doc/html/rfc2131#page-9
func (e *Encoder) EncodeYIADDR(d *dhcpv4.DHCPv4, namespace string) error {
	key := fmt.Sprintf("%v.%v.Header.yiaddr", keyNamespace, namespace)
	if d != nil && d.YourIPAddr != nil {
		e.Attributes = append(e.Attributes, attribute.String(key, d.YourIPAddr.String()))
		return nil
	}

	return &optNotFoundError{optName: key}
}

// EncodeSIADDR takes the siaddr header from a DHCP packet and adds an OTEL
// key/value pair to the Encoder.Attributes. See https://datatracker.ietf.org/doc/html/rfc2131#page-9
func (e *Encoder) EncodeSIADDR(d *dhcpv4.DHCPv4, namespace string) error {
	key := fmt.Sprintf("%v.%v.Header.siaddr", keyNamespace, namespace)
	if d != nil && d.ServerIPAddr != nil {
		e.Attributes = append(e.Attributes, attribute.String(key, d.ServerIPAddr.String()))
		return nil
	}

	return &optNotFoundError{optName: key}
}

// EncodeCHADDR takes the CHADDR header from a DHCP packet and adds an OTEL
// key/value pair to the Encoder.Attributes. See https://datatracker.ietf.org/doc/html/rfc2131#page-9
func (e *Encoder) EncodeCHADDR(d *dhcpv4.DHCPv4, namespace string) error {
	key := fmt.Sprintf("%v.%v.Header.chaddr", keyNamespace, namespace)
	if d != nil && d.ClientHWAddr != nil {
		e.Attributes = append(e.Attributes, attribute.String(key, d.ClientHWAddr.String()))
		return nil
	}

	return &optNotFoundError{optName: key}
}

// EncodeFILE takes the file header from a DHCP packet and adds an OTEL
// key/value pair to the Encoder.Attributes. See https://datatracker.ietf.org/doc/html/rfc2131#page-9
func (e *Encoder) EncodeFILE(d *dhcpv4.DHCPv4, namespace string) error {
	key := fmt.Sprintf("%v.%v.Header.file", keyNamespace, namespace)
	if d != nil && d.BootFileName != "" {
		e.Attributes = append(e.Attributes, attribute.String(key, d.BootFileName))
		return nil
	}

	return &optNotFoundError{optName: key}
}

// TraceparentFromContext extracts the binary trace id, span id, and trace flags
// from the running span in ctx and returns a 26 byte []byte with the traceparent
// encoded and ready to pass into a suboption (most likely 69) of opt43.
func TraceparentFromContext(ctx context.Context) []byte {
	sc := trace.SpanContextFromContext(ctx)
	tpBytes := make([]byte, 0, 26)

	// the otel spec says 16 bytes for trace id and 8 for spans are good enough
	// for everyone copy them into a []byte that we can deliver over option43
	tid := [16]byte(sc.TraceID()) // type TraceID [16]byte
	sid := [8]byte(sc.SpanID())   // type SpanID [8]byte

	tpBytes = append(tpBytes, 0x00)      // traceparent version
	tpBytes = append(tpBytes, tid[:]...) // trace id
	tpBytes = append(tpBytes, sid[:]...) // span id
	if sc.IsSampled() {
		tpBytes = append(tpBytes, 0x01) // trace flags
	} else {
		tpBytes = append(tpBytes, 0x00)
	}

	return tpBytes
}

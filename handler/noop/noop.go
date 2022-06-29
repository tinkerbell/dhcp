// Package noop is a handler that does nothing.
package noop

import (
	"log"
	"net"
	"os"

	"github.com/go-logr/logr"
	"github.com/go-logr/stdr"
	"github.com/insomniacslk/dhcp/dhcpv4"
)

// Handler is a noop handler.
type Handler struct {
	Log logr.Logger
}

// Handle is the noop handler function.
func (n *Handler) Handle(_ net.PacketConn, _ net.Addr, _ *dhcpv4.DHCPv4) {
	msg := "no handler specified. please specify a handler"
	if n.Log.GetSink() == nil {
		stdr.New(log.New(os.Stdout, "", log.Lshortfile)).Info(msg)
	} else {
		n.Log.Info(msg)
	}
}

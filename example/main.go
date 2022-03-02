package main

import (
	"context"
	"log"
	"net/url"
	"os"
	"os/signal"
	"syscall"

	"github.com/equinix-labs/otel-init-go/otelinit"
	"github.com/go-logr/logr"
	"github.com/go-logr/stdr"
	"github.com/tinkerbell/dhcp"
	"github.com/tinkerbell/dhcp/backend/file"
	"inet.af/netaddr"
)

func main() {
	ctx, done := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGHUP, syscall.SIGTERM)
	defer done()
	ctx, otelShutdown := otelinit.InitOpenTelemetry(ctx, "github.com/tinkerbell/dhcp")
	defer otelShutdown(ctx)

	l := stdr.New(log.New(os.Stdout, "", log.Lshortfile))
	l = l.WithName("github.com/tinkerbell/dhcp")
	b, err := backendFile(ctx, l, "./backend/file/testdata/example.yaml")
	if err != nil {
		panic(err)
	}

	s := &dhcp.Server{
		Log:               l,
		Listener:          netaddr.IPPortFrom(netaddr.IPv4(192, 168, 2, 225), 67),
		IPAddr:            netaddr.IPv4(192, 168, 2, 225),
		IPXEBinServerTFTP: netaddr.IPPortFrom(netaddr.IPv4(192, 168, 1, 34), 69),
		IPXEBinServerHTTP: &url.URL{Scheme: "http", Host: "192.168.1.34:8080"},
		IPXEScriptURL:     &url.URL{Scheme: "https", Host: "boot.netboot.xyz"},
		NetbootEnabled:    true,
		Backend:           b,
	}
	l.Info("starting server", "addr", s.Listener)
	l.Error(s.ListenAndServe(ctx), "done")
	l.Info("done")
}

func backendFile(ctx context.Context, l logr.Logger, f string) (dhcp.BackendReader, error) {
	fb, err := file.NewWatcher(l, f)
	if err != nil {
		return nil, err
	}
	go fb.Start(ctx)
	return fb, nil
}

// package main is an example of how to use the dhcp package with the kube backend.
package main

import (
	"context"
	"log"
	"net/netip"
	"net/url"
	"os"
	"os/signal"
	"syscall"

	"github.com/equinix-labs/otel-init-go/otelinit"
	"github.com/go-logr/stdr"
	"github.com/tinkerbell/dhcp"
	"github.com/tinkerbell/dhcp/backend/kube"
	"github.com/tinkerbell/dhcp/handler/reservation"
	"k8s.io/client-go/tools/clientcmd"
	"k8s.io/client-go/tools/clientcmd/api"
)

func main() {
	ctx, done := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGHUP, syscall.SIGTERM)
	defer done()
	ctx, otelShutdown := otelinit.InitOpenTelemetry(ctx, "github.com/tinkerbell/dhcp")
	defer otelShutdown(ctx)

	l := stdr.New(log.New(os.Stdout, "", log.Lshortfile))
	l = l.WithName("github.com/tinkerbell/dhcp")
	// 1. create the backend
	// 2. create the handler(backend)
	// 3. create the listener(handler)
	backend, err := kubeBackend(ctx)
	if err != nil {
		panic(err)
	}

	handler := &reservation.Handler{
		Log:    l,
		IPAddr: netip.MustParseAddr("192.168.2.221"),
		Netboot: reservation.Netboot{
			IPXEBinServerTFTP: netip.MustParseAddrPort("192.168.1.34:69"),
			IPXEBinServerHTTP: &url.URL{Scheme: "http", Host: "192.168.1.34:8080"},
			IPXEScriptURL:     &url.URL{Scheme: "https", Host: "boot.netboot.xyz"},
			Enabled:           true,
		},
		OTELEnabled: true,
		Backend:     backend,
	}
	listener := &dhcp.Listener{}
	l.Info("starting server", "addr", handler.IPAddr)
	l.Error(listener.ListenAndServe(ctx, handler), "done")
	l.Info("done")
}

func kubeBackend(ctx context.Context) (reservation.BackendReader, error) {
	ccfg := clientcmd.NewNonInteractiveDeferredLoadingClientConfig(
		&clientcmd.ClientConfigLoadingRules{
			ExplicitPath: "/home/tink/.kube/config",
		},
		&clientcmd.ConfigOverrides{
			Context: api.Context{
				Namespace: "tink-system",
			},
		},
	)

	config, err := ccfg.ClientConfig()
	if err != nil {
		return nil, err
	}

	k, err := kube.NewBackend(config)
	if err != nil {
		return nil, err
	}

	go func() {
		_ = k.Start(ctx)
	}()

	return k, nil
}

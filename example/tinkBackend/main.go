package main

import (
	"context"
	"crypto/x509"
	"flag"
	"fmt"
	"io/ioutil"
	"log"
	"net/http"
	"net/url"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/equinix-labs/otel-init-go/otelinit"
	"github.com/go-logr/logr"
	"github.com/go-logr/stdr"
	"github.com/tinkerbell/dhcp"
	"github.com/tinkerbell/dhcp/backend/tink"
	"github.com/tinkerbell/tink/protos/hardware"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials"
	"inet.af/netaddr"
)

func main() {
	ctx, done := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGHUP, syscall.SIGTERM)
	defer done()
	ctx, otelShutdown := otelinit.InitOpenTelemetry(ctx, "github.com/tinkerbell/dhcp")
	defer otelShutdown(ctx)
	l := stdr.New(log.New(os.Stdout, "", log.Lshortfile))
	l = l.WithName("github.com/tinkerbell/dhcp")
	tinkIP := "127.0.0.1"
	flag.StringVar(&tinkIP, "ip", tinkIP, "IP address of the tink server")
	flag.Parse()
	b, client, err := tinkBackend(ctx, l, tinkIP, time.Second*10)
	if err != nil {
		panic(err)
	}
	defer client.Close() // nolint: errcheck // ok for an example file?

	s := &dhcp.Server{
		Log:               l,
		Listener:          netaddr.IPPortFrom(netaddr.IPv4(192, 168, 2, 225), 67),
		IPAddr:            netaddr.IPv4(192, 168, 2, 225),
		IPXEBinServerTFTP: netaddr.IPPortFrom(netaddr.IPv4(192, 168, 1, 34), 69),
		IPXEBinServerHTTP: &url.URL{Scheme: "http", Host: "192.168.1.34:8080"},
		IPXEScriptURL:     &url.URL{Scheme: "https", Host: "boot.netboot.xyz"},
		NetbootEnabled:    true,
		OTELEnabled:       true,
		Backend:           b,
	}
	l.Info("starting server", "addr", s.Listener, "tinkIP", tinkIP)
	l.Error(s.ListenAndServe(ctx), "done")
	l.Info("done")
}

func tinkBackend(ctx context.Context, l logr.Logger, tinkIP string, timeout time.Duration) (*tink.Config, *grpc.ClientConn, error) {
	ctx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()
	dialOpt, err := LoadTLSFromValue(ctx, fmt.Sprintf("http://%v:42114/cert", tinkIP))
	if err != nil {
		return nil, nil, fmt.Errorf("unable to create gRPC client TLS dial option: %w", err)
	}

	client, err := grpc.DialContext(ctx, fmt.Sprintf("%v:42113", tinkIP), dialOpt, grpc.WithBlock())
	if err != nil {
		return nil, nil, fmt.Errorf("error connecting to tink server: %w", err)
	}

	return &tink.Config{Log: l, Client: hardware.NewHardwareServiceClient(client)}, client, nil
}

// LoadTLSFromValue handles reading a cert from an HTTP endpoint and forming a TLS grpc.DialOption.
func LoadTLSFromValue(ctx context.Context, u string) (grpc.DialOption, error) {
	req, err := http.NewRequestWithContext(ctx, "GET", u, nil)
	if err != nil {
		return nil, fmt.Errorf("unable to create http request: %w", err)
	}
	client := http.DefaultClient
	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("unable to get cert from %s: %w", u, err)
	}
	defer resp.Body.Close() // nolint: errcheck // whats the alternative?

	cert, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}

	return grpc.WithTransportCredentials(toCreds(cert)), nil
}

// toCreds takes a byte string, assumed to be a tls cert, and creates a transport credential.
func toCreds(pemCerts []byte) credentials.TransportCredentials {
	cp := x509.NewCertPool()
	ok := cp.AppendCertsFromPEM(pemCerts)
	if !ok {
		return nil
	}
	return credentials.NewClientTLSFromCert(cp, "")
}

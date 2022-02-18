package dhcp

import (
	"context"
	"fmt"
	"net"
	"sync"
	"testing"

	"github.com/google/go-cmp/cmp"
	"golang.org/x/sync/errgroup"
	"inet.af/netaddr"
)

func TestListenAndServe(t *testing.T) {
	tests := map[string]struct {
		wantErr error
		port    uint16
	}{
		"success": {
			wantErr: &net.OpError{
				Op:   "read",
				Net:  "udp",
				Addr: &net.UDPAddr{IP: net.IPv4(0, 0, 0, 0), Port: 6767},
				Err:  fmt.Errorf("use of closed network connection"),
			},
			port: 6767,
		},
	}
	for name, tt := range tests {
		t.Run(name, func(t *testing.T) {
			got := &Server{
				Listener: netaddr.IPPortFrom(netaddr.IPv4(127, 0, 0, 1), tt.port),
			}
			ctx, cn := context.WithCancel(context.Background())

			g, ctx := errgroup.WithContext(ctx)
			g.Go(func() error {
				return got.ListenAndServe(ctx)
			})
			cn()
			err := g.Wait()

			switch {
			case tt.wantErr == nil && err != nil:
				t.Fatalf("expected nil error, got: %T (%[1]v)", err)
			case tt.wantErr != nil && err == nil:
				t.Errorf("expected error, got: nil")
			case tt.wantErr != nil && err != nil:
				if diff := cmp.Diff(err.Error(), tt.wantErr.Error()); diff != "" {
					t.Fatal(diff)
				}
			}
		})
	}
}

func TestServe(t *testing.T) {
	tests := map[string]struct {
		wantErr    error
		wantUDPErr bool
	}{
		"success": {
			wantErr: &net.OpError{
				Op:   "read",
				Net:  "udp",
				Addr: &net.UDPAddr{IP: net.IPv4(127, 0, 0, 1), Port: 6767},
				Err:  fmt.Errorf("use of closed network connection"),
			},
		},
	}
	for name, tt := range tests {
		t.Run(name, func(t *testing.T) {
			got := &Server{}
			ctx, cn := context.WithCancel(context.Background())

			var uconn net.PacketConn
			var err error
			if !tt.wantUDPErr {
				a, err := net.ResolveUDPAddr("udp", "127.0.0.1:6767")
				if err != nil {
					t.Fatal(err)
				}
				uconn, err = net.ListenUDP("udp", a)
				if err != nil {
					t.Fatal(err)
				}
			}

			var wg sync.WaitGroup
			wg.Add(1)
			go func() {
				err = got.Serve(ctx, uconn)
				wg.Done()
			}()
			cn()
			wg.Wait()

			switch {
			case tt.wantErr == nil && err != nil:
				t.Errorf("expected nil error, got: %T", err)
			case tt.wantErr != nil && err == nil:
				t.Errorf("expected error, got: nil")
			case tt.wantErr != nil && err != nil:
				if diff := cmp.Diff(err.Error(), tt.wantErr.Error()); diff != "" {
					t.Fatal(diff)
				}
			}
		})
	}
}

func TestDefaultIP(t *testing.T) {
	tests := map[string]struct {
		want netaddr.IP
	}{
		"success": {netaddr.IPv4(0, 0, 0, 0)},
	}
	for name, tt := range tests {
		t.Run(name, func(t *testing.T) {
			got := defaultIP()
			if got.Compare(tt.want) == 0 {
				t.Fatalf("defaultIP() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestGetInterfaceByIP(t *testing.T) {
	tests := map[string]struct {
		ip     string
		wantIF []string
	}{
		"success": {
			ip:     "127.0.0.1",
			wantIF: []string{"lo0", "lo"},
		},
		"not found": {
			ip:     "1.1.1.1",
			wantIF: []string{""},
		},
	}
	for name, tt := range tests {
		t.Run(name, func(t *testing.T) {
			var diffs []string
			for _, want := range tt.wantIF {
				if diff := cmp.Diff(getInterfaceByIP(tt.ip), want); diff != "" {
					diffs = append(diffs, diff)
				}
			}
			if len(diffs) == len(tt.wantIF) {
				t.Fatalf("%v", diffs)
			}
		})
	}
}

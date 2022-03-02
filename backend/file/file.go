// Package file watches a file for changes and updates the in memory DHCP data.
package file

import (
	"context"
	"fmt"
	"net"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"sync"

	"github.com/fsnotify/fsnotify"
	"github.com/ghodss/yaml"
	"github.com/go-logr/logr"
	"github.com/tinkerbell/dhcp/data"
	"inet.af/netaddr"
)

// Errors used by the file watcher.
var (
	// errFileFormat is returned when the file is not in the correct format, e.g. not valid YAML.
	errFileFormat     = fmt.Errorf("invalid file format")
	errRecordNotFound = fmt.Errorf("record not found")
	errParseIP        = fmt.Errorf("failed to parse IP from File")
	errParseSubnet    = fmt.Errorf("failed to parse subnet mask from File")
	errParseURL       = fmt.Errorf("failed to parse URL")
)

// Watcher represents the backend for watching a file for changes and updating the in memory DHCP data.
type Watcher struct {
	fileMu sync.RWMutex // protects FilePath for reads

	// FilePath is the path to the file to watch.
	FilePath string

	// Log is the logger to be used in the File backend.
	Log     logr.Logger
	dataMu  sync.RWMutex // protects data
	data    []byte       // data from file
	watcher *fsnotify.Watcher
}

// NewWatcher creates a new file watcher.
func NewWatcher(l logr.Logger, f string) (*Watcher, error) {
	watcher, err := fsnotify.NewWatcher()
	if err != nil {
		return nil, err
	}
	if err := watcher.Add(f); err != nil {
		return nil, err
	}

	w := &Watcher{
		FilePath: f,
		watcher:  watcher,
		Log:      l,
	}

	w.fileMu.RLock()
	w.data, err = os.ReadFile(filepath.Clean(f))
	w.fileMu.RUnlock()
	if err != nil {
		return nil, err
	}

	return w, nil
}

// Read is the implementation of the Backend interface.
// It reads a given file from the in memory data (w.data).
func (w *Watcher) Read(_ context.Context, mac net.HardwareAddr) (*data.DHCP, *data.Netboot, error) {
	// get data from file, translate it, then pass it into setDHCPOpts and setNetworkBootOpts
	w.dataMu.RLock()
	d := w.data
	w.dataMu.RUnlock()
	r := make(map[string]dhcp)
	if err := yaml.Unmarshal(d, &r); err != nil {
		return nil, nil, fmt.Errorf("%v: %w", err, errFileFormat)
	}
	for k, v := range r {
		if strings.EqualFold(k, mac.String()) {
			// found a record for this mac address
			v.MacAddress = mac
			return w.translate(v)
		}
	}

	return nil, nil, fmt.Errorf("%w: %s", errRecordNotFound, mac.String())
}

// Start starts watching a file for changes and updates the in memory data (w.data) on changes.
// Start is a blocking method. Use a context cancellation to exit.
func (w *Watcher) Start(ctx context.Context) {
	for {
		select {
		case <-ctx.Done():
			w.Log.Info("stopping watcher")
			return
		case event, ok := <-w.watcher.Events:
			if !ok {
				continue
			}
			if event.Op&fsnotify.Write == fsnotify.Write {
				w.Log.Info("file changed, updating cache")
				w.fileMu.RLock()
				d, err := os.ReadFile(w.FilePath)
				w.fileMu.RUnlock()
				if err != nil {
					w.Log.Error(err, "failed to read file", "file", w.FilePath)
					break
				}
				w.dataMu.Lock()
				w.data = d
				w.dataMu.Unlock()
			}
		case err, ok := <-w.watcher.Errors:
			if !ok {
				continue
			}
			w.Log.Info("error watching file", "err", err)
		}
	}
}

// translate converts the data from the file into a data.DHCP and data.Netboot structs.
func (w *Watcher) translate(r dhcp) (*data.DHCP, *data.Netboot, error) {
	d := new(data.DHCP)
	n := new(data.Netboot)

	d.MacAddress = r.MacAddress
	// ip address, required
	ip, err := netaddr.ParseIP(r.IPAddress)
	if err != nil {
		return nil, nil, fmt.Errorf("%v: %w", err, errParseIP)
	}
	d.IPAddress = ip

	// subnet mask, required
	sm, err := netaddr.ParseIP(r.SubnetMask)
	if err != nil {
		return nil, nil, fmt.Errorf("%v: %w", err, errParseSubnet)
	}
	d.SubnetMask = sm.IPAddr().IP.DefaultMask()

	// default gateway, optional
	if dg, err := netaddr.ParseIP(r.DefaultGateway); err != nil {
		w.Log.Info("failed to parse default gateway", "defaultGateway", r.DefaultGateway, "err", err)
	} else {
		d.DefaultGateway = dg
	}

	// name servers, optional
	for _, s := range r.NameServers {
		ip := net.ParseIP(s)
		if ip == nil {
			w.Log.Info("failed to parse name server", "nameServer", s)
			break
		}
		d.NameServers = append(d.NameServers, ip)
	}

	// hostname, optional
	d.Hostname = r.Hostname

	// domain name, optional
	d.DomainName = r.DomainName

	// broadcast address, optional
	if ba, err := netaddr.ParseIP(r.BroadcastAddress); err != nil {
		w.Log.Info("failed to parse broadcast address", "broadcastAddress", r.BroadcastAddress, "err", err)
	} else {
		d.BroadcastAddress = ba
	}

	// ntp servers, optional
	for _, s := range r.NTPServers {
		ip := net.ParseIP(s)
		if ip == nil {
			w.Log.Info("failed to parse ntp server", "ntpServer", s)
			break
		}
		d.NTPServers = append(d.NTPServers, ip)
	}

	// lease time
	// TODO(jacobweinstock): write some validations. > 0, etc.
	d.LeaseTime = uint32(r.LeaseTime)

	// domain search
	d.DomainSearch = r.DomainSearch

	n.AllowNetboot = r.Netboot.AllowPXE
	if r.Netboot.IPXEScriptURL != "" {
		u, err := url.Parse(r.Netboot.IPXEScriptURL)
		if err != nil {
			return nil, nil, fmt.Errorf("%v: %w", err, errParseURL)
		}
		n.IpxeScriptURL = u
	}

	return d, n, nil
}

// netboot is the structure for the data expected in a file.
type netboot struct {
	AllowPXE      bool   `yaml:"allowPxe"`      // If true, the client will be provided netboot options in the DHCP offer/ack.
	IPXEScriptURL string `yaml:"ipxeScriptUrl"` // Overrides default value of that is passed into DHCP on startup.
}

// dhcp is the structure for the data expected in a file.
type dhcp struct {
	MacAddress       net.HardwareAddr // The MAC address of the client.
	IPAddress        string           `yaml:"ipAddress"`        // yiaddr DHCP header.
	SubnetMask       string           `yaml:"subnetMask"`       // DHCP option 1.
	DefaultGateway   string           `yaml:"defaultGateway"`   // DHCP option 3.
	NameServers      []string         `yaml:"nameServers"`      // DHCP option 6.
	Hostname         string           `yaml:"hostname"`         // DHCP option 12.
	DomainName       string           `yaml:"domainName"`       // DHCP option 15.
	BroadcastAddress string           `yaml:"broadcastAddress"` // DHCP option 28.
	NTPServers       []string         `yaml:"ntpServers"`       // DHCP option 42.
	LeaseTime        int              `yaml:"leaseRime"`        // DHCP option 51.
	DomainSearch     []string         `yaml:"domainSearch"`     // DHCP option 119.
	Netboot          netboot          `yaml:"netboot"`
}

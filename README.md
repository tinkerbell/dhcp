[![Test and Build](https://github.com/tinkerbell/dhcp/actions/workflows/ci.yaml/badge.svg)](https://github.com/tinkerbell/dhcp/actions/workflows/ci.yaml)
[![codecov](https://codecov.io/gh/tinkerbell/dhcp/branch/main/graph/badge.svg)](https://codecov.io/gh/tinkerbell/dhcp)
[![Go Report Card](https://goreportcard.com/badge/github.com/tinkerbell/dhcp)](https://goreportcard.com/report/github.com/tinkerbell/dhcp)
[![Go Reference](https://pkg.go.dev/badge/github.com/tinkerbell/dhcp.svg)](https://pkg.go.dev/github.com/tinkerbell/dhcp)

# dhcp

DHCP library and CLI server with multiple backends. All IP addresses are served as DHCP reservations. There are no leases.

## Backends

- [Tink server](https://github.com/tinkerbell/tink)
  - This backend is the main use case.
  It pulls hardware data from the Tink API for use in the DHCP server.
- File based
  - This backend is for mainly for testing and development.
  It reads a file for hardware data. See [examples/dhcp.yaml](./example/dhcp.yaml) for the data model.
- [Cacher server](https://github.com/packethost/cacher)
  - This backend is mainly for backward compatibility in [Boots](https://github.com/tinkerbell/boots).
  It pulls hardware data from the Cacher API for use in the DHCP server.
  It is planned for deprecation.

## Definitions

**DHCP Reservation:**
A fixed IP address that is reserved for a specific client.

**DHCP Lease:**
An IP address, that can potentially change, that is assigned to a client by the DHCP server.
The IP is typically pulled from a pool or subnet of available IP addresses.

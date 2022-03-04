# Tink Server Backend

This document gives an overview of the Tink server backend.
This backend will query a [Tink](https://github.com/tinkerbell/tink) server for values to be used when responding to DHCP requests.

## Why

This backend exists for backward compatibility with the way the current [Boots](https://github.com/tinkerbell/boots) works.
It will make a call to the Tink server every time a DHCP request is received.

## Usage

There is a very basic example of how to use this backend in [example/main.go](example/main.go).

```bash
go run example/main.go -ip <tink server ip>
```

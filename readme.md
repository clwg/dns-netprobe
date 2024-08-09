# dns-netprobe

dns-netprobe is a dns based scanning tool for recursive DNS server discovery, authoritative testing and censorship measurement.

## Building

The project requires gcc to build cgo.

```bash
export CGO_ENABLED=1
go mod tidy
go build -o dns-netprobe cmd/dns-netprobe/main.go
```

## Usage

```bash
./dns-netprobe -domain example.com -network 1.1.1.0/24 
```

All results are stored in the sqlite database file dns.db.

### Configuration Flags

The application accepts several command-line flags to configure its behavior:

- domain: Specifies the primary domain to query.
- network: Defines the network range to query.
- timeout: Sets the timeout for DNS queries in seconds (default is 5 seconds).
- domains: Provides a comma-separated list of additional domains to query.
- db: Specifies the SQLite database file to use (default is dns.db).
- concurrent: Limits the number of concurrent goroutines (default is 256).
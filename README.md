
# nolo

a fast http benchmarking tool written in go.

## install

```sh
go install github.com/wronguy-debug/nolo@latest
```

if `nolo` is not found after install, add go's bin directory to your path:

```sh
fish_add_path ~/go/bin
```

or build from source:

```sh
git clone https://github.com/wronguy-debug/nolo
cd nolo
go build -o nolo .
sudo mv nolo /usr/local/bin/
```

## usage

```
nolo <url> -n <requests> -q <rms> [options]
```

### flags

| flag | description |
|------|-------------|
| `-n <int>` | total number of requests to send (required) |
| `-q, --rms <int>` | requests per millisecond rate limit |
| `-c, --concurrency <int>` | number of concurrent workers (default: 10) |
| `-k, --insecure` | skip tls certificate verification |
| `-cr` | fire all requests at once, no rate limiting |
| `-h, --help` | show help message |

### examples

```sh
nolo https://example.com -n 1000 -q 100
```

```sh
nolo localhost:8080 -n 500 -q 50 -c 20
```

```sh
nolo https://api.example.com -n 10000 -q 500 -k
```

```sh
nolo https://example.com -n 500 -cr
```

## output

nolo prints a live progress line during the run and a final summary with:

- total elapsed time
- requests sent, successful, and failed
- actual requests per second achieved
- latency percentiles (min, p50, p90, p99, p999, max)
- http status code distribution
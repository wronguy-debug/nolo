# nolo

a fast http benchmarking tool written in go.

## install

```sh
go install github.com/wronguy/nolo@latest
```

or build from source:

```sh
git clone https://github.com/wronguy/nolo
cd nolo
go build -o nolo .
sudo mv nolo /usr/local/bin/
```

## usage

```
nolo <url> -n <requests> -q <rps> [options]
```

### flags

| flag | description |
|------|-------------|
| `-n <int>` | total number of requests to send (required) |
| `-q, --rps <int>` | requests per second rate limit (required) |
| `-c, --concurrency <int>` | number of concurrent workers (default: 10) |
| `-k, --insecure` | skip tls certificate verification |
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

## output

nolo prints a live progress line during the run and a final summary with:

- total elapsed time
- requests sent, successful, and failed
- actual requests per second achieved
- latency percentiles (min, p50, p90, p99, p999, max)
- http status code distribution
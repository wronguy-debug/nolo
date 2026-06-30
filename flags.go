package main

import (
	"fmt"
	"os"
	"strings"
)

type config struct {
	url         string
	total       int64
	rps         int64
	concurrency int64
	insecure    bool
	cannonball  bool
	help        bool
}

var cfg config

func parseFlags(args []string) config {
	c := config{
		concurrency: 10,
	}

	needsHelp := len(args) == 0

	for i := 0; i < len(args); i++ {
		switch args[i] {
		case "-h", "--help":
			c.help = true
		case "-n":
			i++
			c.total = parseInt(args, &i, "-n")
		case "-q", "--rps":
			i++
			c.rps = parseInt(args, &i, "-q")
		case "-c", "--concurrency":
			i++
			c.concurrency = parseInt(args, &i, "-c")
		case "-k", "--insecure":
			c.insecure = true
		case "-cr":
			c.cannonball = true
		default:
			if !strings.HasPrefix(args[i], "-") && c.url == "" {
				c.url = args[i]
			}
		}
	}

	if c.help {
		printHelp()
		os.Exit(0)
	}

	if c.url == "" {
		needsHelp = true
	}
	if c.total <= 0 {
		needsHelp = true
	}
	if !c.cannonball && c.rps <= 0 {
		needsHelp = true
	}

	if needsHelp {
		printHelp()
		os.Exit(1)
	}

	if !strings.HasPrefix(c.url, "http://") && !strings.HasPrefix(c.url, "https://") {
		c.url = "https://" + c.url
	}

	cfg = c
	return c
}

func parseInt(args []string, i *int, flag string) int64 {
	if *i >= len(args) {
		fmt.Fprintf(os.Stderr, "nolo: %s requires a value\n", flag)
		os.Exit(1)
	}
	var v int64
	_, err := fmt.Sscanf(args[*i], "%d", &v)
	if err != nil || v <= 0 {
		fmt.Fprintf(os.Stderr, "nolo: %s requires a positive integer, got %q\n", flag, args[*i])
		os.Exit(1)
	}
	return v
}

func printHelp() {
	fmt.Print(`nolo — http benchmarking tool

usage:
  nolo <url> -n <requests> -q <rps> [options]

arguments:
  <url>               target url (http[s]:// prefix auto-added if missing)

flags:
  -n <int>            total number of requests to send
  -q, --rps <int>     requests per second (rate limit)
  -c, --concurrency <int>   number of concurrent workers (default: 10)
  -k, --insecure      skip tls certificate verification
  -cr                 fire all requests simultaneously (no rate limit)
  -h, --help          show this help message

examples:
  nolo https://example.com -n 1000 -q 100
  nolo localhost:8080 -n 500 -q 50 -c 20
  nolo https://api.example.com -n 10000 -q 500 -k
  nolo https://api.example.com -n 500 -cr

nolo sends https GET requests at a fixed rate and prints latency
statistics and status code distribution to stdout.
`)
}

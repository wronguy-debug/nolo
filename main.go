package main

import (
	"context"
	"crypto/tls"
	"fmt"
	"math"
	"net/http"
	"os"
	"os/signal"
	"sort"
	"sync"
	"sync/atomic"
	"syscall"
	"time"
)

func main() {
	cfg := parseFlags(os.Args[1:])

	transport := &http.Transport{
		TLSClientConfig: &tls.Config{
			InsecureSkipVerify: cfg.insecure,
		},
		MaxIdleConnsPerHost:   int(cfg.concurrency),
		MaxConnsPerHost:       int(cfg.concurrency),
		DisableCompression:    true,
		DisableKeepAlives:     true,
		ResponseHeaderTimeout: 30 * time.Second,
	}

	client := &http.Client{
		Transport: transport,
		Timeout:   60 * time.Second,
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)
	go func() {
		<-sigCh
		cancel()
	}()

	var sent atomic.Int64
	var success atomic.Int64
	var fail atomic.Int64
	var totalLatency atomic.Int64

	statusCounts := &sync.Map{}
	latencies := make([]int64, 0, cfg.total)
	latMu := &sync.Mutex{}

	reqCh := make(chan int64, cfg.concurrency*4)

	startTime := time.Now()

	cannonball := cfg.cannonball
	var interval time.Duration
	if !cannonball {
		interval = time.Duration(float64(time.Second) / float64(cfg.rps))
		if interval < time.Microsecond {
			interval = time.Microsecond
		}
	}

	var wg sync.WaitGroup
	for i := int64(0); i < cfg.concurrency; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for seq := range reqCh {
				select {
				case <-ctx.Done():
					return
				default:
				}

				reqStart := time.Now()
				req, err := http.NewRequestWithContext(ctx, http.MethodGet, cfg.url, nil)
				if err != nil {
					fail.Add(1)
					sent.Add(1)
					continue
				}
				req.Header.Set("User-Agent", "nolo/1.0.0")
				req.Header.Set("Accept", "*/*")
				req.Header.Set("Connection", "close")

				resp, err := client.Do(req)
				elapsed := time.Since(reqStart).Microseconds()
				totalLatency.Add(elapsed)
				sent.Add(1)

				latMu.Lock()
				latencies = append(latencies, elapsed)
				latMu.Unlock()

				if err != nil {
					fail.Add(1)
					continue
				}

				statusBucket := resp.StatusCode / 100
				key := fmt.Sprintf("%dxx", statusBucket)
				if statusBucket == 0 || statusBucket > 5 {
					key = fmt.Sprintf("%d", resp.StatusCode)
				}
				val, _ := statusCounts.LoadOrStore(key, new(atomic.Int64))
				val.(*atomic.Int64).Add(1)
				resp.Body.Close()

				if resp.StatusCode >= 200 && resp.StatusCode < 400 {
					success.Add(1)
				} else {
					fail.Add(1)
				}

				_ = seq
			}
		}()
	}

	if cannonball {
		go func() {
			for seq := int64(0); seq < cfg.total; seq++ {
				select {
				case <-ctx.Done():
					close(reqCh)
					return
				case reqCh <- seq:
				}
			}
			close(reqCh)
		}()
	} else {
		go func() {
			ticker := time.NewTicker(interval)
			defer ticker.Stop()
			for seq := int64(0); seq < cfg.total; seq++ {
				select {
				case <-ctx.Done():
					close(reqCh)
					return
				case <-ticker.C:
					select {
					case reqCh <- seq:
					default:
						go func(s int64) {
							select {
							case reqCh <- s:
							case <-ctx.Done():
							}
						}(seq)
					}
				}
			}
			close(reqCh)
		}()
	}

	ticker := time.NewTicker(500 * time.Millisecond)
	defer ticker.Stop()

	done := make(chan struct{})
	go func() {
		wg.Wait()
		close(done)
	}()

	printHeader()

loop:
	for {
		select {
		case <-done:
			break loop
		case <-ticker.C:
			printProgress(startTime, sent.Load(), cfg.total, success.Load(), fail.Load(), totalLatency.Load())
		case <-ctx.Done():
			ticker.Stop()
			printProgress(startTime, sent.Load(), cfg.total, success.Load(), fail.Load(), totalLatency.Load())
			break loop
		}
	}

	wg.Wait()
	elapsed := time.Since(startTime)
	printSummary(
		elapsed,
		sent.Load(),
		success.Load(),
		fail.Load(),
		totalLatency.Load(),
		latencies,
		statusCounts,
	)
}

func printHeader() {
	fmt.Println()
	fmt.Printf("%-12s %-12s %-12s %-12s %-14s %-12s\n",
		"Elapsed", "Sent", "Total", "Success", "Fail", "Avg Latency")
	fmt.Println("---------------------------------------------------------------")
}

func printProgress(start time.Time, sent, total, success, fail, totalLat int64) {
	elapsed := time.Since(start)
	avgLat := "-"
	if sent > 0 {
		avgLat = formatLatency(float64(totalLat) / float64(sent))
	}
	fmt.Printf("\r\033[K%-12s %-12d %-12d %-12d %-14d %-12s",
		formatDuration(elapsed), sent, total, success, fail, avgLat)
}

func printSummary(elapsed time.Duration, sent, success, fail, totalLat int64, latencies []int64, statusCounts *sync.Map) {
	fmt.Printf("\r\033[K")
	fmt.Println()
	fmt.Println("===============================================================")
	fmt.Println("                       RESULTS")
	fmt.Println("===============================================================")

	actualRPS := float64(sent) / elapsed.Seconds()
	successRate := float64(success) / float64(sent) * 100

	fmt.Printf("  url:               %s\n", cfg.url)
	fmt.Printf("  total elapsed:     %s\n", formatDuration(elapsed))
	fmt.Printf("  requests sent:     %d\n", sent)
	fmt.Printf("  successful:        %d (%.1f%%)\n", success, successRate)
	fmt.Printf("  failed:            %d\n", fail)
	fmt.Printf("  actual rps:        %.1f req/s\n", actualRPS)

	if sent > 0 {
		avgLat := float64(totalLat) / float64(sent)
		fmt.Printf("  avg latency:       %s\n", formatLatency(avgLat))

		sort.Slice(latencies, func(i, j int) bool { return latencies[i] < latencies[j] })
		p50 := latencies[len(latencies)*50/100]
		p90 := latencies[len(latencies)*90/100]
		p99 := latencies[len(latencies)*99/100]
		p999 := latencies[len(latencies)*999/1000]
		max := latencies[len(latencies)-1]
		min := latencies[0]

		fmt.Printf("  min latency:       %s\n", formatLatency(float64(min)))
		fmt.Printf("  p50 latency:       %s\n", formatLatency(float64(p50)))
		fmt.Printf("  p90 latency:       %s\n", formatLatency(float64(p90)))
		fmt.Printf("  p99 latency:       %s\n", formatLatency(float64(p99)))
		fmt.Printf("  p999 latency:      %s\n", formatLatency(float64(p999)))
		fmt.Printf("  max latency:       %s\n", formatLatency(float64(max)))
	}

	fmt.Println()
	fmt.Println("  status distribution:")
	statusCounts.Range(func(key, value interface{}) bool {
		count := value.(*atomic.Int64).Load()
		fmt.Printf("    %-10s %d\n", key.(string), count)
		return true
	})
	fmt.Println()
}

func formatDuration(d time.Duration) string {
	if d < time.Microsecond {
		return fmt.Sprintf("%.0fns", float64(d.Nanoseconds()))
	}
	if d < time.Millisecond {
		return fmt.Sprintf("%.1fµs", float64(d.Microseconds()))
	}
	if d < time.Second {
		return fmt.Sprintf("%.1fms", float64(d.Milliseconds()))
	}
	if d < time.Minute {
		return fmt.Sprintf("%.1fs", d.Seconds())
	}
	m := int(d.Minutes())
	s := math.Mod(d.Seconds(), 60)
	return fmt.Sprintf("%dm%.0fs", m, s)
}

func formatLatency(us float64) string {
	if us < 1000 {
		return fmt.Sprintf("%.0fµs", us)
	}
	if us < 1_000_000 {
		return fmt.Sprintf("%.1fms", us/1000)
	}
	return fmt.Sprintf("%.1fs", us/1_000_000)
}

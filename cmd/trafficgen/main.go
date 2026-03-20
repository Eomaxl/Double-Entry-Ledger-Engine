package main

import (
	"bytes"
	"context"
	"encoding/csv"
	"encoding/json"
	"flag"
	"fmt"
	"math/rand"
	"net/http"
	"os"
	"sort"
	"sync"
	"sync/atomic"
	"time"

	"github.com/google/uuid"
)

type generatorConfig struct {
	BaseURL      string
	APIKey       string
	Duration     time.Duration
	Concurrency  int
	RPS          int
	BatchSize    int
	AccountCount int
	Currency     string
	MinAmount    int
	MaxAmount    int
	CSVOutput    string
}

type createAccountRequest struct {
	AccountType string `json:"account_type"`
	Currencies  []struct {
		CurrencyCode  string `json:"currency_code"`
		AllowNegative bool   `json:"allow_negative"`
	} `json:"currencies"`
	Metadata map[string]interface{} `json:"metadata,omitempty"`
}

type createAccountResponse struct {
	AccountID string `json:"account_id"`
}

type postBatchRequest struct {
	Transactions []postTransactionRequest `json:"transactions"`
}

type postTransactionRequest struct {
	IdempotencyKey string                 `json:"idempotency_key,omitempty"`
	State          string                 `json:"state"`
	Entries        []entryRequest         `json:"entries"`
	Metadata       map[string]interface{} `json:"metadata,omitempty"`
}

type entryRequest struct {
	AccountID    string `json:"account_id"`
	CurrencyCode string `json:"currency_code"`
	Amount       string `json:"amount"`
	EntryType    string `json:"entry_type"`
	Description  string `json:"description,omitempty"`
}

type runStats struct {
	success      int64
	failed       int64
	totalNanos   int64
	latencyMu    sync.Mutex
	latencyNanos []int64
	perSecond    []secondStat
}

type secondStat struct {
	Second      int
	Success     int64
	Failed      int64
	AchievedRPS float64
}

func main() {
	cfg := parseFlags()
	if err := validateConfig(cfg); err != nil {
		fmt.Fprintf(os.Stderr, "invalid config: %v\n", err)
		os.Exit(1)
	}

	client := &http.Client{Timeout: 10 * time.Second}

	fmt.Printf("trafficgen: base=%s duration=%s concurrency=%d rps=%d batch=%d accounts=%d\n",
		cfg.BaseURL, cfg.Duration, cfg.Concurrency, cfg.RPS, cfg.BatchSize, cfg.AccountCount)

	accountIDs, err := seedAccounts(client, cfg)
	if err != nil {
		fmt.Fprintf(os.Stderr, "failed to seed accounts: %v\n", err)
		os.Exit(1)
	}
	fmt.Printf("seeded %d accounts\n", len(accountIDs))

	stats := &runStats{
		latencyNanos: make([]int64, 0, 200000),
		perSecond:    make([]secondStat, 0, int(cfg.Duration.Seconds())+2),
	}

	ctx, cancel := context.WithTimeout(context.Background(), cfg.Duration)
	defer cancel()

	workCh := make(chan struct{}, cfg.Concurrency*2)
	var workers sync.WaitGroup
	for i := 0; i < cfg.Concurrency; i++ {
		workers.Add(1)
		go func(workerID int) {
			defer workers.Done()
			r := rand.New(rand.NewSource(time.Now().UnixNano() + int64(workerID)))
			for range workCh {
				start := time.Now()
				err := postSyntheticBatch(client, cfg, accountIDs, r)
				latency := time.Since(start)
				record(stats, latency, err == nil)
			}
		}(i)
	}

	start := time.Now()
	go printLiveStats(ctx, stats, start)
	if cfg.RPS > 0 {
		ticker := time.NewTicker(time.Second / time.Duration(cfg.RPS))
		defer ticker.Stop()
		for {
			select {
			case <-ctx.Done():
				close(workCh)
				workers.Wait()
				printSummary(cfg, start, stats)
				return
			case <-ticker.C:
				workCh <- struct{}{}
			}
		}
	}

	for {
		select {
		case <-ctx.Done():
			close(workCh)
			workers.Wait()
			printSummary(cfg, start, stats)
			return
		case workCh <- struct{}{}:
		}
	}
}

func parseFlags() generatorConfig {
	cfg := generatorConfig{}
	flag.StringVar(&cfg.BaseURL, "base-url", envOrDefault("TRAFFICGEN_BASE_URL", "http://localhost:8080"), "API base URL")
	flag.StringVar(&cfg.APIKey, "api-key", envOrDefault("TRAFFICGEN_API_KEY", "dev-key-12345"), "API key")
	flag.DurationVar(&cfg.Duration, "duration", envDurationOrDefault("TRAFFICGEN_DURATION", 30*time.Second), "test duration")
	flag.IntVar(&cfg.Concurrency, "concurrency", envIntOrDefault("TRAFFICGEN_CONCURRENCY", 20), "worker concurrency")
	flag.IntVar(&cfg.RPS, "rps", envIntOrDefault("TRAFFICGEN_RPS", 1000), "target request rate (0=unlimited)")
	flag.IntVar(&cfg.BatchSize, "batch-size", envIntOrDefault("TRAFFICGEN_BATCH_SIZE", 5), "transactions per batch request")
	flag.IntVar(&cfg.AccountCount, "accounts", envIntOrDefault("TRAFFICGEN_ACCOUNTS", 100), "number of synthetic accounts")
	flag.StringVar(&cfg.Currency, "currency", envOrDefault("TRAFFICGEN_CURRENCY", "USD"), "currency code")
	flag.IntVar(&cfg.MinAmount, "min-amount", envIntOrDefault("TRAFFICGEN_MIN_AMOUNT", 1), "minimum whole amount")
	flag.IntVar(&cfg.MaxAmount, "max-amount", envIntOrDefault("TRAFFICGEN_MAX_AMOUNT", 100), "maximum whole amount")
	flag.StringVar(&cfg.CSVOutput, "csv-output", envOrDefault("TRAFFICGEN_CSV_OUTPUT", ""), "optional CSV output path for summary and per-second stats")
	flag.Parse()
	return cfg
}

func validateConfig(cfg generatorConfig) error {
	if cfg.BaseURL == "" {
		return fmt.Errorf("base-url is required")
	}
	if cfg.APIKey == "" {
		return fmt.Errorf("api-key is required")
	}
	if cfg.Duration <= 0 {
		return fmt.Errorf("duration must be > 0")
	}
	if cfg.Concurrency <= 0 {
		return fmt.Errorf("concurrency must be > 0")
	}
	if cfg.BatchSize <= 0 {
		return fmt.Errorf("batch-size must be > 0")
	}
	if cfg.AccountCount < 2 {
		return fmt.Errorf("accounts must be >= 2")
	}
	if cfg.MinAmount <= 0 || cfg.MaxAmount < cfg.MinAmount {
		return fmt.Errorf("invalid amount range")
	}
	return nil
}

func seedAccounts(client *http.Client, cfg generatorConfig) ([]string, error) {
	accountIDs := make([]string, 0, cfg.AccountCount)
	for i := 0; i < cfg.AccountCount; i++ {
		reqBody := createAccountRequest{
			AccountType: "asset",
			Currencies: []struct {
				CurrencyCode  string `json:"currency_code"`
				AllowNegative bool   `json:"allow_negative"`
			}{
				{CurrencyCode: cfg.Currency, AllowNegative: true},
			},
			Metadata: map[string]interface{}{"source": "trafficgen", "idx": i},
		}
		body, _ := json.Marshal(reqBody)
		req, err := http.NewRequest(http.MethodPost, cfg.BaseURL+"/v1/accounts", bytes.NewReader(body))
		if err != nil {
			return nil, err
		}
		req.Header.Set("X-API-Key", cfg.APIKey)
		req.Header.Set("Content-Type", "application/json")

		resp, err := client.Do(req)
		if err != nil {
			return nil, err
		}
		var parsed createAccountResponse
		decodeErr := json.NewDecoder(resp.Body).Decode(&parsed)
		resp.Body.Close()
		if resp.StatusCode < 200 || resp.StatusCode >= 300 {
			return nil, fmt.Errorf("account creation failed: status=%d", resp.StatusCode)
		}
		if decodeErr != nil {
			return nil, decodeErr
		}
		if parsed.AccountID == "" {
			return nil, fmt.Errorf("empty account_id in create account response")
		}
		accountIDs = append(accountIDs, parsed.AccountID)
	}
	return accountIDs, nil
}

func postSyntheticBatch(client *http.Client, cfg generatorConfig, accountIDs []string, r *rand.Rand) error {
	transactions := make([]postTransactionRequest, 0, cfg.BatchSize)
	for i := 0; i < cfg.BatchSize; i++ {
		debitIdx := r.Intn(len(accountIDs))
		creditIdx := r.Intn(len(accountIDs) - 1)
		if creditIdx >= debitIdx {
			creditIdx++
		}

		amt := r.Intn(cfg.MaxAmount-cfg.MinAmount+1) + cfg.MinAmount
		amount := fmt.Sprintf("%d.00", amt)

		transactions = append(transactions, postTransactionRequest{
			IdempotencyKey: uuid.NewString(),
			State:          "settled",
			Entries: []entryRequest{
				{
					AccountID:    accountIDs[debitIdx],
					CurrencyCode: cfg.Currency,
					Amount:       amount,
					EntryType:    "debit",
				},
				{
					AccountID:    accountIDs[creditIdx],
					CurrencyCode: cfg.Currency,
					Amount:       amount,
					EntryType:    "credit",
				},
			},
			Metadata: map[string]interface{}{"source": "trafficgen"},
		})
	}

	payload, _ := json.Marshal(postBatchRequest{Transactions: transactions})
	req, err := http.NewRequest(http.MethodPost, cfg.BaseURL+"/v1/transactions/batch", bytes.NewReader(payload))
	if err != nil {
		return err
	}
	req.Header.Set("X-API-Key", cfg.APIKey)
	req.Header.Set("Content-Type", "application/json")

	resp, err := client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return fmt.Errorf("status %d", resp.StatusCode)
	}
	return nil
}

func record(stats *runStats, latency time.Duration, ok bool) {
	if ok {
		atomic.AddInt64(&stats.success, 1)
	} else {
		atomic.AddInt64(&stats.failed, 1)
	}
	atomic.AddInt64(&stats.totalNanos, latency.Nanoseconds())

	stats.latencyMu.Lock()
	if len(stats.latencyNanos) < cap(stats.latencyNanos) {
		stats.latencyNanos = append(stats.latencyNanos, latency.Nanoseconds())
	}
	stats.latencyMu.Unlock()
}

func printSummary(cfg generatorConfig, startedAt time.Time, stats *runStats) {
	totalReq := atomic.LoadInt64(&stats.success) + atomic.LoadInt64(&stats.failed)
	elapsed := time.Since(startedAt)
	if totalReq == 0 {
		fmt.Println("no requests sent")
		return
	}

	stats.latencyMu.Lock()
	samples := append([]int64(nil), stats.latencyNanos...)
	stats.latencyMu.Unlock()
	sort.Slice(samples, func(i, j int) bool { return samples[i] < samples[j] })

	p50 := percentile(samples, 0.50)
	p95 := percentile(samples, 0.95)
	p99 := percentile(samples, 0.99)
	avg := time.Duration(atomic.LoadInt64(&stats.totalNanos) / totalReq)

	fmt.Println("----- trafficgen summary -----")
	fmt.Printf("duration:      %s\n", elapsed.Round(time.Millisecond))
	fmt.Printf("requests:      %d\n", totalReq)
	fmt.Printf("success:       %d\n", atomic.LoadInt64(&stats.success))
	fmt.Printf("failed:        %d\n", atomic.LoadInt64(&stats.failed))
	fmt.Printf("achieved rps:  %.2f\n", float64(totalReq)/elapsed.Seconds())
	fmt.Printf("avg latency:   %s\n", avg)
	fmt.Printf("p50 latency:   %s\n", p50)
	fmt.Printf("p95 latency:   %s\n", p95)
	fmt.Printf("p99 latency:   %s\n", p99)
	fmt.Printf("batch size:    %d\n", cfg.BatchSize)
	if cfg.CSVOutput != "" {
		if err := writeCSV(cfg.CSVOutput, elapsed, totalReq, atomic.LoadInt64(&stats.success), atomic.LoadInt64(&stats.failed), avg, p50, p95, p99, cfg.BatchSize, stats); err != nil {
			fmt.Printf("csv export failed: %v\n", err)
		} else {
			fmt.Printf("csv output:    %s\n", cfg.CSVOutput)
		}
	}
}

func percentile(samples []int64, p float64) time.Duration {
	if len(samples) == 0 {
		return 0
	}
	idx := int(float64(len(samples)-1) * p)
	if idx < 0 {
		idx = 0
	}
	if idx >= len(samples) {
		idx = len(samples) - 1
	}
	return time.Duration(samples[idx])
}

func envOrDefault(key, fallback string) string {
	v := os.Getenv(key)
	if v == "" {
		return fallback
	}
	return v
}

func envIntOrDefault(key string, fallback int) int {
	v := os.Getenv(key)
	if v == "" {
		return fallback
	}
	var parsed int
	if _, err := fmt.Sscanf(v, "%d", &parsed); err != nil {
		return fallback
	}
	return parsed
}

func envDurationOrDefault(key string, fallback time.Duration) time.Duration {
	v := os.Getenv(key)
	if v == "" {
		return fallback
	}
	parsed, err := time.ParseDuration(v)
	if err != nil {
		return fallback
	}
	return parsed
}

func printLiveStats(ctx context.Context, stats *runStats, startedAt time.Time) {
	ticker := time.NewTicker(1 * time.Second)
	defer ticker.Stop()

	prevSuccess := int64(0)
	prevFailed := int64(0)
	sec := 0
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			sec++
			s := atomic.LoadInt64(&stats.success)
			f := atomic.LoadInt64(&stats.failed)
			deltaSuccess := s - prevSuccess
			deltaFailed := f - prevFailed
			prevSuccess = s
			prevFailed = f
			elapsed := time.Since(startedAt).Seconds()
			overallRPS := 0.0
			if elapsed > 0 {
				overallRPS = float64(s+f) / elapsed
			}
			fmt.Printf("live t=%02ds success=%d failed=%d overall_rps=%.1f\n", sec, deltaSuccess, deltaFailed, overallRPS)
			stats.latencyMu.Lock()
			stats.perSecond = append(stats.perSecond, secondStat{
				Second:      sec,
				Success:     deltaSuccess,
				Failed:      deltaFailed,
				AchievedRPS: overallRPS,
			})
			stats.latencyMu.Unlock()
		}
	}
}

func writeCSV(path string, elapsed time.Duration, total, success, failed int64, avg, p50, p95, p99 time.Duration, batchSize int, stats *runStats) error {
	f, err := os.Create(path)
	if err != nil {
		return err
	}
	defer f.Close()

	w := csv.NewWriter(f)
	defer w.Flush()

	if err := w.Write([]string{"metric", "value"}); err != nil {
		return err
	}
	rows := [][]string{
		{"duration_ms", fmt.Sprintf("%d", elapsed.Milliseconds())},
		{"requests_total", fmt.Sprintf("%d", total)},
		{"success_total", fmt.Sprintf("%d", success)},
		{"failed_total", fmt.Sprintf("%d", failed)},
		{"avg_latency_ms", fmt.Sprintf("%.3f", float64(avg.Microseconds())/1000.0)},
		{"p50_latency_ms", fmt.Sprintf("%.3f", float64(p50.Microseconds())/1000.0)},
		{"p95_latency_ms", fmt.Sprintf("%.3f", float64(p95.Microseconds())/1000.0)},
		{"p99_latency_ms", fmt.Sprintf("%.3f", float64(p99.Microseconds())/1000.0)},
		{"batch_size", fmt.Sprintf("%d", batchSize)},
	}
	for _, r := range rows {
		if err := w.Write(r); err != nil {
			return err
		}
	}
	if err := w.Write([]string{}); err != nil {
		return err
	}
	if err := w.Write([]string{"second", "success", "failed", "overall_rps"}); err != nil {
		return err
	}

	stats.latencyMu.Lock()
	defer stats.latencyMu.Unlock()
	for _, s := range stats.perSecond {
		if err := w.Write([]string{
			fmt.Sprintf("%d", s.Second),
			fmt.Sprintf("%d", s.Success),
			fmt.Sprintf("%d", s.Failed),
			fmt.Sprintf("%.3f", s.AchievedRPS),
		}); err != nil {
			return err
		}
	}
	return nil
}

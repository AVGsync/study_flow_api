package main

import (
	"bytes"
	"context"
	"crypto/tls"
	"errors"
	"flag"
	"fmt"
	"io"
	"math"
	"net"
	"net/http"
	"net/url"
	"os"
	"sort"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"text/tabwriter"
	"time"

	"github.com/gorilla/websocket"
)

type successMatcher struct {
	accept2xx bool
	exact     map[int]struct{}
}

func (m successMatcher) Match(code int) bool {
	if m.accept2xx && code >= 200 && code <= 299 {
		return true
	}
	_, ok := m.exact[code]
	return ok
}

type config struct {
	baseURL           string
	endpoint          string
	method            string
	body              string
	contentType       string
	token             string
	tokens            []string
	startConcurrency  int
	maxConcurrency    int
	stepConcurrency   int
	requestsPerWorker int
	requestTimeout    time.Duration
	maxErrorRate      float64
	p95Limit          time.Duration
	okStatuses        successMatcher
	warmupRequests    int
	insecureTLS       bool
	wsEndpoint        string
	skipWSCheck       bool
	largePayloadKB    int
}

type sample struct {
	statusCode int
	latency    time.Duration
	err        error
	timeout    bool
}

type loadResult struct {
	concurrency int
	total       int
	success     int
	httpErrors  int
	netErrors   int
	timeouts    int
	throughput  float64
	errorRate   float64
	min         time.Duration
	avg         time.Duration
	p50         time.Duration
	p95         time.Duration
	p99         time.Duration
	max         time.Duration
	stable      bool
}

const (
	statusPass = "PASS"
	statusWarn = "WARN"
	statusFail = "FAIL"
	statusSkip = "SKIP"
)

type securityResult struct {
	name   string
	status string
	detail string
}

type requestPlan struct {
	targetURL   string
	method      string
	body        string
	contentType string
	tokens      []string
	nextToken   uint64
}

func newRequestPlan(cfg config, targetURL string) *requestPlan {
	return &requestPlan{
		targetURL:   targetURL,
		method:      cfg.method,
		body:        cfg.body,
		contentType: cfg.contentType,
		tokens:      cfg.tokens,
	}
}

// Для нагрузки раздаем реальные JWT пользователей по кругу, чтобы нагрузка была ближе к живому трафику.
func (p *requestPlan) BuildRequest() (*http.Request, error) {
	token := p.pickToken()
	return buildRequest(p.method, p.targetURL, p.body, p.contentType, token, token != "")
}

func (p *requestPlan) pickToken() string {
	if len(p.tokens) == 0 {
		return ""
	}

	idx := atomic.AddUint64(&p.nextToken, 1) - 1
	return p.tokens[int(idx%uint64(len(p.tokens)))]
}

func main() {
	cfg, err := parseConfig()
	if err != nil {
		fmt.Printf("Ошибка конфигурации: %v\n", err)
		os.Exit(1)
	}

	targetURL, err := resolveTargetURL(cfg.baseURL, cfg.endpoint)
	if err != nil {
		fmt.Printf("Ошибка формирования URL: %v\n", err)
		os.Exit(1)
	}

	plan := newRequestPlan(cfg, targetURL)
	client := newHTTPClient(cfg)
	fmt.Println("============================================")
	fmt.Println("  Проверка API: нагрузка + базовая безопасность")
	fmt.Println("============================================")
	fmt.Printf("Цель: %s %s\n", cfg.method, targetURL)
	fmt.Printf("Критерий устойчивости: ошибки <= %.2f%%, p95 <= %s\n", cfg.maxErrorRate, cfg.p95Limit)
	fmt.Println()

	if cfg.token == "" {
		fmt.Println("Внимание: токен не передан. Для защищенных эндпоинтов часть тестов покажет 401/403.")
		fmt.Println()
	}

	if cfg.warmupRequests > 0 {
		runWarmup(client, cfg, plan)
	}

	results := runLoadTest(client, cfg, plan)
	printLoadResults(results)
	printCapacitySummary(results)

	security := runSecurityChecks(client, cfg, targetURL)
	printSecurityResults(security)
}

func parseConfig() (config, error) {
	baseURL := flag.String("base-url", "http://localhost:8082", "Базовый URL API (например, http://localhost:8082)")
	endpoint := flag.String("endpoint", "/api/user", "Эндпоинт для нагрузочного теста")
	method := flag.String("method", "GET", "HTTP-метод для нагрузочного теста")
	body := flag.String("body", "", "Тело запроса для нагрузочного теста (JSON строкой)")
	contentType := flag.String("content-type", "application/json", "Content-Type для запросов с телом")
	token := flag.String("token", "", "JWT токен без префикса Bearer")
	tokensRaw := flag.String("tokens", "", "JWT токены через запятую без префикса Bearer")
	tokensFile := flag.String("tokens-file", "", "Путь к файлу с JWT токенами: один токен на строку")
	startConcurrency := flag.Int("start-concurrency", 10, "Начальная конкурентность")
	maxConcurrency := flag.Int("max-concurrency", 200, "Максимальная конкурентность")
	stepConcurrency := flag.Int("step", 10, "Шаг увеличения конкурентности")
	requestsPerWorker := flag.Int("requests-per-worker", 30, "Количество запросов на один воркер")
	requestTimeout := flag.Duration("request-timeout", 5*time.Second, "Таймаут одного запроса")
	maxErrorRate := flag.Float64("max-error-rate", 5.0, "Порог ошибок в процентах для устойчивой нагрузки")
	p95Limit := flag.Duration("p95-limit", 700*time.Millisecond, "Порог p95 для устойчивой нагрузки")
	okStatusesRaw := flag.String("ok-statuses", "2xx", "Успешные коды: 2xx или список через запятую, например 200,204")
	warmupRequests := flag.Int("warmup-requests", 20, "Количество разогревочных запросов перед тестом")
	insecureTLS := flag.Bool("insecure-tls", false, "Игнорировать ошибки TLS сертификата (только для локальных тестов)")
	wsEndpoint := flag.String("ws-endpoint", "/api/ws", "WebSocket эндпоинт для проверки Origin-политики")
	skipWSCheck := flag.Bool("skip-ws-check", false, "Пропустить WebSocket проверку Origin")
	largePayloadKB := flag.Int("large-payload-kb", 256, "Размер большого JSON payload для security-проверки (KB)")

	flag.Parse()

	okStatuses, err := parseOKStatuses(*okStatusesRaw)
	if err != nil {
		return config{}, err
	}

	tokens, err := loadTokens(*token, *tokensRaw, *tokensFile)
	if err != nil {
		return config{}, err
	}

	primaryToken := ""
	if len(tokens) > 0 {
		primaryToken = tokens[0]
	}

	cfg := config{
		baseURL:           strings.TrimSpace(*baseURL),
		endpoint:          strings.TrimSpace(*endpoint),
		method:            strings.ToUpper(strings.TrimSpace(*method)),
		body:              *body,
		contentType:       strings.TrimSpace(*contentType),
		token:             primaryToken,
		tokens:            tokens,
		startConcurrency:  *startConcurrency,
		maxConcurrency:    *maxConcurrency,
		stepConcurrency:   *stepConcurrency,
		requestsPerWorker: *requestsPerWorker,
		requestTimeout:    *requestTimeout,
		maxErrorRate:      *maxErrorRate,
		p95Limit:          *p95Limit,
		okStatuses:        okStatuses,
		warmupRequests:    *warmupRequests,
		insecureTLS:       *insecureTLS,
		wsEndpoint:        strings.TrimSpace(*wsEndpoint),
		skipWSCheck:       *skipWSCheck,
		largePayloadKB:    *largePayloadKB,
	}

	if cfg.baseURL == "" {
		return config{}, fmt.Errorf("base-url не должен быть пустым")
	}
	if cfg.endpoint == "" {
		return config{}, fmt.Errorf("endpoint не должен быть пустым")
	}
	if cfg.startConcurrency < 1 || cfg.maxConcurrency < 1 || cfg.stepConcurrency < 1 {
		return config{}, fmt.Errorf("конкурентность и шаг должны быть > 0")
	}
	if cfg.startConcurrency > cfg.maxConcurrency {
		return config{}, fmt.Errorf("start-concurrency не может быть больше max-concurrency")
	}
	if cfg.requestsPerWorker < 1 {
		return config{}, fmt.Errorf("requests-per-worker должен быть > 0")
	}
	if cfg.requestTimeout <= 0 {
		return config{}, fmt.Errorf("request-timeout должен быть > 0")
	}
	if cfg.maxErrorRate < 0 || cfg.maxErrorRate > 100 {
		return config{}, fmt.Errorf("max-error-rate должен быть в диапазоне 0..100")
	}
	if cfg.p95Limit <= 0 {
		return config{}, fmt.Errorf("p95-limit должен быть > 0")
	}
	if cfg.largePayloadKB < 1 {
		return config{}, fmt.Errorf("large-payload-kb должен быть > 0")
	}

	return cfg, nil
}

func parseOKStatuses(raw string) (successMatcher, error) {
	parts := strings.Split(raw, ",")
	matcher := successMatcher{
		exact: make(map[int]struct{}),
	}

	for _, p := range parts {
		token := strings.TrimSpace(strings.ToLower(p))
		if token == "" {
			continue
		}
		if token == "2xx" {
			matcher.accept2xx = true
			continue
		}

		code, err := strconv.Atoi(token)
		if err != nil {
			return successMatcher{}, fmt.Errorf("ok-statuses: не удалось разобрать %q", token)
		}
		if code < 100 || code > 599 {
			return successMatcher{}, fmt.Errorf("ok-statuses: код %d вне диапазона 100..599", code)
		}
		matcher.exact[code] = struct{}{}
	}

	if !matcher.accept2xx && len(matcher.exact) == 0 {
		return successMatcher{}, fmt.Errorf("ok-statuses: нужно указать 2xx или хотя бы один код")
	}

	return matcher, nil
}

func loadTokens(singleToken, tokensRaw, tokensFile string) ([]string, error) {
	seen := make(map[string]struct{})
	tokens := make([]string, 0)

	// Собираем токены из всех источников в один пул, чтобы потом крутить реальные пользовательские запросы по round-robin.
	addToken := func(raw string) {
		token := strings.TrimSpace(raw)
		if token == "" {
			return
		}

		if strings.HasPrefix(strings.ToLower(token), "bearer ") {
			token = strings.TrimSpace(token[len("bearer "):])
		}
		if token == "" {
			return
		}

		if _, exists := seen[token]; exists {
			return
		}

		seen[token] = struct{}{}
		tokens = append(tokens, token)
	}

	addToken(singleToken)

	for _, raw := range strings.Split(tokensRaw, ",") {
		addToken(raw)
	}

	if strings.TrimSpace(tokensFile) == "" {
		return tokens, nil
	}

	data, err := os.ReadFile(strings.TrimSpace(tokensFile))
	if err != nil {
		return nil, fmt.Errorf("tokens-file: %w", err)
	}

	for _, line := range strings.Split(string(data), "\n") {
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		addToken(line)
	}

	return tokens, nil
}

func newHTTPClient(cfg config) *http.Client {
	transport := &http.Transport{
		Proxy:                 http.ProxyFromEnvironment,
		DialContext:           (&net.Dialer{Timeout: cfg.requestTimeout}).DialContext,
		ForceAttemptHTTP2:     true,
		MaxIdleConns:          cfg.maxConcurrency * 4,
		MaxIdleConnsPerHost:   cfg.maxConcurrency * 2,
		IdleConnTimeout:       90 * time.Second,
		TLSHandshakeTimeout:   cfg.requestTimeout,
		ExpectContinueTimeout: 1 * time.Second,
		TLSClientConfig: &tls.Config{
			InsecureSkipVerify: cfg.insecureTLS,
		},
	}

	return &http.Client{
		Timeout:   cfg.requestTimeout,
		Transport: transport,
	}
}

func resolveTargetURL(baseURL, endpoint string) (string, error) {
	base, err := url.Parse(baseURL)
	if err != nil {
		return "", err
	}
	ref, err := url.Parse(endpoint)
	if err != nil {
		return "", err
	}
	return base.ResolveReference(ref).String(), nil
}

func runWarmup(client *http.Client, cfg config, plan *requestPlan) {
	fmt.Printf("Разогрев: %d запросов...\n", cfg.warmupRequests)
	ok := 0
	for i := 0; i < cfg.warmupRequests; i++ {
		req, err := plan.BuildRequest()
		if err != nil {
			continue
		}
		resp, err := client.Do(req)
		if err == nil {
			io.Copy(io.Discard, io.LimitReader(resp.Body, 1<<20))
			resp.Body.Close()
			if cfg.okStatuses.Match(resp.StatusCode) {
				ok++
			}
		}
	}
	fmt.Printf("Разогрев завершен: успешных %d из %d.\n\n", ok, cfg.warmupRequests)
}

func runLoadTest(client *http.Client, cfg config, plan *requestPlan) []loadResult {
	results := make([]loadResult, 0, ((cfg.maxConcurrency-cfg.startConcurrency)/cfg.stepConcurrency)+1)

	for c := cfg.startConcurrency; c <= cfg.maxConcurrency; c += cfg.stepConcurrency {
		result := executeLevel(client, cfg, plan, c)
		result.stable = result.errorRate <= cfg.maxErrorRate && result.p95 <= cfg.p95Limit
		results = append(results, result)
	}

	return results
}

func executeLevel(client *http.Client, cfg config, plan *requestPlan, concurrency int) loadResult {
	totalRequests := concurrency * cfg.requestsPerWorker
	samples := make(chan sample, totalRequests)
	var wg sync.WaitGroup

	started := time.Now()
	for worker := 0; worker < concurrency; worker++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for i := 0; i < cfg.requestsPerWorker; i++ {
				req, err := plan.BuildRequest()
				if err != nil {
					samples <- sample{err: err}
					continue
				}

				reqStart := time.Now()
				resp, err := client.Do(req)
				latency := time.Since(reqStart)
				if err != nil {
					samples <- sample{
						latency: latency,
						err:     err,
						timeout: isTimeoutErr(err),
					}
					continue
				}

				io.Copy(io.Discard, io.LimitReader(resp.Body, 1<<20))
				resp.Body.Close()
				samples <- sample{
					statusCode: resp.StatusCode,
					latency:    latency,
				}
			}
		}()
	}

	go func() {
		wg.Wait()
		close(samples)
	}()

	latencies := make([]time.Duration, 0, totalRequests)
	result := loadResult{
		concurrency: concurrency,
		total:       totalRequests,
	}

	for s := range samples {
		if s.err != nil {
			result.netErrors++
			if s.timeout {
				result.timeouts++
			}
			if s.latency > 0 {
				latencies = append(latencies, s.latency)
			}
			continue
		}

		latencies = append(latencies, s.latency)
		if cfg.okStatuses.Match(s.statusCode) {
			result.success++
		} else {
			result.httpErrors++
		}
	}

	elapsed := time.Since(started)
	if elapsed > 0 {
		result.throughput = float64(result.total) / elapsed.Seconds()
	}

	failures := result.httpErrors + result.netErrors
	if result.total > 0 {
		result.errorRate = (float64(failures) / float64(result.total)) * 100
	}

	if len(latencies) > 0 {
		sort.Slice(latencies, func(i, j int) bool { return latencies[i] < latencies[j] })
		result.min = latencies[0]
		result.max = latencies[len(latencies)-1]
		result.avg = averageLatency(latencies)
		result.p50 = percentile(latencies, 50)
		result.p95 = percentile(latencies, 95)
		result.p99 = percentile(latencies, 99)
	}

	return result
}

func averageLatency(values []time.Duration) time.Duration {
	if len(values) == 0 {
		return 0
	}
	var total time.Duration
	for _, v := range values {
		total += v
	}
	return total / time.Duration(len(values))
}

func percentile(sorted []time.Duration, p float64) time.Duration {
	if len(sorted) == 0 {
		return 0
	}
	if p <= 0 {
		return sorted[0]
	}
	if p >= 100 {
		return sorted[len(sorted)-1]
	}

	rank := int(math.Ceil((p/100.0)*float64(len(sorted)))) - 1
	if rank < 0 {
		rank = 0
	}
	if rank >= len(sorted) {
		rank = len(sorted) - 1
	}
	return sorted[rank]
}

func buildRequest(method, targetURL, body, contentType, token string, withAuth bool) (*http.Request, error) {
	var reader io.Reader
	if body != "" {
		reader = bytes.NewBufferString(body)
	}

	req, err := http.NewRequestWithContext(context.Background(), method, targetURL, reader)
	if err != nil {
		return nil, err
	}

	if body != "" && contentType != "" {
		req.Header.Set("Content-Type", contentType)
	}
	if withAuth && token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}
	return req, nil
}

func isTimeoutErr(err error) bool {
	if err == nil {
		return false
	}
	if errors.Is(err, context.DeadlineExceeded) {
		return true
	}
	var nerr net.Error
	return errors.As(err, &nerr) && nerr.Timeout()
}

func printLoadResults(results []loadResult) {
	fmt.Println("Результаты нагрузочного теста:")
	w := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
	fmt.Fprintln(w, "Конкурентность\tЗапросов\tУспешно\tHTTP ошибки\tСетевые ошибки\tТаймауты\tRPS\tAvg\tP95\tP99\tСтатус")
	for _, r := range results {
		state := "НЕ выдержал"
		if r.stable {
			state = "Выдержал"
		}
		fmt.Fprintf(
			w,
			"%d\t%d\t%d\t%d\t%d\t%d\t%.1f\t%s\t%s\t%s\t%s\n",
			r.concurrency,
			r.total,
			r.success,
			r.httpErrors,
			r.netErrors,
			r.timeouts,
			r.throughput,
			r.avg,
			r.p95,
			r.p99,
			state,
		)
	}
	w.Flush()
	fmt.Println()
}

func printCapacitySummary(results []loadResult) {
	var bestStable *loadResult
	var bestThroughput *loadResult

	for i := range results {
		if bestThroughput == nil || results[i].throughput > bestThroughput.throughput {
			bestThroughput = &results[i]
		}
		if results[i].stable {
			if bestStable == nil || results[i].concurrency > bestStable.concurrency {
				bestStable = &results[i]
			}
		}
	}

	fmt.Println("Сводка по производительности:")
	if bestStable == nil {
		fmt.Println("- Устойчивый уровень не найден по заданным порогам.")
		fmt.Println("- Рекомендация: уменьшите `step`, `requests-per-worker` или ослабьте пороги, чтобы найти рабочий диапазон.")
	} else {
		fmt.Printf("- Максимальная устойчиво выдерживаемая конкурентность: %d\n", bestStable.concurrency)
		fmt.Printf("- На этом уровне: RPS=%.1f, ошибки=%.2f%%, P95=%s\n", bestStable.throughput, bestStable.errorRate, bestStable.p95)
	}
	if bestThroughput != nil {
		fmt.Printf("- Пиковая пропускная способность: %.1f RPS (конкурентность %d)\n", bestThroughput.throughput, bestThroughput.concurrency)
	}
	fmt.Println()
}

func runSecurityChecks(client *http.Client, cfg config, targetURL string) []securityResult {
	results := make([]securityResult, 0, 7)

	results = append(results, checkTLS(cfg.baseURL))
	results = append(results, checkAuthBypass(client, cfg, targetURL))
	results = append(results, checkSecurityHeaders(client, cfg, targetURL))
	results = append(results, checkTraceMethod(client, cfg, targetURL))
	results = append(results, checkSQLInjectionProbe(client, cfg, targetURL))
	results = append(results, checkLargePayload(client, cfg, targetURL))
	results = append(results, checkWebSocketOrigin(client, cfg))

	return results
}

func checkTLS(baseURL string) securityResult {
	u, err := url.Parse(baseURL)
	if err != nil {
		return securityResult{
			name:   "TLS",
			status: statusWarn,
			detail: fmt.Sprintf("Не удалось разобрать URL: %v", err),
		}
	}
	if strings.EqualFold(u.Scheme, "https") {
		return securityResult{name: "TLS", status: statusPass, detail: "Используется HTTPS."}
	}
	return securityResult{
		name:   "TLS",
		status: statusWarn,
		detail: "Используется HTTP без TLS. Для production это риск перехвата трафика.",
	}
}

func checkAuthBypass(client *http.Client, cfg config, targetURL string) securityResult {
	req, err := buildRequest(cfg.method, targetURL, cfg.body, cfg.contentType, "", false)
	if err != nil {
		return securityResult{name: "Auth bypass", status: statusWarn, detail: fmt.Sprintf("Не удалось собрать запрос: %v", err)}
	}

	resp, err := client.Do(req)
	if err != nil {
		return securityResult{name: "Auth bypass", status: statusWarn, detail: fmt.Sprintf("Ошибка запроса: %v", err)}
	}
	io.Copy(io.Discard, io.LimitReader(resp.Body, 1<<20))
	resp.Body.Close()

	switch {
	case resp.StatusCode == http.StatusUnauthorized || resp.StatusCode == http.StatusForbidden:
		return securityResult{name: "Auth bypass", status: statusPass, detail: fmt.Sprintf("Без токена доступ закрыт (%d).", resp.StatusCode)}
	case resp.StatusCode >= 200 && resp.StatusCode <= 299:
		return securityResult{name: "Auth bypass", status: statusFail, detail: fmt.Sprintf("Эндпоинт доступен без токена (%d).", resp.StatusCode)}
	default:
		return securityResult{name: "Auth bypass", status: statusWarn, detail: fmt.Sprintf("Получен код %d; проверьте вручную, действительно ли эндпоинт должен быть защищен.", resp.StatusCode)}
	}
}

func checkSecurityHeaders(client *http.Client, cfg config, targetURL string) securityResult {
	req, err := buildRequest(http.MethodGet, targetURL, "", cfg.contentType, cfg.token, true)
	if err != nil {
		return securityResult{name: "Security headers", status: statusWarn, detail: fmt.Sprintf("Не удалось собрать запрос: %v", err)}
	}
	req.Header.Set("Accept", "application/json")

	resp, err := client.Do(req)
	if err != nil {
		return securityResult{name: "Security headers", status: statusWarn, detail: fmt.Sprintf("Ошибка запроса: %v", err)}
	}
	io.Copy(io.Discard, io.LimitReader(resp.Body, 1<<20))
	resp.Body.Close()

	required := []string{
		"X-Content-Type-Options",
		"X-Frame-Options",
		"Referrer-Policy",
	}
	missing := make([]string, 0, len(required))
	for _, h := range required {
		if strings.TrimSpace(resp.Header.Get(h)) == "" {
			missing = append(missing, h)
		}
	}

	if len(missing) == 0 {
		return securityResult{name: "Security headers", status: statusPass, detail: "Базовые защитные заголовки присутствуют."}
	}
	return securityResult{
		name:   "Security headers",
		status: statusWarn,
		detail: fmt.Sprintf("Отсутствуют заголовки: %s.", strings.Join(missing, ", ")),
	}
}

func checkTraceMethod(client *http.Client, cfg config, targetURL string) securityResult {
	req, err := buildRequest(http.MethodTrace, targetURL, "", cfg.contentType, cfg.token, cfg.token != "")
	if err != nil {
		return securityResult{name: "TRACE method", status: statusWarn, detail: fmt.Sprintf("Не удалось собрать запрос: %v", err)}
	}

	resp, err := client.Do(req)
	if err != nil {
		return securityResult{name: "TRACE method", status: statusWarn, detail: fmt.Sprintf("Ошибка запроса: %v", err)}
	}
	io.Copy(io.Discard, io.LimitReader(resp.Body, 1<<20))
	resp.Body.Close()

	if resp.StatusCode >= 200 && resp.StatusCode <= 299 {
		return securityResult{name: "TRACE method", status: statusFail, detail: fmt.Sprintf("TRACE разрешен (%d).", resp.StatusCode)}
	}
	return securityResult{name: "TRACE method", status: statusPass, detail: fmt.Sprintf("TRACE не разрешен (%d).", resp.StatusCode)}
}

func checkSQLInjectionProbe(client *http.Client, cfg config, targetURL string) securityResult {
	u, err := url.Parse(targetURL)
	if err != nil {
		return securityResult{name: "SQLi probe", status: statusWarn, detail: fmt.Sprintf("Не удалось разобрать URL: %v", err)}
	}
	q := u.Query()
	q.Set("id", "' OR 1=1 --")
	u.RawQuery = q.Encode()

	req, err := buildRequest(http.MethodGet, u.String(), "", cfg.contentType, cfg.token, true)
	if err != nil {
		return securityResult{name: "SQLi probe", status: statusWarn, detail: fmt.Sprintf("Не удалось собрать запрос: %v", err)}
	}

	resp, err := client.Do(req)
	if err != nil {
		return securityResult{name: "SQLi probe", status: statusWarn, detail: fmt.Sprintf("Ошибка запроса: %v", err)}
	}
	io.Copy(io.Discard, io.LimitReader(resp.Body, 1<<20))
	resp.Body.Close()

	if resp.StatusCode >= 500 {
		return securityResult{name: "SQLi probe", status: statusFail, detail: fmt.Sprintf("На инъекционный payload сервер ответил %d (внутренняя ошибка).", resp.StatusCode)}
	}
	return securityResult{name: "SQLi probe", status: statusPass, detail: fmt.Sprintf("Инъекционный payload не привел к 5xx (код %d).", resp.StatusCode)}
}

func checkLargePayload(client *http.Client, cfg config, targetURL string) securityResult {
	large := strings.Repeat("A", cfg.largePayloadKB*1024)
	payload := fmt.Sprintf(`{"login":"%s","email":"load.test@example.com"}`, large)
	req, err := buildRequest(http.MethodPatch, targetURL, payload, "application/json", cfg.token, true)
	if err != nil {
		return securityResult{name: "Large payload", status: statusWarn, detail: fmt.Sprintf("Не удалось собрать запрос: %v", err)}
	}

	resp, err := client.Do(req)
	if err != nil {
		return securityResult{name: "Large payload", status: statusWarn, detail: fmt.Sprintf("Ошибка запроса: %v", err)}
	}
	io.Copy(io.Discard, io.LimitReader(resp.Body, 1<<20))
	resp.Body.Close()

	if resp.StatusCode >= 500 {
		return securityResult{name: "Large payload", status: statusFail, detail: fmt.Sprintf("Большой payload вызывает 5xx (%d).", resp.StatusCode)}
	}
	return securityResult{name: "Large payload", status: statusPass, detail: fmt.Sprintf("Большой payload обработан без 5xx (код %d).", resp.StatusCode)}
}

func checkWebSocketOrigin(client *http.Client, cfg config) securityResult {
	if cfg.skipWSCheck {
		return securityResult{name: "WebSocket Origin", status: statusSkip, detail: "Проверка пропущена флагом --skip-ws-check."}
	}
	if cfg.token == "" {
		return securityResult{name: "WebSocket Origin", status: statusSkip, detail: "Нет JWT токена для аутентифицированного подключения к WebSocket."}
	}

	wsURL, err := resolveWebSocketURL(cfg.baseURL, cfg.wsEndpoint)
	if err != nil {
		return securityResult{name: "WebSocket Origin", status: statusWarn, detail: fmt.Sprintf("Не удалось собрать WS URL: %v", err)}
	}

	dialer := websocket.Dialer{
		HandshakeTimeout: 5 * time.Second,
	}
	header := http.Header{}
	header.Set("Authorization", "Bearer "+cfg.token)
	header.Set("Origin", "https://evil.example")

	conn, resp, err := dialer.Dial(wsURL, header)
	if err != nil {
		if resp != nil {
			if resp.StatusCode == http.StatusForbidden || resp.StatusCode == http.StatusUnauthorized {
				return securityResult{name: "WebSocket Origin", status: statusPass, detail: fmt.Sprintf("Подключение с чужим Origin отклонено (%d).", resp.StatusCode)}
			}
			return securityResult{name: "WebSocket Origin", status: statusWarn, detail: fmt.Sprintf("Подключение не установлено (код %d). Проверьте Origin-политику вручную.", resp.StatusCode)}
		}
		return securityResult{name: "WebSocket Origin", status: statusWarn, detail: fmt.Sprintf("Ошибка подключения: %v", err)}
	}
	conn.Close()

	return securityResult{
		name:   "WebSocket Origin",
		status: statusFail,
		detail: "Соединение установлено с поддельным Origin. Возможна CSWSH-уязвимость.",
	}
}

func resolveWebSocketURL(baseURL, wsEndpoint string) (string, error) {
	u, err := url.Parse(baseURL)
	if err != nil {
		return "", err
	}
	switch strings.ToLower(u.Scheme) {
	case "http":
		u.Scheme = "ws"
	case "https":
		u.Scheme = "wss"
	default:
		return "", fmt.Errorf("неподдерживаемая схема %q", u.Scheme)
	}

	ref, err := url.Parse(wsEndpoint)
	if err != nil {
		return "", err
	}

	return u.ResolveReference(ref).String(), nil
}

func printSecurityResults(results []securityResult) {
	fmt.Println("Базовая проверка безопасности:")
	w := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
	fmt.Fprintln(w, "Проверка\tСтатус\tДетали")
	passCount := 0
	warnCount := 0
	failCount := 0

	for _, r := range results {
		statusRu := statusToRU(r.status)
		fmt.Fprintf(w, "%s\t%s\t%s\n", r.name, statusRu, r.detail)

		switch r.status {
		case statusPass:
			passCount++
		case statusWarn:
			warnCount++
		case statusFail:
			failCount++
		}
	}
	w.Flush()
	fmt.Println()

	totalEffective := passCount + warnCount + failCount
	score := 0.0
	if totalEffective > 0 {
		score = (float64(passCount*2+warnCount) / float64(totalEffective*2)) * 100
	}

	fmt.Println("Сводка по безопасности:")
	fmt.Printf("- Критичных проблем (FAIL): %d\n", failCount)
	fmt.Printf("- Предупреждений (WARN): %d\n", warnCount)
	fmt.Printf("- Пройденных проверок (PASS): %d\n", passCount)
	fmt.Printf("- Оценка по базовым автоматическим проверкам: %.1f/100\n", score)
	if failCount > 0 {
		fmt.Println("- Итог: есть критичные риски, требуется исправление до production.")
	} else if warnCount > 0 {
		fmt.Println("- Итог: критичных автоматических срабатываний нет, но есть риски/пробелы в настройках.")
	} else {
		fmt.Println("- Итог: базовые автоматические проверки пройдены.")
	}
}

func statusToRU(status string) string {
	switch status {
	case statusPass:
		return "ПРОЙДЕНО"
	case statusWarn:
		return "ПРЕДУПРЕЖДЕНИЕ"
	case statusFail:
		return "КРИТИЧНО"
	case statusSkip:
		return "ПРОПУЩЕНО"
	default:
		return status
	}
}

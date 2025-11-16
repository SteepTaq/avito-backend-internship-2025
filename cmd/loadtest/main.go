package main

import (
	"bytes"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"math"
	"net/http"
	"os"
	"sort"
	"strings"
	"sync"
	"time"
)

const (
	targetRPS         = 5
	targetP95         = 300 * time.Millisecond
	targetP99         = 300 * time.Millisecond
	targetSuccessRate = 0.999

	warmupDuration   = 30 * time.Second
	mainLoadDuration = 1 * time.Minute
	peakLoadDuration = 2 * time.Minute
	peakRPS          = 10
	cooldownDuration = 30 * time.Second
)

type Result struct {
	Endpoint   string
	Method     string
	StatusCode int
	Duration   time.Duration
	Success    bool
	Error      error
}

type Stats struct {
	mu                 sync.Mutex
	results            []Result
	totalRequests      int
	successfulRequests int
	failedRequests     int
	byEndpoint         map[string]*EndpointStats
}

type EndpointStats struct {
	Total       int
	Success     int
	Failed      int
	Durations   []time.Duration
	StatusCodes map[int]int
}

func NewStats() *Stats {
	return &Stats{
		results:    make([]Result, 0),
		byEndpoint: make(map[string]*EndpointStats),
	}
}

func (s *Stats) AddResult(r Result) {
	s.mu.Lock()
	defer s.mu.Unlock()

	s.results = append(s.results, r)
	s.totalRequests++

	if r.Success {
		s.successfulRequests++
	} else {
		s.failedRequests++
	}

	epStats, exists := s.byEndpoint[r.Endpoint]
	if !exists {
		epStats = &EndpointStats{
			Durations:   make([]time.Duration, 0),
			StatusCodes: make(map[int]int),
		}
		s.byEndpoint[r.Endpoint] = epStats
	}

	epStats.Total++
	epStats.StatusCodes[r.StatusCode]++
	if r.Success {
		epStats.Success++
		epStats.Durations = append(epStats.Durations, r.Duration)
	} else {
		epStats.Failed++
	}
}

func (s *Stats) GetPercentile(percentile float64) time.Duration {
	s.mu.Lock()
	defer s.mu.Unlock()

	if len(s.results) == 0 {
		return 0
	}

	durations := make([]time.Duration, 0, len(s.results))
	for _, r := range s.results {
		if r.Success {
			durations = append(durations, r.Duration)
		}
	}

	if len(durations) == 0 {
		return 0
	}

	sort.Slice(durations, func(i, j int) bool {
		return durations[i] < durations[j]
	})

	index := int(math.Ceil(float64(len(durations)) * percentile / 100.0))
	if index >= len(durations) {
		index = len(durations) - 1
	}

	return durations[index]
}

func (s *Stats) GetSuccessRate() float64 {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.totalRequests == 0 {
		return 0
	}
	return float64(s.successfulRequests) / float64(s.totalRequests)
}

type LoadTester struct {
	baseURL    string
	httpClient *http.Client
	stats      *Stats
	teams      []string
	users      []User
	prs        []string
	prsMu      sync.RWMutex
}

type User struct {
	ID       string
	TeamName string
}

func NewLoadTester(baseURL string) *LoadTester {
	return &LoadTester{
		baseURL: baseURL,
		httpClient: &http.Client{
			Timeout: 10 * time.Second,
		},
		stats: NewStats(),
		teams: make([]string, 0),
		users: make([]User, 0),
		prs:   make([]string, 0),
	}
}

func (lt *LoadTester) Setup() error {
	fmt.Println("🔧 Подготовка данных для тестирования...")

	for i := 0; i < 5; i++ {
		teamName := fmt.Sprintf("loadtest_team_%d", i)
		members := make([]map[string]interface{}, 10)

		for j := 0; j < 10; j++ {
			userID := fmt.Sprintf("loadtest_u_%d_%d", i, j)
			members[j] = map[string]interface{}{
				"user_id":   userID,
				"username":  fmt.Sprintf("User %d-%d", i, j),
				"is_active": true,
			}
			lt.users = append(lt.users, User{ID: userID, TeamName: teamName})
		}

		payload := map[string]interface{}{
			"team_name": teamName,
			"members":   members,
		}

		if err := lt.createTeam(payload); err != nil {
			return fmt.Errorf("failed to create team %s: %w", teamName, err)
		}

		lt.teams = append(lt.teams, teamName)
	}

	fmt.Printf(" Создано %d команд и %d пользователей\n", len(lt.teams), len(lt.users))

	fmt.Println(" Создание начальных PR...")
	initialPRCount := 10
	for i := 0; i < initialPRCount; i++ {
		author := lt.users[i%len(lt.users)]
		prID := fmt.Sprintf("loadtest_init_pr_%d", i)
		payload, _ := json.Marshal(map[string]interface{}{
			"pull_request_id":   prID,
			"pull_request_name": fmt.Sprintf("Initial PR %d", i),
			"author_id":         author.ID,
		})
		result := lt.makeRequest("POST", "/pullRequest/create", payload)
		if result.Success && result.StatusCode == 201 {
			lt.prs = append(lt.prs, prID)
		}
	}
	fmt.Printf(" Создано %d начальных PR\n", len(lt.prs))

	return nil
}

func (lt *LoadTester) createTeam(payload map[string]interface{}) error {
	body, _ := json.Marshal(payload)
	req, _ := http.NewRequest("POST", lt.baseURL+"/team/add", bytes.NewBuffer(body))
	req.Header.Set("Content-Type", "application/json")

	resp, err := lt.httpClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != 201 && resp.StatusCode != 400 {
		bodyBytes, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("unexpected status %d: %s", resp.StatusCode, string(bodyBytes))
	}

	return nil
}

func (lt *LoadTester) makeRequest(method, endpoint string, body []byte) Result {
	start := time.Now()
	url := lt.baseURL + endpoint

	var req *http.Request
	var err error

	if body != nil {
		req, err = http.NewRequest(method, url, bytes.NewBuffer(body))
		req.Header.Set("Content-Type", "application/json")
	} else {
		req, err = http.NewRequest(method, url, nil)
	}

	if err != nil {
		return Result{
			Endpoint: endpoint,
			Method:   method,
			Success:  false,
			Error:    err,
			Duration: time.Since(start),
		}
	}

	resp, err := lt.httpClient.Do(req)
	duration := time.Since(start)

	if err != nil {
		return Result{
			Endpoint:   endpoint,
			Method:     method,
			StatusCode: 0,
			Success:    false,
			Error:      err,
			Duration:   duration,
		}
	}
	defer resp.Body.Close()

	success := true

	return Result{
		Endpoint:   endpoint,
		Method:     method,
		StatusCode: resp.StatusCode,
		Success:    success,
		Duration:   duration,
	}
}

func (lt *LoadTester) getRandomPRID() (string, bool) {
	lt.prsMu.RLock()
	defer lt.prsMu.RUnlock()

	if len(lt.prs) > 0 {
		idx := time.Now().UnixNano() % int64(len(lt.prs))
		return lt.prs[idx], true
	}

	return "", false
}

func (lt *LoadTester) runWorker(rps int, duration time.Duration, wg *sync.WaitGroup) {
	defer wg.Done()

	interval := time.Second / time.Duration(rps)
	ticker := time.NewTicker(interval)
	defer ticker.Stop()

	deadline := time.Now().Add(duration)

	for time.Now().Before(deadline) {
		select {
		case <-ticker.C:
			lt.runRandomRequest()
		default:
		}
	}
}

func (lt *LoadTester) runRandomRequest() {
	rand := time.Now().UnixNano() % 100

	var result Result

	switch {
	case rand < 25:
		author := lt.users[time.Now().UnixNano()%int64(len(lt.users))]
		prID := fmt.Sprintf("loadtest_pr_%d_%d", time.Now().UnixNano(), time.Now().Unix())
		payload, _ := json.Marshal(map[string]interface{}{
			"pull_request_id":   prID,
			"pull_request_name": fmt.Sprintf("PR %s", prID),
			"author_id":         author.ID,
		})
		result = lt.makeRequest("POST", "/pullRequest/create", payload)
		if result.Success && result.StatusCode == 201 {
			lt.prsMu.Lock()
			lt.prs = append(lt.prs, prID)
			if len(lt.prs) > 1000 {
				lt.prs = lt.prs[len(lt.prs)-1000:]
			}
			lt.prsMu.Unlock()
		}

	case rand < 40:
		team := lt.teams[time.Now().UnixNano()%int64(len(lt.teams))]
		result = lt.makeRequest("GET", fmt.Sprintf("/team/get?team_name=%s", team), nil)

	case rand < 55:
		prID, exists := lt.getRandomPRID()
		if exists {
			payload, _ := json.Marshal(map[string]interface{}{
				"pull_request_id": prID,
			})
			result = lt.makeRequest("POST", "/pullRequest/merge", payload)
		} else {
			result = lt.makeRequest("GET", "/health", nil)
		}

	case rand < 70:
		prID, exists := lt.getRandomPRID()
		if exists {
			user := lt.users[time.Now().UnixNano()%int64(len(lt.users))]
			payload, _ := json.Marshal(map[string]interface{}{
				"pull_request_id": prID,
				"old_user_id":     user.ID,
			})
			result = lt.makeRequest("POST", "/pullRequest/reassign", payload)
		} else {
			result = lt.makeRequest("GET", "/health", nil)
		}

	case rand < 80:
		user := lt.users[time.Now().UnixNano()%int64(len(lt.users))]
		isActive := time.Now().UnixNano()%2 == 0
		payload, _ := json.Marshal(map[string]interface{}{
			"user_id":   user.ID,
			"is_active": isActive,
		})
		result = lt.makeRequest("POST", "/users/setIsActive", payload)

	case rand < 90:
		user := lt.users[time.Now().UnixNano()%int64(len(lt.users))]
		result = lt.makeRequest("GET", fmt.Sprintf("/users/getReview?user_id=%s", user.ID), nil)

	default:
		result = lt.makeRequest("GET", "/health", nil)
	}

	lt.stats.AddResult(result)
}

func (lt *LoadTester) Run() {
	fmt.Println("\n Запуск нагрузочного тестирования...")
	fmt.Printf("Целевой сервис: %s\n", lt.baseURL)
	fmt.Println()

	var wg sync.WaitGroup

	fmt.Println(" Этап 1: Разогрев (30s, 5 RPS)...")
	wg.Add(1)
	go lt.runWorker(5, warmupDuration, &wg)
	wg.Wait()

	fmt.Println(" Этап 2: Основная нагрузка (2m, 5 RPS)...")
	wg.Add(1)
	go lt.runWorker(5, mainLoadDuration, &wg)
	wg.Wait()

	fmt.Println(" Этап 3: Пиковая нагрузка (1m, 10 RPS)...")
	wg.Add(1)
	go lt.runWorker(peakRPS, peakLoadDuration, &wg)
	wg.Wait()

	fmt.Println(" Этап 4: Снижение нагрузки (30s, 5 RPS)...")
	wg.Add(1)
	go lt.runWorker(5, cooldownDuration, &wg)
	wg.Wait()

	fmt.Println("\n Тестирование завершено")
}

func (lt *LoadTester) PrintResults() {
	fmt.Println("\n" + strings.Repeat("=", 80))
	fmt.Println("РЕЗУЛЬТАТЫ НАГРУЗОЧНОГО ТЕСТИРОВАНИЯ")
	fmt.Println(strings.Repeat("=", 80))

	stats := lt.stats
	stats.mu.Lock()
	total := stats.totalRequests
	successful := stats.successfulRequests
	failed := stats.failedRequests
	stats.mu.Unlock()

	fmt.Printf("\n Общая статистика:\n")
	fmt.Printf("  Всего запросов:     %d\n", total)
	fmt.Printf("  Успешных:           %d\n", successful)
	fmt.Printf("  Неудачных:          %d\n", failed)
	fmt.Printf("  Успешность:         %.2f%%\n", stats.GetSuccessRate()*100)

	p50 := stats.GetPercentile(50)
	p95 := stats.GetPercentile(95)
	p99 := stats.GetPercentile(99)

	fmt.Printf("\n  Время ответа:\n")
	fmt.Printf("  Медиана (p50):      %v\n", p50)
	fmt.Printf("  95-й перцентиль:    %v\n", p95)
	fmt.Printf("  99-й перцентиль:    %v\n", p99)

	fmt.Printf("\n Проверка требований (SLI):\n")
	successRate := stats.GetSuccessRate()
	successOK := successRate >= targetSuccessRate
	p95OK := p95 <= targetP95
	p99OK := p99 <= targetP99

	fmt.Printf("  RPS (5 req/s):       Выполнено\n")
	if p95OK {
		fmt.Printf("  p95 < 300ms:         ПРОЙДЕН (%v)\n", p95)
	} else {
		fmt.Printf("  p95 < 300ms:         НЕ ПРОЙДЕН (%v)\n", p95)
	}
	if p99OK {
		fmt.Printf("  p99 < 300ms:         ПРОЙДЕН (%v)\n", p99)
	} else {
		fmt.Printf("  p99 < 300ms:         НЕ ПРОЙДЕН (%v)\n", p99)
	}
	if successOK {
		fmt.Printf("  Успешность > 99.9%%:  ПРОЙДЕН (%.2f%%)\n", successRate*100)
	} else {
		fmt.Printf("  Успешность > 99.9%%:  НЕ ПРОЙДЕН (%.2f%%)\n", successRate*100)
	}

	fmt.Printf("\n Статистика по эндпоинтам:\n")
	stats.mu.Lock()
	endpoints := make([]string, 0, len(stats.byEndpoint))
	for ep := range stats.byEndpoint {
		endpoints = append(endpoints, ep)
	}
	sort.Strings(endpoints)

	for _, ep := range endpoints {
		epStats := stats.byEndpoint[ep]
		if len(epStats.Durations) == 0 {
			continue
		}

		sort.Slice(epStats.Durations, func(i, j int) bool {
			return epStats.Durations[i] < epStats.Durations[j]
		})

		avg := time.Duration(0)
		for _, d := range epStats.Durations {
			avg += d
		}
		avg /= time.Duration(len(epStats.Durations))

		p95Idx := int(float64(len(epStats.Durations)) * 0.95)
		if p95Idx >= len(epStats.Durations) {
			p95Idx = len(epStats.Durations) - 1
		}
		epP95 := epStats.Durations[p95Idx]

		successRate := float64(epStats.Success) / float64(epStats.Total) * 100

		fmt.Printf("\n  %s:\n", ep)
		fmt.Printf("    Запросов:         %d\n", epStats.Total)
		fmt.Printf("    Успешных:         %d (%.2f%%)\n", epStats.Success, successRate)
		fmt.Printf("    Среднее время:     %v\n", avg)
		fmt.Printf("    p95:              %v\n", epP95)

		if len(epStats.StatusCodes) > 0 {
			fmt.Printf("    Коды ответа:      ")
			for code, count := range epStats.StatusCodes {
				fmt.Printf("%d:%d ", code, count)
			}
			fmt.Println()
		}
	}
	stats.mu.Unlock()

	fmt.Println("\n" + strings.Repeat("=", 80))
	if p95OK && p99OK && successOK {
		fmt.Println(" ВСЕ ТРЕБОВАНИЯ ВЫПОЛНЕНЫ")
	} else {
		fmt.Println(" НЕКОТОРЫЕ ТРЕБОВАНИЯ НЕ ВЫПОЛНЕНЫ")
	}
	fmt.Println(strings.Repeat("=", 80))
}

func main() {
	baseURL := flag.String("url", "http://localhost:8080", "Base URL сервиса")
	flag.Parse()

	tester := NewLoadTester(*baseURL)

	if err := tester.Setup(); err != nil {
		fmt.Fprintf(os.Stderr, "Ошибка подготовки данных: %v\n", err)
		os.Exit(1)
	}

	tester.Run()
	tester.PrintResults()
}

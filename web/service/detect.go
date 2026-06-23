package service

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"os/exec"
	"strings"
	"sync"
	"time"
)

const (
	detectUserAgent       = "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/120.0 Safari/537.36"
	detectBodyLimit       = 512 * 1024
	detectIPCacheTTL      = 10 * time.Minute
	detectUnlockCacheTTL  = 5 * time.Minute
	detectRequestTimeout  = 8 * time.Second
	detectRequestParallel = 4
)

type DetectService struct{}

type DetectIPResult struct {
	IP        string `json:"ip"`
	Country   string `json:"country"`
	City      string `json:"city"`
	ASN       string `json:"asn"`
	Org       string `json:"org"`
	IPType    string `json:"ipType"`
	Provider  string `json:"provider"`
	ElapsedMs int64  `json:"elapsedMs"`
	Cached    bool   `json:"cached"`
	CheckedAt int64  `json:"checkedAt"`
	Error     string `json:"error,omitempty"`
}

type DetectUnlockResult struct {
	Platform  string `json:"platform"`
	Name      string `json:"name"`
	Status    string `json:"status"`
	Region    string `json:"region"`
	Hint      string `json:"hint"`
	ElapsedMs int64  `json:"elapsedMs"`
	Cached    bool   `json:"cached"`
	CheckedAt int64  `json:"checkedAt"`
}

type DetectAllResult struct {
	IP     *DetectIPResult      `json:"ip"`
	Unlock []DetectUnlockResult `json:"unlock"`
}

type detectIPProvider struct {
	name  string
	url   string
	parse func([]byte) (*DetectIPResult, error)
}

type detectPlatform struct {
	id    string
	name  string
	check func(context.Context, *http.Client) DetectUnlockResult
}

var (
	detectHTTPClient = &http.Client{Timeout: 3 * time.Second}
	detectIPCacheMu  sync.RWMutex
	detectIPCache    *DetectIPResult

	detectUnlockCacheMu sync.RWMutex
	detectUnlockCache   = map[string]DetectUnlockResult{}
)

func (s *DetectService) DetectIP(force bool) *DetectIPResult {
	if !force {
		if cached := getCachedIPResult(); cached != nil {
			cached.Cached = true
			return cached
		}
	}

	ctx, cancel := context.WithTimeout(context.Background(), detectRequestTimeout)
	defer cancel()

	var lastErr error
	for _, provider := range detectIPProviders() {
		start := time.Now()
		body, err := detectGet(ctx, provider.url)
		if err != nil {
			lastErr = err
			continue
		}
		result, err := provider.parse(body)
		if err != nil {
			lastErr = err
			continue
		}
		result.Provider = provider.name
		result.ElapsedMs = time.Since(start).Milliseconds()
		result.CheckedAt = time.Now().Unix()
		setCachedIPResult(result)
		return result
	}

	message := "检测失败"
	if lastErr != nil {
		message = safeDetectError(lastErr)
	}
	return &DetectIPResult{CheckedAt: time.Now().Unix(), Error: message}
}

func (s *DetectService) DetectUnlock(platforms []string, force bool) []DetectUnlockResult {
	selected := selectDetectPlatforms(platforms)
	results := make([]DetectUnlockResult, len(selected))
	var wg sync.WaitGroup
	sem := make(chan struct{}, detectRequestParallel)
	ctx, cancel := context.WithTimeout(context.Background(), detectRequestTimeout)
	defer cancel()

	for i, platform := range selected {
		if !force {
			if cached, ok := getCachedUnlockResult(platform.id); ok {
				cached.Cached = true
				results[i] = cached
				continue
			}
		}

		wg.Add(1)
		go func(index int, platform detectPlatform) {
			defer wg.Done()
			sem <- struct{}{}
			defer func() { <-sem }()

			start := time.Now()
			result := platform.check(ctx, detectHTTPClient)
			result.Platform = platform.id
			result.Name = platform.name
			result.ElapsedMs = time.Since(start).Milliseconds()
			result.CheckedAt = time.Now().Unix()
			setCachedUnlockResult(platform.id, result)
			results[index] = result
		}(i, platform)
	}

	wg.Wait()
	return results
}

func (s *DetectService) DetectAll(platforms []string, force bool) *DetectAllResult {
	return &DetectAllResult{
		IP:     s.DetectIP(force),
		Unlock: s.DetectUnlock(platforms, force),
	}
}

func detectIPProviders() []detectIPProvider {
	return []detectIPProvider{
		{name: "ipapi.is", url: "https://api.ipapi.is", parse: parseIPAPIIs},
		{name: "ip-api.com", url: "http://ip-api.com/json/?fields=status,message,countryCode,city,isp,as,mobile,proxy,hosting,query", parse: parseIPAPICom},
		{name: "ippure.com", url: "https://my.ippure.com/v1/info", parse: parseIPPure},
	}
}

func detectPlatforms() []detectPlatform {
	return []detectPlatform{
		{id: "apple", name: "Apple", check: checkDetectApple},
		{id: "bing", name: "Bing", check: checkDetectBing},
		{id: "google_play", name: "Google Play", check: checkDetectGooglePlay},
		{id: "openai", name: "OpenAI", check: checkDetectOpenAI},
		{id: "youtube", name: "YouTube", check: checkDetectYouTube},
		{id: "claude", name: "Claude", check: checkDetectClaude},
		{id: "gemini", name: "Gemini", check: checkDetectGemini},
		{id: "netflix", name: "Netflix", check: checkDetectNetflix},
	}
}

func selectDetectPlatforms(ids []string) []detectPlatform {
	available := detectPlatforms()
	if len(ids) == 0 {
		return available
	}
	byID := make(map[string]detectPlatform, len(available))
	for _, platform := range available {
		byID[platform.id] = platform
	}
	selected := make([]detectPlatform, 0, len(ids))
	seen := map[string]bool{}
	for _, id := range ids {
		id = strings.TrimSpace(strings.ToLower(id))
		if seen[id] {
			continue
		}
		if platform, ok := byID[id]; ok {
			selected = append(selected, platform)
			seen[id] = true
		}
	}
	if len(selected) == 0 {
		return available
	}
	return selected
}

// detectHostAllowlist 是检测页允许请求的外部域名白名单。
// 仅这些 host 才允许被 detectGet/detectClientGet 访问，防止 SSRF。
var detectHostAllowlist = map[string]bool{
	"api.ipapi.is":           true,
	"ip-api.com":             true,
	"my.ippure.com":          true,
	"gspe1-ssl.ls.apple.com": true,
	"www.bing.com":           true,
	"play.google.com":        true,
	"chat.openai.com":        true,
	"www.youtube.com":        true,
	"claude.ai":              true,
	"gemini.google.com":      true,
	"netflix.com":            true,
}

func assertDetectHostAllowed(rawURL string) error {
	parsed, err := url.Parse(rawURL)
	if err != nil {
		return fmt.Errorf("invalid url: %w", err)
	}
	host := strings.ToLower(parsed.Hostname())
	if host == "" || !detectHostAllowlist[host] {
		return fmt.Errorf("host not allowed: %s", host)
	}
	return nil
}

func detectGet(ctx context.Context, rawURL string) ([]byte, error) {
	if err := assertDetectHostAllowed(rawURL); err != nil {
		return nil, err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, rawURL, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("User-Agent", detectUserAgent)
	req.Header.Set("Accept-Language", "en-US,en;q=0.9")
	resp, err := detectHTTPClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 400 {
		return nil, fmt.Errorf("status %d", resp.StatusCode)
	}
	return io.ReadAll(io.LimitReader(resp.Body, detectBodyLimit))
}

func parseIPAPIIs(body []byte) (*DetectIPResult, error) {
	var payload struct {
		IP  string `json:"ip"`
		ASN struct {
			ASN     int    `json:"asn"`
			Descr   string `json:"descr"`
			Org     string `json:"org"`
			Type    string `json:"type"`
			Country string `json:"country"`
		} `json:"asn"`
		Company struct {
			Name string `json:"name"`
			Type string `json:"type"`
		} `json:"company"`
		Location struct {
			CountryCode string `json:"country_code"`
			City        string `json:"city"`
		} `json:"location"`
		IsMobile bool `json:"is_mobile"`
	}
	if err := json.Unmarshal(body, &payload); err != nil {
		return nil, err
	}
	if payload.IP == "" {
		return nil, errors.New("missing ip")
	}
	ipType := firstNonEmpty(payload.Company.Type, payload.ASN.Type)
	if payload.IsMobile {
		ipType = "mobile"
	}
	asn := payload.ASN.Descr
	if asn == "" && payload.ASN.ASN != 0 {
		asn = fmt.Sprintf("AS%d", payload.ASN.ASN)
	}
	return &DetectIPResult{
		IP:      payload.IP,
		Country: strings.ToUpper(firstNonEmpty(payload.Location.CountryCode, payload.ASN.Country)),
		City:    payload.Location.City,
		ASN:     asn,
		Org:     firstNonEmpty(payload.ASN.Org, payload.Company.Name),
		IPType:  ipType,
	}, nil
}

func parseIPAPICom(body []byte) (*DetectIPResult, error) {
	var payload struct {
		Status      string `json:"status"`
		Message     string `json:"message"`
		CountryCode string `json:"countryCode"`
		City        string `json:"city"`
		ISP         string `json:"isp"`
		AS          string `json:"as"`
		Mobile      bool   `json:"mobile"`
		Proxy       bool   `json:"proxy"`
		Hosting     bool   `json:"hosting"`
		Query       string `json:"query"`
	}
	if err := json.Unmarshal(body, &payload); err != nil {
		return nil, err
	}
	if payload.Status != "success" || payload.Query == "" {
		return nil, errors.New(firstNonEmpty(payload.Message, "query failed"))
	}
	ipType := "isp"
	if payload.Hosting {
		ipType = "hosting"
	} else if payload.Mobile {
		ipType = "mobile"
	} else if payload.Proxy {
		ipType = "proxy"
	}
	return &DetectIPResult{
		IP:      payload.Query,
		Country: strings.ToUpper(payload.CountryCode),
		City:    payload.City,
		ASN:     payload.AS,
		Org:     payload.ISP,
		IPType:  ipType,
	}, nil
}

func parseIPPure(body []byte) (*DetectIPResult, error) {
	var payload struct {
		IP             string `json:"ip"`
		ASN            int    `json:"asn"`
		AsOrganization string `json:"asOrganization"`
		CountryCode    string `json:"countryCode"`
		City           string `json:"city"`
		IsResidential  bool   `json:"isResidential"`
	}
	if err := json.Unmarshal(body, &payload); err != nil {
		return nil, err
	}
	if payload.IP == "" {
		return nil, errors.New("missing ip")
	}
	ipType := "hosting"
	if payload.IsResidential {
		ipType = "isp"
	}
	asn := ""
	if payload.ASN != 0 {
		asn = fmt.Sprintf("AS%d", payload.ASN)
	}
	return &DetectIPResult{
		IP:      payload.IP,
		Country: strings.ToUpper(payload.CountryCode),
		City:    payload.City,
		ASN:     asn,
		Org:     payload.AsOrganization,
		IPType:  ipType,
	}, nil
}

func checkDetectApple(ctx context.Context, client *http.Client) DetectUnlockResult {
	body, err := detectClientGet(ctx, client, "https://gspe1-ssl.ls.apple.com/pep/gcc")
	if err != nil {
		return detectUnknown("检测失败")
	}
	region := strings.ToUpper(strings.TrimSpace(string(body)))
	if region == "" {
		return detectUnknown("未返回地区")
	}
	return DetectUnlockResult{Status: "unlocked", Region: region, Hint: "可获取 Apple 地区"}
}

func checkDetectBing(ctx context.Context, client *http.Client) DetectUnlockResult {
	body, err := detectClientGet(ctx, client, "https://www.bing.com/search?q=x-ui")
	if err != nil {
		return detectUnknown("检测失败")
	}
	content := string(body)
	region := strings.ToUpper(extractBetween(content, `Region:"`, `"`))
	if strings.Contains(content, "cn.bing.com") {
		return DetectUnlockResult{Status: "blocked", Region: firstNonEmpty(region, "CN"), Hint: "Bing 可能被重定向到中国区"}
	}
	return DetectUnlockResult{Status: "unlocked", Region: region, Hint: "Bing 可访问"}
}

func checkDetectGooglePlay(ctx context.Context, client *http.Client) DetectUnlockResult {
	body, err := detectClientGet(ctx, client, "https://play.google.com/")
	if err != nil {
		return detectUnknown("检测失败")
	}
	content := string(body)
	region := extractBetween(content, `<div class="yVZQTb">`, `<`)
	if idx := strings.Index(region, " ("); idx >= 0 {
		region = region[:idx]
	}
	if region == "" {
		return DetectUnlockResult{Status: "unknown", Hint: "Google Play 可访问，但未识别地区"}
	}
	return DetectUnlockResult{Status: "unlocked", Region: region, Hint: "Google Play 可访问"}
}

func checkDetectOpenAI(ctx context.Context, client *http.Client) DetectUnlockResult {
	body, err := detectClientGet(ctx, client, "https://chat.openai.com/cdn-cgi/trace")
	if err != nil {
		return detectUnknown("检测失败")
	}
	content := string(body)
	region := strings.ToUpper(extractLineValue(content, "loc="))
	if strings.Contains(strings.ToLower(content), "warp=on") {
		return DetectUnlockResult{Status: "unlocked", Region: region, Hint: "OpenAI trace 可访问"}
	}
	return DetectUnlockResult{Status: "unlocked", Region: region, Hint: "OpenAI trace 可访问"}
}

func checkDetectYouTube(ctx context.Context, client *http.Client) DetectUnlockResult {
	body, err := detectClientGet(ctx, client, "https://www.youtube.com/premium")
	if err != nil {
		return detectUnknown("检测失败")
	}
	content := string(body)
	region := strings.ToUpper(extractBetween(content, `"countryCode":"`, `"`))
	if strings.Contains(strings.ToLower(content), "premium") {
		return DetectUnlockResult{Status: "unlocked", Region: region, Hint: "YouTube 可访问"}
	}
	return DetectUnlockResult{Status: "unknown", Region: region, Hint: "YouTube 可访问，但未识别 Premium 状态"}
}

func checkDetectClaude(ctx context.Context, client *http.Client) DetectUnlockResult {
	resp, err := detectClientHead(ctx, client, "https://claude.ai/")
	if err != nil {
		return detectUnknown("检测失败")
	}
	if resp >= 400 && resp != 403 {
		return DetectUnlockResult{Status: "blocked", Hint: fmt.Sprintf("Claude 返回 %d", resp)}
	}
	return DetectUnlockResult{Status: "unlocked", Hint: "Claude 可访问"}
}

func checkDetectGemini(ctx context.Context, client *http.Client) DetectUnlockResult {
	resp, err := detectClientHead(ctx, client, "https://gemini.google.com/")
	if err != nil {
		return detectUnknown("检测失败")
	}
	if resp >= 400 && resp != 403 {
		return DetectUnlockResult{Status: "blocked", Hint: fmt.Sprintf("Gemini 返回 %d", resp)}
	}
	return DetectUnlockResult{Status: "unlocked", Hint: "Gemini 可访问"}
}

func checkDetectNetflix(ctx context.Context, client *http.Client) DetectUnlockResult {
	body, err := detectClientGet(ctx, client, "https://netflix.com/")
	if err != nil {
		return detectUnknown("检测失败")
	}
	content := strings.ToLower(string(body))
	if strings.Contains(content, "page-404") || strings.Contains(content, "not available") {
		return DetectUnlockResult{Status: "blocked", Hint: "Netflix 不可用"}
	}
	region := strings.ToUpper(extractBetween(string(body), `"countryCode":"`, `"`))
	return DetectUnlockResult{Status: "unlocked", Region: region, Hint: "Netflix 可访问"}
}

func detectClientHead(ctx context.Context, client *http.Client, rawURL string) (int, error) {
	if err := assertDetectHostAllowed(rawURL); err != nil {
		return 0, err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodHead, rawURL, nil)
	if err != nil {
		return 0, err
	}
	req.Header.Set("User-Agent", detectUserAgent)
	resp, err := client.Do(req)
	if err != nil {
		if ctx.Err() != nil {
			return 0, ctx.Err()
		}
		return 0, err
	}
	defer resp.Body.Close()
	return resp.StatusCode, nil
}

func detectClientGet(ctx context.Context, client *http.Client, rawURL string) ([]byte, error) {
	if err := assertDetectHostAllowed(rawURL); err != nil {
		return nil, err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, rawURL, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("User-Agent", detectUserAgent)
	req.Header.Set("Accept-Language", "en-US,en;q=0.9")
	resp, err := client.Do(req)
	if err != nil {
		if ctx.Err() != nil {
			return nil, ctx.Err()
		}
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode >= 400 {
		return nil, fmt.Errorf("status %d", resp.StatusCode)
	}
	return io.ReadAll(io.LimitReader(resp.Body, detectBodyLimit))
}

func getCachedIPResult() *DetectIPResult {
	detectIPCacheMu.RLock()
	defer detectIPCacheMu.RUnlock()
	if detectIPCache == nil || time.Since(time.Unix(detectIPCache.CheckedAt, 0)) > detectIPCacheTTL {
		return nil
	}
	copy := *detectIPCache
	return &copy
}

func setCachedIPResult(result *DetectIPResult) {
	detectIPCacheMu.Lock()
	defer detectIPCacheMu.Unlock()
	copy := *result
	copy.Cached = false
	detectIPCache = &copy
}

func getCachedUnlockResult(platform string) (DetectUnlockResult, bool) {
	detectUnlockCacheMu.RLock()
	defer detectUnlockCacheMu.RUnlock()
	result, ok := detectUnlockCache[platform]
	if !ok || time.Since(time.Unix(result.CheckedAt, 0)) > detectUnlockCacheTTL {
		return DetectUnlockResult{}, false
	}
	return result, true
}

func setCachedUnlockResult(platform string, result DetectUnlockResult) {
	detectUnlockCacheMu.Lock()
	defer detectUnlockCacheMu.Unlock()
	result.Cached = false
	detectUnlockCache[platform] = result
}

func detectUnknown(hint string) DetectUnlockResult {
	return DetectUnlockResult{Status: "unknown", Hint: hint}
}

func extractBetween(content, start, end string) string {
	index := strings.Index(content, start)
	if index < 0 {
		return ""
	}
	index += len(start)
	endIndex := strings.Index(content[index:], end)
	if endIndex < 0 {
		return ""
	}
	return content[index : index+endIndex]
}

func extractLineValue(content, prefix string) string {
	for _, line := range strings.Split(content, "\n") {
		if strings.HasPrefix(line, prefix) {
			return strings.TrimSpace(strings.TrimPrefix(line, prefix))
		}
	}
	return ""
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if strings.TrimSpace(value) != "" {
			return strings.TrimSpace(value)
		}
	}
	return ""
}

func safeDetectError(err error) string {
	if err == nil {
		return ""
	}
	if errors.Is(err, context.DeadlineExceeded) || errors.Is(err, context.Canceled) {
		return "检测超时"
	}
	return "外部检测服务不可用"
}

// ---- 三网回程路由检测 ----

type BackRouteResult struct {
	City      string `json:"city"`
	Telecom   string `json:"telecom"`
	Unicom    string `json:"unicom"`
	Mobile    string `json:"mobile"`
	UpdatedAt int64  `json:"updatedAt"`
	Error     string `json:"error,omitempty"`
}

type routeTarget struct {
	City string
	ISP  string
	IPs  []string
}

var routeTargets = []routeTarget{
	{City: "shanghai", ISP: "telecom", IPs: []string{"202.96.209.5", "61.129.7.1", "180.169.255.1"}},
	{City: "shanghai", ISP: "unicom", IPs: []string{"210.22.97.1", "58.247.0.1", "116.228.111.1"}},
	{City: "shanghai", ISP: "mobile", IPs: []string{"211.136.112.50", "117.131.0.1", "120.196.0.1"}},
	{City: "beijing", ISP: "telecom", IPs: []string{"219.141.140.10", "202.106.0.20", "61.51.0.1"}},
	{City: "beijing", ISP: "unicom", IPs: []string{"123.123.123.123", "202.106.50.1", "60.247.0.1"}},
	{City: "beijing", ISP: "mobile", IPs: []string{"221.130.33.52", "211.137.130.1", "120.197.0.1"}},
	{City: "guangzhou", ISP: "telecom", IPs: []string{"119.145.0.1", "113.108.0.1", "58.60.0.1"}},
	{City: "guangzhou", ISP: "unicom", IPs: []string{"210.21.4.1", "120.80.0.1", "183.56.0.1"}},
	{City: "guangzhou", ISP: "mobile", IPs: []string{"120.196.165.24", "211.139.0.1", "117.135.0.1"}},
}

type mtrReport struct {
	Report struct {
		Hubs []struct {
			Host string  `json:"host"`
			Loss float64 `json:"Loss%"`
		} `json:"hubs"`
	} `json:"report"`
}

func (s *DetectService) DetectBackRoute() []BackRouteResult {
	results := make([]routeResult, len(routeTargets))
	var wg sync.WaitGroup
	for i, target := range routeTargets {
		wg.Add(1)
		go func(idx int, t routeTarget) {
			defer wg.Done()
			hosts, err := runMTR(t.IPs)
			line := "检测失败"
			if err == nil {
				line = detectLine(t.ISP, hosts)
			}
			results[idx] = routeResult{City: t.City, ISP: t.ISP, Line: line}
		}(i, target)
	}
	wg.Wait()

	cityMap := map[string]*BackRouteResult{}
	order := []string{}
	for _, r := range results {
		if _, ok := cityMap[r.City]; !ok {
			cityMap[r.City] = &BackRouteResult{City: r.City, UpdatedAt: time.Now().Unix()}
			order = append(order, r.City)
		}
		switch r.ISP {
		case "telecom":
			cityMap[r.City].Telecom = r.Line
		case "unicom":
			cityMap[r.City].Unicom = r.Line
		case "mobile":
			cityMap[r.City].Mobile = r.Line
		}
	}
	out := make([]BackRouteResult, 0, len(order))
	for _, city := range order {
		out = append(out, *cityMap[city])
	}
	return out
}

type routeResult struct {
	City string
	ISP  string
	Line string
}

func runMTR(ips []string) ([]string, error) {
	for _, ip := range ips {
		ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
		out, err := exec.CommandContext(ctx, "mtr", "--tcp", "--port", "80",
			"--report", "--report-cycles", "5", "--json", ip).Output()
		cancel()
		if err != nil {
			continue
		}
		var report mtrReport
		if err := json.Unmarshal(out, &report); err != nil {
			continue
		}
		var hosts []string
		for _, hub := range report.Report.Hubs {
			if hub.Host != "???" && hub.Loss < 100 {
				hosts = append(hosts, hub.Host)
			}
		}
		if len(hosts) > 0 {
			return hosts, nil
		}
	}
	return nil, errors.New("all IPs failed")
}

func ipHasPrefix(ip, prefix string) bool {
	parts := strings.Split(prefix, ".")
	ipParts := strings.Split(ip, ".")
	for i, p := range parts {
		if p == "*" {
			continue
		}
		if i >= len(ipParts) || ipParts[i] != p {
			return false
		}
	}
	return true
}

func isPublicIP(host string) bool {
	ip := net.ParseIP(host)
	return ip != nil && !ip.IsPrivate() && !ip.IsLoopback()
}

func detectTelecom(hosts []string) string {
	cn2Count := 0
	for _, h := range hosts {
		if !isPublicIP(h) {
			continue
		}
		if ipHasPrefix(h, "59.43.*.*") {
			cn2Count++
		}
	}
	if cn2Count >= 2 {
		return "CN2 GIA"
	}
	if cn2Count == 1 {
		return "CN2 GT"
	}
	return "163"
}

func detectUnicom(hosts []string) string {
	for _, h := range hosts {
		if !isPublicIP(h) {
			continue
		}
		if ipHasPrefix(h, "210.51.*.*") || ipHasPrefix(h, "218.105.*.*") {
			return "9929"
		}
	}
	for _, h := range hosts {
		if !isPublicIP(h) {
			continue
		}
		if ipHasPrefix(h, "219.158.*.*") {
			return "4837"
		}
	}
	return "169"
}

func detectMobile(hosts []string) string {
	for _, h := range hosts {
		if !isPublicIP(h) {
			continue
		}
		if ipHasPrefix(h, "223.120.*.*") || ipHasPrefix(h, "223.119.*.*") || ipHasPrefix(h, "223.118.*.*") {
			return "CMIN2"
		}
	}
	for _, h := range hosts {
		if !isPublicIP(h) {
			continue
		}
		if ipHasPrefix(h, "221.183.*.*") || ipHasPrefix(h, "211.136.*.*") || ipHasPrefix(h, "211.137.*.*") {
			return "CMI Basic"
		}
	}
	return "CMI"
}

func detectLine(isp string, hosts []string) string {
	switch isp {
	case "telecom":
		return detectTelecom(hosts)
	case "unicom":
		return detectUnicom(hosts)
	case "mobile":
		return detectMobile(hosts)
	}
	return "未知"
}

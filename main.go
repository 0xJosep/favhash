package main

import (
	"encoding/base64"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"math/rand"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/PuerkitoBio/goquery"
	"github.com/fatih/color"
	"github.com/spaolacci/murmur3"
	"gopkg.in/yaml.v3"
)

const (
	version = "1.1"
	banner  = `
███████╗ █████╗ ██╗   ██╗██╗  ██╗ █████╗ ███████╗██╗  ██╗
██╔════╝██╔══██╗██║   ██║██║  ██║██╔══██╗██╔════╝██║  ██║
█████╗  ███████║██║   ██║███████║███████║███████╗███████║
██╔══╝  ██╔══██║╚██╗ ██╔╝██╔══██║██╔══██║╚════██║██╔══██║
██║     ██║  ██║ ╚████╔╝ ██║  ██║██║  ██║███████║██║  ██║
╚═╝     ╚═╝  ╚═╝  ╚═══╝  ╚═╝  ╚═╝╚═╝  ╚═╝╚══════╝╚═╝  ╚═╝
                                           v%s by @0xJosep
`
	historyFile     = ".favhash_history.json"
	apiStatusFile   = ".favhash_api_status.json"
	resultsDir      = "results"
	maxRetries      = 5
	rateLimitWait   = 5 * time.Second
	creditWarnLevel = 10
)

var (
	infoColor    = color.New(color.FgCyan)
	successColor = color.New(color.FgGreen)
	errorColor   = color.New(color.FgRed)
	warnColor    = color.New(color.FgYellow)
	resultColor  = color.New(color.FgHiMagenta)
	debugColor   = color.New(color.FgHiCyan)
)

var userAgents = []string{
	"Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/91.0.4472.124 Safari/537.36",
	"Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/91.0.4472.124 Safari/537.36",
	"Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Edge/91.0.864.59",
	"Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7) AppleWebKit/605.1.15 (KHTML, like Gecko) Version/14.1.1 Safari/605.1.15",
	"Favhash/1.1 (+https://github.com/0xJosep/favhash)",
}

var commonFaviconPaths = []string{
	"/favicon.ico",
	"/favicon.png",
	"/favicon.gif",
	"/favicon.svg",
	"/assets/favicon.ico",
	"/assets/images/favicon.ico",
	"/assets/img/favicon.ico",
	"/static/favicon.ico",
	"/static/images/favicon.ico",
	"/static/img/favicon.ico",
	"/images/favicon.ico",
	"/img/favicon.ico",
	"/public/favicon.ico",
	"/resources/favicon.ico",
	"/wp-content/themes/favicon.ico",
	"/wp-content/uploads/favicon.ico",
}

var validFaviconTypes = []string{
	"image/x-icon",
	"image/vnd.microsoft.icon",
	"image/ico",
	"image/icon",
	"image/png",
	"image/gif",
	"image/svg+xml",
	"application/octet-stream",
}

// Structures
type Config struct {
	UserAgent      string
	Timeout        time.Duration
	RetryCount     int
	RetryDelay     time.Duration
	ProxyURL       string
	OutputFormat   string
	FollowRedirect bool
	Debug          bool
	SaveResults    bool
	BatchMode      bool
	NoHistory      bool
}

type ShodanPlanDetails struct {
	Name          string         `json:"name"`
	ValidUntil    time.Time      `json:"valid_until"`
	Capabilities  []string       `json:"capabilities"`
	MaxCredits    int            `json:"max_credits"`
	RenewalDate   time.Time      `json:"renewal_date"`
	RenewalAmount int            `json:"renewal_amount"`
	RateLimits    map[string]int `json:"rate_limits"`
}

type ShodanAPIInfo struct {
	QueryCredits int               `json:"query_credits"`
	ScanCredits  int               `json:"scan_credits"`
	Plan         string            `json:"plan"`
	HTTPS        bool              `json:"https"`
	Unlocked     bool              `json:"unlocked"`
	PlanDetails  ShodanPlanDetails `json:"plan_details"`
}

type HashResult struct {
	URL          string    `json:"url"`
	Hash         int32     `json:"hash"`
	DateTime     time.Time `json:"datetime"`
	Success      bool      `json:"success"`
	ErrorMessage string    `json:"error_message,omitempty"`
	ResponseTime float64   `json:"response_time"`
}

type HashHistory struct {
	Hashes []HashResult `json:"hashes"`
}

type APIStatus struct {
	LastCheck    time.Time `json:"last_check"`
	IsValid      bool      `json:"is_valid"`
	ErrorCount   int       `json:"error_count"`
	LastError    string    `json:"last_error"`
	RateLimitHit bool      `json:"rate_limit_hit"`
}

type ShodanResponse struct {
	Matches []struct {
		IP        string   `json:"ip_str"`
		Hostnames []string `json:"hostnames"`
		Domains   []string `json:"domains"`
		Port      int      `json:"port"`
		Location  struct {
			Country string `json:"country_name"`
			City    string `json:"city"`
		} `json:"location"`
		LastUpdate string   `json:"last_update"`
		Tags       []string `json:"tags"`
	} `json:"matches"`
	Total int `json:"total"`
}

type FaviconFinder struct {
	client      *http.Client
	config      *Config
	shodanKey   string
	history     *HashHistory
	apiStatus   *APIStatus
	currentHash int32
	startTime   time.Time
}

// Initialize directories and files
func init() {
	os.MkdirAll(resultsDir, 0755)
}

// Create new FaviconFinder instance
func NewFaviconFinder(shodanKey string, config *Config) *FaviconFinder {
	transport := &http.Transport{
		MaxIdleConns:        10,
		IdleConnTimeout:     30 * time.Second,
		DisableCompression:  false,
		TLSHandshakeTimeout: 10 * time.Second,
	}

	if config.ProxyURL != "" {
		if proxyURL, err := url.Parse(config.ProxyURL); err == nil {
			transport.Proxy = http.ProxyURL(proxyURL)
		}
	}

	client := &http.Client{
		Transport: transport,
		Timeout:   config.Timeout,
	}

	if !config.FollowRedirect {
		client.CheckRedirect = func(req *http.Request, via []*http.Request) error {
			return http.ErrUseLastResponse
		}
	}

	ff := &FaviconFinder{
		client:    client,
		config:    config,
		shodanKey: shodanKey,
		startTime: time.Now(),
	}

	if !config.NoHistory {
		ff.loadHistory()
		ff.loadAPIStatus()
	}

	return ff
}

func (f *FaviconFinder) loadHistory() {
	f.history = &HashHistory{}
	data, err := os.ReadFile(historyFile)
	if err == nil {
		json.Unmarshal(data, f.history)
	}
}

// Save search history
func (f *FaviconFinder) saveHistory() {
	if f.history != nil {
		data, err := json.MarshalIndent(f.history, "", "  ")
		if err == nil {
			os.WriteFile(historyFile, data, 0644)
		}
	}
}

// Load API status
func (f *FaviconFinder) loadAPIStatus() {
	f.apiStatus = &APIStatus{}
	data, err := os.ReadFile(apiStatusFile)
	if err == nil {
		json.Unmarshal(data, f.apiStatus)
	}
}

// Save API status
func (f *FaviconFinder) saveAPIStatus() {
	if f.apiStatus != nil {
		data, err := json.MarshalIndent(f.apiStatus, "", "  ")
		if err == nil {
			os.WriteFile(apiStatusFile, data, 0644)
		}
	}
}

func (f *FaviconFinder) debug(format string, args ...interface{}) {
	if f.config.Debug {
		debugColor.Printf("[DEBUG] "+format+"\n", args...)
	}
}

// Make HTTP request with retries and improved error handling
func (f *FaviconFinder) makeRequest(reqURL string) (*http.Response, error) {
	var lastErr error
	for attempt := 0; attempt <= f.config.RetryCount; attempt++ {
		if attempt > 0 {
			f.debug("Retry attempt %d for URL: %s", attempt, reqURL)
			time.Sleep(f.config.RetryDelay)
		}

		req, err := http.NewRequest("GET", reqURL, nil)
		if err != nil {
			lastErr = err
			continue
		}

		// Add headers
		userAgent := f.config.UserAgent
		if userAgent == "" {
			userAgent = userAgents[rand.Intn(len(userAgents))]
		}
		req.Header.Set("User-Agent", userAgent)
		req.Header.Set("Accept", "*/*")
		req.Header.Set("Accept-Language", "en-US,en;q=0.9")
		req.Header.Set("Connection", "keep-alive")

		start := time.Now()
		resp, err := f.client.Do(req)
		duration := time.Since(start)

		if err != nil {
			f.debug("Request failed: %v (took %v)", err, duration)
			lastErr = err
			continue
		}

		// Handle rate limiting
		if resp.StatusCode == 429 {
			if resp.Body != nil {
				resp.Body.Close()
			}
			f.debug("Rate limit hit, waiting %v", rateLimitWait)
			if f.apiStatus != nil {
				f.apiStatus.RateLimitHit = true
				f.saveAPIStatus()
			}
			time.Sleep(rateLimitWait)
			continue
		}

		f.debug("Request completed: status=%d, took=%v", resp.StatusCode, duration)
		return resp, nil
	}

	return nil, fmt.Errorf("failed after %d attempts, last error: %v", f.config.RetryCount, lastErr)
}

// Check Shodan API key and get plan information
func (f *FaviconFinder) checkAPIKey() (*ShodanAPIInfo, error) {
	f.debug("Checking Shodan API key status")
	resp, err := f.makeRequest(fmt.Sprintf("https://api.shodan.io/api-info?key=%s", f.shodanKey))
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("invalid API key (status %d): %s", resp.StatusCode, string(body))
	}

	var info ShodanAPIInfo
	if err := json.NewDecoder(resp.Body).Decode(&info); err != nil {
		return nil, fmt.Errorf("failed to decode API info: %v", err)
	}

	// Update API status
	if f.apiStatus != nil {
		f.apiStatus.LastCheck = time.Now()
		f.apiStatus.IsValid = true
		f.apiStatus.ErrorCount = 0
		f.saveAPIStatus()
	}

	return &info, nil
}

// Generate Shodan search URL for manual search
func (f *FaviconFinder) generateShodanSearchURL(hash int32) string {
	return fmt.Sprintf("https://www.shodan.io/search?query=http.favicon.hash:%d", hash)
}

// Save results to file
func (f *FaviconFinder) saveResults(results *ShodanResponse, hash int32) error {
	if !f.config.SaveResults {
		return nil
	}

	filename := filepath.Join(resultsDir, fmt.Sprintf("favhash_%d_%s.json",
		hash, time.Now().Format("20060102_150405")))

	data, err := json.MarshalIndent(results, "", "  ")
	if err != nil {
		return err
	}

	return os.WriteFile(filename, data, 0644)
}

func (f *FaviconFinder) findFaviconInHTML(targetURL string) (string, error) {
	f.debug("Checking HTML for favicon links")

	resp, err := f.makeRequest(targetURL)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		return "", fmt.Errorf("got status code %d", resp.StatusCode)
	}

	doc, err := goquery.NewDocumentFromReader(resp.Body)
	if err != nil {
		return "", err
	}

	// Check various link tags for favicon
	selectors := []struct {
		rel  string
		attr string
	}{
		{"shortcut icon", "href"},
		{"icon", "href"},
		{"apple-touch-icon", "href"},
		{"apple-touch-icon-precomposed", "href"},
		{"mask-icon", "href"},
		{"fluid-icon", "href"},
	}

	for _, selector := range selectors {
		selection := doc.Find(fmt.Sprintf("link[rel='%s']", selector.rel)).First()
		if href, exists := selection.Attr(selector.attr); exists {
			f.debug("Found %s: %s", selector.rel, href)
			resolvedURL, err := f.resolveURL(targetURL, href)
			if err == nil {
				// Validate favicon
				if f.validateFavicon(resolvedURL) {
					return resolvedURL, nil
				}
			}
		}
	}

	// Check meta tags
	metaIcon := doc.Find("meta[property='og:image']").First()
	if content, exists := metaIcon.Attr("content"); exists {
		f.debug("Found og:image: %s", content)
		resolvedURL, err := f.resolveURL(targetURL, content)
		if err == nil && f.validateFavicon(resolvedURL) {
			return resolvedURL, nil
		}
	}

	return "", fmt.Errorf("no valid favicon found in HTML")
}

// Validate favicon URL
func (f *FaviconFinder) validateFavicon(faviconURL string) bool {
	resp, err := f.makeRequest(faviconURL)
	if err != nil {
		f.debug("Favicon validation failed: %v", err)
		return false
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		f.debug("Favicon returned status code: %d", resp.StatusCode)
		return false
	}

	contentType := resp.Header.Get("Content-Type")
	for _, validType := range validFaviconTypes {
		if strings.HasPrefix(contentType, validType) {
			contentLength := resp.ContentLength
			if contentLength > 0 {
				f.debug("Valid favicon found: type=%s, size=%d", contentType, contentLength)
				return true
			}
		}
	}

	return false
}

// Check common paths for favicon
func (f *FaviconFinder) checkCommonPaths(targetURL string) (string, error) {
	f.debug("Checking common favicon paths")

	parsedURL, err := url.Parse(targetURL)
	if err != nil {
		return "", err
	}

	for _, path := range commonFaviconPaths {
		faviconURL := fmt.Sprintf("%s://%s%s", parsedURL.Scheme, parsedURL.Host, path)
		f.debug("Trying: %s", faviconURL)

		if f.validateFavicon(faviconURL) {
			return faviconURL, nil
		}
	}

	return "", fmt.Errorf("no valid favicon found at common paths")
}

// Resolve relative URLs
func (f *FaviconFinder) resolveURL(baseURL, path string) (string, error) {
	base, err := url.Parse(baseURL)
	if err != nil {
		return "", err
	}

	ref, err := url.Parse(path)
	if err != nil {
		return "", err
	}

	return base.ResolveReference(ref).String(), nil
}

// Calculate MMH3 hash
func calculateMMH3(data []byte) (int32, error) {
	if len(data) == 0 {
		return 0, fmt.Errorf("empty favicon data")
	}

	b64 := base64.StdEncoding.EncodeToString(data)
	hash := murmur3.Sum32([]byte(b64))
	return int32(hash), nil
}

// Search Shodan
func (f *FaviconFinder) searchShodan(hash int32) (*ShodanResponse, error) {
	searchURL := fmt.Sprintf("https://api.shodan.io/shodan/host/search?key=%s&query=http.favicon.hash:%d",
		f.shodanKey, hash)

	resp, err := f.makeRequest(searchURL)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("Shodan API error (status %d): %s", resp.StatusCode, string(body))
	}

	var result ShodanResponse
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		// Check if the error is due to invalid JSON
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("failed to parse Shodan response: %v (body: %s)", err, string(body))
	}

	return &result, nil
}

// Format and output results
func (f *FaviconFinder) outputResults(results *ShodanResponse, format string) error {
	switch format {
	case "json":
		encoder := json.NewEncoder(os.Stdout)
		encoder.SetIndent("", "  ")
		return encoder.Encode(results)
	case "yaml":
		encoder := yaml.NewEncoder(os.Stdout)
		return encoder.Encode(results)
	default:
		if results.Total > 0 {
			resultColor.Println("\n[+] Results:")
			for _, match := range results.Matches {
				resultColor.Printf("\n    IP: %s\n", match.IP)
				resultColor.Printf("    Port: %d\n", match.Port)
				if len(match.Hostnames) > 0 {
					resultColor.Printf("    Hostnames: %s\n", strings.Join(match.Hostnames, ", "))
				}
				if len(match.Domains) > 0 {
					resultColor.Printf("    Domains: %s\n", strings.Join(match.Domains, ", "))
				}
				resultColor.Printf("    Location: %s, %s\n", match.Location.City, match.Location.Country)
				if len(match.Tags) > 0 {
					resultColor.Printf("    Tags: %s\n", strings.Join(match.Tags, ", "))
				}
				resultColor.Printf("    Last Update: %s\n", match.LastUpdate)
				resultColor.Println("    ---")
			}
		}
	}
	return nil
}

func (f *FaviconFinder) analyze(targetURL string, hashOnly bool) error {
	if !strings.HasPrefix(targetURL, "http") {
		targetURL = "https://" + targetURL
	}

	infoColor.Printf("\n[*] Target URL: %s\n", targetURL)

	var apiInfo *ShodanAPIInfo
	var err error

	// Check API key if not in hash-only mode
	if !hashOnly {
		infoColor.Printf("\n[*] Checking Shodan API key status...\n")
		apiInfo, err = f.checkAPIKey()
		if err != nil {
			return fmt.Errorf("API key error: %v", err)
		}

		successColor.Println("\n[+] API Key Information:")
		resultColor.Printf("    Plan: %s\n", apiInfo.Plan)
		resultColor.Printf("    Query Credits: %d\n", apiInfo.QueryCredits)
		resultColor.Printf("    Scan Credits: %d\n", apiInfo.ScanCredits)

		if apiInfo.Plan == "dev" || apiInfo.Plan == "free" {
			warnColor.Println("\n[!] Warning: You are using a free/dev API key. Results might be limited.")
			warnColor.Println("[!] Consider upgrading to a paid plan for full access to Shodan search.")
		}

		if apiInfo.QueryCredits < creditWarnLevel {
			warnColor.Printf("\n[!] Warning: Low query credits remaining (%d)\n", apiInfo.QueryCredits)
		}

		fmt.Println()
	}

	// Find favicon
	var faviconURL string

	// Try HTML detection first
	faviconURL, err = f.findFaviconInHTML(targetURL)
	if err != nil {
		f.debug("HTML detection failed: %v", err)
		// Try common paths
		faviconURL, err = f.checkCommonPaths(targetURL)
		if err != nil {
			return fmt.Errorf("failed to find favicon: %v", err)
		}
	}

	successColor.Printf("\n[+] Found favicon at: %s\n", faviconURL)

	// Download and process favicon
	resp, err := f.makeRequest(faviconURL)
	if err != nil {
		return fmt.Errorf("failed to download favicon: %v", err)
	}
	defer resp.Body.Close()

	faviconData, err := io.ReadAll(resp.Body)
	if err != nil {
		return fmt.Errorf("failed to read favicon data: %v", err)
	}

	hash, err := calculateMMH3(faviconData)
	if err != nil {
		return fmt.Errorf("failed to calculate hash: %v", err)
	}

	f.currentHash = hash
	successColor.Printf("[+] Favicon MMH3 hash: %d\n", hash)

	// Save to history
	if !f.config.NoHistory {
		f.history.Hashes = append(f.history.Hashes, HashResult{
			URL:          targetURL,
			Hash:         hash,
			DateTime:     time.Now(),
			Success:      true,
			ResponseTime: time.Since(f.startTime).Seconds(),
		})
		f.saveHistory()
	}

	if hashOnly {
		// Generate Shodan search URL for manual search
		searchURL := f.generateShodanSearchURL(hash)
		infoColor.Printf("\n[*] Shodan search URL: %s\n", searchURL)
		return nil
	}

	// Validate API credits
	if apiInfo != nil && apiInfo.QueryCredits <= 0 {
		warnColor.Println("\n[!] No query credits available")
		warnColor.Printf("[!] Shodan search URL: %s\n", f.generateShodanSearchURL(hash))
		return nil
	}

	// Search Shodan
	infoColor.Printf("[*] Searching Shodan...\n")
	results, err := f.searchShodan(hash)
	if err != nil {
		// If search fails, provide manual search URL
		warnColor.Printf("\n[!] Shodan search failed: %v\n", err)
		warnColor.Printf("[!] Try searching manually: %s\n", f.generateShodanSearchURL(hash))
		return err
	}

	if results.Total == 0 && apiInfo != nil && (apiInfo.Plan == "dev" || apiInfo.Plan == "free") {
		warnColor.Println("\n[!] No results found. This might be due to API key limitations.")
		warnColor.Printf("[!] Try searching manually: %s\n", f.generateShodanSearchURL(hash))
		return nil
	}

	successColor.Printf("[+] Found %d matches\n", results.Total)

	// Save results if enabled
	if f.config.SaveResults {
		if err := f.saveResults(results, hash); err != nil {
			warnColor.Printf("\n[!] Failed to save results: %v\n", err)
		}
	}

	return f.outputResults(results, f.config.OutputFormat)
}

func main() {
	var (
		shodanKey    = flag.String("k", "", "Shodan API key")
		hashOnly     = flag.Bool("hash", false, "Only calculate hash without Shodan search")
		help         = flag.Bool("h", false, "Show help")
		debug        = flag.Bool("debug", false, "Enable debug output")
		outputFormat = flag.String("o", "text", "Output format (text, json, yaml)")
		timeout      = flag.Duration("t", 10*time.Second, "Timeout for requests")
		retryCount   = flag.Int("r", 3, "Number of retries for failed requests")
		retryDelay   = flag.Duration("delay", 2*time.Second, "Delay between retries")
		proxyURL     = flag.String("proxy", "", "Proxy URL (e.g., http://127.0.0.1:8080)")
		noRedirect   = flag.Bool("no-redirect", false, "Disable following redirects")
		customUA     = flag.String("ua", "", "Custom User-Agent string")
		saveResults  = flag.Bool("save", false, "Save results to file")
		noHistory    = flag.Bool("no-history", false, "Disable search history")
	)

	flag.Usage = func() {
		fmt.Printf(banner, version)
		fmt.Printf("\nUsage: favhash [options] <url>\n\n")
		fmt.Printf("Options:\n")
		flag.PrintDefaults()
		fmt.Printf("\nExamples:\n")
		fmt.Printf("  favhash -k YOUR_SHODAN_KEY example.com\n")
		fmt.Printf("  favhash -hash example.com\n")
		fmt.Printf("  favhash -k YOUR_SHODAN_KEY -o json -t 15s example.com\n")
		fmt.Printf("  favhash -k YOUR_SHODAN_KEY -proxy http://127.0.0.1:8080 example.com\n")
	}

	flag.Parse()

	if *help || flag.NArg() == 0 {
		flag.Usage()
		os.Exit(0)
	}

	if !*hashOnly && *shodanKey == "" {
		errorColor.Println("[-] Error: Shodan API key is required for searching. Use -k flag to provide it.")
		warnColor.Println("[!] Tip: Use -hash flag if you only want to calculate the hash.")
		os.Exit(1)
	}

	config := &Config{
		UserAgent:      *customUA,
		Timeout:        *timeout,
		RetryCount:     *retryCount,
		RetryDelay:     *retryDelay,
		ProxyURL:       *proxyURL,
		OutputFormat:   *outputFormat,
		FollowRedirect: !*noRedirect,
		Debug:          *debug,
		SaveResults:    *saveResults,
		NoHistory:      *noHistory,
	}

	// Initialize random seed for user agent rotation
	rand.Seed(time.Now().UnixNano())

	// Print banner
	fmt.Printf(banner, version)

	finder := NewFaviconFinder(*shodanKey, config)
	if err := finder.analyze(flag.Arg(0), *hashOnly); err != nil {
		errorColor.Printf("[-] Error: %v\n", err)
		os.Exit(1)
	}
}

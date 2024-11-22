package main

import (
	"encoding/base64"
	"encoding/json"
	"encoding/xml"
	"flag"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"strings"
	"time"

	"github.com/PuerkitoBio/goquery"
	"github.com/fatih/color"
	"github.com/spaolacci/murmur3"
)

const banner = `
███████╗ █████╗ ██╗   ██╗██╗  ██╗ █████╗ ███████╗██╗  ██╗
██╔════╝██╔══██╗██║   ██║██║  ██║██╔══██╗██╔════╝██║  ██║
█████╗  ███████║██║   ██║███████║███████║███████╗███████║
██╔══╝  ██╔══██║╚██╗ ██╔╝██╔══██║██╔══██║╚════██║██╔══██║
██║     ██║  ██║ ╚████╔╝ ██║  ██║██║  ██║███████║██║  ██║
╚═╝     ╚═╝  ╚═╝  ╚═══╝  ╚═╝  ╚═╝╚═╝  ╚═╝╚══════╝╚═╝  ╚═╝
                                           v1.0 by @0xJosep
`

var (
	infoColor    = color.New(color.FgCyan)
	successColor = color.New(color.FgGreen)
	errorColor   = color.New(color.FgRed)
	warnColor    = color.New(color.FgYellow)
	resultColor  = color.New(color.FgHiMagenta)
)

type WebAppManifest struct {
	Icons []struct {
		Src   string `json:"src"`
		Sizes string `json:"sizes"`
		Type  string `json:"type"`
	} `json:"icons"`
}

type BrowserConfig struct {
	XMLName xml.Name `xml:"browserconfig"`
	MSApp   struct {
		Tile struct {
			Square70x70Logo   string `xml:"square70x70logo"`
			Square150x150Logo string `xml:"square150x150logo"`
			Square310x310Logo string `xml:"square310x310logo"`
		} `xml:"tile"`
	} `xml:"msapplication"`
}

type ShodanAPIInfo struct {
	QueryCredits int    `json:"query_credits"`
	ScanCredits  int    `json:"scan_credits"`
	Plan         string `json:"plan"`
	HTTPS        bool   `json:"https"`
	Unlocked     bool   `json:"unlocked"`
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
	} `json:"matches"`
	Total int `json:"total"`
}

type FaviconFinder struct {
	client    *http.Client
	shodanKey string
}

func NewFaviconFinder(shodanKey string) *FaviconFinder {
	return &FaviconFinder{
		client: &http.Client{
			Timeout: 10 * time.Second,
		},
		shodanKey: shodanKey,
	}
}

func (f *FaviconFinder) checkAPIKey() (*ShodanAPIInfo, error) {
	infoURL := fmt.Sprintf("https://api.shodan.io/api-info?key=%s", f.shodanKey)

	resp, err := f.client.Get(infoURL)
	if err != nil {
		return nil, fmt.Errorf("failed to check API key: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		return nil, fmt.Errorf("invalid API key")
	}

	var info ShodanAPIInfo
	if err := json.NewDecoder(resp.Body).Decode(&info); err != nil {
		return nil, fmt.Errorf("failed to decode API info: %v", err)
	}

	return &info, nil
}

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

func (f *FaviconFinder) findFaviconInHTML(targetURL string) (string, error) {
	infoColor.Printf("[*] Checking HTML for favicon links\n")

	resp, err := f.client.Get(targetURL)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	doc, err := goquery.NewDocumentFromReader(resp.Body)
	if err != nil {
		return "", err
	}

	// Check for shortcut icon
	shortcutIcon := doc.Find("link[rel='shortcut icon']").First()
	if href, exists := shortcutIcon.Attr("href"); exists {
		infoColor.Printf("[*] Found shortcut icon: %s\n", href)
		return f.resolveURL(targetURL, href)
	}

	// Check for regular icon
	icon := doc.Find("link[rel='icon']").First()
	if href, exists := icon.Attr("href"); exists {
		infoColor.Printf("[*] Found icon: %s\n", href)
		return f.resolveURL(targetURL, href)
	}

	return "", fmt.Errorf("no favicon found in HTML")
}

func (f *FaviconFinder) checkCommonPaths(targetURL string) (string, error) {
	infoColor.Printf("[*] Checking common favicon paths\n")

	commonPaths := []string{
		"/favicon.ico",
		"/favicon.png",
		"/assets/favicon.ico",
		"/assets/favicon.png",
		"/static/favicon.ico",
		"/static/favicon.png",
		"/images/favicon.ico",
		"/images/favicon.png",
		"/img/favicon.ico",
		"/img/favicon.png",
	}

	parsedURL, err := url.Parse(targetURL)
	if err != nil {
		return "", err
	}

	for _, path := range commonPaths {
		faviconURL := fmt.Sprintf("%s://%s%s", parsedURL.Scheme, parsedURL.Host, path)
		infoColor.Printf("[*] Trying: %s\n", faviconURL)

		resp, err := f.client.Head(faviconURL)
		if err != nil {
			continue
		}
		resp.Body.Close()

		if resp.StatusCode == 200 {
			return faviconURL, nil
		}
	}

	return "", fmt.Errorf("no favicon found at common paths")
}

func calculateMMH3(data []byte) (int32, error) {
	b64 := base64.StdEncoding.EncodeToString(data)
	hash := murmur3.Sum32([]byte(b64))
	return int32(hash), nil
}

func (f *FaviconFinder) searchShodan(hash int32) (*ShodanResponse, error) {
	searchURL := fmt.Sprintf("https://api.shodan.io/shodan/host/search?key=%s&query=http.favicon.hash:%d",
		f.shodanKey, hash)

	resp, err := f.client.Get(searchURL)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		return nil, fmt.Errorf("Shodan API returned status code %d", resp.StatusCode)
	}

	var result ShodanResponse
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, err
	}

	return &result, nil
}

func (f *FaviconFinder) analyze(targetURL string, hashOnly bool) error {
	if !strings.HasPrefix(targetURL, "http") {
		targetURL = "https://" + targetURL
	}

	infoColor.Printf("\n[*] Target URL: %s\n", targetURL)

	// Check API key if not in hash-only mode
	var apiInfo *ShodanAPIInfo
	var err error
	if !hashOnly {
		infoColor.Printf("\n[*] Checking Shodan API key status...\n")
		apiInfo, err = f.checkAPIKey()
		if err != nil {
			errorColor.Printf("[-] API key error: %v\n", err)
			return err
		}

		// Display API information
		successColor.Println("\n[+] API Key Information:")
		resultColor.Printf("    Plan: %s\n", apiInfo.Plan)
		resultColor.Printf("    Query Credits: %d\n", apiInfo.QueryCredits)
		resultColor.Printf("    Scan Credits: %d\n", apiInfo.ScanCredits)

		if apiInfo.Plan == "dev" || apiInfo.Plan == "free" {
			warnColor.Println("\n[!] Warning: You are using a free/dev API key. Results might be limited.")
			warnColor.Println("[!] Consider upgrading to a paid plan for full access to Shodan search.")
		}
		fmt.Println()
	}

	// Find favicon
	var faviconURL string

	// Try HTML first
	faviconURL, err = f.findFaviconInHTML(targetURL)
	if err != nil {
		infoColor.Printf("[*] HTML detection failed, trying common paths\n")
		faviconURL, err = f.checkCommonPaths(targetURL)
		if err != nil {
			return fmt.Errorf("failed to find favicon: %v", err)
		}
	}

	successColor.Printf("\n[+] Found favicon at: %s\n", faviconURL)

	// Download and process favicon
	resp, err := f.client.Get(faviconURL)
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

	successColor.Printf("[+] Favicon MMH3 hash: %d\n", hash)

	if hashOnly {
		return nil
	}

	// Search Shodan
	infoColor.Printf("[*] Searching Shodan...\n")
	results, err := f.searchShodan(hash)
	if err != nil {
		return fmt.Errorf("Shodan search failed: %v", err)
	}

	if results.Total == 0 && apiInfo != nil && (apiInfo.Plan == "dev" || apiInfo.Plan == "free") {
		warnColor.Println("\n[!] No results found. This might be due to API key limitations.")
		warnColor.Printf("[!] Try searching for this hash manually on Shodan: http.favicon.hash:%d\n", hash)
		return nil
	}

	successColor.Printf("[+] Found %d matches\n", results.Total)
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
			resultColor.Println("    ---")
		}
	}

	return nil
}

func main() {
	var (
		shodanKey = flag.String("k", "", "Shodan API key")
		hashOnly  = flag.Bool("hash", false, "Only calculate hash without Shodan search")
		help      = flag.Bool("h", false, "Show help")
	)

	flag.Usage = func() {
		fmt.Println(banner)
		fmt.Printf("\nUsage: favhash [options] <url>\n\n")
		fmt.Printf("Options:\n")
		flag.PrintDefaults()
		fmt.Printf("\nExamples:\n")
		fmt.Printf("  favhash -k YOUR_SHODAN_KEY example.com\n")
		fmt.Printf("  favhash -hash example.com\n")
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

	color.Cyan(banner)

	finder := NewFaviconFinder(*shodanKey)
	if err := finder.analyze(flag.Arg(0), *hashOnly); err != nil {
		errorColor.Printf("[-] Error: %v\n", err)
		os.Exit(1)
	}
}

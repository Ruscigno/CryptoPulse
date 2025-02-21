package crawler

import (
	"fmt"
	"net/http"
	"time"

	"github.com/gocolly/colly"
	"go.uber.org/zap"
)

// ProxyManager handles proxy initialization and retry logic
type ProxyManager struct {
	proxies []string
}

func NewProxyManager() *ProxyManager {
	return &ProxyManager{}
}

// Initialize fetches public proxies
func (pm *ProxyManager) Initialize() error {
	proxies, err := fetchPublicProxies()
	if err != nil {
		return err
	}
	pm.proxies = proxies
	zap.L().Info("Fetched proxies", zap.Int("count", len(proxies)))
	return nil
}

// TryVisit attempts to visit a URL with no proxy first, then each proxy
func (pm *ProxyManager) TryVisit(collector *colly.Collector, url string) error {
	if err := collector.Visit(url); err == nil {
		return nil
	}

	if len(pm.proxies) == 0 {
		err := pm.Initialize()
		if err != nil {
			return fmt.Errorf("failed to initialize proxies: %v", err)
		}
	}
	// Retry with each proxy
	for _, proxy := range pm.proxies {
		collector.SetProxy(proxy)
		err := collector.Visit(url)
		if err == nil {
			return nil
		}
		zap.L().Warn("Visit failed with proxy", zap.String("proxy", proxy), zap.Error(err))
	}
	return fmt.Errorf("all %d proxies failed", len(pm.proxies))
}

// fetchPublicProxies fetches a list of HTTP proxies from free-proxy-list.net
func fetchPublicProxies() ([]string, error) {
	url := "https://free-proxy-list.net/"
	client := &http.Client{Timeout: 10 * time.Second}
	resp, err := client.Get(url)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch proxy list: %v", err)
	}
	defer resp.Body.Close()

	collector := colly.NewCollector()
	var proxies []string

	collector.OnHTML("#list > div > div.table-responsive > div > table > tbody > tr", func(e *colly.HTMLElement) {
		ip := e.ChildText("td:nth-child(1)")
		port := e.ChildText("td:nth-child(2)")
		https := e.ChildText("td:nth-child(7)") // "Yes" or "No" for HTTPS support

		if ip != "" && port != "" && https == "yes" {
			proxies = append(proxies, fmt.Sprintf("http://%s:%s", ip, port))
		}
	})

	collector.OnError(func(r *colly.Response, err error) {
		zap.L().Warn("Error scraping proxy list", zap.Error(err))
	})

	if err := collector.Visit(url); err != nil {
		return nil, fmt.Errorf("failed to scrape proxies: %v", err)
	}
	collector.Wait()

	if len(proxies) == 0 {
		return nil, fmt.Errorf("no proxies found")
	}
	return proxies, nil
}

// User-Agent pool (unchanged)
var userAgents = []string{
	"Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/91.0.4472.124 Safari/537.36",
	"Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7) AppleWebKit/605.1.15 (KHTML, like Gecko) Version/14.0 Safari/605.1.15",
	"Mozilla/5.0 (X11; Ubuntu; Linux x86_64; rv:89.0) Gecko/20100101 Firefox/89.0",
}

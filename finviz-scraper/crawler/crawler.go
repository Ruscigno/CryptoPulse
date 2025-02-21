package crawler

import (
	"context"
	"fmt"
	"net/url"
	"strconv"
	"time"

	"github.com/Ruscigno/stockscreener/finviz-scraper/storage"
	"github.com/Ruscigno/stockscreener/models"
	"github.com/gocolly/colly"
	"go.uber.org/zap"
)

type Crawler struct {
	storage      *storage.MongoStorage
	proxyManager *ProxyManager
}

func NewCrawler(store *storage.MongoStorage) *Crawler {
	return &Crawler{
		storage:      store,
		proxyManager: NewProxyManager(),
	}
}

func (c *Crawler) Scrape(ctx context.Context, job *models.ScrapeJob) error {
	rule, err := c.storage.GetRule(ctx, job.RuleID)
	if err != nil {
		return fmt.Errorf("failed to get rule: %v", err)
	}

	collector := c.setupCollector(job)
	var allStocks []map[string]string
	collector.OnHTML(rule.Table.Selector, func(e *colly.HTMLElement) {
		zap.L().Info("Scraping table", zap.String("url", e.Request.URL.String()))
		e.ForEach(rule.Table.Rows.Selector, func(i int, row *colly.HTMLElement) {
			if i == 0 {
				return // Skip header row
			}
			stock := make(map[string]string)
			for _, field := range rule.Table.Rows.Fields {
				stock[field.Field] = row.ChildText(field.Selector)
			}
			allStocks = append(allStocks, stock)
		})
	})

	collector.OnScraped(func(r *colly.Response) {
		zap.L().Info("Scraping completed", zap.String("url", r.Request.URL.String()))
	})

	// Scrape each page
	pageURL := job.BaseURL
	for offset := 1; ; offset += job.OffsetIncrement {
		if rule.NextPage != nil && offset > 1 {
			pageURL = c.buildURL(job.BaseURL, offset)
		}
		if err := c.visitWithRetry(collector, pageURL); err != nil {
			return fmt.Errorf("failed to scrape page %s: %v", pageURL, err)
		}
		collector.Wait()
		time.Sleep(time.Duration(job.Delay) * time.Millisecond)

		if rule.NextPage == nil || offset >= job.OffsetMax {
			break
		}
	}

	// Save results
	return c.storage.SaveStocks(ctx, allStocks)
}

func (c *Crawler) setupCollector(job *models.ScrapeJob) *colly.Collector {
	collector := colly.NewCollector(
		colly.UserAgent(job.UserAgent),
		colly.MaxDepth(job.MaxDepth),
	)
	collector.Limit(&colly.LimitRule{
		DomainGlob:  job.DomainGlobal,
		Parallelism: job.Parallelism,
		Delay:       time.Duration(job.Delay) * time.Millisecond,
	})
	return collector
}

func (c *Crawler) visitWithRetry(collector *colly.Collector, url string) error {
	return c.proxyManager.TryVisit(collector, url)
}

func (c *Crawler) buildURL(baseURL string, offset int) string {
	u, _ := url.Parse(baseURL)
	q := u.Query()
	q.Set("r", strconv.Itoa(offset))
	u.RawQuery = q.Encode()
	return u.String()
}

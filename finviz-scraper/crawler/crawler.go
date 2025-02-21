package crawler

import (
	"context"
	"encoding/json"
	"fmt"
	"net/url"
	"strconv"
	"strings"
	"time"

	"github.com/PuerkitoBio/goquery"
	"github.com/Ruscigno/stockscreener/finviz-scraper/storage"
	"github.com/Ruscigno/stockscreener/models"
	"github.com/gocolly/colly"
	"go.mongodb.org/mongo-driver/bson"
)

type Crawler struct {
	storage *storage.MongoStorage
}

func NewCrawler(store *storage.MongoStorage) *Crawler {
	return &Crawler{storage: store}
}

func (c *Crawler) Scrape(ctx context.Context, job *models.ScrapeJob) error {
	rule, err := c.storage.GetRule(ctx, job.RuleID)
	if err != nil {
		return fmt.Errorf("failed to get rule: %v", err)
	}

	collector := colly.NewCollector(
		colly.UserAgent("FinViz Scraper"),
		colly.MaxDepth(0),
	)
	collector.Limit(&colly.LimitRule{
		DomainGlob:  "*.finviz.com/*",
		Parallelism: 10,
		Delay:       100 * time.Millisecond, // 10 req/sec
	})

	collector.OnHTML("table.screener-table", func(e *colly.HTMLElement) {
		e.DOM.Find("tr").Each(func(i int, row *goquery.Selection) {
			if i == 0 {
				return // Skip header row
			}
			data := make(map[string]string)
			for _, r := range rule.Fields {
				data[r.Field] = strings.TrimSpace(row.Find(r.Selector).Text())
			}
			jsonData, _ := json.Marshal(data)
			c.storage.SaveData(ctx, bson.Raw(jsonData))
		})
	})

	baseURL := job.BaseURL
	offset := 1
	for {
		url := c.buildURL(baseURL, rule, offset)
		if err := collector.Visit(url); err != nil {
			return err
		}
		collector.Wait()

		if rule.NextPage == nil || offset >= 1000 { // Arbitrary max pages
			break
		}
		offset += 20
	}
	return nil
}

func (c *Crawler) buildURL(baseURL string, rule *models.Rule, offset int) string {
	if rule.NextPage == nil || offset == 1 {
		return baseURL
	}
	u, _ := url.Parse(baseURL)
	q := u.Query()
	q.Set("r", strconv.Itoa(offset))
	u.RawQuery = q.Encode()
	return u.String()
}

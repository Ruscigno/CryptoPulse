# FinViz Scraper

A modular, scalable web scraper built in **Golang** to extract data from the [FinViz stock screener](https://finviz.com/screener.ashx). This project follows **SOLID principles** and **Clean Code practices**, providing a flexible system for scraping tabular data, managing scraping rules via JSON, and storing results in MongoDB.

## Features
- **Dynamic Scraping Rules**: Define scraping logic in JSON files with CSS selectors, stored in MongoDB, and editable via RESTful endpoints.
- **Concurrent Processing**: Uses a worker pool with a queue to process pages at a controlled rate (default: 10 requests/second).
- **Staged Pipeline**: Separates downloading, processing, and storing for maintainability.
- **RESTful API**: Manage scraping rules and jobs through HTTP endpoints.
- **Pagination Support**: Automatically navigates paginated results using configurable rules.
- **NoSQL Storage**: Stores scraped data as JSON in MongoDB.

## Prerequisites
- **Golang**: Version 1.21 or higher.
- **MongoDB**: A running instance (local or remote).
- **Git**: To clone the repository.

## Installation

1. **Clone the Repository**:
   ```bash
   git clone https://github.com/Sander Ruscigno/finviz-scraper.git
   cd finviz-scraper
   ```

2. **Install Dependencies**:
   ```bash
   go mod tidy
   ```

3. **Set Environment Variables**:
   Create a `.env` file or export variables:
   ```bash
   export MONGO_URI="mongodb://localhost:27017"
   export DB_NAME="finviz"
   ```

## Running the Scraper

1. **Start the Server**:
   ```bash
   go run cmd/server/main.go
   ```
   The server will listen on `http://localhost:3001`.

2. **Create a Scraping Rule**:
   Use `curl` or a tool like Postman to define a rule:
   ```bash
   curl --location 'http://localhost:3001/rules' \
      --header 'Content-Type: application/json' \
      --data '{
         "id": "FinViz001",
         "name": "FinViz Screener overview tag",
         "fields": [
            {
                  "field": "ticker",
                  "selector": "td:nth-child(2) a"
            },
            {
                  "field": "company",
                  "selector": "td:nth-child(3) a"
            },
            {
                  "field": "sector",
                  "selector": "td:nth-child(4)"
            }
         ],
         "next_page": {
            "pattern": "r={offset}",
            "limit": 20
         }
      }'
   ```

3. **Start a Scraping Job**:
   Submit a job to scrape the FinViz screener:
   ```bash
   curl --location 'http://localhost:3001/jobs' \
      --header 'Content-Type: application/json' \
      --data '{
         "id": "FinViz-job1",
         "base_url": "https://finviz.com/screener.ashx?v=141&ft=4&o=-perfytd&ar=180",
         "rule_id": "FinViz001"
      }'
   ```

4. **View Results**:
   Check the `scraped_data` collection in your MongoDB database `finviz`. Example document:
   ```json
   {
     "_id": "some-object-id",
     "ticker": "AAPL",
     "company": "Apple Inc.",
     "sector": "Technology"
   }
   ```

## API Endpoints

| Method | Endpoint         | Description                     | Request Body Example                     |
|--------|------------------|----------------------------------|------------------------------------------|
| `POST` | `/rules`         | Create a new scraping rule      | `{"id": "rule1", "fields": [...], ...}` |
| `GET`  | `/rules/:id`     | Retrieve a rule by ID           | N/A                                      |
| `PUT`  | `/rules`         | Update an existing rule         | `{"id": "rule1", "fields": [...], ...}` |
| `DELETE` | `/rules/:id`   | Delete a rule by ID             | N/A                                      |
| `POST` | `/jobs`          | Start a scraping job            | `{"id": "job1", "base_url": "...", ...}`|

## Project Structure

```
/finviz-scraper
├── /cmd
│   └── /server          # Application entry point
│       └── main.go
├── /internal
│   ├── /config          # Configuration management
│   ├── /crawler         # Scraping logic
│   ├── /worker          # Worker pool and queue
│   ├── /storage         # MongoDB storage layer
│   ├── /api             # RESTful API handlers and routes
│   └── /models          # Data models (jobs, rules)
├── /pkg                 # Reusable utilities (empty for now)
├── go.mod               # Go module definition
└── README.md            # This file
```

## Architecture
- **Crawler**: Fetches pages using `colly` and extracts data based on JSON rules.
- **Worker Pool**: Manages concurrency with a queue, rate-limited to 10 requests/second.
- **Storage**: Persists rules, jobs, and scraped data in MongoDB.
- **API**: Provides endpoints to manage rules and trigger jobs using `gin`.

## Extending the Project
- **Detail Pages**: Add logic in `crawler.go` to follow company links and apply additional rules.
- **Custom Rate Limits**: Modify `colly.LimitRule` in `crawler.go`.
- **New Data Sources**: Update the base URL and rules to target other websites.

## Dependencies
- `github.com/gocolly/colly`: Web scraping framework.
- `github.com/PuerkitoBio/goquery`: HTML parsing with CSS selectors.
- `github.com/gin-gonic/gin`: HTTP server framework.
- `go.mongodb.org/mongo-driver`: MongoDB driver.

## Contributing
Feel free to submit issues or pull requests to improve the project!

## License
MIT License - see [LICENSE](LICENSE) for details.

---
Built with ❤️ by [Sander Ruscigno] on February 21, 2025.
package worker

import (
	"context"

	"github.com/Ruscigno/stockscreener/models"
	"go.uber.org/zap"
)

type Crawler interface {
	Scrape(ctx context.Context, job *models.ScrapeJob) error
}

type Worker struct {
	jobs    <-chan *models.ScrapeJob
	crawler Crawler
	stop    chan struct{}
}

func NewWorker(jobs <-chan *models.ScrapeJob, crawler Crawler) *Worker {
	return &Worker{
		jobs:    jobs,
		crawler: crawler,
		stop:    make(chan struct{}),
	}
}

func (w *Worker) Start() {
	go func() {
		for {
			select {
			case job, ok := <-w.jobs:
				if !ok {
					return
				}
				if err := w.crawler.Scrape(context.Background(), job); err != nil {
					zap.L().Error("Worker failed to process job", zap.String("job_id", job.ID), zap.Error(err))
				}
				zap.L().Info("Worker processed job", zap.String("job_id", job.ID))
			case <-w.stop:
				return
			}
		}
	}()
}

func (w *Worker) Stop() {
	close(w.stop)
}

package worker

import (
	"github.com/Ruscigno/stockscreener/models"
)

type WorkQueue struct {
	jobs    chan *models.ScrapeJob
	workers []*Worker
}

func NewWorkQueue(numWorkers int, crawler Crawler) *WorkQueue {
	wq := &WorkQueue{
		jobs:    make(chan *models.ScrapeJob, 100),
		workers: make([]*Worker, numWorkers),
	}
	for i := 0; i < numWorkers; i++ {
		wq.workers[i] = NewWorker(wq.jobs, crawler)
		wq.workers[i].Start()
	}
	return wq
}

func (wq *WorkQueue) Enqueue(job *models.ScrapeJob) {
	wq.jobs <- job
}

func (wq *WorkQueue) Stop() {
	close(wq.jobs)
	for _, w := range wq.workers {
		w.Stop()
	}
}

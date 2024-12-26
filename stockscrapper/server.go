package stockscrapper

import (
	"log"
	"os"
	"os/signal"
	"sync"
	"syscall"

	"github.com/google/uuid"
)

const (
	NumberOfWorkers = 5
)

type Server struct {
	scraper StockScrapper
}

// Task represents a unit of work with an idempotent key.
type Task struct {
	ID        string
	Symbol    string
	TimeFrame string
}

// WorkerPool manages a pool of workers that process tasks.
type WorkerPool struct {
	wg        sync.WaitGroup
	tasks     chan Task
	workers   int
	quit      chan struct{}
	processed map[string]bool // Set to track processed task IDs
	scraper   StockScrapper
}

// NewWorkerPool creates a new WorkerPool with the specified number of workers.
func NewWorkerPool(workers int, scraper StockScrapper) *WorkerPool {
	return &WorkerPool{
		workers:   workers,
		tasks:     make(chan Task, 100), // Buffered channel for tasks
		quit:      make(chan struct{}),
		processed: make(map[string]bool),
		scraper:   scraper,
	}
}

// Start starts the worker pool.
func (wp *WorkerPool) Start() {
	for i := 0; i < wp.workers; i++ {
		wp.wg.Add(1)
		go wp.worker()
	}
}

// Stop gracefully stops the worker pool.
func (wp *WorkerPool) Stop() {
	close(wp.quit)
	wp.wg.Wait()
}

// Enqueue adds a task to the queue.
func (wp *WorkerPool) Enqueue(task Task) {
	wp.tasks <- task
}

// worker is the function executed by each worker goroutine.
func (wp *WorkerPool) worker() {
	defer wp.wg.Done()

	for {
		select {
		case task := <-wp.tasks:
			if _, exists := wp.processed[task.ID]; !exists {
				// Process the task
				log.Printf("Worker processing task: %s-%s", task.Symbol, task.TimeFrame)
				// Simulate task processing with a short delay
				wp.scraper.DownloadStockData(task.Symbol, task.TimeFrame)
				wp.processed[task.ID] = true

				// Requeue the task
				wp.Enqueue(task)
			} else {
				log.Printf("Task already processed: %s", task.ID)
			}
		case <-wp.quit:
			return
		}
	}
}

func NewServer() *Server {
	return &Server{
		scraper: NewStockScrapper(),
	}
}

func (s *Server) Start(symbolList []string) {
	// Create a new worker pool with 5 workers
	pool := NewWorkerPool(NumberOfWorkers, s.scraper)
	pool.Start()

	// Generate and enqueue some sample tasks
	for _, symbol := range symbolList {
		task := Task{ID: uuid.New().String(), Symbol: symbol, TimeFrame: "1m"}
		pool.Enqueue(task)
	}

	// Handle graceful shutdown
	sigs := make(chan os.Signal, 1)
	signal.Notify(sigs, syscall.SIGINT, syscall.SIGTERM)
	<-sigs
	log.Println("Received termination signal")
	pool.Stop()
}

package api

import (
	"github.com/Ruscigno/stockscreener/finviz-scraper/storage"
	"github.com/Ruscigno/stockscreener/finviz-scraper/worker"
	"github.com/gin-gonic/gin"
)

func SetupRouter(store *storage.MongoStorage, queue *worker.WorkQueue) *gin.Engine {
	r := gin.Default()
	h := NewHandler(store, queue)

	r.POST("/rules", h.CreateRule)
	r.GET("/rules/:id", h.GetRule)
	r.PUT("/rules", h.UpdateRule)
	r.DELETE("/rules/:id", h.DeleteRule)
	r.POST("/jobs", h.StartJob)

	return r
}

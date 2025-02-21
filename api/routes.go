package api

import (
	"github.com/Ruscigno/stockscreener/finviz-scraper/storage"
	"github.com/Ruscigno/stockscreener/finviz-scraper/worker"
	"github.com/gin-gonic/gin"
	"go.uber.org/zap"
)

func SetupRouter(store *storage.MongoStorage, queue *worker.WorkQueue) *gin.Engine {
	r := gin.Default()
	h := NewHandler(store, queue)
	// I want Gin to use zap.L() as the logger
	r.Use(LoggerMiddleware())

	r.POST("/rules", h.CreateRule)
	r.GET("/rules/:id", h.GetRule)
	r.PUT("/rules", h.UpdateRule)
	r.DELETE("/rules/:id", h.DeleteRule)
	r.POST("/jobs", h.StartJob)

	return r
}

// LoggerMiddleware is a middleware that sets the logger to zap.L()
func LoggerMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		c.Set("logger", zap.L())
		c.Next()
	}
}

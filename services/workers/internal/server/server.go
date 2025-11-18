package server

import (
	"context"
	"log"
	"net/http"

	"github.com/gin-gonic/gin"
	"go.opentelemetry.io/contrib/instrumentation/github.com/gin-gonic/gin/otelgin"

	"github.com/oteldemo/workers/internal/config"
	"github.com/oteldemo/workers/internal/redis"
)

// Server represents the HTTP server
type Server struct {
	cfg         *config.Config
	redis       *redis.Client
	httpServer  *http.Server
}

// NewServer creates a new HTTP server
func NewServer(cfg *config.Config, redisClient *redis.Client) *Server {
	return &Server{
		cfg:   cfg,
		redis: redisClient,
	}
}

// Start starts the HTTP server
func (s *Server) Start() error {
	gin.SetMode(gin.ReleaseMode)

	router := gin.New()
	router.Use(gin.Recovery())
	router.Use(gin.Logger())

	// Add OpenTelemetry middleware
	router.Use(otelgin.Middleware(s.cfg.ServiceName))

	// Health check endpoint
	router.GET("/health", s.healthCheck)

	// Status endpoint
	router.GET("/status", s.status)

	s.httpServer = &http.Server{
		Addr:    ":" + s.cfg.HTTPPort,
		Handler: router,
	}

	log.Printf("HTTP server starting on port %s", s.cfg.HTTPPort)
	return s.httpServer.ListenAndServe()
}

// Shutdown gracefully shuts down the server
func (s *Server) Shutdown(ctx context.Context) error {
	if s.httpServer != nil {
		return s.httpServer.Shutdown(ctx)
	}
	return nil
}

// healthCheck handles health check requests
func (s *Server) healthCheck(c *gin.Context) {
	redisHealthy := s.redis.IsHealthy(c.Request.Context())

	status := "healthy"
	httpStatus := http.StatusOK

	if !redisHealthy {
		status = "unhealthy"
		httpStatus = http.StatusServiceUnavailable
	}

	c.JSON(httpStatus, gin.H{
		"status":        status,
		"service":       s.cfg.ServiceName,
		"location":      s.cfg.Location,
		"redis_healthy": redisHealthy,
	})
}

// status provides detailed status information
func (s *Server) status(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{
		"service":  s.cfg.ServiceName,
		"location": s.cfg.Location,
		"streams": gin.H{
			"tasks":   s.cfg.TasksStream,
			"results": s.cfg.ResultsStream,
		},
		"consumer_group": s.cfg.ConsumerGroup,
	})
}

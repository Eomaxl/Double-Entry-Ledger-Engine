package handlers

import (
	"context"
	"net/http"
	"runtime"
	"time"

	"github.com/Eomaxl/double-entry-ledger-engine/internal/config"
	"github.com/Eomaxl/double-entry-ledger-engine/internal/infrastructure/database"
	"github.com/gin-gonic/gin"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"go.uber.org/zap"
)

// HealthHandler handles HTTP requests for health and observability endpoints
type HealthHandler struct {
	db     database.PoolProbe
	config *config.Config
	logger *zap.Logger
	probe  Probe
}

type Probe interface {
	IsLive() bool
	IsReady() bool
}

// NewHealthHandler creates a new health handler
func NewHealthHandler(
	db database.PoolProbe,
	config *config.Config,
	logger *zap.Logger,
	probe Probe,
) *HealthHandler {
	return &HealthHandler{
		db:     db,
		config: config,
		logger: logger,
		probe:  probe,
	}
}

// HealthResponse represents the health check response
type HealthResponse struct {
	Status    string           `json:"status"`
	Timestamp time.Time        `json:"timestamp"`
	Version   string           `json:"version"`
	Checks    map[string]Check `json:"checks"`
	Uptime    string           `json:"uptime"`
}

// Check represents an individual health check
type Check struct {
	Status  string `json:"status"`
	Message string `json:"message,omitempty"`
}

// SystemInfo represents system information
type SystemInfo struct {
	Version     string           `json:"version"`
	BuildTime   string           `json:"build_time"`
	GoVersion   string           `json:"go_version"`
	Environment string           `json:"environment"`
	Config      SystemConfigInfo `json:"config"`
	Runtime     RuntimeInfo      `json:"runtime"`
	Timestamp   time.Time        `json:"timestamp"`
}

// SystemConfigInfo represents configuration information
type SystemConfigInfo struct {
	DatabaseHost        string        `json:"database_host"`
	DatabaseName        string        `json:"database_name"`
	MaxConnections      int           `json:"max_connections"`
	IdempotencyTTL      time.Duration `json:"idempotency_ttl"`
	MaxBatchSize        int           `json:"max_batch_size"`
	SupportedCurrencies []string      `json:"supported_currencies"`
}

// RuntimeInfo represents runtime information
type RuntimeInfo struct {
	NumGoroutines int         `json:"num_goroutines"`
	NumCPU        int         `json:"num_cpu"`
	MemoryStats   MemoryStats `json:"memory_stats"`
}

// MemoryStats represents memory statistics
type MemoryStats struct {
	Alloc      uint64 `json:"alloc_bytes"`
	TotalAlloc uint64 `json:"total_alloc_bytes"`
	Sys        uint64 `json:"sys_bytes"`
	NumGC      uint32 `json:"num_gc"`
}

var (
	startTime = time.Now()
	version   = "dev"     // Set during build
	buildTime = "unknown" // Set during build
)

// HealthCheck handles GET /health - Health check with database connectivity
func (h *HealthHandler) HealthCheck(c *gin.Context) {
	checks := make(map[string]Check)
	overallStatus := "healthy"

	// Check database connectivity
	dbStatus := h.checkDatabase(c.Request.Context())
	checks["database"] = dbStatus
	if dbStatus.Status != "healthy" {
		overallStatus = "unhealthy"
	}

	// Check memory usage
	memStatus := h.checkMemory()
	checks["memory"] = memStatus
	if memStatus.Status != "healthy" {
		overallStatus = "degraded"
	}

	response := HealthResponse{
		Status:    overallStatus,
		Timestamp: time.Now(),
		Version:   version,
		Checks:    checks,
		Uptime:    time.Since(startTime).String(),
	}

	statusCode := http.StatusOK
	if overallStatus == "unhealthy" {
		statusCode = http.StatusServiceUnavailable
	} else if overallStatus == "degraded" {
		statusCode = http.StatusOK // Still return 200 for degraded
	}

	c.JSON(statusCode, response)
}

// Liveness handles GET /livez - process liveness only.
func (h *HealthHandler) Liveness(c *gin.Context) {
	if h.probe != nil && !h.probe.IsLive() {
		c.JSON(http.StatusServiceUnavailable, gin.H{
			"status": "unhealthy",
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"status": "alive",
	})
}

// Readiness handles GET /readyz - readiness for serving traffic.
func (h *HealthHandler) Readiness(c *gin.Context) {
	if h.probe != nil && !h.probe.IsReady() {
		c.JSON(http.StatusServiceUnavailable, gin.H{
			"status": "not_ready",
		})
		return
	}

	dbStatus := h.checkDatabase(c.Request.Context())
	if dbStatus.Status == "unhealthy" {
		c.JSON(http.StatusServiceUnavailable, gin.H{
			"status": "not_ready",
			"check":  dbStatus,
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"status": "ready",
	})
}

// Metrics handles GET /metrics - Prometheus metrics endpoint
func (h *HealthHandler) Metrics(c *gin.Context) {
	// Use Prometheus handler directly
	promhttp.Handler().ServeHTTP(c.Writer, c.Request)
}

// SystemInfo handles GET /v1/system/info - System information
func (h *HealthHandler) SystemInfo(c *gin.Context) {
	var memStats runtime.MemStats
	runtime.ReadMemStats(&memStats)

	info := SystemInfo{
		Version:     version,
		BuildTime:   buildTime,
		GoVersion:   runtime.Version(),
		Environment: h.config.Tracing.Environment,
		Config: SystemConfigInfo{
			DatabaseHost:        h.config.Database.Host,
			DatabaseName:        h.config.Database.Database,
			MaxConnections:      h.config.Database.MaxConnections,
			IdempotencyTTL:      h.config.Idempotency.RetentionPeriod,
			MaxBatchSize:        h.config.Performance.MaxBatchSize,
			SupportedCurrencies: h.config.Currencies.Supported,
		},
		Runtime: RuntimeInfo{
			NumGoroutines: runtime.NumGoroutine(),
			NumCPU:        runtime.NumCPU(),
			MemoryStats: MemoryStats{
				Alloc:      memStats.Alloc,
				TotalAlloc: memStats.TotalAlloc,
				Sys:        memStats.Sys,
				NumGC:      memStats.NumGC,
			},
		},
		Timestamp: time.Now(),
	}

	c.JSON(http.StatusOK, info)
}

// checkDatabase checks database connectivity and pool status
func (h *HealthHandler) checkDatabase(ctx context.Context) Check {
	if h.db == nil {
		return Check{
			Status:  "healthy",
			Message: "Database check skipped (no pool configured)",
		}
	}

	checkCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()

	if err := h.db.Ping(checkCtx); err != nil {
		h.logger.Warn("database health check failed", zap.Error(err))
		return Check{
			Status:  "unhealthy",
			Message: "Database connection failed: " + err.Error(),
		}
	}

	if h.db.AcquiredConnections() >= int32(h.config.Database.MaxConnections) {
		return Check{
			Status:  "degraded",
			Message: "Database connection pool exhausted",
		}
	}

	return Check{
		Status: "healthy",
	}
}

// checkMemory checks memory usage
func (h *HealthHandler) checkMemory() Check {
	var memStats runtime.MemStats
	runtime.ReadMemStats(&memStats)

	// Convert to MB for easier reading
	allocMB := memStats.Alloc / 1024 / 1024
	sysMB := memStats.Sys / 1024 / 1024

	// Simple heuristic: warn if allocated memory is over 1GB
	if allocMB > 1024 {
		return Check{
			Status:  "degraded",
			Message: "High memory usage detected",
		}
	}

	// Warn if system memory is over 2GB
	if sysMB > 2048 {
		return Check{
			Status:  "degraded",
			Message: "High system memory usage detected",
		}
	}

	return Check{
		Status: "healthy",
	}
}

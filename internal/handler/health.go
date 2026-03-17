package handler

import (
	"net/http"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/redis/go-redis/v9"
)

type HealthHandler struct {
	pool      *pgxpool.Pool
	redis     *redis.Client
	startTime time.Time
}

func NewHealthHandler(pool *pgxpool.Pool, redisClient *redis.Client) *HealthHandler {
	return &HealthHandler{
		pool:      pool,
		redis:     redisClient,
		startTime: time.Now(),
	}
}

func (h *HealthHandler) HandleHealth(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	pgStatus := "connected"
	redisStatus := "connected"
	status := "ok"
	httpStatus := http.StatusOK

	if err := h.pool.Ping(ctx); err != nil {
		pgStatus = "disconnected"
		status = "error"
		httpStatus = http.StatusServiceUnavailable
	}

	if err := h.redis.Ping(ctx).Err(); err != nil {
		redisStatus = "disconnected"
		status = "error"
		httpStatus = http.StatusServiceUnavailable
	}

	var accountsActive int
	if pgStatus == "connected" {
		_ = h.pool.QueryRow(ctx, "SELECT COUNT(*) FROM accounts WHERE is_active = true").Scan(&accountsActive)
	}

	uptime := int(time.Since(h.startTime).Seconds())

	writeJSON(w, httpStatus, map[string]any{
		"status":          status,
		"postgres":        pgStatus,
		"redis":           redisStatus,
		"accounts_active": accountsActive,
		"uptime_seconds":  uptime,
	})
}

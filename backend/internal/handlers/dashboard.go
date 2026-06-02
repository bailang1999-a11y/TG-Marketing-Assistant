package handlers

import (
	"net/http"
	"runtime"
	"time"

	"codex3/backend/internal/models"
	"codex3/backend/pkg/utils"

	"github.com/gin-gonic/gin"
)

const dashboardCacheTTL = 30 * time.Second // 优化：5s -> 30s，减少聚合查询频率

type dashboardCacheEntry struct {
	expiresAt time.Time
	payload   gin.H
}

func (s *Server) Dashboard(c *gin.Context) {
	ctx := c.Request.Context()
	now := time.Now()
	cacheKey := s.tenantID(c).String()
	if cached, ok := s.dashboardCache.Load(cacheKey); ok {
		entry, _ := cached.(dashboardCacheEntry)
		if now.Before(entry.expiresAt) {
			c.Header("X-Codex3-Cache", "dashboard-hit")
			utils.OK(c, entry.payload)
			return
		}
		s.dashboardCache.Delete(cacheKey)
	}
	todayStart := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, now.Location())
	weekStart := todayStart.AddDate(0, 0, -6)

	var terminalTotal, terminalOnline, taskActive, queueBacklog int64
	var todayNotify, totalNotify, todayFailed, totalFailed int64
	var todayHits, totalHits, tasksLastHour int64

	if err := s.db.WithContext(ctx).Model(&models.Terminal{}).Count(&terminalTotal).Error; err != nil {
		utils.Fail(c, http.StatusInternalServerError, "读取仪表盘失败")
		return
	}
	_ = s.db.WithContext(ctx).Model(&models.Terminal{}).Where("status = ?", "online").Count(&terminalOnline).Error
	_ = s.db.WithContext(ctx).Model(&models.Task{}).Where("status IN ?", []string{"queued", "running", "retrying"}).Count(&taskActive).Error
	_ = s.db.WithContext(ctx).Model(&models.Task{}).Where("status = ?", "queued").Count(&queueBacklog).Error
	_ = s.db.WithContext(ctx).Model(&models.Terminal{}).Select("COALESCE(SUM(today_success),0)").Scan(&todayNotify).Error
	_ = s.db.WithContext(ctx).Model(&models.Terminal{}).Select("COALESCE(SUM(total_success),0)").Scan(&totalNotify).Error
	_ = s.db.WithContext(ctx).Model(&models.Terminal{}).Select("COALESCE(SUM(today_failed),0)").Scan(&todayFailed).Error
	_ = s.db.WithContext(ctx).Model(&models.Terminal{}).Select("COALESCE(SUM(total_failed),0)").Scan(&totalFailed).Error
	_ = s.db.WithContext(ctx).Model(&models.Target{}).Where("updated_at >= ?", todayStart).Select("COALESCE(SUM(notification_count),0)").Scan(&todayHits).Error
	_ = s.db.WithContext(ctx).Model(&models.Target{}).Select("COALESCE(SUM(notification_count),0)").Scan(&totalHits).Error
	_ = s.db.WithContext(ctx).Model(&models.Task{}).Where("created_at >= ?", now.Add(-time.Hour)).Count(&tasksLastHour).Error

	var latest []models.Task
	_ = s.db.WithContext(ctx).Order("updated_at desc").Limit(8).Find(&latest).Error

	var notifyRows []struct {
		Day   time.Time `gorm:"column:day"`
		Total int64     `gorm:"column:total"`
	}
	var failedRows []struct {
		Day   time.Time `gorm:"column:day"`
		Total int64     `gorm:"column:total"`
	}
	var terminalRows []struct {
		Day   time.Time `gorm:"column:day"`
		Total int64     `gorm:"column:total"`
	}

	_ = s.db.WithContext(ctx).
		Model(&models.Task{}).
		Select("DATE(created_at) AS day, COUNT(*) AS total").
		Where("type IN ?", []string{"event_outreach", "workflow_execution"}).
		Where("created_at >= ?", weekStart).
		Group("DATE(created_at)").
		Order("DATE(created_at) ASC").
		Scan(&notifyRows).Error
	_ = s.db.WithContext(ctx).
		Model(&models.Task{}).
		Select("DATE(updated_at) AS day, COUNT(*) AS total").
		Where("status IN ?", []string{"failed", "error", "stopped"}).
		Where("updated_at >= ?", weekStart).
		Group("DATE(updated_at)").
		Order("DATE(updated_at) ASC").
		Scan(&failedRows).Error
	_ = s.db.WithContext(ctx).
		Model(&models.Terminal{}).
		Select("DATE(last_online_at) AS day, COUNT(*) AS total").
		Where("last_online_at IS NOT NULL").
		Where("last_online_at >= ?", weekStart).
		Group("DATE(last_online_at)").
		Order("DATE(last_online_at) ASC").
		Scan(&terminalRows).Error

	notifyMap := make(map[string]int64, len(notifyRows))
	failedMap := make(map[string]int64, len(failedRows))
	terminalMap := make(map[string]int64, len(terminalRows))
	for _, row := range notifyRows {
		notifyMap[row.Day.Format("2006-01-02")] = row.Total
	}
	for _, row := range failedRows {
		failedMap[row.Day.Format("2006-01-02")] = row.Total
	}
	for _, row := range terminalRows {
		terminalMap[row.Day.Format("2006-01-02")] = row.Total
	}

	var mem runtime.MemStats
	runtime.ReadMemStats(&mem)

	trend := make([]gin.H, 0, 7)
	for i := 6; i >= 0; i-- {
		dayTime := todayStart.AddDate(0, 0, -i)
		dayKey := dayTime.Format("2006-01-02")
		trend = append(trend, gin.H{
			"day":       dayTime.Format("01-02"),
			"notify":    notifyMap[dayKey],
			"failed":    failedMap[dayKey],
			"terminals": terminalMap[dayKey],
		})
	}

	payload := gin.H{
		"stats": gin.H{
			"today_notify":    todayNotify,
			"total_notify":    totalNotify,
			"today_failed":    todayFailed,
			"total_failed":    totalFailed,
			"online_terminal": terminalOnline,
			"total_terminal":  terminalTotal,
			"today_hits":      todayHits,
			"total_hits":      totalHits,
		},
		"resources": gin.H{
			"memory_mb":       mem.Alloc / 1024 / 1024,
			"goroutines":      runtime.NumGoroutine(),
			"queue_backlog":   queueBacklog,
			"ws_connections":  s.wsConnections.Load(),
			"active_task":     taskActive,
			"tasks_last_hour": tasksLastHour,
		},
		"trend":        trend,
		"latest_tasks": s.enrichTasks(ctx, latest),
	}
	s.dashboardCache.Store(cacheKey, dashboardCacheEntry{
		expiresAt: now.Add(dashboardCacheTTL),
		payload:   payload,
	})
	c.Header("X-Codex3-Cache", "dashboard-miss")
	utils.OK(c, payload)
}

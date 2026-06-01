package handlers

import (
	"bufio"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"sync/atomic"
	"syscall"
	"time"

	"codex3/backend/internal/models"
	"codex3/backend/pkg/telegram_client"
	"codex3/backend/pkg/utils"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"gorm.io/datatypes"
	"gorm.io/gorm"
)

type scrmListenerRuntime struct {
	key                 string
	task                models.Task
	rule                models.SCRMKeywordRule
	terminals           []models.Terminal
	targets             []models.Target
	ownerUserID         *uuid.UUID
	startedAt           time.Time
	cancel              context.CancelFunc
	activeWorkers       atomic.Int32
	matchCount          atomic.Int64
	coveredTargetCount  atomic.Int64
	lastRefreshCovered  atomic.Int64
	lastEventAtUnix     atomic.Int64
	lastHeartbeatAtUnix atomic.Int64
	lastSilenceLogUnix  atomic.Int64
	lastCoverageRefresh atomic.Int64
	stopping            atomic.Bool
	mu                  sync.Mutex
	pushSubscriber      *models.BotSubscriber
}

const scrmListenerCoverageRefreshInterval = 2 * time.Minute

func scrmListenerRuntimeKey(tenantID uuid.UUID, subscriberID uuid.UUID) string {
	if subscriberID == uuid.Nil {
		return tenantID.String()
	}
	return tenantID.String() + ":bot:" + subscriberID.String()
}

func scrmListenerRuntimeKeyForSubscriber(tenantID uuid.UUID, subscriber *models.BotSubscriber) string {
	if subscriber == nil {
		return scrmListenerRuntimeKey(tenantID, uuid.Nil)
	}
	return scrmListenerRuntimeKey(tenantID, subscriber.ID)
}

type scrmListenerStatusResponse struct {
	Running          bool     `json:"running"`
	TaskID           string   `json:"task_id,omitempty"`
	RuleID           string   `json:"rule_id,omitempty"`
	StartedAt        string   `json:"started_at,omitempty"`
	TargetCount      int      `json:"target_count"`
	CoveredTargets   int      `json:"covered_target_count"`
	UncoveredTargets int      `json:"uncovered_target_count"`
	TerminalCount    int      `json:"terminal_count"`
	MatchCount       int64    `json:"match_count"`
	LastEventAt      string   `json:"last_event_at,omitempty"`
	LastHeartbeatAt  string   `json:"last_heartbeat_at,omitempty"`
	HealthStatus     string   `json:"health_status"`
	HealthText       string   `json:"health_text"`
	SilenceSeconds   int64    `json:"silence_seconds"`
	StrikeEnabled    bool     `json:"strike_enabled"`
	MonitorTerminal  []string `json:"monitor_terminal_labels,omitempty"`
}

type scrmListenTarget struct {
	ID         string `json:"id"`
	Identifier string `json:"identifier"`
	Name       string `json:"name"`
	Type       string `json:"type"`
}

type scrmListenEvent struct {
	Type           string `json:"type"`
	Reason         string `json:"reason"`
	Terminal       string `json:"terminal"`
	Source         string `json:"source"`
	SelfSent       bool   `json:"self_sent"`
	ResolvedCount  int    `json:"resolved_count"`
	TargetID       string `json:"target_id"`
	SourceChatID   string `json:"source_chat_id"`
	SourceChatName string `json:"source_chat_name"`
	MessageID      string `json:"message_id"`
	SenderIsBot    bool   `json:"sender_is_bot"`
	SenderType     string `json:"sender_type"`
	UserNickname   string `json:"user_nickname"`
	UserAccount    string `json:"user_account"`
	TriggerWord    string `json:"trigger_word"`
	TriggerMessage string `json:"trigger_message"`
	HitAt          string `json:"hit_at"`
}

func listenerMessagePreview(value string) string {
	value = strings.TrimSpace(value)
	if value == "" {
		return "[无文本内容]"
	}
	value = strings.ReplaceAll(value, "\n", " ")
	runes := []rune(value)
	if len(runes) > 60 {
		return string(runes[:60]) + "..."
	}
	return value
}

func listenerMessageOriginLabel(selfSent bool) string {
	if selfSent {
		return "自身发出"
	}
	return "外部"
}

func scrmListenEventFromRealtimeUser(event scrmListenEvent) bool {
	if event.SelfSent || event.SenderIsBot {
		return false
	}
	senderType := strings.ToLower(strings.TrimSpace(event.SenderType))
	if senderType != "" && senderType != "user" {
		return false
	}
	account := strings.ToLower(strings.TrimSpace(event.UserAccount))
	nickname := strings.ToLower(strings.TrimSpace(event.UserNickname))
	if strings.HasSuffix(strings.TrimPrefix(account, "@"), "bot") {
		return false
	}
	if strings.Contains(nickname, "bot") || strings.Contains(nickname, "机器人") {
		return false
	}
	return strings.TrimSpace(event.UserAccount) != "" || strings.TrimSpace(event.UserNickname) != ""
}

func (s *Server) GetSCRMListenerStatus(c *gin.Context) {
	tenantKey := scrmListenerRuntimeKey(s.tenantID(c), uuid.Nil)

	s.listenerMu.Lock()
	runtime := s.listeners[tenantKey]
	s.listenerMu.Unlock()

	if runtime == nil {
		s.markStaleSCRMListenerTasksStopped(c.Request.Context(), s.tenantID(c))
		utils.OK(c, scrmListenerStatusResponse{Running: false, HealthStatus: "stopped", HealthText: "监听未启动"})
		return
	}

	lastEventAt := runtime.lastEventAtUnix.Load()
	lastHeartbeatAt := runtime.lastHeartbeatAtUnix.Load()
	monitorLabels := make([]string, 0, len(runtime.terminals))
	for _, terminal := range runtime.terminals {
		monitorLabels = append(monitorLabels, listenerTerminalLabel(terminal))
	}
	healthStatus, healthText, silenceSeconds := s.scrmListenerHealth(runtime, lastEventAt, lastHeartbeatAt)
	coveredTargets := int(runtime.coveredTargetCount.Load())
	if coveredTargets < 0 {
		coveredTargets = 0
	}
	if coveredTargets > len(runtime.targets) {
		coveredTargets = len(runtime.targets)
	}

	response := scrmListenerStatusResponse{
		Running:          !runtime.stopping.Load(),
		TaskID:           runtime.task.ID.String(),
		RuleID:           runtime.rule.ID.String(),
		StartedAt:        runtime.startedAt.Format(time.RFC3339),
		TargetCount:      len(runtime.targets),
		CoveredTargets:   coveredTargets,
		UncoveredTargets: maxInt(len(runtime.targets)-coveredTargets, 0),
		TerminalCount:    len(runtime.terminals),
		MatchCount:       runtime.matchCount.Load(),
		HealthStatus:     healthStatus,
		HealthText:       healthText,
		SilenceSeconds:   silenceSeconds,
		StrikeEnabled:    runtime.rule.StrikeEnabled,
		MonitorTerminal:  monitorLabels,
	}
	if lastEventAt > 0 {
		response.LastEventAt = time.Unix(lastEventAt, 0).Format(time.RFC3339)
	}
	if lastHeartbeatAt > 0 {
		response.LastHeartbeatAt = time.Unix(lastHeartbeatAt, 0).Format(time.RFC3339)
	}
	utils.OK(c, response)
}

func (s *Server) scrmListenerHealth(runtime *scrmListenerRuntime, lastEventAt int64, lastHeartbeatAt int64) (string, string, int64) {
	if runtime == nil {
		return "stopped", "监听未运行", 0
	}
	if runtime.activeWorkers.Load() <= 0 {
		if runtime.coveredTargetCount.Load() <= 0 && len(runtime.targets) > 0 {
			return "waiting_coverage", "等待监听号加入目标群，加入后会自动开始实时监听", 0
		}
		return "stopped", "监听未运行", 0
	}
	now := time.Now().Unix()
	if lastHeartbeatAt == 0 {
		return "starting", "监听进程启动中，等待首次心跳", 0
	}
	if now-lastHeartbeatAt > 90 {
		return "stale", "监听心跳中断，请查看任务日志", now - lastHeartbeatAt
	}
	lastActivity := lastEventAt
	if lastActivity == 0 {
		lastActivity = runtime.startedAt.Unix()
	}
	silenceSeconds := now - lastActivity
	settings := s.readSystemSettings(context.Background(), runtime.task.TenantID)
	threshold := settings.ListenerHealth.SilenceAlertMinutes
	if threshold <= 0 {
		threshold = 15
	}
	if silenceSeconds >= int64(threshold*60) {
		return "silent", fmt.Sprintf("监听进程有心跳，但已 %d 分钟未收到群消息", silenceSeconds/60), silenceSeconds
	}
	if lastEventAt == 0 {
		return "idle", "监听进程有心跳，暂未收到群消息", silenceSeconds
	}
	return "healthy", "监听进程有心跳，消息接收正常", silenceSeconds
}

func (s *Server) markStaleSCRMListenerTasksStopped(ctx context.Context, tenantID uuid.UUID) {
	now := time.Now()
	_ = s.db.WithContext(ctx).
		Model(&models.Task{}).
		Where("tenant_id = ? AND type = ? AND status = ?", tenantID, "scrm_listener", "running").
		Updates(map[string]any{"status": "stopped", "progress": 100, "updated_at": now}).Error
	_ = s.db.WithContext(ctx).
		Model(&models.SCRMKeywordRule{}).
		Where("tenant_id = ? AND status = ?", tenantID, "running").
		Updates(map[string]any{"status": "paused", "updated_at": now}).Error
}

func (s *Server) StartSCRMListener(c *gin.Context) {
	s.startSCRMListener(c, uuid.Nil)
}

func (s *Server) StartSCRMListenerRule(c *gin.Context) {
	ruleID, err := uuid.Parse(c.Param("id"))
	if err != nil {
		utils.Fail(c, 400, "监听任务 ID 无效")
		return
	}
	s.startSCRMListener(c, ruleID)
}

func (s *Server) startSCRMListener(c *gin.Context, ruleID uuid.UUID) {
	ctx := c.Request.Context()
	tenantID := s.tenantID(c)

	var rule models.SCRMKeywordRule
	var err error
	if ruleID == uuid.Nil {
		rule, err = s.loadActiveSCRMRule(ctx, tenantID)
	} else {
		rule, err = s.loadSCRMRuleByIDForRequest(c, ruleID)
		tenantID = rule.TenantID
	}
	if err != nil {
		utils.Fail(c, httpStatusForSCRMError(err), err.Error())
		return
	}

	task, err := s.startSCRMListenerRuntime(ctx, tenantID, rule, s.userIDPtr(c), nil)
	if err != nil {
		utils.Fail(c, httpStatusForSCRMError(err), err.Error())
		return
	}

	utils.OK(c, gin.H{
		"task":   task,
		"status": "running",
	})
}

func (s *Server) startSCRMListenerRuntime(ctx context.Context, tenantID uuid.UUID, rule models.SCRMKeywordRule, createdBy *uuid.UUID, pushSubscriber *models.BotSubscriber) (models.Task, error) {
	rule.ListenGroupID = nil
	rule.StrikeGroupID = nil
	rule.MonitorGroupID = nil
	rule.MonitorTerminalIDs = datatypes.JSON([]byte("[]"))
	rule.StrikeEnabled = false

	targets, err := s.loadSCRMListenerTargets(ctx, tenantID, rule.ListenGroupID)
	if err != nil {
		return models.Task{}, err
	}
	terminals, err := s.loadSCRMMonitorTerminals(ctx, tenantID, rule.MonitorTerminalIDs, rule.MonitorGroupID)
	if err != nil {
		return models.Task{}, err
	}
	terminals = s.sortTerminalsByTargetCoverage(ctx, uuid.Nil, accountJoinKindListener, terminals, targets)

	task, err := s.createSCRMListenerTask(ctx, tenantID, rule, targets, terminals, createdBy, pushSubscriber)
	if err != nil {
		return models.Task{}, errors.New("创建监听任务失败")
	}

	tenantKey := scrmListenerRuntimeKeyForSubscriber(tenantID, pushSubscriber)
	s.listenerMu.Lock()
	if existing := s.listeners[tenantKey]; existing != nil {
		existing.stopping.Store(true)
		existing.cancel()
	}
	runCtx, cancel := context.WithCancel(context.Background())
	ownerUserID := createdBy
	if rule.OwnerUserID != nil {
		ownerUserID = rule.OwnerUserID
	}
	if pushSubscriber != nil && pushSubscriber.UserID != nil {
		ownerUserID = pushSubscriber.UserID
	}
	runtime := &scrmListenerRuntime{
		key:            tenantKey,
		task:           task,
		rule:           rule,
		terminals:      terminals,
		targets:        targets,
		ownerUserID:    ownerUserID,
		startedAt:      time.Now(),
		cancel:         cancel,
		pushSubscriber: pushSubscriber,
	}
	runtime.coveredTargetCount.Store(s.countCoveredListenerTargets(ctx, targets))
	runtime.lastRefreshCovered.Store(runtime.coveredTargetCount.Load())
	if runtime.coveredTargetCount.Load() > 0 {
		runtime.lastCoverageRefresh.Store(time.Now().Unix())
	}
	s.listeners[tenantKey] = runtime
	s.listenerMu.Unlock()

	_ = s.db.WithContext(ctx).Model(&models.SCRMKeywordRule{}).
		Where("tenant_id = ? AND id = ?", tenantID, rule.ID).
		Updates(map[string]any{"status": "running", "updated_at": time.Now()}).Error
	s.updateTaskState(ctx, task.ID, "running", 5, nil)
	coveredTargets := int(runtime.coveredTargetCount.Load())
	s.logTaskBackground(ctx, task, "INFO", "start", fmt.Sprintf("开始监听：监听组 %d 个目标，已覆盖 %d 个，待覆盖 %d 个，监听号 %d 个，匹配模式 %s，出击 %t", len(targets), coveredTargets, maxInt(len(targets)-coveredTargets, 0), len(terminals), rule.MatchMode, rule.StrikeEnabled))
	go s.monitorSCRMListenerHealth(runCtx, runtime)
	if pushSubscriber == nil {
		s.ensureSCRMListenerCoverageTask(ctx, tenantID, task, createdBy)
	}

	if coveredTargets == 0 {
		s.logTaskBackground(ctx, task, "INFO", "coverage_wait", "当前还没有监听号覆盖目标群，已启动自动补覆盖任务，首批目标加入后会自动刷新实时监听。")
		return task, nil
	}
	for _, terminal := range terminals {
		runtime.activeWorkers.Add(1)
		go s.runSCRMListenerWorker(runCtx, tenantID, runtime, terminal)
	}
	return task, nil
}

func (s *Server) restartSCRMListenerRuntimeFromCoverage(ctx context.Context, runtime *scrmListenerRuntime) {
	if runtime == nil || runtime.stopping.Load() || runtime.pushSubscriber != nil {
		return
	}
	runtime.stopping.Store(true)
	runtime.cancel()
	if _, err := s.startSCRMListenerRuntime(ctx, runtime.task.TenantID, runtime.rule, runtime.ownerUserID, nil); err != nil {
		runtime.stopping.Store(false)
		s.logTaskBackground(context.Background(), runtime.task, "ERROR", "coverage_refresh", "监听覆盖刷新失败："+err.Error())
		return
	}
	s.updateTaskState(context.Background(), runtime.task.ID, "stopped", 100, s.listenerSummary(runtime))
	s.logTaskBackground(context.Background(), runtime.task, "INFO", "coverage_refresh", "监听目标覆盖已增长，已自动刷新实时监听进程。")
}

func (s *Server) ensureSCRMListenerCoverageTask(ctx context.Context, tenantID uuid.UUID, listenerTask models.Task, createdBy *uuid.UUID) {
	if s.hasRunningListenerCoverageTask(ctx) {
		s.logTaskBackground(ctx, listenerTask, "INFO", "coverage_task", "监听号自动补覆盖任务已在运行，本次复用现有任务。")
		return
	}
	var accounts []models.ListenerAccount
	if err := s.db.WithContext(ctx).Where("tenant_id = ?", uuid.Nil).Order("created_at asc").Find(&accounts).Error; err != nil || len(accounts) == 0 {
		s.logTaskBackground(ctx, listenerTask, "WARN", "coverage_task", "无法启动监听号自动补覆盖：没有可用监听号。")
		return
	}
	var targets []models.ListenerTarget
	if err := s.db.WithContext(ctx).Where("tenant_id = ?", uuid.Nil).Order("created_at asc").Find(&targets).Error; err != nil || len(targets) == 0 {
		s.logTaskBackground(ctx, listenerTask, "WARN", "coverage_task", "无法启动监听号自动补覆盖：没有监听目标。")
		return
	}
	coveredTargets := int(s.countCoveredListenerTargets(ctx, listenerTargetsAsTargets(targets)))
	uncoveredTargets := maxInt(len(targets)-coveredTargets, 0)
	if uncoveredTargets == 0 {
		s.logTaskBackground(ctx, listenerTask, "INFO", "coverage_task", "监听矩阵目标已经全部有监听号覆盖。")
		return
	}
	req := normalizeListenerJoinTargetsRequest(listenerJoinTargetsRequest{
		AccountScope:    "all",
		TargetScope:     "all",
		DailyLimit:      5,
		IntervalMinutes: 30,
		MaxJoins:        uncoveredTargets,
		PreferUncovered: true,
	})
	payload, _ := json.Marshal(req)
	task := models.Task{
		ID:        uuid.New(),
		TenantID:  uuid.Nil,
		Name:      "监听号自动补覆盖",
		Type:      "listener_join_targets",
		Status:    "queued",
		Progress:  0,
		Payload:   datatypes.JSON(payload),
		CreatedBy: createdBy,
	}
	if err := s.db.WithContext(ctx).Create(&task).Error; err != nil {
		s.logTaskBackground(ctx, listenerTask, "WARN", "coverage_task", "创建监听号自动补覆盖任务失败："+err.Error())
		return
	}
	_ = s.createTaskLog(ctx, task, "INFO", "created", fmt.Sprintf("监听启动后自动补覆盖：%d 个监听号，%d 个监听目标，已覆盖 %d 个，待覆盖 %d 个，每号每日 %d 个，间隔 %d 分钟", len(accounts), len(targets), coveredTargets, uncoveredTargets, req.DailyLimit, req.IntervalMinutes), "", "")
	s.logTaskBackground(ctx, listenerTask, "INFO", "coverage_task", "已启动监听号自动补覆盖任务："+task.ID.String())
	if !s.enqueueTask(ctx, task, "run") {
		go s.runListenerJoinTargetsTask(context.Background(), task, accounts, targets, req)
	}
}

func (s *Server) hasRunningListenerCoverageTask(ctx context.Context) bool {
	var count int64
	_ = s.db.WithContext(ctx).Model(&models.Task{}).
		Where("tenant_id = ? AND type = ? AND status IN ?", uuid.Nil, "listener_join_targets", []string{"queued", "running"}).
		Count(&count).Error
	return count > 0
}

func (s *Server) PauseSCRMListenerRule(c *gin.Context) {
	ruleID, err := uuid.Parse(c.Param("id"))
	if err != nil {
		utils.Fail(c, 400, "监听任务 ID 无效")
		return
	}

	rule, err := s.loadSCRMRuleByIDForRequest(c, ruleID)
	if err != nil {
		utils.Fail(c, httpStatusForSCRMError(err), err.Error())
		return
	}
	tenantID := rule.TenantID
	tenantKey := scrmListenerRuntimeKey(tenantID, uuid.Nil)

	s.listenerMu.Lock()
	runtime := s.listeners[tenantKey]
	if runtime != nil && runtime.rule.ID == ruleID {
		runtime.stopping.Store(true)
		runtime.cancel()
		delete(s.listeners, tenantKey)
	}
	s.listenerMu.Unlock()

	if runtime != nil && runtime.rule.ID == ruleID {
		s.updateTaskState(c.Request.Context(), runtime.task.ID, "paused", 1, nil)
		s.logTaskBackground(c.Request.Context(), runtime.task, "INFO", "pause", "监听任务已暂停")
	}

	if err := s.db.WithContext(c.Request.Context()).Model(&models.SCRMKeywordRule{}).
		Where("tenant_id = ? AND id = ?", tenantID, ruleID).
		Updates(map[string]any{"status": "paused", "updated_at": time.Now()}).Error; err != nil {
		utils.Fail(c, 500, "暂停监听任务失败")
		return
	}
	utils.OK(c, gin.H{"status": "paused", "rule_id": ruleID.String()})
}

func (s *Server) StopSCRMListener(c *gin.Context) {
	tenantKey := scrmListenerRuntimeKey(s.tenantID(c), uuid.Nil)

	s.listenerMu.Lock()
	runtime := s.listeners[tenantKey]
	if runtime != nil {
		runtime.stopping.Store(true)
		runtime.cancel()
		delete(s.listeners, tenantKey)
	}
	s.listenerMu.Unlock()

	if runtime == nil {
		utils.OK(c, gin.H{"status": "stopped"})
		return
	}

	s.updateTaskState(c.Request.Context(), runtime.task.ID, "stopped", 100, nil)
	_ = s.db.WithContext(c.Request.Context()).Model(&models.SCRMKeywordRule{}).
		Where("tenant_id = ? AND id = ?", s.tenantID(c), runtime.rule.ID).
		Updates(map[string]any{"status": "stopped", "updated_at": time.Now()}).Error
	s.logTaskBackground(c.Request.Context(), runtime.task, "INFO", "stop", "监听任务已手动停止")
	utils.OK(c, gin.H{"status": "stopped", "task_id": runtime.task.ID.String()})
}

func (s *Server) cleanupSCRMListenerProcesses() {
	scriptPath, err := filepath.Abs(runtimeResolvePath(s.cfg.TelegramListenScript))
	if err != nil || strings.TrimSpace(scriptPath) == "" {
		return
	}
	if _, statErr := os.Stat(scriptPath); statErr != nil {
		return
	}
	_ = exec.Command("pkill", "-f", scriptPath).Run()
}

func (s *Server) loadActiveSCRMRule(ctx context.Context, tenantID uuid.UUID) (models.SCRMKeywordRule, error) {
	var rule models.SCRMKeywordRule
	if err := s.db.WithContext(ctx).
		Where("tenant_id = ? AND status = ?", tenantID, "active").
		Order("updated_at desc").
		First(&rule).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return models.SCRMKeywordRule{}, errors.New("请先保存监听规则，再启动监听")
		}
		return models.SCRMKeywordRule{}, err
	}
	return rule, nil
}

func (s *Server) loadSCRMRuleByID(ctx context.Context, tenantID uuid.UUID, ruleID uuid.UUID) (models.SCRMKeywordRule, error) {
	var rule models.SCRMKeywordRule
	if err := s.db.WithContext(ctx).
		Where("tenant_id = ? AND id = ?", tenantID, ruleID).
		First(&rule).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return models.SCRMKeywordRule{}, errors.New("未找到对应监听任务")
		}
		return models.SCRMKeywordRule{}, err
	}
	return rule, nil
}

func (s *Server) loadSCRMRuleByIDForRequest(c *gin.Context, ruleID uuid.UUID) (models.SCRMKeywordRule, error) {
	var rule models.SCRMKeywordRule
	query := s.db.WithContext(c.Request.Context()).Where("id = ?", ruleID)
	query = s.applySCRMOwnerScope(c, query)
	if err := query.First(&rule).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return models.SCRMKeywordRule{}, errors.New("未找到对应监听任务")
		}
		return models.SCRMKeywordRule{}, err
	}
	return rule, nil
}

func (s *Server) loadSCRMListenerTargets(ctx context.Context, tenantID uuid.UUID, groupID *uuid.UUID) ([]models.Target, error) {
	var listenerTargets []models.ListenerTarget
	listenerQuery := s.db.WithContext(ctx).Where("tenant_id = ?", uuid.Nil).Order("created_at asc")
	if groupID != nil {
		listenerQuery = listenerQuery.Where("group_id = ?", *groupID)
	}
	if err := listenerQuery.Find(&listenerTargets).Error; err != nil {
		return nil, fmt.Errorf("读取监听目标失败：%w", err)
	}
	if len(listenerTargets) > 0 {
		targets := make([]models.Target, 0, len(listenerTargets))
		for _, target := range listenerTargets {
			if !isJoinableTargetType(target.Type) {
				continue
			}
			targets = append(targets, models.Target{
				ID:         target.ID,
				TenantID:   tenantID,
				GroupID:    target.GroupID,
				Identifier: target.Identifier,
				Name:       target.Name,
				Type:       target.Type,
				Size:       target.Size,
				CreatedAt:  target.CreatedAt,
				UpdatedAt:  target.UpdatedAt,
			})
		}
		if len(targets) == 0 {
			return nil, errors.New("管理员监听目标里没有可监听的公开群组或邀请链接")
		}
		return targets, nil
	}

	var targets []models.Target
	query := s.db.WithContext(ctx).Where("tenant_id = ?", tenantID).Order("created_at asc")
	if groupID != nil {
		query = s.applyTargetGroupFilter(ctx, query, tenantID, *groupID)
	}
	if err := query.Find(&targets).Error; err != nil {
		return nil, fmt.Errorf("读取监听目标失败：%w", err)
	}
	if len(targets) == 0 {
		return nil, errors.New("监听组下没有可监听的目标")
	}
	filtered := make([]models.Target, 0, len(targets))
	for _, target := range targets {
		if isJoinableTargetType(target.Type) {
			filtered = append(filtered, target)
		}
	}
	if len(filtered) == 0 {
		return nil, errors.New("当前监听组里没有可监听的群组、频道或邀请链接")
	}
	return filtered, nil
}

func (s *Server) loadSCRMMonitorTerminals(ctx context.Context, tenantID uuid.UUID, rawIDs datatypes.JSON, monitorGroupID *uuid.UUID) ([]models.Terminal, error) {
	var selectedIDs []string
	if len(rawIDs) > 0 {
		_ = json.Unmarshal(rawIDs, &selectedIDs)
	}

	listenerQuery := s.db.WithContext(ctx).Where("tenant_id = ?", uuid.Nil).Order("created_at asc")
	if monitorGroupID != nil {
		listenerQuery = listenerQuery.Where("group_id = ?", *monitorGroupID)
	} else if len(selectedIDs) > 0 {
		listenerQuery = listenerQuery.Where("id IN ?", selectedIDs)
	}
	var listenerAccounts []models.ListenerAccount
	if err := listenerQuery.Find(&listenerAccounts).Error; err != nil {
		return nil, fmt.Errorf("读取监听账号失败：%w", err)
	}
	listenerReady := make([]models.Terminal, 0, len(listenerAccounts))
	for _, account := range listenerAccounts {
		if !listenerAccountReadyForJoin(account) {
			continue
		}
		listenerReady = append(listenerReady, listenerAccountAsTerminal(account, tenantID))
	}
	if len(listenerReady) > 0 {
		return listenerReady, nil
	}
	if monitorGroupID != nil {
		return nil, errors.New("监听号组里没有可用监听号")
	}
	return nil, errors.New("监听矩阵里没有可用监听号")
}

func (s *Server) createSCRMListenerTask(ctx context.Context, tenantID uuid.UUID, rule models.SCRMKeywordRule, targets []models.Target, terminals []models.Terminal, createdBy *uuid.UUID, pushSubscriber *models.BotSubscriber) (models.Task, error) {
	summary, _ := json.Marshal(gin.H{
		"target_count":   len(targets),
		"terminal_count": len(terminals),
		"match_mode":     rule.MatchMode,
		"strike_enabled": rule.StrikeEnabled,
	})
	payload := datatypes.JSON([]byte("{}"))
	if pushSubscriber != nil {
		rawPayload, _ := json.Marshal(gin.H{
			"bot_subscriber_id": pushSubscriber.ID.String(),
			"bot_push_chat_id":  botSubscriberPushChatID(*pushSubscriber),
		})
		payload = datatypes.JSON(rawPayload)
	}
	task := models.Task{
		ID:              uuid.New(),
		TenantID:        tenantID,
		Name:            firstNonEmpty(rule.Name, "SCRM 监听任务"),
		Type:            "scrm_listener",
		TerminalGroupID: rule.StrikeGroupID,
		TargetGroupID:   rule.ListenGroupID,
		Status:          "queued",
		Progress:        0,
		Payload:         payload,
		Summary:         datatypes.JSON(summary),
		CreatedBy:       createdBy,
	}
	if err := s.db.WithContext(ctx).Create(&task).Error; err != nil {
		return models.Task{}, err
	}
	return task, nil
}

func (s *Server) runSCRMListenerWorker(ctx context.Context, tenantID uuid.UUID, runtime *scrmListenerRuntime, terminal models.Terminal) {
	defer func() {
		remaining := runtime.activeWorkers.Add(-1)
		if remaining > 0 {
			return
		}
		if runtime.stopping.Load() {
			return
		}
		s.listenerMu.Lock()
		if current := s.listeners[runtime.key]; current == runtime {
			delete(s.listeners, runtime.key)
		}
		s.listenerMu.Unlock()
		s.updateTaskState(context.Background(), runtime.task.ID, "failed", 100, nil)
		s.logTaskBackground(context.Background(), runtime.task, "ERROR", "worker_exit", "所有监听进程都已退出，监听已停止")
	}()

	keywordList, _ := extractKeywordList(runtime.rule.Keywords)
	listenTargets := s.prepareSCRMListenerWorkerTargets(ctx, tenantID, runtime, terminal)
	if len(listenTargets) == 0 {
		s.logTaskBackground(context.Background(), runtime.task, "ERROR", "listener_targets", fmt.Sprintf("监听号 %s 当前没有可挂载目标", listenerTerminalLabel(terminal)))
		return
	}
	targetPayload := make([]scrmListenTarget, 0, len(runtime.targets))
	for _, target := range listenTargets {
		targetPayload = append(targetPayload, scrmListenTarget{
			ID:         target.ID.String(),
			Identifier: target.Identifier,
			Name:       target.Name,
			Type:       target.Type,
		})
	}

	targetsJSON, _ := json.Marshal(targetPayload)
	keywordsJSON, _ := json.Marshal(keywordList)

	pythonPath, _ := filepath.Abs(runtimeResolvePath(s.cfg.TelegramSyncPython))
	scriptPath, _ := filepath.Abs(runtimeResolvePath(s.cfg.TelegramListenScript))
	terminalFilePath, terminalPathErr := filepath.Abs(runtimeResolvePath(terminal.FilePath))
	if _, err := os.Stat(pythonPath); err != nil {
		s.logTaskBackground(context.Background(), runtime.task, "ERROR", "adapter", "监听执行器不可用："+err.Error())
		return
	}
	if _, err := os.Stat(scriptPath); err != nil {
		s.logTaskBackground(context.Background(), runtime.task, "ERROR", "script", "监听脚本不存在："+err.Error())
		return
	}
	if terminalPathErr != nil {
		s.logTaskBackground(context.Background(), runtime.task, "ERROR", "terminal_path", fmt.Sprintf("监听号 %s 会话路径解析失败：%v", listenerTerminalLabel(terminal), terminalPathErr))
		return
	}
	if _, err := os.Stat(terminalFilePath); err != nil {
		s.logTaskBackground(context.Background(), runtime.task, "ERROR", "terminal_path", fmt.Sprintf("监听号 %s 会话文件不存在：%v", listenerTerminalLabel(terminal), err))
		return
	}

	executionFilePath, cleanup, copyErr := telegram_client.PrepareSessionExecutionPath(terminalFilePath, terminal.AccessType)
	if copyErr != nil {
		s.logTaskBackground(context.Background(), runtime.task, "ERROR", "terminal_copy", fmt.Sprintf("监听号 %s 会话副本准备失败：%v", listenerTerminalLabel(terminal), copyErr))
		return
	}
	defer cleanup()

	cmd := exec.CommandContext(ctx, pythonPath,
		scriptPath,
		"--file", executionFilePath,
		"--access-type", terminal.AccessType,
		"--targets-json", string(targetsJSON),
		"--keywords-json", string(keywordsJSON),
		"--match-mode", runtime.rule.MatchMode,
		"--terminal-label", listenerTerminalLabel(terminal),
	)
	if proxyConfig := s.listenerAccountProxyConfigByID(ctx, uuid.Nil, terminal.ID); proxyConfig.Enabled() {
		cmd.Args = telegram_client.AppendProxyArgs(cmd.Args, proxyConfig)
	}
	cmd.Dir = filepath.Dir(scriptPath)
	cmd.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		s.logTaskBackground(context.Background(), runtime.task, "ERROR", "stdout", "监听输出管道创建失败："+err.Error())
		return
	}
	stderr, err := cmd.StderrPipe()
	if err != nil {
		s.logTaskBackground(context.Background(), runtime.task, "ERROR", "stderr", "监听错误管道创建失败："+err.Error())
		return
	}
	if err := cmd.Start(); err != nil {
		s.logTaskBackground(context.Background(), runtime.task, "ERROR", "start_worker", "监听进程启动失败："+err.Error())
		return
	}
	if cmd.Process != nil {
		go func(pid int) {
			<-ctx.Done()
			_ = syscall.Kill(-pid, syscall.SIGKILL)
			_ = cmd.Process.Kill()
		}(cmd.Process.Pid)
	}

	go s.consumeSCRMListenerStream(ctx, runtime, terminal, stdout)
	go s.consumeSCRMListenerErrors(ctx, runtime, terminal, stderr)

	if err := cmd.Wait(); err != nil && !runtime.stopping.Load() && ctx.Err() == nil {
		s.logTaskBackground(context.Background(), runtime.task, "ERROR", "worker_wait", fmt.Sprintf("监听号 %s 已退出：%v", listenerTerminalLabel(terminal), err))
	}
}

func (s *Server) prepareSCRMListenerWorkerTargets(ctx context.Context, tenantID uuid.UUID, runtime *scrmListenerRuntime, terminal models.Terminal) []models.Target {
	sortedTargets := s.sortTargetsByJoinCoverage(ctx, uuid.Nil, accountJoinKindListener, runtime.targets)
	readyTargets := make([]models.Target, 0, len(sortedTargets))
	for _, target := range sortedTargets {
		if !isJoinableTargetType(target.Type) {
			continue
		}
		if s.accountTargetAlreadyJoined(ctx, uuid.Nil, accountJoinKindListener, terminal.ID, target) {
			readyTargets = append(readyTargets, target)
			continue
		}
	}
	return readyTargets
}

func (s *Server) consumeSCRMListenerStream(ctx context.Context, runtime *scrmListenerRuntime, terminal models.Terminal, stream io.ReadCloser) {
	defer stream.Close()
	scanner := bufio.NewScanner(stream)
	scanner.Buffer(make([]byte, 0, 4096), 1024*1024)

	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" {
			continue
		}

		var event scrmListenEvent
		if err := json.Unmarshal([]byte(line), &event); err != nil {
			s.logTaskBackground(context.Background(), runtime.task, "WARN", "listener_parse", fmt.Sprintf("监听输出无法解析：%s", line))
			continue
		}

		switch event.Type {
		case "ready":
			runtime.lastHeartbeatAtUnix.Store(time.Now().Unix())
			s.logTaskBackground(context.Background(), runtime.task, "INFO", "ready", fmt.Sprintf("监听号 %s 已就绪，成功挂载 %d 个目标", listenerTerminalLabel(terminal), event.ResolvedCount))
		case "heartbeat":
			runtime.lastHeartbeatAtUnix.Store(time.Now().Unix())
		case "message":
			if !scrmListenEventFromRealtimeUser(event) {
				continue
			}
			runtime.lastEventAtUnix.Store(time.Now().Unix())
			runtime.lastHeartbeatAtUnix.Store(time.Now().Unix())
			s.markListenerAccountMessageSeen(context.Background(), terminal.ID)
			senderLabel := firstNonEmpty(event.UserAccount, event.UserNickname)
			sourceLabel := firstNonEmpty(event.SourceChatName, event.SourceChatID)
			if strings.TrimSpace(event.TriggerWord) != "" {
				s.logTaskBackground(
					context.Background(),
					runtime.task,
					"INFO",
					"message_received",
					fmt.Sprintf(
						"监听号 %s 收到%s消息，来源 %s，发送者 %s，命中关键词“%s”，内容：%s",
						listenerTerminalLabel(terminal),
						listenerMessageOriginLabel(event.SelfSent),
						sourceLabel,
						senderLabel,
						event.TriggerWord,
						listenerMessagePreview(event.TriggerMessage),
					),
				)
			} else {
				s.logTaskBackground(
					context.Background(),
					runtime.task,
					"INFO",
					"message_received",
					fmt.Sprintf(
						"监听号 %s 收到%s消息但未命中，来源 %s，发送者 %s，内容：%s",
						listenerTerminalLabel(terminal),
						listenerMessageOriginLabel(event.SelfSent),
						sourceLabel,
						senderLabel,
						listenerMessagePreview(event.TriggerMessage),
					),
				)
			}
		case "warning":
			s.logTaskBackground(context.Background(), runtime.task, "WARN", "warning", fmt.Sprintf("监听号 %s：%s", listenerTerminalLabel(terminal), event.Reason))
		case "match":
			if !scrmListenEventFromRealtimeUser(event) {
				s.logTaskBackground(context.Background(), runtime.task, "INFO", "match_skip", fmt.Sprintf("监听号 %s 跳过非真人实时消息：发送者 %s，内容：%s", listenerTerminalLabel(terminal), firstNonEmpty(event.UserAccount, event.UserNickname, "未知发送者"), listenerMessagePreview(event.TriggerMessage)))
				continue
			}
			runtime.matchCount.Add(1)
			runtime.lastEventAtUnix.Store(time.Now().Unix())
			runtime.lastHeartbeatAtUnix.Store(time.Now().Unix())
			s.markListenerAccountMessageSeen(context.Background(), terminal.ID)
			s.persistSCRMLeadEvent(ctx, runtime, terminal, event)
		case "error":
			s.logTaskBackground(context.Background(), runtime.task, "ERROR", "listener_error", fmt.Sprintf("监听号 %s：%s", listenerTerminalLabel(terminal), event.Reason))
		}
	}
}

func (s *Server) monitorSCRMListenerHealth(ctx context.Context, runtime *scrmListenerRuntime) {
	ticker := time.NewTicker(30 * time.Second)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			if runtime == nil || runtime.stopping.Load() {
				return
			}
			status, text, _ := s.scrmListenerHealth(runtime, runtime.lastEventAtUnix.Load(), runtime.lastHeartbeatAtUnix.Load())
			coveredTargets := s.countCoveredListenerTargets(context.Background(), runtime.targets)
			previousCovered := runtime.coveredTargetCount.Load()
			if coveredTargets > previousCovered {
				runtime.coveredTargetCount.Store(coveredTargets)
				s.logTaskBackground(context.Background(), runtime.task, "INFO", "coverage_update", fmt.Sprintf("监听目标覆盖已增长：%d/%d", coveredTargets, len(runtime.targets)))
			}
			if runtime.pushSubscriber == nil && coveredTargets > runtime.lastRefreshCovered.Load() {
				now := time.Now().Unix()
				lastRefresh := runtime.lastCoverageRefresh.Load()
				if now-lastRefresh >= int64(scrmListenerCoverageRefreshInterval.Seconds()) && runtime.lastCoverageRefresh.CompareAndSwap(lastRefresh, now) {
					go s.restartSCRMListenerRuntimeFromCoverage(context.Background(), runtime)
				}
			}
			s.updateTaskState(context.Background(), runtime.task.ID, "running", 100, s.listenerSummary(runtime))
			if status == "silent" || status == "stale" {
				now := time.Now().Unix()
				lastLogged := runtime.lastSilenceLogUnix.Load()
				if now-lastLogged >= 300 && runtime.lastSilenceLogUnix.CompareAndSwap(lastLogged, now) {
					s.logTaskBackground(context.Background(), runtime.task, "WARN", "listener_health", text)
				}
			}
		}
	}
}

func (s *Server) markListenerAccountMessageSeen(ctx context.Context, accountID uuid.UUID) {
	if accountID == uuid.Nil {
		return
	}
	now := time.Now()
	_ = s.db.WithContext(ctx).Model(&models.ListenerAccount{}).
		Where("id = ?", accountID).
		Updates(map[string]any{"last_message_at": now, "updated_at": now}).Error
}

func (s *Server) consumeSCRMListenerErrors(ctx context.Context, runtime *scrmListenerRuntime, terminal models.Terminal, stream io.ReadCloser) {
	defer stream.Close()
	scanner := bufio.NewScanner(stream)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" {
			continue
		}
		s.logTaskBackground(context.Background(), runtime.task, "WARN", "listener_stderr", fmt.Sprintf("监听号 %s：%s", listenerTerminalLabel(terminal), line))
	}
}

func (s *Server) persistSCRMLeadEvent(ctx context.Context, runtime *scrmListenerRuntime, terminal models.Terminal, event scrmListenEvent) {
	if !scrmListenEventFromRealtimeUser(event) {
		s.logTaskBackground(context.Background(), runtime.task, "INFO", "match_skip", fmt.Sprintf("监听号 %s 跳过非真人实时消息：发送者 %s，内容：%s", listenerTerminalLabel(terminal), firstNonEmpty(event.UserAccount, event.UserNickname, "未知发送者"), listenerMessagePreview(event.TriggerMessage)))
		return
	}

	targetID, err := uuid.Parse(event.TargetID)
	if err != nil {
		s.logTaskBackground(context.Background(), runtime.task, "WARN", "match_skip", "命中事件缺少有效目标 ID")
		return
	}

	hitAt := time.Now()
	if parsed, parseErr := time.Parse(time.RFC3339, strings.TrimSpace(event.HitAt)); parseErr == nil {
		hitAt = parsed
	}
	if hitAt.Before(runtime.startedAt.Add(-2 * time.Second)) {
		s.logTaskBackground(context.Background(), runtime.task, "INFO", "history_skip", fmt.Sprintf("监听号 %s 跳过任务启动前的历史消息：%s", listenerTerminalLabel(terminal), listenerMessagePreview(event.TriggerMessage)))
		return
	}
	if s.scrmTaskUserBlacklisted(context.Background(), runtime.task.TenantID, runtime.task.ID, event.UserAccount, event.UserNickname, targetID) {
		label := firstNonEmpty(strings.TrimSpace(event.UserAccount), strings.TrimSpace(event.UserNickname), targetID.String())
		s.logTaskBackground(context.Background(), runtime.task, "INFO", "match_skip", fmt.Sprintf("用户 %s 已在当前监听任务黑名单，跳过线索推送", label))
		return
	}

	status := "captured"
	if runtime.rule.StrikeEnabled {
		status = "pending_strike"
	}

	lead := models.SCRMLead{
		ID:             uuid.New(),
		TenantID:       runtime.task.TenantID,
		OwnerUserID:    runtime.ownerUserID,
		SourceTaskID:   &runtime.task.ID,
		TargetID:       targetID,
		UserNickname:   strings.TrimSpace(event.UserNickname),
		UserAccount:    strings.TrimSpace(event.UserAccount),
		SourceChatID:   strings.TrimSpace(event.SourceChatID),
		SourceChatName: strings.TrimSpace(event.SourceChatName),
		TriggerWord:    strings.TrimSpace(event.TriggerWord),
		TriggerMessage: strings.TrimSpace(event.TriggerMessage),
		MessageID:      strings.TrimSpace(event.MessageID),
		Status:         status,
		HitAt:          &hitAt,
		CreatedAt:      time.Now(),
		UpdatedAt:      time.Now(),
	}

	runtime.mu.Lock()
	defer runtime.mu.Unlock()
	if s.scrmLeadEventAlreadyCaptured(context.Background(), runtime.task.TenantID, targetID, event, hitAt) {
		return
	}
	if err := s.db.WithContext(context.Background()).Create(&lead).Error; err != nil {
		s.logTaskBackground(context.Background(), runtime.task, "ERROR", "persist_lead", fmt.Sprintf("保存命中线索失败：%v", err))
		return
	}

	s.logTaskBackground(context.Background(), runtime.task, "INFO", "match", fmt.Sprintf("监听号 %s 命中关键词“%s”，来源 %s，用户 %s", listenerTerminalLabel(terminal), lead.TriggerWord, lead.SourceChatName, firstNonEmpty(lead.UserAccount, lead.UserNickname)))
	if runtime.rule.PushToBot {
		go s.pushSCRMLeadToBot(context.Background(), runtime.task, lead)
	}
	s.updateTaskState(context.Background(), runtime.task.ID, "running", 100, s.listenerSummary(runtime))
}

func (s *Server) scrmLeadEventAlreadyCaptured(ctx context.Context, tenantID uuid.UUID, targetID uuid.UUID, event scrmListenEvent, hitAt time.Time) bool {
	sourceChatID := strings.TrimSpace(event.SourceChatID)
	messageID := strings.TrimSpace(event.MessageID)
	query := s.db.WithContext(ctx).Model(&models.SCRMLead{}).Where("tenant_id = ?", tenantID)
	if sourceChatID != "" && messageID != "" {
		query = query.Where("source_chat_id = ? AND message_id = ?", sourceChatID, messageID)
	} else {
		cutoffStart := hitAt.Add(-3 * time.Second)
		cutoffEnd := hitAt.Add(3 * time.Second)
		query = query.Where(
			"target_id = ? AND source_chat_name = ? AND trigger_message = ? AND COALESCE(hit_at, created_at) BETWEEN ? AND ?",
			targetID,
			strings.TrimSpace(event.SourceChatName),
			strings.TrimSpace(event.TriggerMessage),
			cutoffStart,
			cutoffEnd,
		)
	}
	var existing int64
	_ = query.Count(&existing).Error
	return existing > 0
}

func (s *Server) listenerSummary(runtime *scrmListenerRuntime) datatypes.JSON {
	coveredTargets := int(runtime.coveredTargetCount.Load())
	if coveredTargets < 0 {
		coveredTargets = 0
	}
	if coveredTargets > len(runtime.targets) {
		coveredTargets = len(runtime.targets)
	}
	summary, _ := json.Marshal(gin.H{
		"target_count":           len(runtime.targets),
		"covered_target_count":   coveredTargets,
		"uncovered_target_count": maxInt(len(runtime.targets)-coveredTargets, 0),
		"terminal_count":         len(runtime.terminals),
		"match_count":            runtime.matchCount.Load(),
		"started_at":             runtime.startedAt.Format(time.RFC3339),
		"last_event_at":          unixTimeString(runtime.lastEventAtUnix.Load()),
		"last_heartbeat_at":      unixTimeString(runtime.lastHeartbeatAtUnix.Load()),
		"strike_enabled":         runtime.rule.StrikeEnabled,
	})
	return datatypes.JSON(summary)
}

func unixTimeString(value int64) string {
	if value <= 0 {
		return ""
	}
	return time.Unix(value, 0).Format(time.RFC3339)
}

func extractKeywordList(raw datatypes.JSON) ([]string, error) {
	var payload struct {
		List []string `json:"list"`
	}
	if len(raw) == 0 {
		return nil, nil
	}
	if err := json.Unmarshal(raw, &payload); err != nil {
		return nil, err
	}
	return payload.List, nil
}

func listenerTerminalLabel(terminal models.Terminal) string {
	if strings.TrimSpace(terminal.Phone) != "" {
		return terminal.Phone
	}
	if strings.TrimSpace(terminal.Nickname) != "" {
		return terminal.Nickname
	}
	return terminal.ID.String()
}

func runtimeResolvePath(path string) string {
	if filepath.IsAbs(path) {
		return path
	}
	return path
}

func httpStatusForSCRMError(err error) int {
	switch {
	case err == nil:
		return 200
	case strings.Contains(err.Error(), "请先保存"), strings.Contains(err.Error(), "没有"), strings.Contains(err.Error(), "不存在"):
		return 400
	default:
		return 500
	}
}

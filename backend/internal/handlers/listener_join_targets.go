package handlers

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"time"

	"codex3/backend/internal/models"
	"codex3/backend/pkg/telegram_client"
	"codex3/backend/pkg/utils"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"gorm.io/datatypes"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

type listenerJoinTargetsRequest struct {
	AccountScope      string `json:"account_scope"`
	AccountGroupID    string `json:"account_group_id"`
	TargetScope       string `json:"target_scope"`
	TargetGroupID     string `json:"target_group_id"`
	DailyLimit        int    `json:"daily_limit"`
	IntervalMinutes   int    `json:"interval_minutes"`
	MaxJoins          int    `json:"max_joins"`
	PreferUncovered   bool   `json:"prefer_uncovered"`
	SkipAlreadyJoined bool   `json:"skip_already_joined"`
}

type listenerJoinTargetsSummary struct {
	TaskID        string                 `json:"task_id"`
	Accounts      int                    `json:"accounts"`
	Targets       int                    `json:"targets"`
	Total         int                    `json:"total"`
	Success       int                    `json:"success"`
	Failed        int                    `json:"failed"`
	Skipped       int                    `json:"skipped"`
	Pending       int                    `json:"pending"`
	DailyLimit    int                    `json:"daily_limit"`
	Interval      int                    `json:"interval_minutes"`
	Waiting       bool                   `json:"waiting"`
	WaitingReason string                 `json:"waiting_reason,omitempty"`
	WaitingUntil  string                 `json:"waiting_until,omitempty"`
	CurrentTarget string                 `json:"current_target,omitempty"`
	TopSkipReason string                 `json:"top_skip_reason,omitempty"`
	SkipReasons   map[string]int         `json:"skip_reasons,omitempty"`
	Items         []joinTargetsResultRow `json:"items"`
}

func (s *Server) CreateListenerJoinTargetsTask(c *gin.Context) {
	var req listenerJoinTargetsRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		utils.Fail(c, http.StatusBadRequest, "无效的加群任务参数")
		return
	}
	req = normalizeListenerJoinTargetsRequest(req)
	accounts, err := s.resolveListenerJoinAccounts(c, req)
	if err != nil {
		utils.Fail(c, http.StatusBadRequest, err.Error())
		return
	}
	targets, err := s.resolveListenerJoinTargets(c, req)
	if err != nil {
		utils.Fail(c, http.StatusBadRequest, err.Error())
		return
	}
	payload, _ := json.Marshal(req)
	task := models.Task{
		ID:        uuid.New(),
		TenantID:  uuid.Nil,
		Name:      "监听号自动加群",
		Type:      "listener_join_targets",
		Status:    "queued",
		Progress:  0,
		Payload:   datatypes.JSON(payload),
		CreatedBy: s.userIDPtr(c),
	}
	if err := s.db.WithContext(c.Request.Context()).Create(&task).Error; err != nil {
		utils.Fail(c, http.StatusInternalServerError, "创建监听号加群任务失败")
		return
	}
	_ = s.createTaskLog(c.Request.Context(), task, "INFO", "created", fmt.Sprintf("监听号自动加群任务已创建：%d 个监听号，%d 个监听群，每号每日 %d 个，间隔 %d 分钟", len(accounts), len(targets), req.DailyLimit, req.IntervalMinutes), "", "")
	if !s.enqueueTask(c.Request.Context(), task, "run") {
		go s.runListenerJoinTargetsTask(context.Background(), task, accounts, targets, req)
	}
	utils.Created(c, gin.H{"task": task})
}

func normalizeListenerJoinTargetsRequest(req listenerJoinTargetsRequest) listenerJoinTargetsRequest {
	req.AccountScope = strings.ToLower(strings.TrimSpace(firstNonEmpty(req.AccountScope, "all")))
	req.TargetScope = strings.ToLower(strings.TrimSpace(firstNonEmpty(req.TargetScope, "all")))
	if req.DailyLimit <= 0 {
		req.DailyLimit = 5
	}
	if req.DailyLimit > 200 {
		req.DailyLimit = 200
	}
	if req.IntervalMinutes <= 0 {
		req.IntervalMinutes = 30
	}
	if req.IntervalMinutes > 24*60 {
		req.IntervalMinutes = 24 * 60
	}
	if req.MaxJoins < 0 {
		req.MaxJoins = 0
	}
	if !req.PreferUncovered {
		req.PreferUncovered = true
	}
	req.SkipAlreadyJoined = true
	return req
}

func (s *Server) resolveListenerJoinAccounts(c *gin.Context, req listenerJoinTargetsRequest) ([]models.ListenerAccount, error) {
	query := s.db.WithContext(c.Request.Context()).Where("tenant_id = ?", uuid.Nil).Order("created_at asc")
	switch req.AccountScope {
	case "all":
	case "group":
		groupID, err := uuid.Parse(strings.TrimSpace(req.AccountGroupID))
		if err != nil {
			return nil, fmt.Errorf("监听号分组无效")
		}
		query = query.Where("group_id = ?", groupID)
	default:
		return nil, fmt.Errorf("请选择全部监听号或指定监听号分组")
	}
	var accounts []models.ListenerAccount
	if err := query.Find(&accounts).Error; err != nil {
		return nil, err
	}
	if len(accounts) == 0 {
		return nil, fmt.Errorf("当前范围内没有监听号")
	}
	return accounts, nil
}

func (s *Server) resolveListenerJoinTargets(c *gin.Context, req listenerJoinTargetsRequest) ([]models.ListenerTarget, error) {
	query := s.db.WithContext(c.Request.Context()).Where("tenant_id = ?", uuid.Nil).Order("created_at asc")
	switch req.TargetScope {
	case "all":
	case "group":
		groupID, err := uuid.Parse(strings.TrimSpace(req.TargetGroupID))
		if err != nil {
			return nil, fmt.Errorf("监听群分组无效")
		}
		query = query.Where("group_id = ?", groupID)
	default:
		return nil, fmt.Errorf("请选择全部监听群或指定监听群分组")
	}
	var targets []models.ListenerTarget
	if err := query.Find(&targets).Error; err != nil {
		return nil, err
	}
	if len(targets) == 0 {
		return nil, fmt.Errorf("当前范围内没有监听群")
	}
	return targets, nil
}

func (s *Server) RunListenerJoinTargetsTask(taskID uuid.UUID) {
	ctx := context.Background()
	claimed, release := s.claimTaskRun(ctx, taskID, "listener_join_targets")
	if !claimed {
		return
	}
	defer release()

	var task models.Task
	if err := s.db.WithContext(ctx).Where("id = ? AND type = ?", taskID, "listener_join_targets").First(&task).Error; err != nil {
		return
	}
	var req listenerJoinTargetsRequest
	if err := json.Unmarshal(task.Payload, &req); err != nil {
		s.finishListenerJoinTargetsTask(ctx, task, "failed", listenerJoinTargetsSummary{TaskID: task.ID.String(), Items: []joinTargetsResultRow{}}, "监听号自动加群任务参数解析失败："+err.Error())
		return
	}
	req = normalizeListenerJoinTargetsRequest(req)
	accounts, targets, err := s.loadListenerJoinTargetsTaskSelection(ctx, req)
	if err != nil {
		s.finishListenerJoinTargetsTask(ctx, task, "failed", listenerJoinTargetsSummary{TaskID: task.ID.String(), Items: []joinTargetsResultRow{}}, err.Error())
		return
	}
	s.runListenerJoinTargetsTask(ctx, task, accounts, targets, req)
}

func (s *Server) loadListenerJoinTargetsTaskSelection(ctx context.Context, req listenerJoinTargetsRequest) ([]models.ListenerAccount, []models.ListenerTarget, error) {
	accountQuery := s.db.WithContext(ctx).Where("tenant_id = ?", uuid.Nil).Order("created_at asc")
	if req.AccountScope == "group" {
		groupID, err := uuid.Parse(strings.TrimSpace(req.AccountGroupID))
		if err != nil {
			return nil, nil, fmt.Errorf("监听号分组无效")
		}
		accountQuery = accountQuery.Where("group_id = ?", groupID)
	}
	var accounts []models.ListenerAccount
	if err := accountQuery.Find(&accounts).Error; err != nil {
		return nil, nil, err
	}
	targetQuery := s.db.WithContext(ctx).Where("tenant_id = ?", uuid.Nil).Order("created_at asc")
	if req.TargetScope == "group" {
		groupID, err := uuid.Parse(strings.TrimSpace(req.TargetGroupID))
		if err != nil {
			return nil, nil, fmt.Errorf("监听群分组无效")
		}
		targetQuery = targetQuery.Where("group_id = ?", groupID)
	}
	var targets []models.ListenerTarget
	if err := targetQuery.Find(&targets).Error; err != nil {
		return nil, nil, err
	}
	if len(accounts) == 0 || len(targets) == 0 {
		return nil, nil, fmt.Errorf("监听号或监听群为空，无法执行自动加群")
	}
	return accounts, targets, nil
}

func (s *Server) runListenerJoinTargetsTask(ctx context.Context, task models.Task, accounts []models.ListenerAccount, listenerTargets []models.ListenerTarget, req listenerJoinTargetsRequest) {
	s.updateTaskState(ctx, task.ID, "running", 1, nil)
	targets := listenerTargetsAsTargets(listenerTargets)
	if req.PreferUncovered {
		targets = s.sortTargetsByJoinCoverage(ctx, uuid.Nil, accountJoinKindListener, targets)
	}
	if req.MaxJoins > 0 && req.MaxJoins < len(targets) {
		targets = targets[:req.MaxJoins]
	}
	summary := listenerJoinTargetsSummary{
		TaskID:      task.ID.String(),
		Accounts:    len(accounts),
		Targets:     len(listenerTargets),
		Total:       len(targets),
		Pending:     len(targets),
		DailyLimit:  req.DailyLimit,
		Interval:    req.IntervalMinutes,
		SkipReasons: map[string]int{},
		Items:       []joinTargetsResultRow{},
	}
	_ = s.createTaskLog(ctx, task, "INFO", "start", fmt.Sprintf("开始监听号自动加群：%d 个监听号，%d 个监听群，优先未覆盖目标", len(accounts), len(targets)), "", "")
	joiner := telegram_client.NewJoiner(s.cfg)
	inspector := telegram_client.NewTargetInspector(s.cfg)
	done := 0
targetLoop:
	for _, target := range targets {
		targetRef := targetJoinLabel(target)
		summary.CurrentTarget = targetRef
		summary.Waiting = false
		summary.WaitingReason = ""
		summary.WaitingUntil = ""
		row := joinTargetsResultRow{TargetID: target.ID.String(), Target: targetRef}
		if !isJoinableTargetType(target.Type) {
			row.Status = "skipped"
			row.Reason = joinUnsupportedReason(target.Type)
			summary.Skipped++
			accumulateSkipReason(summary.SkipReasons, row.Reason)
			summary.Items = append(summary.Items, row)
			_ = s.createTaskLog(ctx, task, "WARN", "join_skipped", row.Reason, "", targetRef)
			done++
			s.updateListenerJoinTargetsProgress(ctx, task.ID, done, summary.Total, summary)
			continue
		}
		var accountIndex int
		var account models.ListenerAccount
		for {
			var quotaErr error
			accounts = s.refreshListenerJoinAccounts(ctx, accounts)
			accountIndex, account, quotaErr = s.pickListenerJoinAccount(ctx, accounts, target, req, inspector)
			if quotaErr == nil {
				break
			}
			accounts = s.refreshListenerJoinAccounts(ctx, accounts)
			waitUntil, waitReason := s.nextListenerJoinAccountRetryAt(ctx, accounts, target, req)
			if waitUntil == nil || !waitUntil.After(time.Now()) {
				if isListenerJoinResolutionExhaustedReason(quotaErr.Error()) {
					row.Status = "failed"
					summary.Failed++
					_ = s.createTaskLog(ctx, task, "ERROR", "join_failed", quotaErr.Error(), "", targetRef)
				} else {
					row.Status = "skipped"
					summary.Skipped++
					_ = s.createTaskLog(ctx, task, "WARN", "join_skipped", quotaErr.Error(), "", targetRef)
				}
				row.Reason = quotaErr.Error()
				if row.Status == "skipped" {
					accumulateSkipReason(summary.SkipReasons, row.Reason)
				}
				summary.Items = append(summary.Items, row)
				done++
				s.updateListenerJoinTargetsProgress(ctx, task.ID, done, summary.Total, summary)
				continue targetLoop
			}
			waitUntilText := waitUntil.In(terminalQuotaLocation()).Format("2006-01-02 15:04:05")
			detail := fmt.Sprintf("%s：%s，已加入 %d 个，剩余 %d 个，冷却结束后继续监听群 %s", waitReason, waitUntilText, summary.Success, maxInt(summary.Total-done, 0), targetRef)
			summary.Waiting = true
			summary.WaitingReason = waitReason
			summary.WaitingUntil = waitUntilText
			s.updateListenerJoinTargetsProgress(ctx, task.ID, done, summary.Total, summary)
			_ = s.createTaskLog(ctx, task, "INFO", "quota_wait", detail, "", targetRef)
			timer := time.NewTimer(time.Until(*waitUntil))
			heartbeat := time.NewTicker(time.Minute)
			waitFinished := false
			for !waitFinished {
				select {
				case <-ctx.Done():
					timer.Stop()
					heartbeat.Stop()
					row.Status = "skipped"
					row.Reason = "任务等待监听号冷却时被取消"
					summary.Skipped++
					accumulateSkipReason(summary.SkipReasons, row.Reason)
					summary.Items = append(summary.Items, row)
					_ = s.createTaskLog(ctx, task, "WARN", "join_skipped", row.Reason, "", targetRef)
					done++
					s.updateListenerJoinTargetsProgress(ctx, task.ID, done, summary.Total, summary)
					continue targetLoop
				case <-heartbeat.C:
					_ = s.db.WithContext(ctx).Model(&models.Task{}).Where("id = ? AND status = ?", task.ID, "running").Updates(map[string]any{
						"run_locked_at": time.Now(),
						"updated_at":    time.Now(),
					}).Error
				case <-timer.C:
					waitFinished = true
				}
			}
			heartbeat.Stop()
			summary.Waiting = false
			summary.WaitingReason = ""
			summary.WaitingUntil = ""
		}
		accountRef := listenerAccountJoinLabel(account)
		row.TerminalID = account.ID.String()
		row.Terminal = accountRef
		start := time.Now()
		result, err := joiner.Join(ctx, telegram_client.JoinRequest{
			FilePath:   account.FilePath,
			AccessType: account.AccessType,
			TargetType: target.Type,
			Identifier: target.Identifier,
			Proxy:      s.listenerAccountProxyConfig(ctx, uuid.Nil, account),
		})
		duration := time.Since(start).Milliseconds()
		row.Status = firstNonEmpty(result.Status, "failed")
		row.Reason = result.Reason
		if row.Reason == "" && err != nil {
			row.Reason = err.Error()
		}
		if err == nil && result.OK {
			summary.Success++
			accounts[accountIndex] = account
			s.recordAccountTargetJoin(ctx, uuid.Nil, accountJoinKindListener, account.ID, target, &task.ID)
			_ = s.createTaskLogWithDuration(ctx, task, "INFO", "join_success", firstNonEmpty(result.Reason, "监听号已加入监听群"), accountRef, targetRef, duration)
			s.notifySCRMListenerCoverageChanged(ctx)
		} else {
			summary.Failed++
			_ = s.createTaskLogWithDuration(ctx, task, "ERROR", "join_failed", firstNonEmpty(row.Reason, "监听号加群失败"), accountRef, targetRef, duration)
		}
		summary.Items = append(summary.Items, row)
		done++
		s.updateListenerJoinTargetsProgress(ctx, task.ID, done, summary.Total, summary)
	}
	summary.Waiting = false
	summary.WaitingReason = ""
	summary.WaitingUntil = ""
	summary.CurrentTarget = ""
	status := "success"
	if summary.Success == 0 && (summary.Failed > 0 || summary.Skipped > 0) {
		status = "failed"
	} else if summary.Failed > 0 || summary.Skipped > 0 {
		status = "partial_success"
	}
	summary.TopSkipReason = topSkipReason(summary.SkipReasons)
	s.finishListenerJoinTargetsTask(ctx, task, status, summary, fmt.Sprintf("监听号自动加群完成：成功 %d，失败 %d，跳过 %d", summary.Success, summary.Failed, summary.Skipped))
}

func listenerTargetsAsTargets(items []models.ListenerTarget) []models.Target {
	targets := make([]models.Target, 0, len(items))
	for _, item := range items {
		targets = append(targets, models.Target{
			ID:         item.ID,
			TenantID:   item.TenantID,
			GroupID:    item.GroupID,
			Identifier: item.Identifier,
			Name:       item.Name,
			Type:       item.Type,
			Size:       item.Size,
			CreatedAt:  item.CreatedAt,
			UpdatedAt:  item.UpdatedAt,
		})
	}
	return targets
}

func (s *Server) pickListenerJoinAccount(ctx context.Context, accounts []models.ListenerAccount, target models.Target, req listenerJoinTargetsRequest, inspector telegram_client.TargetInspector) (int, models.ListenerAccount, error) {
	reasons := []string{}
	resolutionFailures := []string{}
	for index, account := range accounts {
		if req.SkipAlreadyJoined && s.accountTargetAlreadyJoined(ctx, uuid.Nil, accountJoinKindListener, account.ID, target) {
			reasons = append(reasons, fmt.Sprintf("%s 已加入该监听群", listenerAccountJoinLabel(account)))
			continue
		}
		if !listenerAccountReadyForJoin(account) {
			reasons = append(reasons, fmt.Sprintf("%s：监听账号当前不可用于加群", listenerAccountJoinLabel(account)))
			continue
		}
		if listenerJoinShouldPreflightResolve(target) {
			result, err := inspector.Inspect(ctx, telegram_client.TargetInspectRequest{
				FilePath:   account.FilePath,
				AccessType: account.AccessType,
				Target:     target.Identifier,
			})
			if err != nil || !result.OK {
				reason := firstNonEmpty(result.Reason, errorString(err), "监听号无法解析该目标")
				if listenerJoinShouldTryAnotherAccount(reason) {
					resolutionFailures = append(resolutionFailures, fmt.Sprintf("%s：%s", listenerAccountJoinLabel(account), reason))
					continue
				}
				reasons = append(reasons, fmt.Sprintf("%s：%s", listenerAccountJoinLabel(account), reason))
				continue
			}
		}
		reserved, err := s.reserveListenerJoinQuotaWithPolicy(ctx, account.ID, req.DailyLimit, req.IntervalMinutes)
		if err != nil {
			reasons = append(reasons, fmt.Sprintf("%s：%s", listenerAccountJoinLabel(account), err.Error()))
			continue
		}
		return index, reserved, nil
	}
	if len(resolutionFailures) > 0 && len(reasons) == 0 {
		return -1, models.ListenerAccount{}, fmt.Errorf("没有可解析该监听群的监听号：%s", strings.Join(resolutionFailures, "；"))
	}
	if len(resolutionFailures) > 0 {
		reasons = append(reasons, resolutionFailures...)
	}
	if len(reasons) > 0 {
		return -1, models.ListenerAccount{}, fmt.Errorf("没有可用监听号：%s", strings.Join(reasons, "；"))
	}
	return -1, models.ListenerAccount{}, fmt.Errorf("没有可用监听号")
}

func listenerJoinShouldPreflightResolve(target models.Target) bool {
	switch strings.ToLower(strings.TrimSpace(target.Type)) {
	case "group", "channel":
		return strings.TrimSpace(target.Identifier) != ""
	default:
		return false
	}
}

func listenerJoinShouldTryAnotherAccount(reason string) bool {
	lowered := strings.ToLower(strings.TrimSpace(reason))
	return strings.Contains(lowered, "no user has") && strings.Contains(lowered, "as username")
}

func isListenerJoinResolutionExhaustedReason(reason string) bool {
	return strings.Contains(strings.TrimSpace(reason), "没有可解析该监听群的监听号")
}

func (s *Server) refreshListenerJoinAccounts(ctx context.Context, accounts []models.ListenerAccount) []models.ListenerAccount {
	if len(accounts) == 0 {
		return accounts
	}
	ids := make([]uuid.UUID, 0, len(accounts))
	for _, account := range accounts {
		ids = append(ids, account.ID)
	}
	var latest []models.ListenerAccount
	if err := s.db.WithContext(ctx).Where("id IN ? AND tenant_id = ?", ids, uuid.Nil).Find(&latest).Error; err != nil {
		return accounts
	}
	byID := make(map[uuid.UUID]models.ListenerAccount, len(latest))
	for _, account := range latest {
		byID[account.ID] = account
	}
	refreshed := make([]models.ListenerAccount, len(accounts))
	copy(refreshed, accounts)
	for index, account := range accounts {
		if item, ok := byID[account.ID]; ok {
			refreshed[index] = item
		}
	}
	return refreshed
}

func (s *Server) nextListenerJoinAccountRetryAt(ctx context.Context, accounts []models.ListenerAccount, target models.Target, req listenerJoinTargetsRequest) (*time.Time, string) {
	now := time.Now()
	var earliest *time.Time
	for _, account := range accounts {
		if req.SkipAlreadyJoined && s.accountTargetAlreadyJoined(ctx, uuid.Nil, accountJoinKindListener, account.ID, target) {
			continue
		}
		if !listenerAccountReadyForJoin(account) {
			continue
		}
		blockedUntil := listenerJoinAccountBlockedUntil(now, account, req)
		if blockedUntil == nil {
			return nil, ""
		}
		if earliest == nil || blockedUntil.Before(*earliest) {
			value := *blockedUntil
			earliest = &value
		}
	}
	if earliest == nil {
		return nil, ""
	}
	return earliest, "可用监听号均在冷却或每日限额窗口中，任务将等待下一次可用时间"
}

func listenerJoinAccountBlockedUntil(now time.Time, account models.ListenerAccount, req listenerJoinTargetsRequest) *time.Time {
	var blockedUntil *time.Time
	pushBlockedUntil := func(value *time.Time) {
		if value == nil || !value.After(now) {
			return
		}
		if blockedUntil == nil || value.After(*blockedUntil) {
			next := *value
			blockedUntil = &next
		}
	}
	pushBlockedUntil(account.JoinCooldownUntil)
	dailyCount, dailyResetAt := resetTerminalQuotaWindow(now, account.JoinDailyCount, account.JoinDailyResetAt, "day")
	if req.DailyLimit > 0 && dailyCount >= req.DailyLimit {
		pushBlockedUntil(dailyResetAt)
	}
	return blockedUntil
}

func (s *Server) reserveListenerJoinQuotaWithPolicy(ctx context.Context, listenerAccountID uuid.UUID, dailyLimit int, intervalMinutes int) (models.ListenerAccount, error) {
	var account models.ListenerAccount
	err := s.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).Where("id = ?", listenerAccountID).First(&account).Error; err != nil {
			return err
		}
		now := time.Now()
		if account.JoinCooldownUntil != nil && account.JoinCooldownUntil.After(now) {
			return fmt.Errorf("监听账号加群冷却中，需等待到 %s", account.JoinCooldownUntil.In(terminalQuotaLocation()).Format("2006-01-02 15:04:05"))
		}
		if !listenerAccountReadyForJoin(account) {
			return fmt.Errorf("监听账号当前不可用于加群")
		}
		dailyCount, dailyResetAt := resetTerminalQuotaWindow(now, account.JoinDailyCount, account.JoinDailyResetAt, "day")
		if dailyLimit > 0 && dailyCount >= dailyLimit {
			return fmt.Errorf("监听账号每日加群限额已达上限（%d）", dailyLimit)
		}
		dailyCount++
		nextCooldown := terminalNextCooldownAt(now, intervalMinutes, 0)
		account.JoinDailyCount = dailyCount
		account.JoinDailyResetAt = dailyResetAt
		account.JoinCooldownUntil = &nextCooldown
		account.LastJoinAt = &now
		return tx.Model(&models.ListenerAccount{}).Where("id = ?", listenerAccountID).Updates(map[string]any{
			"join_daily_limit":    dailyLimit,
			"join_daily_count":    dailyCount,
			"join_daily_reset_at": dailyResetAt,
			"join_cooldown_until": nextCooldown,
			"last_join_at":        now,
			"updated_at":          now,
		}).Error
	})
	return account, err
}

func (s *Server) updateListenerJoinTargetsProgress(ctx context.Context, taskID uuid.UUID, done int, total int, summary listenerJoinTargetsSummary) {
	progress := 100
	if total > 0 {
		progress = 5 + int(float64(done)/float64(total)*90)
		if progress > 95 {
			progress = 95
		}
	}
	summary.Pending = maxInt(total-done, 0)
	payload, _ := json.Marshal(summary)
	_ = s.db.WithContext(ctx).Model(&models.Task{}).Where("id = ?", taskID).Updates(map[string]any{
		"progress": progress,
		"summary":  datatypes.JSON(payload),
	}).Error
}

func (s *Server) finishListenerJoinTargetsTask(ctx context.Context, task models.Task, status string, summary listenerJoinTargetsSummary, detail string) {
	if summary.SkipReasons != nil && len(summary.SkipReasons) == 0 {
		summary.SkipReasons = nil
	}
	payload, _ := json.Marshal(summary)
	s.updateTaskState(ctx, task.ID, status, 100, datatypes.JSON(payload))
	level := "INFO"
	if status == "failed" {
		level = "ERROR"
	} else if status == "partial_success" {
		level = "WARN"
	}
	_ = s.createTaskLog(ctx, task, level, "summary", detail, "", "")
}

func (s *Server) notifySCRMListenerCoverageChanged(ctx context.Context) {
	s.listenerMu.Lock()
	runtimes := make([]*scrmListenerRuntime, 0, len(s.listeners))
	for _, runtime := range s.listeners {
		if runtime != nil {
			runtimes = append(runtimes, runtime)
		}
	}
	s.listenerMu.Unlock()

	for _, runtime := range runtimes {
		if runtime.stopping.Load() || runtime.pushSubscriber != nil {
			continue
		}
		covered := s.countCoveredListenerTargets(ctx, runtime.targets)
		previous := runtime.coveredTargetCount.Load()
		if covered <= previous {
			continue
		}
		runtime.coveredTargetCount.Store(covered)
		s.logTaskBackground(context.Background(), runtime.task, "INFO", "coverage_update", fmt.Sprintf("监听目标覆盖已增长：%d/%d", covered, len(runtime.targets)))
		now := time.Now().Unix()
		lastRefresh := runtime.lastCoverageRefresh.Load()
		if covered > runtime.lastRefreshCovered.Load() && now-lastRefresh >= int64(scrmListenerCoverageRefreshInterval.Seconds()) && runtime.lastCoverageRefresh.CompareAndSwap(lastRefresh, now) {
			go s.restartSCRMListenerRuntimeFromCoverage(context.Background(), runtime)
		}
	}
}

func listenerAccountJoinLabel(account models.ListenerAccount) string {
	if phone := strings.TrimSpace(account.Phone); phone != "" {
		return phone
	}
	if nickname := strings.TrimSpace(account.Nickname); nickname != "" {
		return nickname
	}
	return account.ID.String()
}

package handlers

import (
	"bufio"
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"os"
	"strings"
	"sync"
	"time"

	"codex3/backend/internal/models"
	"codex3/backend/pkg/telegram_client"
	"codex3/backend/pkg/utils"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"golang.org/x/net/proxy"
	"gorm.io/datatypes"
	"gorm.io/gorm"
)

const listenerProxyProbeTarget = "www.gstatic.com:80"
const listenerHTTPProxyProbeURL = "http://www.gstatic.com/generate_204"
const listenerTelegramProbeTarget = "149.154.167.50:443"
const listenerTelegramIPv6ProbeHost = "2001:67c:4e8:f002::a"
const listenerWebTelegramProbeURL = "https://web.telegram.org/k/"

var listenerProxyExitLookupURLs = []string{
	"http://api.ipify.org?format=json",
	"https://api.ipify.org?format=json",
	"http://ip-api.com/json/?fields=status,query,country,countryCode",
	"http://ifconfig.me/ip",
}

func (s *Server) ImportListenerTargets(c *gin.Context) {
	var req struct {
		Content      string `json:"content" binding:"required"`
		GroupID      string `json:"group_id"`
		NewGroupName string `json:"new_group_name"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		utils.Fail(c, http.StatusBadRequest, "请输入监听群链接")
		return
	}
	groupID, groupName, err := s.resolveListenerGroup(c, "listener_target", strings.TrimSpace(req.GroupID), firstNonEmpty(strings.TrimSpace(req.NewGroupName), "监听群"))
	if err != nil {
		utils.Fail(c, http.StatusBadRequest, err.Error())
		return
	}
	summary := listenerImportSummary{GroupID: groupID, GroupName: groupName, Items: []listenerImportResult{}}
	err = s.db.WithContext(c.Request.Context()).Transaction(func(tx *gorm.DB) error {
		for _, raw := range strings.Split(req.Content, "\n") {
			line := cleanTargetImportLine(raw)
			if line == "" {
				summary.Skipped++
				continue
			}
			target, err := parseTargetLine(line)
			if err != nil {
				summary.Failed++
				summary.Items = append(summary.Items, listenerImportResult{Line: line, Status: "failed", Reason: err.Error()})
				continue
			}
			var existing int64
			if err := tx.WithContext(c.Request.Context()).Model(&models.ListenerTarget{}).Where("tenant_id = ? AND identifier = ?", s.tenantID(c), target.Identifier).Count(&existing).Error; err != nil {
				return err
			}
			if existing > 0 {
				summary.Duplicate++
				summary.Items = append(summary.Items, listenerImportResult{Line: line, Identifier: target.Identifier, Status: "duplicate", Reason: "监听群已存在"})
				continue
			}
			item := models.ListenerTarget{ID: uuid.New(), TenantID: s.tenantID(c), GroupID: groupID, Identifier: target.Identifier, Name: target.Name, Type: target.Type, Status: "active"}
			if err := tx.WithContext(c.Request.Context()).Create(&item).Error; err != nil {
				return err
			}
			summary.Success++
			summary.Items = append(summary.Items, listenerImportResult{Line: line, Identifier: target.Identifier, Status: "success"})
		}
		return nil
	})
	if err != nil {
		utils.Fail(c, http.StatusInternalServerError, "导入监听群失败")
		return
	}
	utils.Created(c, summary)
}

func (s *Server) ImportListenerProxies(c *gin.Context) {
	var req struct {
		Content          string `json:"content" binding:"required"`
		DefaultProtocol  string `json:"default_protocol"`
		GroupID          string `json:"group_id"`
		NewGroupName     string `json:"new_group_name"`
		AccountGroupID   string `json:"account_group_id"`
		AssignToAccounts bool   `json:"assign_to_accounts"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		utils.Fail(c, http.StatusBadRequest, "请输入监听代理")
		return
	}
	defaultProtocol := normalizeProxyProtocol(req.DefaultProtocol)
	if defaultProtocol == "" {
		defaultProtocol = "socks5"
	}
	lines := importNonEmptyLines(req.Content)
	task := s.createListenerProxyImportTask(c, len(lines), defaultProtocol, req.AssignToAccounts)
	if task != nil {
		_ = s.createTaskLog(c.Request.Context(), *task, "INFO", "created", fmt.Sprintf("开始导入监听代理：%d 行，默认协议 %s", len(lines), defaultProtocol), "", "")
	}
	groupID, groupName, err := s.resolveListenerGroup(c, "listener_proxy", strings.TrimSpace(req.GroupID), firstNonEmpty(strings.TrimSpace(req.NewGroupName), "监听代理"))
	if err != nil {
		if task != nil {
			s.finishListenerProxyImportTask(c, *task, "failed", 100, fmt.Sprintf("导入失败：%s", err.Error()))
		}
		utils.Fail(c, http.StatusBadRequest, err.Error())
		return
	}
	summary := listenerImportSummary{GroupID: groupID, GroupName: groupName, Items: []listenerImportResult{}}
	err = s.db.WithContext(c.Request.Context()).Transaction(func(tx *gorm.DB) error {
		nextCode, err := s.nextListenerProxyCode(c, tx)
		if err != nil {
			return err
		}
		for index, line := range lines {
			proxy, err := parseProxyLine(line, defaultProtocol)
			if err != nil {
				summary.Failed++
				summary.Items = append(summary.Items, listenerImportResult{Line: line, Status: "failed", Reason: err.Error()})
				if task != nil {
					_ = s.createTaskLog(c.Request.Context(), *task, "ERROR", "import_proxies", fmt.Sprintf("第 %d 行导入失败：%s", index+1, err.Error()), "", "")
				}
				continue
			}
			address := fmt.Sprintf("%s:%d", proxy.IP, proxy.Port)
			var existing models.ListenerProxy
			err = tx.WithContext(c.Request.Context()).Where("tenant_id = ? AND protocol = ? AND ip = ? AND port = ? AND username = ?", s.tenantID(c), proxy.Protocol, proxy.IP, proxy.Port, proxy.Username).First(&existing).Error
			if err == nil {
				if groupID != nil && (existing.GroupID == nil || *existing.GroupID != *groupID) {
					if err := tx.WithContext(c.Request.Context()).Model(&models.ListenerProxy{}).Where("tenant_id = ? AND id = ?", s.tenantID(c), existing.ID).Updates(map[string]any{
						"group_id":   *groupID,
						"updated_at": time.Now(),
					}).Error; err != nil {
						return err
					}
					summary.Success++
					summary.Items = append(summary.Items, listenerImportResult{Line: line, Identifier: address, Status: "success", Reason: "已存在，已移动到所选分组"})
					if task != nil {
						_ = s.createTaskLog(c.Request.Context(), *task, "INFO", "import_proxies", fmt.Sprintf("第 %d 行已存在，已移动到所选分组：%s", index+1, address), "", "")
					}
					continue
				}
				summary.Duplicate++
				summary.Items = append(summary.Items, listenerImportResult{Line: line, Identifier: address, Status: "duplicate", Reason: "监听代理已存在"})
				if task != nil {
					_ = s.createTaskLog(c.Request.Context(), *task, "WARN", "import_proxies", fmt.Sprintf("第 %d 行重复：%s 已存在", index+1, address), "", "")
				}
				continue
			}
			if err != gorm.ErrRecordNotFound {
				return err
			}
			item := models.ListenerProxy{ID: uuid.New(), TenantID: s.tenantID(c), GroupID: groupID, Code: fmt.Sprintf("LP-%06d", nextCode), IP: proxy.IP, Port: proxy.Port, Protocol: proxy.Protocol, Username: proxy.Username, Password: proxy.Password, Country: "未知", Status: "untested"}
			if err := tx.WithContext(c.Request.Context()).Create(&item).Error; err != nil {
				return err
			}
			nextCode++
			summary.Success++
			summary.Items = append(summary.Items, listenerImportResult{Line: line, Identifier: address, Status: "success"})
			if task != nil {
				_ = s.createTaskLog(c.Request.Context(), *task, "INFO", "import_proxies", fmt.Sprintf("第 %d 行导入成功：%s %s", index+1, proxy.Protocol, address), "", "")
			}
		}
		return nil
	})
	if err != nil {
		if task != nil {
			s.finishListenerProxyImportTask(c, *task, "failed", 100, "导入监听代理失败："+err.Error())
		}
		utils.Fail(c, http.StatusInternalServerError, "导入监听代理失败")
		return
	}
	var assign listenerAdminAssignSummary
	assignmentError := ""
	if req.AssignToAccounts {
		assign, err = s.assignListenerProxies(c, groupID, strings.TrimSpace(req.AccountGroupID))
		if err != nil {
			assignmentError = err.Error()
			if task != nil {
				_ = s.createTaskLog(c.Request.Context(), *task, "WARN", "import_proxies", "代理已导入，但自动分配失败："+assignmentError, "", "")
			}
		} else if task != nil {
			_ = s.createTaskLog(c.Request.Context(), *task, "INFO", "import_proxies", fmt.Sprintf("自动分配完成：成功 %d，跳过 %d", assign.Assigned, assign.Skipped), "", "")
		}
	}
	status := listenerProxyImportTaskStatus(summary, assignmentError)
	detail := fmt.Sprintf("监听代理导入完成：成功 %d，重复 %d，失败 %d，跳过 %d", summary.Success, summary.Duplicate, summary.Failed, summary.Skipped)
	if assignmentError != "" {
		detail += "；自动分配失败：" + assignmentError
	}
	if task != nil {
		s.finishListenerProxyImportTask(c, *task, status, 100, detail)
	}
	utils.Created(c, gin.H{"import": summary, "assignment": assign, "assignment_error": assignmentError})
}

func importNonEmptyLines(content string) []string {
	lines := []string{}
	for _, raw := range strings.Split(content, "\n") {
		line := strings.TrimSpace(raw)
		if line != "" {
			lines = append(lines, line)
		}
	}
	return lines
}

func (s *Server) createListenerProxyImportTask(c *gin.Context, lineCount int, defaultProtocol string, assignToAccounts bool) *models.Task {
	payload, _ := json.Marshal(gin.H{
		"line_count":         lineCount,
		"default_protocol":   defaultProtocol,
		"assign_to_accounts": assignToAccounts,
	})
	task := models.Task{
		ID:        uuid.New(),
		TenantID:  s.tenantID(c),
		Name:      "导入代理",
		Type:      "import_proxies",
		Status:    "running",
		Progress:  10,
		Payload:   datatypes.JSON(payload),
		CreatedBy: s.userIDPtr(c),
	}
	if err := s.db.WithContext(c.Request.Context()).Create(&task).Error; err != nil {
		return nil
	}
	return &task
}

func listenerProxyImportTaskStatus(summary listenerImportSummary, assignmentError string) string {
	if summary.Success == 0 && (summary.Failed > 0 || assignmentError != "") {
		return "failed"
	}
	if summary.Failed > 0 || summary.Duplicate > 0 || assignmentError != "" {
		return "partial_success"
	}
	return "success"
}

func (s *Server) finishListenerProxyImportTask(c *gin.Context, task models.Task, status string, progress int, detail string) {
	s.updateTaskState(c.Request.Context(), task.ID, status, progress, nil)
	level := "INFO"
	if status == "failed" {
		level = "ERROR"
	} else if status == "partial_success" {
		level = "WARN"
	}
	_ = s.createTaskLog(c.Request.Context(), task, level, "summary", detail, "", "")
}

type listenerAccountCheckPayload struct {
	GroupID string `json:"group_id"`
}

func (s *Server) CheckListenerAccounts(c *gin.Context) {
	var req struct {
		GroupID string `json:"group_id"`
	}
	_ = c.ShouldBindJSON(&req)

	payload := listenerAccountCheckPayload{GroupID: strings.TrimSpace(req.GroupID)}
	if payload.GroupID != "" {
		if _, err := uuid.Parse(payload.GroupID); err != nil {
			utils.Fail(c, http.StatusBadRequest, "监听号分组无效")
			return
		}
	}
	rawPayload, _ := json.Marshal(payload)
	task := models.Task{
		ID:        uuid.New(),
		TenantID:  s.tenantID(c),
		Name:      "监听账号状态检测",
		Type:      "listener_account_check",
		Status:    "queued",
		Progress:  0,
		Payload:   datatypes.JSON(rawPayload),
		CreatedBy: s.userIDPtr(c),
	}
	if err := s.db.WithContext(c.Request.Context()).Create(&task).Error; err != nil {
		utils.Fail(c, http.StatusInternalServerError, "创建监听账号检测任务失败")
		return
	}
	_ = s.createTaskLog(c.Request.Context(), task, "INFO", "created", "监听账号状态检测任务已创建，等待执行器消费", "", "")
	if !s.enqueueTask(c.Request.Context(), task, "run") {
		go s.RunListenerAccountCheckTask(task.ID)
	}
	utils.Created(c, gin.H{"task": task, "summary": listenerCheckSummary{}})
}

func (s *Server) RunListenerAccountCheckTask(taskID uuid.UUID) {
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Minute)
	defer cancel()
	claimed, release := s.claimTaskRun(ctx, taskID, "listener_account_check")
	if !claimed {
		return
	}
	defer release()
	var task models.Task
	if err := s.db.WithContext(ctx).Where("id = ? AND type = ?", taskID, "listener_account_check").First(&task).Error; err != nil {
		return
	}
	var payload listenerAccountCheckPayload
	if len(task.Payload) > 0 {
		_ = json.Unmarshal(task.Payload, &payload)
	}
	s.runListenerAccountCheckTask(ctx, task, payload)
}

func (s *Server) runListenerAccountCheckTask(ctx context.Context, task models.Task, payload listenerAccountCheckPayload) {
	query := s.db.WithContext(ctx).Where("tenant_id = ?", task.TenantID).Order("created_at desc")
	if strings.TrimSpace(payload.GroupID) != "" {
		groupID, err := uuid.Parse(strings.TrimSpace(payload.GroupID))
		if err != nil {
			s.finishListenerAccountCheckTask(ctx, task, "failed", listenerCheckSummary{}, "监听号分组无效："+err.Error())
			return
		}
		query = query.Where("group_id = ?", groupID)
	}
	var accounts []models.ListenerAccount
	if err := query.Find(&accounts).Error; err != nil {
		s.finishListenerAccountCheckTask(ctx, task, "failed", listenerCheckSummary{}, "读取监听账号失败："+err.Error())
		return
	}

	var targetTotal int64
	_ = s.db.WithContext(ctx).Model(&models.ListenerTarget{}).Where("tenant_id = ?", task.TenantID).Count(&targetTotal).Error
	now := time.Now()
	syncer := telegram_client.NewInspector(s.cfg)
	summary := listenerCheckSummary{Total: len(accounts)}
	exitCache := map[uuid.UUID]proxyExitResult{}
	var exitCacheMu sync.Mutex
	concurrency := s.cfg.ListenerAccountCheckConcurrency
	if concurrency <= 0 {
		concurrency = 4
	}
	if concurrency > len(accounts) && len(accounts) > 0 {
		concurrency = len(accounts)
	}
	s.updateTaskState(ctx, task.ID, "running", 1, nil)
	s.logTaskBackground(ctx, task, "INFO", "start", fmt.Sprintf("开始检测监听账号状态：共 %d 个监听号，并发 %d", len(accounts), concurrency))
	if len(accounts) == 0 {
		s.finishListenerAccountCheckTask(ctx, task, "success", summary, "没有需要检测的监听号")
		return
	}
	var summaryMu sync.Mutex
	var progressMu sync.Mutex
	var completed int
	var firstUpdateErr error
	sem := make(chan struct{}, concurrency)
	var wg sync.WaitGroup
	for index, account := range accounts {
		index := index
		account := account
		wg.Add(1)
		sem <- struct{}{}
		go func() {
			defer wg.Done()
			defer func() { <-sem }()
			startedAt := time.Now()
			status := "normal"
			riskStatus := firstNonEmpty(strings.TrimSpace(account.RiskStatus), "正常")
			reason := "监听号已登记"
			phone := account.Phone
			nickname := account.Nickname
			avatarURL := account.AvatarURL
			lastOnlineAt := account.LastOnlineAt
			joinedTargets := account.JoinedTargets
			accountRef := firstNonEmpty(account.Phone, account.Nickname, account.ID.String())
			s.logTaskBackground(ctx, task, "INFO", "account_check_start", fmt.Sprintf("开始检测监听号 %d/%d：%s", index+1, len(accounts), accountRef))

			if account.FilePath != "" && isStoredTerminalFileReady(account.FilePath) {
				avatarDir, avatarDirErr := prepareTerminalAvatarSyncDir(task.TenantID, account.ID)
				if avatarDirErr != nil {
					reason = firstNonEmpty(reason, "准备头像缓存目录失败")
				}
				syncResult, syncErr := syncer.Sync(ctx, telegram_client.SyncRequest{
					FilePath:   account.FilePath,
					AccessType: telegram_client.NormalizeTelegramAccessType(account.AccessType),
					AvatarDir:  avatarDir,
					Proxy:      s.listenerAccountProxyConfig(ctx, task.TenantID, account),
				})
				if syncErr == nil || strings.TrimSpace(syncResult.Reason) != "" {
					status = listenerNormalizeAccountStatus(syncResult.Status)
					reason = firstNonEmpty(syncResult.Reason, reason)
					phone = firstNonEmpty(syncResult.Phone, phone)
					nickname = firstNonEmpty(syncResult.Nickname, nickname)
					riskStatus = firstNonEmpty(syncResult.RiskStatus, riskStatus)
					if account.ProxyID != nil && listenerAccountProxyFailure(reason) {
						status = "abnormal"
						riskStatus = "代理不可用"
					}
					lastOnlineAt = syncResult.LastOnlineAt
					avatarURL, reason = s.persistTerminalAvatar(task.TenantID, models.Terminal{ID: account.ID}, avatarURL, syncResult, reason)
				} else {
					status = "abnormal"
					riskStatus = "检测失败"
					reason = syncErr.Error()
					if account.ProxyID != nil && listenerAccountProxyFailure(reason) {
						riskStatus = "代理不可用"
					} else if strings.Contains(reason, "资料同步超时") || strings.Contains(strings.ToLower(reason), "deadline exceeded") {
						riskStatus = "检测超时"
					}
				}
				if avatarDir != "" {
					_ = os.RemoveAll(avatarDir)
				}
			} else if strings.TrimSpace(account.FilePath) == "" {
				status = "abnormal"
				riskStatus = "需重新导入"
				reason = "缺少本地会话文件"
			} else if !isStoredTerminalFileReady(account.FilePath) {
				status = "abnormal"
				riskStatus = "需重新导入"
				reason = "本地会话文件不存在"
			} else if strings.TrimSpace(account.Phone) == "" {
				status = "abnormal"
				riskStatus = "缺少手机号"
			}

			if isListenerAccountNormal(status, riskStatus) {
				summaryMu.Lock()
				summary.Normal++
				summaryMu.Unlock()
				joinedTargets = s.countActiveAccountTargetJoins(ctx, uuid.Nil, accountJoinKindListener, account.ID)
			} else if status == "offline" {
				summaryMu.Lock()
				summary.Offline++
				summaryMu.Unlock()
				joinedTargets = s.countActiveAccountTargetJoins(ctx, uuid.Nil, accountJoinKindListener, account.ID)
			} else {
				summaryMu.Lock()
				summary.Abnormal++
				summaryMu.Unlock()
				s.markAccountJoinRecordsUnavailable(ctx, uuid.Nil, accountJoinKindListener, account.ID, reason)
				joinedTargets = s.countActiveAccountTargetJoins(ctx, uuid.Nil, accountJoinKindListener, account.ID)
			}
			if joinedTargets > targetTotal {
				joinedTargets = targetTotal
			}
			normalizedPhone, _, _ := syncTerminalPhoneIdentity(phone, "", "")
			if normalizedPhone == "" {
				normalizedPhone = phone
			}
			exitIP := account.ExitIP
			exitCountry := account.ExitCountry
			exitFlag := account.ExitFlag
			if account.ProxyID != nil {
				exitCacheMu.Lock()
				exit := s.lookupListenerAccountProxyExit(ctx, task.TenantID, *account.ProxyID, exitCache)
				exitCacheMu.Unlock()
				exitIP = exit.IP
				exitCountry = exit.Country
				exitFlag = exit.Flag
			}
			if err := s.db.WithContext(ctx).Model(&models.ListenerAccount{}).Where("tenant_id = ? AND id = ?", task.TenantID, account.ID).Updates(map[string]any{
				"phone":          normalizedPhone,
				"nickname":       nickname,
				"avatar_url":     avatarURL,
				"exit_ip":        exitIP,
				"exit_country":   exitCountry,
				"exit_flag":      exitFlag,
				"status":         status,
				"risk_status":    riskStatus,
				"last_online_at": lastOnlineAt,
				"joined_targets": joinedTargets,
				"updated_at":     now,
			}).Error; err != nil {
				progressMu.Lock()
				if firstUpdateErr == nil {
					firstUpdateErr = err
				}
				progressMu.Unlock()
				s.logTaskBackground(ctx, task, "ERROR", "account_update_failed", fmt.Sprintf("%s -> 更新监听账号状态失败：%s", accountRef, err.Error()))
				return
			}
			level := "INFO"
			if !isListenerAccountNormal(status, riskStatus) && status != "offline" {
				level = "WARN"
			}
			_ = s.createTaskLogWithDuration(ctx, task, level, "check_listener_account", fmt.Sprintf("%s -> %s（%s）", accountRef, firstNonEmpty(riskStatus, status), reason), accountRef, firstNonEmpty(exitIP, exitCountry), time.Since(startedAt).Milliseconds())
			progressMu.Lock()
			completed++
			progress := 1 + int(float64(completed)/float64(len(accounts))*94)
			if progress > 95 {
				progress = 95
			}
			s.updateTaskState(ctx, task.ID, "running", progress, nil)
			progressMu.Unlock()
		}()
	}
	wg.Wait()
	if firstUpdateErr != nil {
		summaryMu.Lock()
		finalSummary := summary
		summaryMu.Unlock()
		s.finishListenerAccountCheckTask(ctx, task, "failed", finalSummary, "更新监听账号状态失败："+firstUpdateErr.Error())
		return
	}
	summaryMu.Lock()
	finalSummary := summary
	summaryMu.Unlock()
	status := "success"
	if finalSummary.Abnormal > 0 {
		status = "partial_success"
	}
	s.finishListenerAccountCheckTask(ctx, task, status, finalSummary, fmt.Sprintf("监听账号状态检测完成：总数 %d，正常 %d，离线 %d，异常 %d，并发 %d", finalSummary.Total, finalSummary.Normal, finalSummary.Offline, finalSummary.Abnormal, concurrency))
}

func (s *Server) finishListenerAccountCheckTask(ctx context.Context, task models.Task, status string, summary listenerCheckSummary, detail string) {
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

func (s *Server) DeleteAbnormalListenerAccounts(c *gin.Context) {
	tenantID := s.tenantID(c)
	var accounts []models.ListenerAccount
	_ = s.db.WithContext(c.Request.Context()).Where(
		"tenant_id = ? AND (status IN ? OR risk_status IN ?)",
		tenantID,
		[]string{"abnormal", "failed", "disabled"},
		[]string{"需重新导入", "资料受限", "检测失败", "缺少手机号", "封禁", "冻结"},
	).Find(&accounts).Error
	result := s.db.WithContext(c.Request.Context()).Where(
		"tenant_id = ? AND (status IN ? OR risk_status IN ?)",
		tenantID,
		[]string{"abnormal", "failed", "disabled"},
		[]string{"需重新导入", "资料受限", "检测失败", "缺少手机号", "封禁", "冻结"},
	).Delete(&models.ListenerAccount{})
	if result.Error != nil {
		utils.Fail(c, http.StatusInternalServerError, "删除异常监听账号失败")
		return
	}
	for _, account := range accounts {
		s.markAccountJoinRecordsUnavailable(c.Request.Context(), uuid.Nil, accountJoinKindListener, account.ID, "异常监听账号已删除")
	}
	utils.OK(c, gin.H{"deleted": result.RowsAffected})
}

func (s *Server) CheckListenerProxies(c *gin.Context) {
	var req struct {
		GroupID string `json:"group_id"`
	}
	_ = c.ShouldBindJSON(&req)
	tenantID := s.tenantID(c)
	payload := listenerProxyCheckPayload{GroupID: strings.TrimSpace(req.GroupID)}
	query := s.db.WithContext(c.Request.Context()).Where("tenant_id = ?", tenantID).Order("created_at desc")
	if payload.GroupID != "" {
		groupID, err := uuid.Parse(payload.GroupID)
		if err != nil {
			utils.Fail(c, http.StatusBadRequest, "代理分组无效")
			return
		}
		query = query.Where("group_id = ?", groupID)
	}
	var proxies []models.ListenerProxy
	if err := query.Find(&proxies).Error; err != nil {
		utils.Fail(c, http.StatusInternalServerError, "读取监听代理失败")
		return
	}

	payloadBytes, _ := json.Marshal(payload)
	task := models.Task{
		ID:        uuid.New(),
		TenantID:  tenantID,
		Name:      "监听代理延迟检测",
		Type:      "listener_proxy_check",
		Status:    "queued",
		Progress:  0,
		Payload:   datatypes.JSON(payloadBytes),
		CreatedBy: s.userIDPtr(c),
	}
	if err := s.db.WithContext(c.Request.Context()).Create(&task).Error; err != nil {
		utils.Fail(c, http.StatusInternalServerError, "创建代理检测任务失败")
		return
	}
	_ = s.createTaskLog(c.Request.Context(), task, "INFO", "created", fmt.Sprintf("代理延迟检测任务已创建：待检测 %d 个代理", len(proxies)), "", "")
	if !s.enqueueTask(c.Request.Context(), task, "run") {
		go s.RunListenerProxyCheckTask(task.ID)
	}
	utils.Created(c, gin.H{"task": task, "summary": listenerProxyCheckSummary{Total: len(proxies)}})
}

type listenerProxyCheckPayload struct {
	GroupID string `json:"group_id,omitempty"`
}

func (s *Server) RunListenerProxyCheckTask(taskID uuid.UUID) {
	ctx := context.Background()
	claimed, release := s.claimTaskRun(ctx, taskID, "listener_proxy_check")
	if !claimed {
		return
	}
	defer release()

	var task models.Task
	if err := s.db.WithContext(ctx).Where("id = ? AND type = ?", taskID, "listener_proxy_check").First(&task).Error; err != nil {
		return
	}
	var payload listenerProxyCheckPayload
	if len(task.Payload) > 0 {
		_ = json.Unmarshal(task.Payload, &payload)
	}
	s.runListenerProxyCheckTask(ctx, task, payload)
}

func (s *Server) runListenerProxyCheckTask(ctx context.Context, task models.Task, payload listenerProxyCheckPayload) {
	query := s.db.WithContext(ctx).Where("tenant_id = ?", task.TenantID).Order("created_at desc")
	if strings.TrimSpace(payload.GroupID) != "" {
		groupID, err := uuid.Parse(strings.TrimSpace(payload.GroupID))
		if err != nil {
			s.finishListenerProxyCheckTask(ctx, task, "failed", listenerProxyCheckSummary{}, "代理分组无效："+err.Error())
			return
		}
		query = query.Where("group_id = ?", groupID)
	}
	var proxies []models.ListenerProxy
	if err := query.Find(&proxies).Error; err != nil {
		s.finishListenerProxyCheckTask(ctx, task, "failed", listenerProxyCheckSummary{}, "读取监听代理失败："+err.Error())
		return
	}
	summary := listenerProxyCheckSummary{Total: len(proxies)}
	concurrency := s.cfg.ListenerProxyCheckConcurrency
	if concurrency <= 0 {
		concurrency = 20
	}
	if concurrency > len(proxies) && len(proxies) > 0 {
		concurrency = len(proxies)
	}
	s.updateTaskState(ctx, task.ID, "running", 1, nil)
	s.logTaskBackground(ctx, task, "INFO", "start", fmt.Sprintf("开始检测监听代理延迟：共 %d 个代理，并发 %d", len(proxies), concurrency))
	if len(proxies) == 0 {
		s.finishListenerProxyCheckTask(ctx, task, "success", summary, "没有需要检测的代理")
		return
	}
	var summaryMu sync.Mutex
	var progressMu sync.Mutex
	var completed int
	var firstUpdateErr error
	sem := make(chan struct{}, concurrency)
	var wg sync.WaitGroup
	for index, item := range proxies {
		index := index
		item := item
		wg.Add(1)
		sem <- struct{}{}
		go func() {
			defer wg.Done()
			defer func() { <-sem }()
			startedAt := time.Now()
			address := net.JoinHostPort(strings.TrimSpace(item.IP), fmt.Sprintf("%d", item.Port))
			s.logTaskBackground(ctx, task, "INFO", "proxy_check_start", fmt.Sprintf("开始检测代理 %d/%d：%s %s", index+1, len(proxies), strings.ToUpper(item.Protocol), address))
			latency, status := measureListenerProxy(item)
			exitIP := ""
			country := firstNonEmpty(item.Country, "未知")
			flag := item.Flag
			if status == "normal" {
				s.logTaskBackground(ctx, task, "INFO", "proxy_exit_lookup", fmt.Sprintf("代理 %d/%d 连通性正常，开始查询真实出口 IP：%s", index+1, len(proxies), address))
				exit := lookupListenerProxyExit(ctx, item)
				exitIP = exit.IP
				if exit.Country != "" {
					country = exit.Country
				}
				if exit.Flag != "" {
					flag = exit.Flag
				}
				item.ExitIP = exitIP
			}
			telegramStatus := "untested"
			telegramError := ""
			webStatus := "untested"
			webError := ""
			if status == "normal" {
				webStatus, webError = measureWebTelegramProxy(item)
				telegramStatus, telegramError = measureTelegramProxy(item)
			}
			summaryMu.Lock()
			switch status {
			case "normal":
				summary.Normal++
			case "timeout":
				summary.Timeout++
			default:
				summary.Failed++
			}
			summaryMu.Unlock()
			if err := s.db.WithContext(ctx).Model(&models.ListenerProxy{}).Where("tenant_id = ? AND id = ?", task.TenantID, item.ID).Updates(map[string]any{
				"latency_ms":      latency,
				"status":          status,
				"exit_ip":         exitIP,
				"country":         country,
				"flag":            flag,
				"web_status":      webStatus,
				"web_error":       webError,
				"telegram_status": telegramStatus,
				"telegram_error":  telegramError,
				"updated_at":      time.Now(),
			}).Error; err != nil {
				summaryMu.Lock()
				summary.Failed++
				summaryMu.Unlock()
				progressMu.Lock()
				if firstUpdateErr == nil {
					firstUpdateErr = err
				}
				progressMu.Unlock()
				s.logTaskBackground(ctx, task, "ERROR", "proxy_update_failed", fmt.Sprintf("%s -> 更新代理延迟失败：%s", address, err.Error()))
				return
			}
			detail := listenerProxyCheckLogDetail(item, status, latency, exitIP, country, webStatus, webError, telegramStatus, telegramError)
			level := "INFO"
			if status == "timeout" {
				level = "WARN"
			} else if status != "normal" {
				level = "ERROR"
			}
			_ = s.createTaskLogWithDuration(ctx, task, level, "test_proxy_latency", detail, address, firstNonEmpty(exitIP, country), time.Since(startedAt).Milliseconds())
			progressMu.Lock()
			completed++
			progress := 1 + int(float64(completed)/float64(len(proxies))*94)
			if progress > 95 {
				progress = 95
			}
			s.updateTaskState(ctx, task.ID, "running", progress, nil)
			progressMu.Unlock()
		}()
	}
	wg.Wait()
	if firstUpdateErr != nil {
		summaryMu.Lock()
		finalSummary := summary
		summaryMu.Unlock()
		s.finishListenerProxyCheckTask(ctx, task, "failed", finalSummary, "更新代理延迟失败："+firstUpdateErr.Error())
		return
	}
	summaryMu.Lock()
	finalSummary := summary
	summaryMu.Unlock()
	status := "success"
	if finalSummary.Failed > 0 || finalSummary.Timeout > 0 {
		status = "partial_success"
	}
	detail := fmt.Sprintf("代理延迟检测完成：总数 %d，正常 %d，失败 %d，超时 %d，并发 %d", finalSummary.Total, finalSummary.Normal, finalSummary.Failed, finalSummary.Timeout, concurrency)
	s.finishListenerProxyCheckTask(ctx, task, status, finalSummary, detail)
}

func (s *Server) finishListenerProxyCheckTask(ctx context.Context, task models.Task, status string, summary listenerProxyCheckSummary, detail string) {
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

func listenerProxyCheckLogDetail(item models.ListenerProxy, status string, latency int64, exitIP string, country string, webStatus string, webError string, telegramStatus string, telegramError string) string {
	address := net.JoinHostPort(strings.TrimSpace(item.IP), fmt.Sprintf("%d", item.Port))
	switch status {
	case "normal":
		exitText := "真实出口 IP 待检测"
		if strings.TrimSpace(exitIP) != "" {
			exitText = fmt.Sprintf("真实出口 IP %s", strings.TrimSpace(exitIP))
			if strings.TrimSpace(country) != "" && strings.TrimSpace(country) != "未知" {
				exitText += "，位置 " + strings.TrimSpace(country)
			}
		}
		webText := "Web 未检测"
		if webStatus == "normal" {
			webText = "Web Telegram 可访问"
		} else if webStatus == "timeout" {
			webText = "Web Telegram 超时"
		} else if webStatus == "failed" {
			webText = "Web Telegram 不通"
			if strings.TrimSpace(webError) != "" {
				webText += "：" + strings.TrimSpace(webError)
			}
		}
		telegramText := "客户端未检测"
		if telegramStatus == "normal" {
			telegramText = "客户端可用"
		} else if telegramStatus == "timeout" {
			telegramText = "客户端超时"
		} else if telegramStatus == "failed" {
			telegramText = "客户端不通"
			if strings.TrimSpace(telegramError) != "" {
				telegramText += "：" + strings.TrimSpace(telegramError)
			}
		}
		return fmt.Sprintf("%s %s 检测正常，延迟 %d ms，%s，%s，%s", strings.ToUpper(item.Protocol), address, latency, exitText, webText, telegramText)
	case "timeout":
		return fmt.Sprintf("%s %s 检测超时，请检查代理是否可连、端口是否开放或账号密码是否正确", strings.ToUpper(item.Protocol), address)
	default:
		return fmt.Sprintf("%s %s 检测失败，请检查代理协议、端口、用户名和密码", strings.ToUpper(item.Protocol), address)
	}
}

type proxyGeoResponse struct {
	Status      string `json:"status"`
	Query       string `json:"query"`
	IP          string `json:"ip"`
	Country     string `json:"country"`
	CountryCode string `json:"countryCode"`
}

type proxyExitResult struct {
	IP      string
	Country string
	Flag    string
}

func measureListenerProxy(item models.ListenerProxy) (int64, string) {
	protocol := strings.ToLower(strings.TrimSpace(item.Protocol))
	switch protocol {
	case "socks5", "sk5":
		return measureSOCKS5Proxy(item)
	case "http", "https":
		return measureHTTPProxy(item)
	default:
		return measureProxyEndpoint(item.IP, item.Port)
	}
}

func measureSOCKS5Proxy(item models.ListenerProxy) (int64, string) {
	address := net.JoinHostPort(strings.TrimSpace(item.IP), fmt.Sprintf("%d", item.Port))
	baseDialer := &net.Dialer{Timeout: 5 * time.Second}
	var auth *proxy.Auth
	if strings.TrimSpace(item.Username) != "" || strings.TrimSpace(item.Password) != "" {
		auth = &proxy.Auth{User: item.Username, Password: item.Password}
	}
	dialer, err := proxy.SOCKS5("tcp", address, auth, baseDialer)
	if err != nil {
		return 0, "failed"
	}
	start := time.Now()
	conn, err := proxyDialWithTimeout(dialer, "tcp", listenerProxyProbeTarget, 7*time.Second)
	if err != nil {
		if os.IsTimeout(err) {
			return 0, "timeout"
		}
		return 0, "failed"
	}
	_ = conn.Close()
	elapsed := time.Since(start).Milliseconds()
	if elapsed <= 0 {
		elapsed = 1
	}
	return elapsed, "normal"
}

func measureHTTPProxy(item models.ListenerProxy) (int64, string) {
	proxyURL, err := listenerHTTPProxyURL(item)
	if err != nil {
		return 0, "failed"
	}
	client := &http.Client{
		Timeout: 5 * time.Second,
		Transport: &http.Transport{
			Proxy: http.ProxyURL(proxyURL),
		},
	}
	start := time.Now()
	req, err := http.NewRequest(http.MethodGet, listenerHTTPProxyProbeURL, nil)
	if err != nil {
		return 0, "failed"
	}
	req.Header.Set("User-Agent", "tg-proxy-latency-test/1.0")
	resp, err := client.Do(req)
	if err != nil {
		if os.IsTimeout(err) {
			return 0, "timeout"
		}
		return 0, "failed"
	}
	_ = resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 400 {
		return 0, "failed"
	}
	elapsed := time.Since(start).Milliseconds()
	if elapsed <= 0 {
		elapsed = 1
	}
	return elapsed, "normal"
}

func measureWebTelegramProxy(item models.ListenerProxy) (string, string) {
	client, err := listenerProxyHTTPClient(item, 10*time.Second)
	if err != nil {
		return "failed", err.Error()
	}
	req, err := http.NewRequest(http.MethodGet, listenerWebTelegramProbeURL, nil)
	if err != nil {
		return "failed", "创建 Web Telegram 检测请求失败"
	}
	req.Header.Set("User-Agent", "tg-web-proxy-test/1.0")
	resp, err := client.Do(req)
	if err != nil {
		if os.IsTimeout(err) {
			return "timeout", "访问 Web Telegram 超时"
		}
		return "failed", sanitizeProxyError(err)
	}
	_ = resp.Body.Close()
	if resp.StatusCode >= 200 && resp.StatusCode < 400 {
		return "normal", ""
	}
	if resp.StatusCode == http.StatusProxyAuthRequired {
		return "failed", "代理认证失败 407"
	}
	return "failed", fmt.Sprintf("Web Telegram 返回 %d", resp.StatusCode)
}

func listenerProxyHTTPClient(item models.ListenerProxy, timeout time.Duration) (*http.Client, error) {
	protocol := strings.ToLower(strings.TrimSpace(item.Protocol))
	transport := &http.Transport{}
	switch protocol {
	case "http", "https":
		proxyURL, err := listenerHTTPProxyURL(item)
		if err != nil {
			return nil, fmt.Errorf("代理地址无效")
		}
		transport.Proxy = http.ProxyURL(proxyURL)
	case "socks5", "sk5":
		address := net.JoinHostPort(strings.TrimSpace(item.IP), fmt.Sprintf("%d", item.Port))
		baseDialer := &net.Dialer{Timeout: 5 * time.Second}
		var auth *proxy.Auth
		if strings.TrimSpace(item.Username) != "" || strings.TrimSpace(item.Password) != "" {
			auth = &proxy.Auth{User: item.Username, Password: item.Password}
		}
		dialer, err := proxy.SOCKS5("tcp", address, auth, baseDialer)
		if err != nil {
			return nil, fmt.Errorf("SOCKS5 初始化失败")
		}
		transport.DialContext = func(ctx context.Context, network, addr string) (net.Conn, error) {
			type dialResult struct {
				conn net.Conn
				err  error
			}
			ch := make(chan dialResult, 1)
			go func() {
				conn, err := proxyDialWithTimeout(dialer, network, addr, 7*time.Second)
				ch <- dialResult{conn: conn, err: err}
			}()
			select {
			case result := <-ch:
				return result.conn, result.err
			case <-ctx.Done():
				return nil, ctx.Err()
			}
		}
	default:
		return nil, fmt.Errorf("未知代理协议")
	}
	return &http.Client{Timeout: timeout, Transport: transport}, nil
}

func measureTelegramProxy(item models.ListenerProxy) (string, string) {
	protocol := strings.ToLower(strings.TrimSpace(item.Protocol))
	switch protocol {
	case "socks5", "sk5":
		return measureTelegramSOCKS5Proxy(item)
	case "http", "https":
		return measureTelegramHTTPProxy(item)
	default:
		_, status := measureProxyEndpoint(item.IP, item.Port)
		return status, "仅检测代理入口，未知是否支持 Telegram"
	}
}

func measureTelegramSOCKS5Proxy(item models.ListenerProxy) (string, string) {
	address := net.JoinHostPort(strings.TrimSpace(item.IP), fmt.Sprintf("%d", item.Port))
	baseDialer := &net.Dialer{Timeout: 5 * time.Second}
	var auth *proxy.Auth
	if strings.TrimSpace(item.Username) != "" || strings.TrimSpace(item.Password) != "" {
		auth = &proxy.Auth{User: item.Username, Password: item.Password}
	}
	dialer, err := proxy.SOCKS5("tcp", address, auth, baseDialer)
	if err != nil {
		return "failed", "SOCKS5 初始化失败"
	}
	conn, err := proxyDialWithTimeout(dialer, "tcp", listenerTelegramProbeTarget, 7*time.Second)
	if err != nil {
		if listenerProxyHasIPv6Exit(item) {
			ipv6Target := net.JoinHostPort(listenerTelegramIPv6ProbeHost, "443")
			ipv6Conn, ipv6Err := proxyDialWithTimeout(dialer, "tcp", ipv6Target, 7*time.Second)
			if ipv6Err == nil {
				_ = ipv6Conn.Close()
				return "normal", "IPv6 Telegram DC 可用"
			}
			if os.IsTimeout(ipv6Err) {
				return "timeout", "IPv6 Telegram DC 超时"
			}
			return "failed", sanitizeProxyError(ipv6Err)
		}
		if os.IsTimeout(err) {
			return "timeout", "连接 Telegram 超时"
		}
		return "failed", sanitizeProxyError(err)
	}
	_ = conn.Close()
	return "normal", ""
}

func measureTelegramHTTPProxy(item models.ListenerProxy) (string, string) {
	address := net.JoinHostPort(strings.TrimSpace(item.IP), fmt.Sprintf("%d", item.Port))
	conn, err := net.DialTimeout("tcp", address, 5*time.Second)
	if err != nil {
		if os.IsTimeout(err) {
			return "timeout", "连接代理超时"
		}
		return "failed", sanitizeProxyError(err)
	}
	defer conn.Close()
	_ = conn.SetDeadline(time.Now().Add(7 * time.Second))

	var builder strings.Builder
	builder.WriteString("CONNECT ")
	builder.WriteString(listenerTelegramProbeTarget)
	builder.WriteString(" HTTP/1.1\r\nHost: ")
	builder.WriteString(listenerTelegramProbeTarget)
	builder.WriteString("\r\nProxy-Connection: close\r\n")
	if strings.TrimSpace(item.Username) != "" || strings.TrimSpace(item.Password) != "" {
		token := base64.StdEncoding.EncodeToString([]byte(item.Username + ":" + item.Password))
		builder.WriteString("Proxy-Authorization: Basic ")
		builder.WriteString(token)
		builder.WriteString("\r\n")
	}
	builder.WriteString("\r\n")
	if _, err := conn.Write([]byte(builder.String())); err != nil {
		return "failed", sanitizeProxyError(err)
	}
	resp, err := http.ReadResponse(bufio.NewReader(conn), nil)
	if err != nil {
		if os.IsTimeout(err) {
			return "timeout", "Telegram CONNECT 超时"
		}
		return "failed", sanitizeProxyError(err)
	}
	_ = resp.Body.Close()
	if resp.StatusCode >= 200 && resp.StatusCode < 300 {
		return "normal", ""
	}
	if resp.StatusCode == http.StatusProxyAuthRequired {
		return "failed", "代理认证失败 407"
	}
	return "failed", fmt.Sprintf("Telegram CONNECT 返回 %d", resp.StatusCode)
}

func measureProxyEndpoint(ip string, port int) (int64, string) {
	address := net.JoinHostPort(strings.TrimSpace(ip), fmt.Sprintf("%d", port))
	start := time.Now()
	conn, err := net.DialTimeout("tcp", address, 5*time.Second)
	if err != nil {
		if os.IsTimeout(err) {
			return 0, "timeout"
		}
		return 0, "failed"
	}
	_ = conn.Close()
	elapsed := time.Since(start).Milliseconds()
	if elapsed <= 0 {
		elapsed = 1
	}
	return elapsed, "normal"
}

func sanitizeProxyError(err error) string {
	if err == nil {
		return ""
	}
	text := strings.TrimSpace(err.Error())
	if text == "" {
		return "代理连接失败"
	}
	text = strings.ReplaceAll(text, "\n", " ")
	if len(text) > 180 {
		text = text[:180]
	}
	return text
}

func listenerAccountProxyFailure(reason string) bool {
	text := strings.ToLower(strings.TrimSpace(reason))
	if text == "" {
		return false
	}
	return strings.Contains(text, "proxy") ||
		strings.Contains(text, "407") ||
		strings.Contains(text, "connection to telegram failed") ||
		strings.Contains(text, "connect call failed") ||
		strings.Contains(text, "connection refused") ||
		strings.Contains(text, "timed out")
}

func lookupProxyCountry(ctx context.Context, ip string) (string, string) {
	ip = strings.TrimSpace(ip)
	if ip == "" || net.ParseIP(ip) == nil {
		return "", ""
	}
	reqCtx, cancel := context.WithTimeout(ctx, 3*time.Second)
	defer cancel()
	url := "http://ip-api.com/json/" + ip + "?fields=status,country,countryCode"
	req, err := http.NewRequestWithContext(reqCtx, http.MethodGet, url, nil)
	if err != nil {
		return "", ""
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return "", ""
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return "", ""
	}
	var geo proxyGeoResponse
	if err := json.NewDecoder(resp.Body).Decode(&geo); err != nil {
		return "", ""
	}
	if !strings.EqualFold(geo.Status, "success") || strings.TrimSpace(geo.Country) == "" {
		return "", ""
	}
	return strings.TrimSpace(geo.Country), countryFlagEmoji(geo.CountryCode)
}

func (s *Server) lookupListenerAccountProxyExit(ctx context.Context, tenantID uuid.UUID, proxyID uuid.UUID, cache map[uuid.UUID]proxyExitResult) proxyExitResult {
	if cached, ok := cache[proxyID]; ok {
		return cached
	}
	var item models.ListenerProxy
	if err := s.db.WithContext(ctx).Where("tenant_id = ? AND id = ?", tenantID, proxyID).First(&item).Error; err != nil {
		cache[proxyID] = proxyExitResult{}
		return proxyExitResult{}
	}
	if strings.TrimSpace(item.ExitIP) != "" {
		exit := proxyExitResult{IP: item.ExitIP, Country: item.Country, Flag: item.Flag}
		cache[proxyID] = exit
		return exit
	}
	exit := lookupListenerProxyExit(ctx, item)
	cache[proxyID] = exit
	return exit
}

func lookupListenerProxyExit(ctx context.Context, item models.ListenerProxy) proxyExitResult {
	lookupCtx, cancelLookup := context.WithTimeout(ctx, 8*time.Second)
	defer cancelLookup()
	client := &http.Client{Timeout: 5 * time.Second}
	switch strings.ToLower(strings.TrimSpace(item.Protocol)) {
	case "socks5", "sk5":
		dialer, err := listenerSOCKS5Dialer(item)
		if err != nil {
			return proxyExitResult{}
		}
		client.Transport = &http.Transport{
			DialContext: func(ctx context.Context, network string, address string) (net.Conn, error) {
				return proxyDialWithContext(ctx, dialer, network, address)
			},
		}
	case "http", "https":
		proxyURL, err := listenerHTTPProxyURL(item)
		if err != nil {
			return proxyExitResult{}
		}
		client.Transport = &http.Transport{Proxy: http.ProxyURL(proxyURL)}
	default:
		return proxyExitResult{}
	}

	for _, lookupURL := range listenerProxyExitLookupURLs {
		if lookupCtx.Err() != nil {
			return proxyExitResult{}
		}
		reqCtx, cancel := context.WithTimeout(lookupCtx, 5*time.Second)
		req, err := http.NewRequestWithContext(reqCtx, http.MethodGet, lookupURL, nil)
		if err != nil {
			cancel()
			continue
		}
		resp, err := client.Do(req)
		if err != nil {
			cancel()
			continue
		}
		payload, readErr := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
		_ = resp.Body.Close()
		cancel()
		if readErr != nil || resp.StatusCode < 200 || resp.StatusCode >= 300 {
			continue
		}
		exit := parseProxyExitPayload(ctx, payload)
		if exit.IP != "" {
			return exit
		}
	}
	return proxyExitResult{}
}

type proxyDialResult struct {
	conn net.Conn
	err  error
}

func proxyDialWithTimeout(dialer proxy.Dialer, network string, address string, timeout time.Duration) (net.Conn, error) {
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()
	return proxyDialWithContext(ctx, dialer, network, address)
}

func proxyDialWithContext(ctx context.Context, dialer proxy.Dialer, network string, address string) (net.Conn, error) {
	ch := make(chan proxyDialResult, 1)
	go func() {
		conn, err := dialer.Dial(network, address)
		ch <- proxyDialResult{conn: conn, err: err}
	}()
	select {
	case <-ctx.Done():
		return nil, ctx.Err()
	case res := <-ch:
		return res.conn, res.err
	}
}

func parseProxyExitPayload(ctx context.Context, payload []byte) proxyExitResult {
	var geo proxyGeoResponse
	if err := json.Unmarshal(payload, &geo); err == nil {
		ip := firstNonEmpty(strings.TrimSpace(geo.Query), strings.TrimSpace(geo.IP))
		if net.ParseIP(ip) == nil {
			return proxyExitResult{}
		}
		country := strings.TrimSpace(geo.Country)
		flag := countryFlagEmoji(geo.CountryCode)
		if country == "" || flag == "" {
			lookupCountry, lookupFlag := lookupProxyCountry(ctx, ip)
			country = firstNonEmpty(country, lookupCountry)
			flag = firstNonEmpty(flag, lookupFlag)
		}
		return proxyExitResult{IP: ip, Country: country, Flag: flag}
	}
	ip := strings.TrimSpace(string(payload))
	if net.ParseIP(ip) == nil {
		return proxyExitResult{}
	}
	country, flag := lookupProxyCountry(ctx, ip)
	return proxyExitResult{IP: ip, Country: country, Flag: flag}
}

func listenerSOCKS5Dialer(item models.ListenerProxy) (proxy.Dialer, error) {
	address := net.JoinHostPort(strings.TrimSpace(item.IP), fmt.Sprintf("%d", item.Port))
	baseDialer := &net.Dialer{Timeout: 5 * time.Second}
	var auth *proxy.Auth
	if strings.TrimSpace(item.Username) != "" || strings.TrimSpace(item.Password) != "" {
		auth = &proxy.Auth{User: item.Username, Password: item.Password}
	}
	return proxy.SOCKS5("tcp", address, auth, baseDialer)
}

func listenerHTTPProxyURL(item models.ListenerProxy) (*url.URL, error) {
	scheme := strings.ToLower(strings.TrimSpace(item.Protocol))
	if scheme != "http" && scheme != "https" {
		scheme = "http"
	}
	proxyURL := &url.URL{
		Scheme: scheme,
		Host:   net.JoinHostPort(strings.TrimSpace(item.IP), fmt.Sprintf("%d", item.Port)),
	}
	if strings.TrimSpace(item.Username) != "" || strings.TrimSpace(item.Password) != "" {
		proxyURL.User = url.UserPassword(item.Username, item.Password)
	}
	return proxyURL, nil
}

func countryFlagEmoji(code string) string {
	code = strings.ToUpper(strings.TrimSpace(code))
	if len(code) != 2 {
		return ""
	}
	runes := []rune(code)
	if runes[0] < 'A' || runes[0] > 'Z' || runes[1] < 'A' || runes[1] > 'Z' {
		return ""
	}
	return string([]rune{0x1F1E6 + runes[0] - 'A', 0x1F1E6 + runes[1] - 'A'})
}

func (s *Server) RefreshListenerTargets(c *gin.Context) {
	var req struct {
		GroupID string `json:"group_id"`
	}
	_ = c.ShouldBindJSON(&req)
	tenantID := s.tenantID(c)
	req.GroupID = strings.TrimSpace(req.GroupID)
	targets, err := s.loadListenerTargetRefreshSelection(c.Request.Context(), tenantID, req.GroupID)
	if err != nil {
		utils.Fail(c, http.StatusBadRequest, err.Error())
		return
	}
	payload, _ := json.Marshal(req)
	task := models.Task{
		ID:        uuid.New(),
		TenantID:  tenantID,
		Name:      "一键刷新群资料",
		Type:      "listener_target_refresh",
		Status:    "queued",
		Progress:  0,
		Payload:   datatypes.JSON(payload),
		CreatedBy: s.userIDPtr(c),
	}
	if err := s.db.WithContext(c.Request.Context()).Create(&task).Error; err != nil {
		utils.Fail(c, http.StatusInternalServerError, "创建监听群资料刷新任务失败")
		return
	}
	_ = s.createTaskLog(c.Request.Context(), task, "INFO", "created", fmt.Sprintf("监听群资料刷新任务已创建：待刷新 %d 个监听群", len(targets)), "", "")
	if !s.enqueueTask(c.Request.Context(), task, "run") {
		go s.runListenerTargetRefreshTask(context.Background(), task, req.GroupID)
	}
	utils.Created(c, gin.H{"task": task, "summary": listenerTargetRefreshSummary{TaskID: task.ID.String(), Total: len(targets)}})
}

func (s *Server) RunListenerTargetRefreshTask(taskID uuid.UUID) {
	ctx := context.Background()
	claimed, release := s.claimTaskRun(ctx, taskID, "listener_target_refresh")
	if !claimed {
		return
	}
	defer release()

	var task models.Task
	if err := s.db.WithContext(ctx).Where("id = ? AND type = ?", taskID, "listener_target_refresh").First(&task).Error; err != nil {
		return
	}
	var req struct {
		GroupID string `json:"group_id"`
	}
	if len(task.Payload) > 0 {
		_ = json.Unmarshal(task.Payload, &req)
	}
	s.runListenerTargetRefreshTask(ctx, task, strings.TrimSpace(req.GroupID))
}

func (s *Server) loadListenerTargetRefreshSelection(ctx context.Context, tenantID uuid.UUID, groupIDText string) ([]models.ListenerTarget, error) {
	query := s.db.WithContext(ctx).Where("tenant_id = ?", tenantID).Order("created_at desc")
	if strings.TrimSpace(groupIDText) != "" {
		groupID, err := uuid.Parse(strings.TrimSpace(groupIDText))
		if err != nil {
			return nil, fmt.Errorf("监听群分组无效")
		}
		query = query.Where("group_id = ?", groupID)
	}
	var targets []models.ListenerTarget
	if err := query.Find(&targets).Error; err != nil {
		return nil, fmt.Errorf("读取监听群失败")
	}
	if len(targets) == 0 {
		return nil, fmt.Errorf("当前范围内没有监听群")
	}
	return targets, nil
}

func (s *Server) runListenerTargetRefreshTask(ctx context.Context, task models.Task, groupIDText string) {
	targets, err := s.loadListenerTargetRefreshSelection(ctx, task.TenantID, groupIDText)
	if err != nil {
		s.finishListenerTargetRefreshTask(ctx, task, "failed", listenerTargetRefreshSummary{TaskID: task.ID.String(), Items: []joinTargetsResultRow{}}, err.Error())
		return
	}
	accounts := s.listListenerInspectorAccounts(ctx, task.TenantID)
	if len(accounts) == 0 {
		s.finishListenerTargetRefreshTask(ctx, task, "failed", listenerTargetRefreshSummary{TaskID: task.ID.String(), Total: len(targets), Items: []joinTargetsResultRow{}}, "没有可用于刷新监听群资料的监听号，请先导入并检测监听号")
		return
	}
	inspector := telegram_client.NewTargetInspector(s.cfg)
	summary := listenerTargetRefreshSummary{TaskID: task.ID.String(), Total: len(targets), Items: []joinTargetsResultRow{}}
	concurrency := 15
	if concurrency > len(targets) {
		concurrency = len(targets)
	}
	_ = s.createTaskLog(ctx, task, "INFO", "start", fmt.Sprintf("开始刷新监听群资料：共 %d 个监听群，可用监听号 %d 个，并发 %d，失败时会自动换号重试", len(targets), len(accounts), concurrency), "", "")
	s.updateTaskState(ctx, task.ID, "running", 1, nil)
	var summaryMu sync.Mutex
	var progressMu sync.Mutex
	var completed int
	var firstUpdateErr error
	sem := make(chan struct{}, concurrency)
	var wg sync.WaitGroup
	for index, target := range targets {
		index := index
		target := target
		wg.Add(1)
		sem <- struct{}{}
		go func() {
			defer wg.Done()
			defer func() { <-sem }()
			targetRef := targetJoinLabel(models.Target{Identifier: target.Identifier, Name: target.Name, Type: target.Type})
			account, result, inspectErr, attempts := s.inspectListenerTargetWithFallback(ctx, inspector, accounts, target.Identifier, index)
			updates := map[string]any{"updated_at": time.Now()}
			row := joinTargetsResultRow{TerminalID: account.ID.String(), Terminal: listenerAccountJoinLabel(account), TargetID: target.ID.String(), Target: targetRef}
			if inspectErr == nil && result.OK {
				updates["identifier"] = firstNonEmpty(result.Identifier, target.Identifier)
				updates["name"] = firstNonEmpty(result.Name, target.Name)
				updates["type"] = firstNonEmpty(result.Type, target.Type)
				updates["size"] = result.Size
				updates["status"] = firstNonEmpty(result.Status, "active")
				row.Status = "success"
				row.Reason = firstNonEmpty(result.Reason, "监听群资料刷新成功")
				_ = s.createTaskLog(ctx, task, "INFO", "target_refresh_success", fmt.Sprintf("监听群 %s 刷新成功：名称 %s，类型 %s，成员数 %d", targetRef, firstNonEmpty(result.Name, target.Name, "-"), firstNonEmpty(result.Type, target.Type, "-"), result.Size), row.Terminal, targetRef)
			} else {
				updates["status"] = "failed"
				row.Status = "failed"
				row.Reason = translateTelegramReasonToChinese(firstNonEmpty(result.Reason, errorString(inspectErr), "监听群资料刷新失败"))
				if attempts != "" {
					row.Reason = row.Reason + "；已尝试：" + attempts
				}
				_ = s.createTaskLog(ctx, task, "ERROR", "target_refresh_failed", fmt.Sprintf("监听群 %s 刷新失败：%s", targetRef, row.Reason), row.Terminal, targetRef)
			}
			if err := s.db.WithContext(ctx).Model(&models.ListenerTarget{}).Where("tenant_id = ? AND id = ?", task.TenantID, target.ID).Updates(updates).Error; err != nil {
				detail := fmt.Sprintf("监听群 %s 数据库更新失败：%s", targetRef, err.Error())
				_ = s.createTaskLog(ctx, task, "ERROR", "target_refresh_failed", detail, row.Terminal, targetRef)
				progressMu.Lock()
				if firstUpdateErr == nil {
					firstUpdateErr = fmt.Errorf("%s", detail)
				}
				progressMu.Unlock()
			}
			summaryMu.Lock()
			if row.Status == "success" {
				summary.Success++
			} else {
				summary.Failed++
			}
			summary.Items = append(summary.Items, row)
			payload, _ := json.Marshal(summary)
			summaryMu.Unlock()
			progressMu.Lock()
			completed++
			progress := 5 + int(float64(completed)/float64(len(targets))*90)
			if progress > 95 {
				progress = 95
			}
			_ = s.db.WithContext(ctx).Model(&models.Task{}).Where("id = ?", task.ID).Updates(map[string]any{
				"status":   "running",
				"progress": progress,
				"summary":  datatypes.JSON(payload),
			}).Error
			progressMu.Unlock()
		}()
	}
	wg.Wait()
	if firstUpdateErr != nil {
		summaryMu.Lock()
		finalSummary := summary
		summaryMu.Unlock()
		s.finishListenerTargetRefreshTask(ctx, task, "failed", finalSummary, firstUpdateErr.Error())
		return
	}
	summaryMu.Lock()
	finalSummary := summary
	summaryMu.Unlock()
	status := "success"
	if finalSummary.Success == 0 && finalSummary.Failed > 0 {
		status = "failed"
	} else if finalSummary.Failed > 0 {
		status = "partial_success"
	}
	s.finishListenerTargetRefreshTask(ctx, task, status, finalSummary, fmt.Sprintf("监听群资料刷新完成：成功 %d，失败 %d，并发 %d", finalSummary.Success, finalSummary.Failed, concurrency))
}

func (s *Server) finishListenerTargetRefreshTask(ctx context.Context, task models.Task, status string, summary listenerTargetRefreshSummary, detail string) {
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

func (s *Server) pickListenerInspectorAccount(ctx context.Context, tenantID uuid.UUID) (models.ListenerAccount, bool) {
	var account models.ListenerAccount
	err := s.db.WithContext(ctx).
		Where("tenant_id = ? AND file_path <> '' AND status NOT IN ?", tenantID, []string{"abnormal", "failed"}).
		Order("updated_at asc").
		First(&account).Error
	if err != nil || strings.TrimSpace(account.FilePath) == "" || !isStoredTerminalFileReady(account.FilePath) {
		return models.ListenerAccount{}, false
	}
	return account, true
}

func (s *Server) listListenerInspectorAccounts(ctx context.Context, tenantID uuid.UUID) []models.ListenerAccount {
	var accounts []models.ListenerAccount
	_ = s.db.WithContext(ctx).
		Where("tenant_id = ? AND file_path <> '' AND status NOT IN ?", tenantID, []string{"abnormal", "failed"}).
		Order("updated_at asc").
		Find(&accounts).Error
	ready := make([]models.ListenerAccount, 0, len(accounts))
	for _, account := range accounts {
		if strings.TrimSpace(account.FilePath) != "" && isStoredTerminalFileReady(account.FilePath) {
			ready = append(ready, account)
		}
	}
	return ready
}

func (s *Server) inspectListenerTargetWithFallback(ctx context.Context, inspector telegram_client.TargetInspector, accounts []models.ListenerAccount, target string, targetIndex int) (models.ListenerAccount, telegram_client.TargetInspectResult, error, string) {
	if len(accounts) == 0 {
		return models.ListenerAccount{}, telegram_client.TargetInspectResult{Status: "failed", Reason: "没有可用监听号", Identifier: target}, fmt.Errorf("没有可用监听号"), ""
	}
	maxAttempts := minInt(len(accounts), 5)
	start := 0
	if len(accounts) > 0 {
		start = targetIndex % len(accounts)
	}
	attemptDetails := make([]string, 0, maxAttempts)
	var lastAccount models.ListenerAccount
	var lastResult telegram_client.TargetInspectResult
	var lastErr error
	for offset := 0; offset < maxAttempts; offset++ {
		account := accounts[(start+offset)%len(accounts)]
		result, err := inspector.Inspect(ctx, telegram_client.TargetInspectRequest{
			FilePath:   account.FilePath,
			AccessType: account.AccessType,
			Target:     target,
		})
		if err == nil && result.OK {
			return account, result, nil, strings.Join(attemptDetails, "；")
		}
		lastAccount = account
		lastResult = result
		lastErr = err
		reason := translateTelegramReasonToChinese(firstNonEmpty(result.Reason, errorString(err), "刷新失败"))
		attemptDetails = append(attemptDetails, listenerAccountJoinLabel(account)+"："+reason)
	}
	return lastAccount, lastResult, lastErr, strings.Join(attemptDetails, "；")
}

func errorString(err error) string {
	if err == nil {
		return ""
	}
	return err.Error()
}

func (s *Server) DeleteListenerAccount(c *gin.Context) {
	id, err := uuid.Parse(strings.TrimSpace(c.Param("id")))
	if err != nil {
		utils.Fail(c, http.StatusBadRequest, "监听账号 ID 无效")
		return
	}
	var account models.ListenerAccount
	_ = s.db.WithContext(c.Request.Context()).Where("tenant_id = ? AND id = ?", s.tenantID(c), id).First(&account).Error
	result := s.db.WithContext(c.Request.Context()).Where("tenant_id = ? AND id = ?", s.tenantID(c), id).Delete(&models.ListenerAccount{})
	if result.Error != nil {
		utils.Fail(c, http.StatusInternalServerError, "删除监听账号失败")
		return
	}
	if account.FilePath != "" {
		_ = removeStoredAssetFile(account.FilePath)
	}
	s.markAccountJoinRecordsUnavailable(c.Request.Context(), uuid.Nil, accountJoinKindListener, id, "监听账号已删除")
	utils.OK(c, gin.H{"deleted": result.RowsAffected})
}

func (s *Server) DeleteListenerTarget(c *gin.Context) {
	id, err := uuid.Parse(strings.TrimSpace(c.Param("id")))
	if err != nil {
		utils.Fail(c, http.StatusBadRequest, "监听群 ID 无效")
		return
	}
	result := s.db.WithContext(c.Request.Context()).Where("tenant_id = ? AND id = ?", s.tenantID(c), id).Delete(&models.ListenerTarget{})
	if result.Error != nil {
		utils.Fail(c, http.StatusInternalServerError, "删除监听群失败")
		return
	}
	utils.OK(c, gin.H{"deleted": result.RowsAffected})
}

func (s *Server) DeleteListenerProxy(c *gin.Context) {
	id, err := uuid.Parse(strings.TrimSpace(c.Param("id")))
	if err != nil {
		utils.Fail(c, http.StatusBadRequest, "代理 ID 无效")
		return
	}
	tenantID := s.tenantID(c)
	err = s.db.WithContext(c.Request.Context()).Transaction(func(tx *gorm.DB) error {
		if err := tx.Where("tenant_id = ? AND id = ?", tenantID, id).Delete(&models.ListenerProxy{}).Error; err != nil {
			return err
		}
		return tx.Model(&models.ListenerAccount{}).Where("tenant_id = ? AND proxy_id = ?", tenantID, id).Updates(map[string]any{
			"proxy_id":     nil,
			"exit_ip":      "",
			"exit_country": "",
			"exit_flag":    "",
			"updated_at":   time.Now(),
		}).Error
	})
	if err != nil {
		utils.Fail(c, http.StatusInternalServerError, "删除监听代理失败")
		return
	}
	utils.OK(c, gin.H{"deleted": 1})
}

func (s *Server) AssignListenerProxies(c *gin.Context) {
	var req struct {
		ProxyGroupID   string `json:"proxy_group_id"`
		AccountGroupID string `json:"account_group_id"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		utils.Fail(c, http.StatusBadRequest, "请选择代理分组")
		return
	}
	proxyGroupID, err := uuid.Parse(strings.TrimSpace(req.ProxyGroupID))
	if err != nil {
		utils.Fail(c, http.StatusBadRequest, "代理分组无效")
		return
	}
	summary, err := s.assignListenerProxies(c, &proxyGroupID, strings.TrimSpace(req.AccountGroupID))
	if err != nil {
		utils.Fail(c, http.StatusBadRequest, err.Error())
		return
	}
	utils.OK(c, summary)
}

const listenerProxyAssignmentLimit int64 = 5

func (s *Server) assignListenerProxies(c *gin.Context, proxyGroupID *uuid.UUID, accountGroupText string) (listenerAdminAssignSummary, error) {
	return s.assignListenerProxiesToAccounts(c, proxyGroupID, accountGroupText, nil, false)
}

func (s *Server) assignListenerProxiesToAccounts(c *gin.Context, proxyGroupID *uuid.UUID, accountGroupText string, accountIDs []uuid.UUID, onlyUnassigned bool) (listenerAdminAssignSummary, error) {
	tenantID := s.tenantID(c)
	var proxies []models.ListenerProxy
	query := s.db.WithContext(c.Request.Context()).
		Where("tenant_id = ? AND status NOT IN ?", tenantID, []string{"failed", "timeout"}).
		Where("telegram_status = ?", "normal").
		Order("bound_accounts asc, created_at asc")
	if proxyGroupID != nil {
		query = query.Where("group_id = ?", *proxyGroupID)
	}
	if err := query.Find(&proxies).Error; err != nil {
		return listenerAdminAssignSummary{}, err
	}
	if len(proxies) == 0 {
		return listenerAdminAssignSummary{}, fmt.Errorf("当前监听代理分组没有 Telegram 可用代理，请先检测代理或更换支持 Telegram 的代理")
	}
	var accounts []models.ListenerAccount
	aq := s.db.WithContext(c.Request.Context()).Where("tenant_id = ?", tenantID).Order("created_at asc")
	if len(accountIDs) > 0 {
		aq = aq.Where("id IN ?", accountIDs)
	}
	if onlyUnassigned {
		aq = aq.Where("proxy_id IS NULL")
	}
	if accountGroupText != "" {
		groupID, err := uuid.Parse(accountGroupText)
		if err != nil {
			return listenerAdminAssignSummary{}, fmt.Errorf("监听号分组无效")
		}
		aq = aq.Where("group_id = ?", groupID)
	}
	if err := aq.Find(&accounts).Error; err != nil {
		return listenerAdminAssignSummary{}, err
	}
	if len(accounts) == 0 {
		return listenerAdminAssignSummary{}, fmt.Errorf("没有可分配的监听号")
	}
	usage := map[uuid.UUID]int64{}
	for _, proxy := range proxies {
		usage[proxy.ID] = 0
	}
	accountIDSet := map[uuid.UUID]struct{}{}
	for _, account := range accounts {
		accountIDSet[account.ID] = struct{}{}
	}
	var proxyUsageRows []struct {
		ProxyID uuid.UUID
		Count   int64
	}
	usageQuery := s.db.WithContext(c.Request.Context()).
		Model(&models.ListenerAccount{}).
		Select("proxy_id, count(*) as count").
		Where("tenant_id = ? AND proxy_id IN ?", tenantID, proxyIDsFromModels(proxies)).
		Where("proxy_id IS NOT NULL")
	if len(accountIDSet) > 0 {
		usageQuery = usageQuery.Where("id NOT IN ?", accountIDsFromModels(accounts))
	}
	if err := usageQuery.Group("proxy_id").Scan(&proxyUsageRows).Error; err != nil {
		return listenerAdminAssignSummary{}, err
	}
	for _, row := range proxyUsageRows {
		usage[row.ProxyID] = row.Count
	}
	summary := listenerAdminAssignSummary{Accounts: len(accounts), Proxies: len(proxies)}
	now := time.Now()
	exitCache := map[uuid.UUID]proxyExitResult{}
	for _, account := range accounts {
		bestIndex := -1
		for index := range proxies {
			if usage[proxies[index].ID] >= listenerProxyAssignmentLimit {
				continue
			}
			if bestIndex < 0 || usage[proxies[index].ID] < usage[proxies[bestIndex].ID] {
				bestIndex = index
			}
		}
		if bestIndex < 0 {
			summary.Skipped++
			continue
		}
		proxy := proxies[bestIndex]
		exit, ok := exitCache[proxy.ID]
		if !ok {
			exit = lookupListenerProxyExit(c.Request.Context(), proxy)
			exitCache[proxy.ID] = exit
		}
		if exit.IP != "" {
			_ = s.db.WithContext(c.Request.Context()).Model(&models.ListenerProxy{}).Where("tenant_id = ? AND id = ?", tenantID, proxy.ID).Updates(map[string]any{
				"exit_ip":    exit.IP,
				"country":    exit.Country,
				"flag":       exit.Flag,
				"updated_at": time.Now(),
			}).Error
		}
		if err := s.db.WithContext(c.Request.Context()).Model(&models.ListenerAccount{}).Where("tenant_id = ? AND id = ?", tenantID, account.ID).Updates(map[string]any{
			"proxy_id":     proxy.ID,
			"exit_ip":      exit.IP,
			"exit_country": exit.Country,
			"exit_flag":    exit.Flag,
			"updated_at":   now,
		}).Error; err != nil {
			return summary, err
		}
		usage[proxy.ID]++
		summary.Assigned++
	}
	for _, proxy := range proxies {
		_ = s.db.WithContext(c.Request.Context()).Model(&models.ListenerProxy{}).Where("tenant_id = ? AND id = ?", tenantID, proxy.ID).Updates(map[string]any{"bound_accounts": usage[proxy.ID], "updated_at": now}).Error
	}
	return summary, nil
}

func proxyIDsFromModels(proxies []models.ListenerProxy) []uuid.UUID {
	ids := make([]uuid.UUID, 0, len(proxies))
	for _, proxy := range proxies {
		ids = append(ids, proxy.ID)
	}
	return ids
}

func accountIDsFromModels(accounts []models.ListenerAccount) []uuid.UUID {
	ids := make([]uuid.UUID, 0, len(accounts))
	for _, account := range accounts {
		ids = append(ids, account.ID)
	}
	return ids
}

func (s *Server) resolveListenerGroup(c *gin.Context, resourceType string, groupIDText string, newGroupName string) (*uuid.UUID, string, error) {
	if groupIDText != "" && newGroupName != "" {
		return nil, "", fmt.Errorf("请选择已有分组或填写新分组，不能同时使用")
	}
	if newGroupName != "" {
		var existing models.Group
		err := s.db.WithContext(c.Request.Context()).
			Where("tenant_id = ? AND resource_type = ? AND name = ?", s.tenantID(c), resourceType, newGroupName).
			First(&existing).Error
		if err == nil {
			return &existing.ID, existing.Name, nil
		}
		if err != nil && err != gorm.ErrRecordNotFound {
			return nil, "", fmt.Errorf("读取分组失败")
		}
		group := models.Group{ID: uuid.New(), TenantID: s.tenantID(c), ResourceType: resourceType, Name: newGroupName}
		if err := s.db.WithContext(c.Request.Context()).Create(&group).Error; err != nil {
			return nil, "", fmt.Errorf("创建分组失败")
		}
		return &group.ID, group.Name, nil
	}
	if groupIDText == "" {
		return nil, "", nil
	}
	parsed, err := uuid.Parse(groupIDText)
	if err != nil {
		return nil, "", fmt.Errorf("分组 ID 无效")
	}
	var group models.Group
	if err := s.db.WithContext(c.Request.Context()).Where("tenant_id = ? AND id = ? AND resource_type = ?", s.tenantID(c), parsed, resourceType).First(&group).Error; err != nil {
		return nil, "", fmt.Errorf("分组不存在")
	}
	return &group.ID, group.Name, nil
}

func (s *Server) nextListenerProxyCode(c *gin.Context, tx *gorm.DB) (int64, error) {
	var count int64
	if err := tx.WithContext(c.Request.Context()).Model(&models.ListenerProxy{}).Count(&count).Error; err != nil {
		return 0, err
	}
	return count + 1, nil
}

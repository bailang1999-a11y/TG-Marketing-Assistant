package handlers

import (
	"fmt"
	"strings"

	"codex3/backend/internal/models"

	"github.com/google/uuid"
)

type listenerAdminSummary struct {
	AccountCount  int64 `json:"account_count"`
	TargetCount   int64 `json:"target_count"`
	ProxyCount    int64 `json:"proxy_count"`
	AssignedCount int64 `json:"assigned_count"`
}

type listenerImportSummary struct {
	Success         int                         `json:"success"`
	Failed          int                         `json:"failed"`
	Duplicate       int                         `json:"duplicate"`
	Skipped         int                         `json:"skipped"`
	GroupID         *uuid.UUID                  `json:"group_id,omitempty"`
	GroupName       string                      `json:"group_name,omitempty"`
	Assignment      *listenerAdminAssignSummary `json:"assignment,omitempty"`
	AssignmentError string                      `json:"assignment_error,omitempty"`
	Items           []listenerImportResult      `json:"items"`
	CreatedIDs      []uuid.UUID                 `json:"-"`
}

type listenerImportResult struct {
	Line       string `json:"line"`
	Identifier string `json:"identifier,omitempty"`
	Status     string `json:"status"`
	Reason     string `json:"reason,omitempty"`
}

type listenerAccountUploadBuild struct {
	Units   []importUnit
	Items   []listenerImportResult
	Skipped int
	Failed  int
}

type listenerAdminAssignSummary struct {
	Accounts int `json:"accounts"`
	Proxies  int `json:"proxies"`
	Assigned int `json:"assigned"`
	Skipped  int `json:"skipped"`
}

type listenerAccountRow struct {
	models.ListenerAccount
	PhoneDisplay      string `json:"phone_display"`
	AvatarURL         string `json:"avatar_url"`
	JoinedTargetCount int64  `json:"joined_target_count"`
	TargetTotalCount  int64  `json:"target_total_count"`
	StatusText        string `json:"status_text"`
}

type listenerTargetRow struct {
	models.ListenerTarget
	GroupName string `json:"group_name"`
	TypeText  string `json:"type_text"`
}

type listenerProxyRow struct {
	models.ListenerProxy
	Endpoint          string `json:"endpoint"`
	BoundDisplay      string `json:"bound_display"`
	LocationDisplay   string `json:"location_display"`
	ProtocolDisplay   string `json:"protocol_display"`
	AssignmentLimit   int64  `json:"assignment_limit"`
	AssignmentPercent int64  `json:"assignment_percent"`
}

type listenerCheckSummary struct {
	Total    int `json:"total"`
	Normal   int `json:"normal"`
	Abnormal int `json:"abnormal"`
	Offline  int `json:"offline"`
}

type listenerProxyCheckSummary struct {
	Total   int `json:"total"`
	Normal  int `json:"normal"`
	Failed  int `json:"failed"`
	Timeout int `json:"timeout"`
}

type listenerTargetRefreshSummary struct {
	TaskID  string                 `json:"task_id,omitempty"`
	Total   int                    `json:"total"`
	Success int                    `json:"success"`
	Failed  int                    `json:"failed"`
	Items   []joinTargetsResultRow `json:"items,omitempty"`
}

func parseListenerAccountLine(line string) (string, string) {
	line = strings.TrimSpace(line)
	if phone, nickname := parseStructuredAccountName(line); phone != "" {
		return phone, nickname
	}
	parts := strings.Fields(strings.ReplaceAll(line, ",", " "))
	if len(parts) == 0 {
		return "", ""
	}
	phone := parts[0]
	nicknameStart := 1
	if strings.HasPrefix(phone, "+") && len(parts) > 1 && isDigits(parts[1]) {
		phone = phone + " " + parts[1]
		nicknameStart = 2
	}
	nickname := ""
	if len(parts) > nicknameStart {
		nickname = strings.Join(parts[nicknameStart:], " ")
	}
	return phone, nickname
}

func parseStructuredAccountName(line string) (string, string) {
	if line == "" {
		return "", ""
	}
	if isDigits(line) {
		return line, ""
	}
	if !strings.HasPrefix(line, "+") {
		return "", ""
	}
	country := ""
	index := 1
	for index < len(line) && line[index] >= '0' && line[index] <= '9' {
		country += string(line[index])
		index++
	}
	if country == "" {
		return "", ""
	}
	rest := line[index:]
	groups := digitGroups(rest)
	number := ""
	for _, group := range groups {
		if len(group) >= 5 {
			number = group
		}
	}
	if number == "" {
		return "", ""
	}
	return "+" + country + " " + number, line
}

func digitGroups(value string) []string {
	groups := []string{}
	current := strings.Builder{}
	for _, char := range value {
		if char >= '0' && char <= '9' {
			current.WriteRune(char)
			continue
		}
		if current.Len() > 0 {
			groups = append(groups, current.String())
			current.Reset()
		}
	}
	if current.Len() > 0 {
		groups = append(groups, current.String())
	}
	return groups
}

func isDigits(value string) bool {
	if value == "" {
		return false
	}
	for _, char := range value {
		if char < '0' || char > '9' {
			return false
		}
	}
	return true
}

func listenerAccountStatusText(status, riskStatus string) string {
	status = strings.ToLower(strings.TrimSpace(status))
	riskStatus = strings.TrimSpace(riskStatus)
	if riskStatus != "" && riskStatus != "正常" && riskStatus != "unknown" {
		return riskStatus
	}
	switch status {
	case "normal", "online":
		return "正常"
	case "offline":
		return "会话有效"
	case "abnormal", "failed":
		return "异常"
	case "unchecked", "":
		return "未检测"
	default:
		return status
	}
}

func listenerNormalizeAccountStatus(status string) string {
	switch strings.ToLower(strings.TrimSpace(status)) {
	case "online", "normal", "success", "active":
		return "normal"
	case "offline":
		return "offline"
	case "unchecked", "":
		return "normal"
	default:
		return "abnormal"
	}
}

func isListenerAccountNormal(status, riskStatus string) bool {
	normalized := strings.ToLower(strings.TrimSpace(status))
	risk := strings.TrimSpace(riskStatus)
	return (normalized == "normal" || normalized == "online") && (risk == "" || risk == "正常" || risk == "unknown")
}

func listenerTargetTypeText(value string) string {
	switch strings.ToLower(strings.TrimSpace(value)) {
	case "channel":
		return "频道"
	case "group", "supergroup":
		return "群组"
	default:
		if strings.TrimSpace(value) == "" {
			return "未知"
		}
		return value
	}
}

func listenerProxyProtocolText(value string) string {
	switch strings.ToLower(strings.TrimSpace(value)) {
	case "socks5", "sk5":
		return "SOCKS5"
	case "http", "https":
		return strings.ToUpper(strings.TrimSpace(value))
	default:
		if strings.TrimSpace(value) == "" {
			return "未知"
		}
		return strings.ToUpper(strings.TrimSpace(value))
	}
}

func minInt64(a, b int64) int64 {
	if a < b {
		return a
	}
	return b
}

func listenerProxyRowFromModel(item models.ListenerProxy) listenerProxyRow {
	bound := item.BoundAccounts
	if bound > listenerProxyAssignmentLimit {
		bound = listenerProxyAssignmentLimit
	}
	location := strings.TrimSpace(strings.TrimSpace(item.Country) + " " + strings.TrimSpace(item.Flag))
	if location == "" {
		location = "未知"
	}
	return listenerProxyRow{
		ListenerProxy:     item,
		Endpoint:          fmt.Sprintf("%s:%d", item.IP, item.Port),
		BoundDisplay:      fmt.Sprintf("%d/%d", bound, listenerProxyAssignmentLimit),
		LocationDisplay:   location,
		ProtocolDisplay:   listenerProxyProtocolText(item.Protocol),
		AssignmentLimit:   listenerProxyAssignmentLimit,
		AssignmentPercent: bound * 100 / listenerProxyAssignmentLimit,
	}
}

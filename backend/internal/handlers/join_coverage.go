package handlers

import (
	"context"
	"hash/fnv"
	"sort"
	"strings"
	"time"

	"codex3/backend/internal/models"

	"github.com/google/uuid"
	"gorm.io/gorm/clause"
)

const (
	accountJoinKindTerminal = "terminal"
	accountJoinKindListener = "listener"

	accountTargetStatusActive             = "active"
	accountTargetStatusAccountUnavailable = "account_unavailable"
	accountTargetStatusNotMember          = "not_member"
	accountTargetStatusKicked             = "kicked"
	accountTargetStatusBanned             = "banned"
	accountTargetStatusInaccessible       = "inaccessible"
	accountTargetStatusTargetInvalid      = "target_invalid"
	accountTargetStatusCheckFailed        = "check_failed"
)

func accountTargetJoinKey(targetType string, targetValue string) string {
	value := strings.TrimSpace(strings.ToLower(targetValue))
	if value == "" {
		return ""
	}
	return strings.Join([]string{strings.TrimSpace(strings.ToLower(targetType)), value}, ":")
}

func (s *Server) sortTargetsByJoinCoverage(ctx context.Context, tenantID uuid.UUID, accountKind string, targets []models.Target) []models.Target {
	return s.sortTargetsByJoinCoverageForAccount(ctx, tenantID, accountKind, uuid.Nil, targets)
}

func (s *Server) sortTargetsByJoinCoverageForAccount(ctx context.Context, tenantID uuid.UUID, accountKind string, accountID uuid.UUID, targets []models.Target) []models.Target {
	if len(targets) == 0 {
		return targets
	}
	keys := make([]string, 0, len(targets))
	for _, target := range targets {
		if key := accountTargetJoinKey(target.Type, target.Identifier); key != "" {
			keys = append(keys, key)
		}
	}
	if len(keys) == 0 {
		return targets
	}

	type coverageRow struct {
		TargetKey string
		Count     int64
	}
	var rows []coverageRow
	query := s.db.WithContext(ctx).
		Model(&models.AccountTargetJoin{}).
		Select("target_key, COUNT(*) AS count").
		Where("account_kind = ? AND active = ? AND target_key IN ?", accountKind, true, keys).
		Group("target_key")
	if tenantID != uuid.Nil {
		query = query.Where("tenant_id = ?", tenantID)
	}
	_ = query.Scan(&rows).Error

	coverage := make(map[string]int64, len(rows))
	for _, row := range rows {
		coverage[row.TargetKey] = row.Count
	}

	sorted := append([]models.Target(nil), targets...)
	sort.SliceStable(sorted, func(i, j int) bool {
		leftKey := accountTargetJoinKey(sorted[i].Type, sorted[i].Identifier)
		rightKey := accountTargetJoinKey(sorted[j].Type, sorted[j].Identifier)
		if coverage[leftKey] == coverage[rightKey] {
			if accountID != uuid.Nil {
				return accountTargetTieBreakRank(accountID, sorted[i]) < accountTargetTieBreakRank(accountID, sorted[j])
			}
			return sorted[i].CreatedAt.Before(sorted[j].CreatedAt)
		}
		return coverage[leftKey] < coverage[rightKey]
	})
	return sorted
}

func accountTargetTieBreakRank(accountID uuid.UUID, target models.Target) uint64 {
	key := accountTargetJoinKey(target.Type, target.Identifier)
	if key == "" {
		key = target.ID.String()
	}
	h := fnv.New64a()
	_, _ = h.Write([]byte(accountID.String()))
	_, _ = h.Write([]byte{0})
	_, _ = h.Write([]byte(key))
	return h.Sum64()
}

func (s *Server) sortTerminalsByTargetCoverage(ctx context.Context, tenantID uuid.UUID, accountKind string, terminals []models.Terminal, targets []models.Target) []models.Terminal {
	if len(terminals) == 0 || len(targets) == 0 {
		return terminals
	}
	targetKeys := make([]string, 0, len(targets))
	targetKeySet := map[string]struct{}{}
	for _, target := range targets {
		key := accountTargetJoinKey(target.Type, target.Identifier)
		if key == "" {
			continue
		}
		if _, ok := targetKeySet[key]; ok {
			continue
		}
		targetKeySet[key] = struct{}{}
		targetKeys = append(targetKeys, key)
	}
	if len(targetKeys) == 0 {
		return terminals
	}
	accountIDs := make([]uuid.UUID, 0, len(terminals))
	for _, terminal := range terminals {
		accountIDs = append(accountIDs, terminal.ID)
	}

	type joinRow struct {
		AccountID uuid.UUID
		TargetKey string
	}
	var rows []joinRow
	query := s.db.WithContext(ctx).
		Model(&models.AccountTargetJoin{}).
		Select("account_id, target_key").
		Where("account_kind = ? AND active = ? AND status = ? AND account_id IN ? AND target_key IN ?", accountKind, true, accountTargetStatusActive, accountIDs, targetKeys)
	if tenantID != uuid.Nil {
		query = query.Where("tenant_id = ?", tenantID)
	}
	_ = query.Scan(&rows).Error

	coverageByAccount := make(map[uuid.UUID]map[string]struct{}, len(terminals))
	for _, row := range rows {
		if row.TargetKey == "" {
			continue
		}
		if coverageByAccount[row.AccountID] == nil {
			coverageByAccount[row.AccountID] = map[string]struct{}{}
		}
		coverageByAccount[row.AccountID][row.TargetKey] = struct{}{}
	}
	if len(coverageByAccount) == 0 {
		return terminals
	}

	remaining := append([]models.Terminal(nil), terminals...)
	selected := make([]models.Terminal, 0, len(terminals))
	covered := map[string]struct{}{}
	for len(remaining) > 0 {
		bestIndex := -1
		bestGain := 0
		bestTotal := 0
		for index, terminal := range remaining {
			keys := coverageByAccount[terminal.ID]
			gain := 0
			for key := range keys {
				if _, ok := covered[key]; !ok {
					gain++
				}
			}
			total := len(keys)
			if gain > bestGain || (gain == bestGain && total > bestTotal) {
				bestIndex = index
				bestGain = gain
				bestTotal = total
			}
		}
		if bestIndex < 0 || bestGain == 0 {
			break
		}
		chosen := remaining[bestIndex]
		selected = append(selected, chosen)
		for key := range coverageByAccount[chosen.ID] {
			covered[key] = struct{}{}
		}
		remaining = append(remaining[:bestIndex], remaining[bestIndex+1:]...)
	}
	if len(selected) == 0 {
		return terminals
	}
	selectedIDs := make(map[uuid.UUID]struct{}, len(selected))
	for _, terminal := range selected {
		selectedIDs[terminal.ID] = struct{}{}
	}
	for _, terminal := range terminals {
		if _, ok := selectedIDs[terminal.ID]; ok {
			continue
		}
		selected = append(selected, terminal)
	}
	return selected
}

func (s *Server) recordAccountTargetJoin(ctx context.Context, tenantID uuid.UUID, accountKind string, accountID uuid.UUID, target models.Target, sourceTaskID *uuid.UUID) {
	key := accountTargetJoinKey(target.Type, target.Identifier)
	if key == "" {
		return
	}
	now := time.Now()
	join := models.AccountTargetJoin{
		ID:           uuid.New(),
		TenantID:     tenantID,
		AccountKind:  accountKind,
		AccountID:    accountID,
		TargetID:     &target.ID,
		TargetType:   target.Type,
		TargetValue:  target.Identifier,
		TargetKey:    key,
		SourceTaskID: sourceTaskID,
		Status:       accountTargetStatusActive,
		StatusReason: "",
		Active:       true,
		JoinedAt:     now,
		LastSeenAt:   &now,
		CreatedAt:    now,
		UpdatedAt:    now,
	}
	_ = s.db.WithContext(ctx).Clauses(clause.OnConflict{
		Columns: []clause.Column{
			{Name: "tenant_id"},
			{Name: "account_kind"},
			{Name: "account_id"},
			{Name: "target_key"},
		},
		DoUpdates: clause.Assignments(map[string]any{
			"target_id":      join.TargetID,
			"target_type":    join.TargetType,
			"target_value":   join.TargetValue,
			"source_task_id": join.SourceTaskID,
			"status":         join.Status,
			"status_reason":  join.StatusReason,
			"active":         join.Active,
			"joined_at":      join.JoinedAt,
			"last_seen_at":   join.LastSeenAt,
			"removed_at":     nil,
			"updated_at":     join.UpdatedAt,
		}),
	}).Create(&join).Error
}

func (s *Server) accountTargetAlreadyJoined(ctx context.Context, tenantID uuid.UUID, accountKind string, accountID uuid.UUID, target models.Target) bool {
	key := accountTargetJoinKey(target.Type, target.Identifier)
	if key == "" {
		return false
	}
	var count int64
	query := s.db.WithContext(ctx).
		Model(&models.AccountTargetJoin{}).
		Where("account_kind = ? AND account_id = ? AND target_key = ? AND active = ?", accountKind, accountID, key, true)
	if tenantID != uuid.Nil {
		query = query.Where("tenant_id = ?", tenantID)
	}
	_ = query.Count(&count).Error
	return count > 0
}

func (s *Server) markAccountJoinRecordsUnavailable(ctx context.Context, tenantID uuid.UUID, accountKind string, accountID uuid.UUID, reason string) int64 {
	now := time.Now()
	query := s.db.WithContext(ctx).Model(&models.AccountTargetJoin{}).
		Where("account_kind = ? AND account_id = ? AND active = ?", accountKind, accountID, true)
	if tenantID != uuid.Nil {
		query = query.Where("tenant_id = ?", tenantID)
	}
	result := query.Updates(map[string]any{
		"status":          accountTargetStatusAccountUnavailable,
		"status_reason":   firstNonEmpty(strings.TrimSpace(reason), "账号不可用，已从目标群有效账号中移除"),
		"active":          false,
		"last_checked_at": now,
		"removed_at":      now,
		"updated_at":      now,
	})
	if result.Error != nil {
		return 0
	}
	return result.RowsAffected
}

func (s *Server) updateAccountTargetJoinMembership(ctx context.Context, join models.AccountTargetJoin, status string, reason string, active bool) {
	now := time.Now()
	normalizedStatus := normalizeAccountTargetMembershipStatus(status, active)
	updates := map[string]any{
		"status":          normalizedStatus,
		"status_reason":   strings.TrimSpace(reason),
		"active":          active,
		"last_checked_at": now,
		"updated_at":      now,
	}
	if active {
		updates["last_seen_at"] = now
		updates["removed_at"] = nil
	} else {
		updates["removed_at"] = now
	}
	_ = s.db.WithContext(ctx).Model(&models.AccountTargetJoin{}).Where("id = ?", join.ID).Updates(updates).Error
}

func (s *Server) countActiveAccountTargetJoins(ctx context.Context, tenantID uuid.UUID, accountKind string, accountID uuid.UUID) int64 {
	var count int64
	query := s.db.WithContext(ctx).Model(&models.AccountTargetJoin{}).
		Where("account_kind = ? AND account_id = ? AND active = ?", accountKind, accountID, true)
	if tenantID != uuid.Nil {
		query = query.Where("tenant_id = ?", tenantID)
	}
	_ = query.Count(&count).Error
	return count
}

func (s *Server) countCoveredListenerTargets(ctx context.Context, targets []models.Target) int64 {
	if len(targets) == 0 {
		return 0
	}
	keys := make([]string, 0, len(targets))
	seen := map[string]struct{}{}
	for _, target := range targets {
		key := accountTargetJoinKey(target.Type, target.Identifier)
		if key == "" {
			continue
		}
		if _, ok := seen[key]; ok {
			continue
		}
		seen[key] = struct{}{}
		keys = append(keys, key)
	}
	if len(keys) == 0 {
		return 0
	}
	var count int64
	_ = s.db.WithContext(ctx).Model(&models.AccountTargetJoin{}).
		Where("account_kind = ? AND active = ? AND status = ? AND target_key IN ?", accountJoinKindListener, true, accountTargetStatusActive, keys).
		Distinct("target_key").
		Count(&count).Error
	return count
}

func normalizeAccountTargetMembershipStatus(status string, active bool) string {
	normalized := strings.TrimSpace(strings.ToLower(status))
	if normalized == "" {
		if active {
			return accountTargetStatusActive
		}
		return accountTargetStatusCheckFailed
	}
	return normalized
}

func accountTargetMembershipActive(status string) bool {
	return normalizeAccountTargetMembershipStatus(status, false) == accountTargetStatusActive
}

func accountTargetMembershipStatusText(status string, active bool) string {
	switch normalizeAccountTargetMembershipStatus(status, active) {
	case accountTargetStatusActive:
		return "仍在群内"
	case accountTargetStatusAccountUnavailable:
		return "账号不可用"
	case accountTargetStatusNotMember:
		return "不在群内"
	case accountTargetStatusKicked:
		return "已被踢出"
	case accountTargetStatusBanned:
		return "已被限制"
	case accountTargetStatusInaccessible:
		return "目标不可访问"
	case accountTargetStatusTargetInvalid:
		return "目标无效"
	case "flood_wait":
		return "限流待复查"
	default:
		if active {
			return "仍在群内"
		}
		return "待复查"
	}
}

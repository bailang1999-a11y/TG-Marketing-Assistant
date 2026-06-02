package models

import (
	"time"

	"github.com/google/uuid"
	"gorm.io/datatypes"
)

type Task struct {
	ID              uuid.UUID      `json:"id" gorm:"type:uuid;primaryKey"`
	TenantID        uuid.UUID      `json:"tenant_id" gorm:"type:uuid;index;index:idx_tasks_tenant_created,priority:1;index:idx_tasks_tenant_status_created,priority:1;index:idx_tasks_tenant_type_created,priority:1;not null"`
	Name            string         `json:"name" gorm:"size:180;index"`
	Type            string         `json:"type" gorm:"size:60;index;index:idx_tasks_tenant_type_created,priority:2;not null"`
	TerminalGroupID *uuid.UUID     `json:"terminal_group_id" gorm:"type:uuid;index"`
	TargetGroupID   *uuid.UUID     `json:"target_group_id" gorm:"type:uuid;index"`
	Status          string         `json:"status" gorm:"size:30;index;index:idx_tasks_tenant_status_created,priority:2;not null"`
	Progress        int            `json:"progress"`
	Payload         datatypes.JSON `json:"payload" gorm:"type:jsonb"`
	Summary         datatypes.JSON `json:"summary" gorm:"type:jsonb"`
	RunID           string         `json:"run_id" gorm:"size:80;index"`
	RunLockedAt     *time.Time     `json:"run_locked_at" gorm:"index"`
	CreatedBy       *uuid.UUID     `json:"created_by" gorm:"type:uuid"`
	CreatedAt       time.Time      `json:"created_at" gorm:"index:idx_tasks_tenant_created,priority:2;index:idx_tasks_tenant_status_created,priority:3;index:idx_tasks_tenant_type_created,priority:3"`
	UpdatedAt       time.Time      `json:"updated_at" gorm:"index"`
}

type TaskLog struct {
	ID          uuid.UUID `json:"id" gorm:"type:uuid;primaryKey"`
	TenantID    uuid.UUID `json:"tenant_id" gorm:"type:uuid;index;index:idx_task_logs_tenant_created,priority:1;index:idx_task_logs_tenant_task_created,priority:1;index:idx_task_logs_tenant_level_created,priority:1;index:idx_task_logs_tenant_category_created,priority:1;not null"`
	TaskID      uuid.UUID `json:"task_id" gorm:"type:uuid;index;index:idx_task_logs_tenant_task_created,priority:2;not null"`
	Level       string    `json:"level" gorm:"size:20;index;index:idx_task_logs_tenant_level_created,priority:2;not null"`
	Category    string    `json:"category" gorm:"size:60;index;index:idx_task_logs_tenant_category_created,priority:2"`
	TerminalRef string    `json:"terminal_ref" gorm:"size:160"`
	TargetRef   string    `json:"target_ref" gorm:"size:160"`
	Action      string    `json:"action" gorm:"size:120"`
	Details     string    `json:"details" gorm:"size:5000"`
	DurationMS  int64     `json:"duration_ms"`
	TraceID     string    `json:"trace_id" gorm:"size:80;index"`
	CreatedAt   time.Time `json:"created_at" gorm:"index;index:idx_task_logs_tenant_created,priority:2;index:idx_task_logs_tenant_task_created,priority:3;index:idx_task_logs_tenant_level_created,priority:3;index:idx_task_logs_tenant_category_created,priority:3"`
}

package domain

import (
	"encoding/json"
	"strings"
	"time"
)

type ReleaseOperation string

const (
	OperationPublish  ReleaseOperation = "publish"
	OperationRollback ReleaseOperation = "rollback"
)

type Release struct {
	ID            int64            `json:"id"`
	ApplicationID int64            `json:"application_id"`
	EnvironmentID int64            `json:"environment_id"`
	Version       int64            `json:"version"`
	Items         []ConfigItem     `json:"items,omitempty"`
	Content       json.RawMessage  `json:"content,omitempty"`
	Checksum      string           `json:"checksum"`
	ChangeSummary string           `json:"change_summary"`
	SourceVersion *int64           `json:"source_version,omitempty"`
	Operation     ReleaseOperation `json:"operation"`
	Operator      string           `json:"operator"`
	CreatedAt     time.Time        `json:"created_at"`
}

type AuditLog struct {
	ID           int64     `json:"id"`
	ResourceType string    `json:"resource_type"`
	ResourceID   int64     `json:"resource_id"`
	Action       string    `json:"action"`
	Operator     string    `json:"operator"`
	RequestID    string    `json:"request_id"`
	Summary      string    `json:"summary"`
	CreatedAt    time.Time `json:"created_at"`
}

func ValidateReleaseInput(summary, operator string) error {
	if len(strings.TrimSpace(summary)) < 2 || len(summary) > 300 {
		return NewError(CodeInvalid, "change summary must contain 2 to 300 characters")
	}
	if len(strings.TrimSpace(operator)) < 2 || len(operator) > 80 {
		return NewError(CodeInvalid, "operator must contain 2 to 80 characters")
	}
	return nil
}

type ReleasePage struct {
	Items  []Release `json:"items"`
	Limit  int       `json:"limit"`
	Offset int       `json:"offset"`
	Total  int       `json:"total"`
}

func NewPublishRelease(appID, envID, version int64, items []ConfigItem, content []byte, checksum, summary, operator string) Release {
	return Release{
		ApplicationID: appID,
		EnvironmentID: envID,
		Version:       version,
		Items:         items,
		Content:       content,
		Checksum:      checksum,
		ChangeSummary: strings.TrimSpace(summary),
		Operation:     OperationPublish,
		Operator:      strings.TrimSpace(operator),
		CreatedAt:     time.Now().UTC(),
	}
}

package repository

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"time"

	"configcenter/internal/domain"
)

func (s *SQLite) GetDraft(ctx context.Context, environmentID int64) ([]domain.ConfigItem, int64, error) {
	var revision int64
	if err := s.db.QueryRowContext(ctx, `SELECT draft_revision FROM environments WHERE id = ?`, environmentID).Scan(&revision); err != nil {
		return nil, 0, mapSQLError(err, "environment")
	}
	items, err := readDraftItems(ctx, s.db, environmentID)
	return items, revision, err
}

type queryer interface {
	QueryContext(context.Context, string, ...any) (*sql.Rows, error)
}

func readDraftItems(ctx context.Context, q queryer, environmentID int64) ([]domain.ConfigItem, error) {
	rows, err := q.QueryContext(ctx, `SELECT config_key, config_value, value_type, description, sensitive
 FROM draft_items WHERE environment_id = ? ORDER BY config_key`, environmentID)
	if err != nil {
		return nil, mapSQLError(err, "draft")
	}
	defer rows.Close()
	items := make([]domain.ConfigItem, 0)
	for rows.Next() {
		var item domain.ConfigItem
		if err := rows.Scan(&item.Key, &item.Value, &item.Type, &item.Description, &item.Sensitive); err != nil {
			return nil, mapSQLError(err, "draft")
		}
		items = append(items, item)
	}
	return items, mapSQLError(rows.Err(), "draft")
}

func (s *SQLite) SaveDraft(ctx context.Context, environmentID, expectedRevision int64, items []domain.ConfigItem, operator, requestID string) (int64, error) {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return 0, mapSQLError(err, "draft")
	}
	defer tx.Rollback()
	result, err := tx.ExecContext(ctx, `UPDATE environments SET draft_revision = draft_revision + 1,
 updated_at = ? WHERE id = ? AND draft_revision = ?`, timeText(time.Now()), environmentID, expectedRevision)
	if err != nil {
		return 0, mapSQLError(err, "draft")
	}
	if affected, _ := result.RowsAffected(); affected == 0 {
		var exists int
		if err := tx.QueryRowContext(ctx, `SELECT 1 FROM environments WHERE id = ?`, environmentID).Scan(&exists); err != nil {
			return 0, mapSQLError(err, "environment")
		}
		return 0, domain.NewError(domain.CodeRevisionConflict, "draft revision has changed")
	}
	if _, err := tx.ExecContext(ctx, `DELETE FROM draft_items WHERE environment_id = ?`, environmentID); err != nil {
		return 0, mapSQLError(err, "draft")
	}
	if err := insertDraftItems(ctx, tx, environmentID, items); err != nil {
		return 0, err
	}
	if err := insertAudit(ctx, tx, "environment", environmentID, "draft.save", operator, requestID,
		fmt.Sprintf("saved %d configuration items", len(items))); err != nil {
		return 0, err
	}
	if err := tx.Commit(); err != nil {
		return 0, mapSQLError(err, "draft")
	}
	return expectedRevision + 1, nil
}

func insertDraftItems(ctx context.Context, tx *sql.Tx, environmentID int64, items []domain.ConfigItem) error {
	statement, err := tx.PrepareContext(ctx, `INSERT INTO draft_items
 (environment_id, config_key, config_value, value_type, description, sensitive) VALUES (?, ?, ?, ?, ?, ?)`)
	if err != nil {
		return mapSQLError(err, "draft")
	}
	defer statement.Close()
	for _, item := range items {
		if _, err := statement.ExecContext(ctx, environmentID, item.Key, item.Value, item.Type,
			item.Description, item.Sensitive); err != nil {
			return mapSQLError(err, "draft")
		}
	}
	return nil
}

func (s *SQLite) GetRelease(ctx context.Context, environmentID, version int64) (domain.Release, error) {
	row := s.db.QueryRowContext(ctx, releaseSelect+` WHERE environment_id = ? AND version = ?`, environmentID, version)
	release, err := scanRelease(row)
	return release, mapSQLError(err, "release")
}

func (s *SQLite) GetCurrentRelease(ctx context.Context, environmentID int64) (domain.Release, error) {
	row := s.db.QueryRowContext(ctx, releaseSelect+` WHERE environment_id = ? ORDER BY version DESC LIMIT 1`, environmentID)
	release, err := scanRelease(row)
	if err == sql.ErrNoRows {
		return domain.Release{}, domain.NewError(domain.CodeNotPublished, "configuration has not been published")
	}
	return release, mapSQLError(err, "release")
}

func (s *SQLite) ListReleases(ctx context.Context, environmentID int64, limit, offset int) (domain.ReleasePage, error) {
	var total int
	if err := s.db.QueryRowContext(ctx, `SELECT COUNT(*) FROM releases WHERE environment_id = ?`, environmentID).Scan(&total); err != nil {
		return domain.ReleasePage{}, mapSQLError(err, "releases")
	}
	rows, err := s.db.QueryContext(ctx, releaseSelect+` WHERE environment_id = ? ORDER BY version DESC LIMIT ? OFFSET ?`,
		environmentID, limit, offset)
	if err != nil {
		return domain.ReleasePage{}, mapSQLError(err, "releases")
	}
	defer rows.Close()
	items := make([]domain.Release, 0, limit)
	for rows.Next() {
		item, err := scanRelease(rows)
		if err != nil {
			return domain.ReleasePage{}, mapSQLError(err, "release")
		}
		item.Items = nil
		item.Content = nil
		items = append(items, item)
	}
	return domain.ReleasePage{Items: items, Limit: limit, Offset: offset, Total: total}, mapSQLError(rows.Err(), "releases")
}

const releaseSelect = `SELECT id, application_id, environment_id, version, items_json, content,
 checksum, change_summary, source_version, operation, operator, created_at FROM releases`

func scanRelease(row scanner) (domain.Release, error) {
	var item domain.Release
	var itemsJSON []byte
	var source sql.NullInt64
	var created string
	err := row.Scan(&item.ID, &item.ApplicationID, &item.EnvironmentID, &item.Version, &itemsJSON,
		&item.Content, &item.Checksum, &item.ChangeSummary, &source, &item.Operation, &item.Operator, &created)
	if err != nil {
		return domain.Release{}, err
	}
	if err := json.Unmarshal(itemsJSON, &item.Items); err != nil {
		return domain.Release{}, err
	}
	if source.Valid {
		item.SourceVersion = &source.Int64
	}
	item.CreatedAt = parseTime(created)
	return item, nil
}

func (s *SQLite) WriteRelease(ctx context.Context, write ReleaseWrite) (domain.Release, error) {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return domain.Release{}, mapSQLError(err, "release")
	}
	defer tx.Rollback()
	itemsJSON, err := json.Marshal(write.Release.Items)
	if err != nil {
		return domain.Release{}, domain.WrapError(domain.CodeInternal, "encode release items", err)
	}
	result, err := tx.ExecContext(ctx, `INSERT INTO releases
 (application_id, environment_id, version, items_json, content, checksum, change_summary,
 source_version, operation, operator, created_at) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		write.Release.ApplicationID, write.Release.EnvironmentID, write.Release.Version, itemsJSON,
		write.Release.Content, write.Release.Checksum, write.Release.ChangeSummary, write.Release.SourceVersion,
		write.Release.Operation, write.Release.Operator, timeText(write.Release.CreatedAt))
	if err != nil {
		return domain.Release{}, mapSQLError(err, "release")
	}
	id, _ := result.LastInsertId()
	updated, err := tx.ExecContext(ctx, `UPDATE environments SET current_version = ?, last_published_at = ?,
 updated_at = ? WHERE id = ? AND current_version = ?`, write.Release.Version, timeText(write.Release.CreatedAt),
		timeText(write.Release.CreatedAt), write.Release.EnvironmentID, write.ExpectedVersion)
	if err != nil {
		return domain.Release{}, mapSQLError(err, "environment")
	}
	if affected, _ := updated.RowsAffected(); affected == 0 {
		return domain.Release{}, domain.NewError(domain.CodeVersionConflict, "current version has changed")
	}
	if write.SyncDraft {
		if _, err := tx.ExecContext(ctx, `DELETE FROM draft_items WHERE environment_id = ?`, write.Release.EnvironmentID); err != nil {
			return domain.Release{}, mapSQLError(err, "draft")
		}
		if err := insertDraftItems(ctx, tx, write.Release.EnvironmentID, write.Release.Items); err != nil {
			return domain.Release{}, err
		}
		if _, err := tx.ExecContext(ctx, `UPDATE environments SET draft_revision = draft_revision + 1 WHERE id = ?`,
			write.Release.EnvironmentID); err != nil {
			return domain.Release{}, mapSQLError(err, "draft")
		}
	}
	if err := insertAudit(ctx, tx, "environment", write.Release.EnvironmentID, string(write.Release.Operation),
		write.Release.Operator, write.RequestID, write.AuditSummary); err != nil {
		return domain.Release{}, err
	}
	if err := tx.Commit(); err != nil {
		return domain.Release{}, mapSQLError(err, "release")
	}
	write.Release.ID = id
	return write.Release, nil
}

func insertAudit(ctx context.Context, tx *sql.Tx, resourceType string, resourceID int64, action, operator, requestID, summary string) error {
	_, err := tx.ExecContext(ctx, `INSERT INTO audit_logs
 (resource_type, resource_id, action, operator, request_id, summary, created_at) VALUES (?, ?, ?, ?, ?, ?, ?)`,
		resourceType, resourceID, action, operator, requestID, summary, timeText(time.Now()))
	return mapSQLError(err, "audit")
}

func (s *SQLite) ListAudits(ctx context.Context, limit, offset int) ([]domain.AuditLog, error) {
	rows, err := s.db.QueryContext(ctx, `SELECT id, resource_type, resource_id, action, operator,
 request_id, summary, created_at FROM audit_logs ORDER BY id DESC LIMIT ? OFFSET ?`, limit, offset)
	if err != nil {
		return nil, mapSQLError(err, "audits")
	}
	defer rows.Close()
	items := make([]domain.AuditLog, 0, limit)
	for rows.Next() {
		var item domain.AuditLog
		var created string
		if err := rows.Scan(&item.ID, &item.ResourceType, &item.ResourceID, &item.Action, &item.Operator,
			&item.RequestID, &item.Summary, &created); err != nil {
			return nil, mapSQLError(err, "audit")
		}
		item.CreatedAt = parseTime(created)
		items = append(items, item)
	}
	return items, mapSQLError(rows.Err(), "audits")
}

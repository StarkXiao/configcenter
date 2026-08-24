package repository

import (
	"context"
	"database/sql"
	"strings"
	"time"

	"configcenter/internal/domain"
)

func (s *SQLite) CreateApplication(ctx context.Context, input CreateApplicationInput) (domain.Application, error) {
	now := time.Now().UTC()
	result, err := s.db.ExecContext(ctx, `INSERT INTO applications
 (name, slug, description, access_token_hash, status, created_at, updated_at)
 VALUES (?, ?, ?, ?, ?, ?, ?)`, input.Name, input.Slug, input.Description,
		input.AccessTokenHash, domain.ApplicationActive, timeText(now), timeText(now))
	if err != nil {
		return domain.Application{}, mapSQLError(err, "application")
	}
	id, _ := result.LastInsertId()
	return s.GetApplicationByID(ctx, id)
}

func (s *SQLite) ListApplications(ctx context.Context, query, status string) ([]domain.Application, error) {
	statement := `SELECT id, name, slug, description, access_token_hash, status, created_at, updated_at
 FROM applications WHERE (? = '' OR lower(name) LIKE ? OR slug LIKE ?)
 AND (? = '' OR status = ?) ORDER BY updated_at DESC`
	like := "%" + strings.ToLower(query) + "%"
	rows, err := s.db.QueryContext(ctx, statement, query, like, like, status, status)
	if err != nil {
		return nil, mapSQLError(err, "applications")
	}
	defer rows.Close()
	applications := make([]domain.Application, 0)
	for rows.Next() {
		application, err := scanApplication(rows)
		if err != nil {
			return nil, mapSQLError(err, "application")
		}
		applications = append(applications, application)
	}
	return applications, mapSQLError(rows.Err(), "applications")
}

func (s *SQLite) GetApplication(ctx context.Context, slug string) (domain.Application, error) {
	row := s.db.QueryRowContext(ctx, `SELECT id, name, slug, description, access_token_hash,
 status, created_at, updated_at FROM applications WHERE slug = ?`, slug)
	application, err := scanApplication(row)
	return application, mapSQLError(err, "application")
}

func (s *SQLite) GetApplicationByID(ctx context.Context, id int64) (domain.Application, error) {
	row := s.db.QueryRowContext(ctx, `SELECT id, name, slug, description, access_token_hash,
 status, created_at, updated_at FROM applications WHERE id = ?`, id)
	application, err := scanApplication(row)
	return application, mapSQLError(err, "application")
}

type scanner interface{ Scan(...any) error }

func scanApplication(row scanner) (domain.Application, error) {
	var application domain.Application
	var created, updated string
	err := row.Scan(&application.ID, &application.Name, &application.Slug, &application.Description,
		&application.AccessTokenHash, &application.Status, &created, &updated)
	application.CreatedAt = parseTime(created)
	application.UpdatedAt = parseTime(updated)
	return application, err
}

func (s *SQLite) ResetApplicationToken(ctx context.Context, id int64, hash, operator, requestID string) error {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return mapSQLError(err, "application")
	}
	defer tx.Rollback()
	result, err := tx.ExecContext(ctx, `UPDATE applications SET access_token_hash = ?, updated_at = ? WHERE id = ?`,
		hash, timeText(time.Now()), id)
	if err != nil {
		return mapSQLError(err, "application")
	}
	if affected, _ := result.RowsAffected(); affected == 0 {
		return domain.NewError(domain.CodeNotFound, "application not found")
	}
	if err := insertAudit(ctx, tx, "application", id, "token.reset", operator, requestID, "client token reset"); err != nil {
		return err
	}
	return mapSQLError(tx.Commit(), "application")
}

func (s *SQLite) CreateEnvironment(ctx context.Context, input CreateEnvironmentInput) (domain.Environment, error) {
	now := time.Now().UTC()
	result, err := s.db.ExecContext(ctx, `INSERT INTO environments
 (application_id, name, code, description, created_at, updated_at) VALUES (?, ?, ?, ?, ?, ?)`,
		input.ApplicationID, input.Name, input.Code, input.Description, timeText(now), timeText(now))
	if err != nil {
		return domain.Environment{}, mapSQLError(err, "environment")
	}
	id, _ := result.LastInsertId()
	return s.getEnvironmentByID(ctx, id)
}

func (s *SQLite) ListEnvironments(ctx context.Context, applicationID int64) ([]domain.Environment, error) {
	rows, err := s.db.QueryContext(ctx, environmentSelect+` WHERE application_id = ? ORDER BY code`, applicationID)
	if err != nil {
		return nil, mapSQLError(err, "environments")
	}
	defer rows.Close()
	items := make([]domain.Environment, 0)
	for rows.Next() {
		item, err := scanEnvironment(rows)
		if err != nil {
			return nil, mapSQLError(err, "environment")
		}
		items = append(items, item)
	}
	return items, mapSQLError(rows.Err(), "environments")
}

func (s *SQLite) GetEnvironment(ctx context.Context, applicationID int64, code string) (domain.Environment, error) {
	row := s.db.QueryRowContext(ctx, environmentSelect+` WHERE application_id = ? AND code = ?`, applicationID, code)
	item, err := scanEnvironment(row)
	return item, mapSQLError(err, "environment")
}

func (s *SQLite) getEnvironmentByID(ctx context.Context, id int64) (domain.Environment, error) {
	row := s.db.QueryRowContext(ctx, environmentSelect+` WHERE id = ?`, id)
	item, err := scanEnvironment(row)
	return item, mapSQLError(err, "environment")
}

const environmentSelect = `SELECT id, application_id, name, code, description, current_version,
 draft_revision, last_published_at, created_at, updated_at FROM environments`

func scanEnvironment(row scanner) (domain.Environment, error) {
	var item domain.Environment
	var published sql.NullString
	var created, updated string
	err := row.Scan(&item.ID, &item.ApplicationID, &item.Name, &item.Code, &item.Description,
		&item.CurrentVersion, &item.DraftRevision, &published, &created, &updated)
	item.CreatedAt = parseTime(created)
	item.UpdatedAt = parseTime(updated)
	if published.Valid {
		value := parseTime(published.String)
		item.LastPublished = &value
	}
	return item, err
}

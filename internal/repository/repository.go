package repository

import (
	"context"

	"configcenter/internal/domain"
)

type CreateApplicationInput struct {
	Name            string
	Slug            string
	Description     string
	AccessTokenHash string
}

type CreateEnvironmentInput struct {
	ApplicationID int64
	Name          string
	Code          string
	Description   string
}

type ReleaseWrite struct {
	Release         domain.Release
	ExpectedVersion int64
	RequestID       string
	AuditSummary    string
	SyncDraft       bool
}

type Repository interface {
	Close() error
	Ping(context.Context) error
	CreateApplication(context.Context, CreateApplicationInput) (domain.Application, error)
	ListApplications(context.Context, string, string) ([]domain.Application, error)
	GetApplication(context.Context, string) (domain.Application, error)
	GetApplicationByID(context.Context, int64) (domain.Application, error)
	ResetApplicationToken(context.Context, int64, string, string, string) error
	CreateEnvironment(context.Context, CreateEnvironmentInput) (domain.Environment, error)
	ListEnvironments(context.Context, int64) ([]domain.Environment, error)
	GetEnvironment(context.Context, int64, string) (domain.Environment, error)
	GetDraft(context.Context, int64) ([]domain.ConfigItem, int64, error)
	SaveDraft(context.Context, int64, int64, []domain.ConfigItem, string, string) (int64, error)
	GetRelease(context.Context, int64, int64) (domain.Release, error)
	GetCurrentRelease(context.Context, int64) (domain.Release, error)
	ListReleases(context.Context, int64, int, int) (domain.ReleasePage, error)
	WriteRelease(context.Context, ReleaseWrite) (domain.Release, error)
	ListAudits(context.Context, int, int) ([]domain.AuditLog, error)
}

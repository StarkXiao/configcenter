package service

import (
	"context"
	"fmt"
	"strings"
	"time"

	"configcenter/internal/domain"
	"configcenter/internal/event"
	"configcenter/internal/repository"
)

type Releases struct {
	repository repository.Repository
	apps       *Applications
	hub        *event.Hub
}

func NewReleases(repo repository.Repository, apps *Applications, hub *event.Hub) *Releases {
	return &Releases{repository: repo, apps: apps, hub: hub}
}

func (s *Releases) Publish(ctx context.Context, slug, code, summary, operator, requestID string, expectedVersion int64) (domain.Release, error) {
	application, environment, err := s.apps.resolve(ctx, slug, code)
	if err != nil {
		return domain.Release{}, err
	}
	if !application.IsActive() {
		return domain.Release{}, domain.NewError(domain.CodeConflict, "application is disabled")
	}
	if environment.CurrentVersion != expectedVersion {
		return domain.Release{}, domain.NewError(domain.CodeVersionConflict, "current version has changed")
	}
	if err := domain.ValidateReleaseInput(summary, operator); err != nil {
		return domain.Release{}, err
	}
	items, _, err := s.repository.GetDraft(ctx, environment.ID)
	if err != nil {
		return domain.Release{}, err
	}
	if err := domain.ValidateItems(items); err != nil {
		return domain.Release{}, err
	}
	content, checksum, err := domain.CanonicalJSON(items)
	if err != nil {
		return domain.Release{}, err
	}
	if environment.HasRelease() {
		current, err := s.repository.GetCurrentRelease(ctx, environment.ID)
		if err != nil {
			return domain.Release{}, err
		}
		if current.Checksum == checksum {
			return domain.Release{}, domain.NewError(domain.CodeConflict, "draft has no published changes")
		}
	}
	release := domain.NewPublishRelease(application.ID, environment.ID, environment.NextVersion(), items,
		content, checksum, summary, operator)
	release, err = s.repository.WriteRelease(ctx, repository.ReleaseWrite{
		Release: release, ExpectedVersion: expectedVersion, RequestID: requestID,
		AuditSummary: fmt.Sprintf("published version %d: %s", release.Version, strings.TrimSpace(summary)),
	})
	if err != nil {
		return domain.Release{}, err
	}
	s.broadcast(ctx, slug, code, release)
	return release, nil
}

func (s *Releases) Rollback(ctx context.Context, slug, code string, targetVersion int64, reason, operator, requestID string, expectedVersion int64) (domain.Release, error) {
	application, environment, err := s.apps.resolve(ctx, slug, code)
	if err != nil {
		return domain.Release{}, err
	}
	if !application.IsActive() {
		return domain.Release{}, domain.NewError(domain.CodeConflict, "application is disabled")
	}
	if err := domain.ValidateReleaseInput(reason, operator); err != nil {
		return domain.Release{}, err
	}
	if environment.CurrentVersion != expectedVersion {
		return domain.Release{}, domain.NewError(domain.CodeVersionConflict, "current version has changed")
	}
	if targetVersion <= 0 || targetVersion == environment.CurrentVersion {
		return domain.Release{}, domain.NewError(domain.CodeInvalid, "select a non-current published version")
	}
	target, err := s.repository.GetRelease(ctx, environment.ID, targetVersion)
	if err != nil {
		return domain.Release{}, err
	}
	current, err := s.repository.GetCurrentRelease(ctx, environment.ID)
	if err != nil {
		return domain.Release{}, err
	}
	if target.Checksum == current.Checksum {
		return domain.Release{}, domain.NewError(domain.CodeConflict, "target content is already current")
	}
	now := time.Now().UTC()
	newVersion := environment.NextVersion()
	release := domain.Release{
		ApplicationID: application.ID, EnvironmentID: environment.ID, Version: newVersion,
		Items: target.Items, Content: target.Content, Checksum: target.Checksum,
		ChangeSummary: strings.TrimSpace(reason), SourceVersion: &targetVersion,
		Operation: domain.OperationRollback, Operator: strings.TrimSpace(operator), CreatedAt: now,
	}
	release, err = s.repository.WriteRelease(ctx, repository.ReleaseWrite{
		Release: release, ExpectedVersion: expectedVersion, RequestID: requestID, SyncDraft: true,
		AuditSummary: fmt.Sprintf("rolled back version %d to content from version %d: %s", newVersion, targetVersion, reason),
	})
	if err != nil {
		return domain.Release{}, err
	}
	s.broadcast(ctx, slug, code, release)
	return release, nil
}

func (s *Releases) List(ctx context.Context, slug, code string, limit, offset int) (domain.ReleasePage, error) {
	_, environment, err := s.apps.resolve(ctx, slug, code)
	if err != nil {
		return domain.ReleasePage{}, err
	}
	if limit <= 0 {
		limit = 20
	}
	if limit > 100 {
		limit = 100
	}
	if offset < 0 {
		offset = 0
	}
	return s.repository.ListReleases(ctx, environment.ID, limit, offset)
}

func (s *Releases) Get(ctx context.Context, slug, code string, version int64) (domain.Release, error) {
	_, environment, err := s.apps.resolve(ctx, slug, code)
	if err != nil {
		return domain.Release{}, err
	}
	return s.repository.GetRelease(ctx, environment.ID, version)
}

func (s *Releases) Compare(ctx context.Context, slug, code string, from, to int64) (domain.Diff, error) {
	left, err := s.Get(ctx, slug, code, from)
	if err != nil {
		return domain.Diff{}, err
	}
	right, err := s.Get(ctx, slug, code, to)
	if err != nil {
		return domain.Diff{}, err
	}
	return domain.Compare(left.Items, right.Items, false), nil
}

func (s *Releases) broadcast(ctx context.Context, slug, code string, release domain.Release) {
	go func() {
		select {
		case <-ctx.Done():
			return
		case <-time.After(time.Millisecond):
			s.hub.Publish(event.Event{Application: slug, Environment: code, Version: release.Version,
				Checksum: release.Checksum, Operation: string(release.Operation)})
		}
	}()
}

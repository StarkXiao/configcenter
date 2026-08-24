package service

import (
	"context"
	"crypto/subtle"
	"strings"

	"configcenter/internal/domain"
	"configcenter/internal/repository"
)

type Configurations struct {
	repository repository.Repository
	apps       *Applications
}

func NewConfigurations(repo repository.Repository, apps *Applications) *Configurations {
	return &Configurations{repository: repo, apps: apps}
}

func (s *Configurations) Draft(ctx context.Context, slug, code string) (domain.Draft, error) {
	_, environment, err := s.apps.resolve(ctx, slug, code)
	if err != nil {
		return domain.Draft{}, err
	}
	items, revision, err := s.repository.GetDraft(ctx, environment.ID)
	if err != nil {
		return domain.Draft{}, err
	}
	return domain.Draft{Items: items, Revision: revision, CurrentVersion: environment.CurrentVersion}, nil
}

func (s *Configurations) SaveDraft(ctx context.Context, slug, code string, revision int64, items []domain.ConfigItem, operator, requestID string) (domain.Draft, error) {
	application, environment, err := s.apps.resolve(ctx, slug, code)
	if err != nil {
		return domain.Draft{}, err
	}
	if !application.IsActive() {
		return domain.Draft{}, domain.NewError(domain.CodeConflict, "application is disabled")
	}
	if err := domain.ValidateItems(items); err != nil {
		return domain.Draft{}, err
	}
	operator = strings.TrimSpace(operator)
	if len(operator) < 2 || len(operator) > 80 {
		return domain.Draft{}, domain.NewError(domain.CodeInvalid, "operator must contain 2 to 80 characters")
	}
	newRevision, err := s.repository.SaveDraft(ctx, environment.ID, revision, items, operator, requestID)
	if err != nil {
		return domain.Draft{}, err
	}
	return domain.Draft{Items: items, Revision: newRevision, CurrentVersion: environment.CurrentVersion}, nil
}

func (s *Configurations) DiffDraft(ctx context.Context, slug, code string) (domain.Diff, error) {
	_, environment, err := s.apps.resolve(ctx, slug, code)
	if err != nil {
		return domain.Diff{}, err
	}
	draft, _, err := s.repository.GetDraft(ctx, environment.ID)
	if err != nil {
		return domain.Diff{}, err
	}
	current := []domain.ConfigItem{}
	if environment.HasRelease() {
		release, err := s.repository.GetCurrentRelease(ctx, environment.ID)
		if err != nil {
			return domain.Diff{}, err
		}
		current = release.Items
	}
	return domain.Compare(current, draft, false), nil
}

func (s *Configurations) Current(ctx context.Context, slug, code, token string) (domain.Release, error) {
	application, err := s.apps.Authenticate(ctx, slug, token)
	if err != nil {
		return domain.Release{}, err
	}
	environment, err := s.repository.GetEnvironment(ctx, application.ID, code)
	if err != nil {
		return domain.Release{}, err
	}
	return s.repository.GetCurrentRelease(ctx, environment.ID)
}

func ConstantTimeToken(header string) string {
	const prefix = "Bearer "
	if len(header) <= len(prefix) || subtle.ConstantTimeCompare([]byte(header[:len(prefix)]), []byte(prefix)) != 1 {
		return ""
	}
	return strings.TrimSpace(header[len(prefix):])
}

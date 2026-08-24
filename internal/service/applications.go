package service

import (
	"context"
	"fmt"
	"strings"

	"configcenter/internal/domain"
	"configcenter/internal/repository"
)

type Applications struct {
	repository repository.Repository
}

type CreatedApplication struct {
	Application domain.Application `json:"application"`
	AccessToken string             `json:"access_token"`
}

func NewApplications(repo repository.Repository) *Applications {
	return &Applications{repository: repo}
}

func (s *Applications) Create(ctx context.Context, name, slug, description string) (CreatedApplication, error) {
	name = strings.TrimSpace(name)
	slug = strings.TrimSpace(slug)
	if err := domain.ValidateApplication(name, slug); err != nil {
		return CreatedApplication{}, err
	}
	token, hash, err := domain.GenerateAccessToken()
	if err != nil {
		return CreatedApplication{}, err
	}
	application, err := s.repository.CreateApplication(ctx, repository.CreateApplicationInput{
		Name:            name,
		Slug:            slug,
		Description:     strings.TrimSpace(description),
		AccessTokenHash: hash,
	})
	if err != nil {
		return CreatedApplication{}, err
	}
	return CreatedApplication{Application: application, AccessToken: token}, nil
}

func (s *Applications) List(ctx context.Context, query, status string) ([]domain.Application, error) {
	return s.repository.ListApplications(ctx, strings.TrimSpace(query), status)
}

func (s *Applications) Get(ctx context.Context, slug string) (domain.Application, error) {
	return s.repository.GetApplication(ctx, slug)
}

func (s *Applications) ResetToken(ctx context.Context, slug, operator, requestID string) (string, error) {
	application, err := s.repository.GetApplication(ctx, slug)
	if err != nil {
		return "", err
	}
	plain, hash, err := domain.GenerateAccessToken()
	if err != nil {
		return "", err
	}
	if err := s.repository.ResetApplicationToken(ctx, application.ID, hash, operator, requestID); err != nil {
		return "", err
	}
	return plain, nil
}

func (s *Applications) CreateEnvironment(ctx context.Context, slug, name, code, description string) (domain.Environment, error) {
	application, err := s.activeApplication(ctx, slug)
	if err != nil {
		return domain.Environment{}, err
	}
	name, code = strings.TrimSpace(name), strings.TrimSpace(code)
	if err := domain.ValidateEnvironment(name, code); err != nil {
		return domain.Environment{}, err
	}
	return s.repository.CreateEnvironment(ctx, repository.CreateEnvironmentInput{
		ApplicationID: application.ID, Name: name, Code: code, Description: strings.TrimSpace(description),
	})
}

func (s *Applications) ListEnvironments(ctx context.Context, slug string) ([]domain.Environment, error) {
	application, err := s.repository.GetApplication(ctx, slug)
	if err != nil {
		return nil, err
	}
	return s.repository.ListEnvironments(ctx, application.ID)
}

func (s *Applications) Authenticate(ctx context.Context, slug, token string) (domain.Application, error) {
	application, err := s.activeApplication(ctx, slug)
	if err != nil {
		return domain.Application{}, domain.NewError(domain.CodeUnauthorized, "invalid client credentials")
	}
	if token == "" || application.AccessTokenHash != domain.HashToken(token) {
		return domain.Application{}, domain.NewError(domain.CodeUnauthorized, "invalid client credentials")
	}
	return application, nil
}

func (s *Applications) activeApplication(ctx context.Context, slug string) (domain.Application, error) {
	application, err := s.repository.GetApplication(ctx, slug)
	if err != nil {
		return domain.Application{}, fmt.Errorf("load application: %v", err)
	}
	if !application.IsActive() {
		return domain.Application{}, domain.NewError(domain.CodeForbidden, "application is disabled")
	}
	return application, nil
}

func (s *Applications) resolve(ctx context.Context, slug, code string) (domain.Application, domain.Environment, error) {
	application, err := s.repository.GetApplication(ctx, slug)
	if err != nil {
		return domain.Application{}, domain.Environment{}, err
	}
	environment, err := s.repository.GetEnvironment(ctx, application.ID, code)
	return application, environment, err
}

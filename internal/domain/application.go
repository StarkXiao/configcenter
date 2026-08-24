package domain

import (
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"regexp"
	"strings"
	"time"
)

type ApplicationStatus string

const (
	ApplicationActive   ApplicationStatus = "active"
	ApplicationDisabled ApplicationStatus = "disabled"
)

var slugPattern = regexp.MustCompile(`^[a-z0-9]+(?:-[a-z0-9]+)*$`)

type Application struct {
	ID              int64             `json:"id"`
	Name            string            `json:"name"`
	Slug            string            `json:"slug"`
	Description     string            `json:"description"`
	AccessTokenHash string            `json:"-"`
	Status          ApplicationStatus `json:"status"`
	CreatedAt       time.Time         `json:"created_at"`
	UpdatedAt       time.Time         `json:"updated_at"`
}

func ValidateApplication(name, slug string) error {
	name = strings.TrimSpace(name)
	if len(name) < 2 || len(name) > 80 {
		return NewError(CodeInvalid, "application name must contain 2 to 80 characters")
	}
	if len(slug) < 2 || len(slug) > 60 || !slugPattern.MatchString(slug) {
		return NewError(CodeInvalid, "slug must use lowercase letters, numbers, and single hyphens")
	}
	return nil
}

func GenerateAccessToken() (plain, hash string, err error) {
	data := make([]byte, 32)
	if _, err = rand.Read(data); err != nil {
		return "", "", WrapError(CodeInternal, "generate access token", err)
	}
	plain = "gcc_" + base64.RawURLEncoding.EncodeToString(data)
	return plain, HashToken(plain), nil
}

func HashToken(token string) string {
	sum := sha256.Sum256([]byte(token))
	return hex.EncodeToString(sum[:])
}

func (a Application) IsActive() bool { return a.Status == ApplicationActive }

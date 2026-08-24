package domain

import (
	"regexp"
	"strings"
	"time"
)

var environmentPattern = regexp.MustCompile(`^[a-z][a-z0-9_-]{1,31}$`)

type Environment struct {
	ID             int64      `json:"id"`
	ApplicationID  int64      `json:"application_id"`
	Name           string     `json:"name"`
	Code           string     `json:"code"`
	Description    string     `json:"description"`
	CurrentVersion int64      `json:"current_version"`
	DraftRevision  int64      `json:"draft_revision"`
	LastPublished  *time.Time `json:"last_published_at,omitempty"`
	CreatedAt      time.Time  `json:"created_at"`
	UpdatedAt      time.Time  `json:"updated_at"`
}

func ValidateEnvironment(name, code string) error {
	name = strings.TrimSpace(name)
	if len(name) < 2 || len(name) > 50 {
		return NewError(CodeInvalid, "environment name must contain 2 to 50 characters")
	}
	if !environmentPattern.MatchString(code) {
		return NewError(CodeInvalid, "environment code must start with a letter and contain 2 to 32 safe characters")
	}
	return nil
}

func (e Environment) HasRelease() bool { return e.CurrentVersion > 0 }

func (e Environment) NextVersion() int64 { return e.CurrentVersion + 1 }

func (e Environment) AcceptsRevision(revision int64) bool {
	return e.DraftRevision == revision
}

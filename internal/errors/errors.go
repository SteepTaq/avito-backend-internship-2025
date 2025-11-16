package errors

import (
	"errors"
)

var (
	ErrTeamExists          = errors.New("team already exists")
	ErrTeamNotFound        = errors.New("team not found")
	ErrTeamNameRequired    = errors.New("team name is required")
	ErrTeamMustHaveMembers = errors.New("team must have at least one member")
	ErrUserNotFound        = errors.New("user not found")
	ErrPRExists            = errors.New("PR already exists")
	ErrPRNotFound          = errors.New("pull request not found")
	ErrPRIDRequired        = errors.New("pull request ID is required")
	ErrPRNameRequired      = errors.New("pull request name is required")
	ErrAuthorIDRequired    = errors.New("author ID is required")
	ErrPRMerged            = errors.New("cannot reassign on merged PR")
	ErrPRInvalidStatus     = errors.New("PR cannot be merged with current status")
	ErrReviewerNotAssigned = errors.New("reviewer is not assigned to this PR")
	ErrNoCandidate         = errors.New("no active replacement candidate in team")
	ErrStringTooLong       = errors.New("string exceeds maximum length")
	ErrUsernameRequired    = errors.New("username is required")
	ErrDuplicateUserInTeam = errors.New("duplicate user ID in team")
)

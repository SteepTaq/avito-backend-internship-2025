package handler

import (
	"strings"
	"unicode/utf8"

	apperrors "github.com/SteepTaq/avito-backend-internship-2025/internal/errors"
	"github.com/SteepTaq/avito-backend-internship-2025/internal/models"
)

const (
	maxStringLength = 255
	minStringLength = 1
)

// validateTeamName проверяет валидность имени команды
func validateTeamName(teamName string) error {
	teamName = strings.TrimSpace(teamName)
	if teamName == "" {
		return apperrors.ErrTeamNameRequired
	}
	if utf8.RuneCountInString(teamName) > maxStringLength {
		return apperrors.ErrStringTooLong
	}
	return nil
}

// validateUserID проверяет валидность ID пользователя
func validateUserID(userID string) error {
	userID = strings.TrimSpace(userID)
	if userID == "" {
		return apperrors.ErrAuthorIDRequired
	}
	if utf8.RuneCountInString(userID) > maxStringLength {
		return apperrors.ErrStringTooLong
	}
	return nil
}

// validatePRID проверяет валидность ID PR
func validatePRID(prID string) error {
	prID = strings.TrimSpace(prID)
	if prID == "" {
		return apperrors.ErrPRIDRequired
	}
	if utf8.RuneCountInString(prID) > maxStringLength {
		return apperrors.ErrStringTooLong
	}
	return nil
}

// validatePRName проверяет валидность имени PR
func validatePRName(prName string) error {
	prName = strings.TrimSpace(prName)
	if prName == "" {
		return apperrors.ErrPRNameRequired
	}
	if utf8.RuneCountInString(prName) > maxStringLength {
		return apperrors.ErrStringTooLong
	}
	return nil
}

// validateTeam проверяет валидность команды
func validateTeam(team *models.Team) error {
	if err := validateTeamName(team.TeamName); err != nil {
		return err
	}
	if len(team.Members) == 0 {
		return apperrors.ErrTeamMustHaveMembers
	}
	for i := range team.Members {
		member := &team.Members[i]
		if err := validateUserID(member.UserID); err != nil {
			return err
		}
		username := strings.TrimSpace(member.Username)
		if username == "" {
			return apperrors.ErrUsernameRequired
		}
		if utf8.RuneCountInString(username) > maxStringLength {
			return apperrors.ErrStringTooLong
		}
		// Проверка на дубликаты user_id в одной команде
		for j := i + 1; j < len(team.Members); j++ {
			if member.UserID == team.Members[j].UserID {
				return apperrors.ErrDuplicateUserInTeam
			}
		}
	}
	return nil
}

// validateCreatePRRequest проверяет валидность запроса на создание PR
func validateCreatePRRequest(req *models.CreatePRRequest) error {
	if err := validatePRID(req.PullRequestID); err != nil {
		return err
	}
	if err := validatePRName(req.PullRequestName); err != nil {
		return err
	}
	if err := validateUserID(req.AuthorID); err != nil {
		return err
	}
	return nil
}

// validateMergePRRequest проверяет валидность запроса на merge PR
func validateMergePRRequest(req *models.MergePRRequest) error {
	if err := validatePRID(req.PullRequestID); err != nil {
		return err
	}
	return nil
}

// validateReassignReviewerRequest проверяет валидность запроса на переназначение ревьювера
func validateReassignReviewerRequest(req *models.ReassignReviewerRequest) error {
	if err := validatePRID(req.PullRequestID); err != nil {
		return err
	}
	if err := validateUserID(req.OldUserID); err != nil {
		return err
	}
	return nil
}

// validateSetUserActiveRequest проверяет валидность запроса на изменение активности пользователя
func validateSetUserActiveRequest(req *models.SetUserActiveRequest) error {
	if err := validateUserID(req.UserID); err != nil {
		return err
	}
	return nil
}

// validateDeactivateTeamUsersRequest проверяет валидность запроса на деактивацию пользователей команды
func validateDeactivateTeamUsersRequest(req *models.DeactivateTeamUsersRequest) error {
	if err := validateTeamName(req.TeamName); err != nil {
		return err
	}
	// userIDs может быть пустым (деактивация всех пользователей команды)
	for _, userID := range req.UserIDs {
		if err := validateUserID(userID); err != nil {
			return err
		}
	}
	return nil
}

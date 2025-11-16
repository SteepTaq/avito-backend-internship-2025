package service

import (
	"context"
	"crypto/rand"
	"errors"
	"fmt"
	"math/big"
	"time"

	appErrors "github.com/SteepTaq/avito-backend-internship-2025/internal/errors"
	"github.com/SteepTaq/avito-backend-internship-2025/internal/models"
	"github.com/SteepTaq/avito-backend-internship-2025/pkg/logger"
)

type UserRepository interface {
	GetUserByID(ctx context.Context, userID string) (*models.User, error)
	SetUserActive(ctx context.Context, userID string, isActive bool) error
	GetActiveUsersByTeam(ctx context.Context, teamName string, excludeUserID string) ([]*models.User, error)
	GetUsersByTeam(ctx context.Context, teamName string) ([]*models.User, error)
	DeactivateUsersByTeam(ctx context.Context, teamName string, userIDs []string) ([]string, error)
}

type TeamRepository interface {
	CreateTeam(ctx context.Context, team *models.Team) error
	GetTeam(ctx context.Context, teamName string) (*models.Team, error)
	TeamExists(ctx context.Context, teamName string) (bool, error)
}

type PullRequestRepository interface {
	CreatePR(ctx context.Context, pr *models.PullRequest) error
	GetPRByID(ctx context.Context, prID string) (*models.PullRequest, error)
	UpdatePRStatus(ctx context.Context, prID string, status models.PRStatus) error
	UpdatePRStatusWithMergedAt(ctx context.Context, prID string, status models.PRStatus) error
	UpdatePRReviewers(ctx context.Context, prID string, reviewers []string) error
	GetPRsByReviewer(ctx context.Context, reviewerID string) ([]*models.PullRequest, error)
	GetOpenPRsByReviewers(ctx context.Context, reviewerIDs []string) ([]*models.PullRequest, error)
	BulkUpdatePRReviewers(ctx context.Context, prReviewers map[string][]string) error
	PRExists(ctx context.Context, prID string) (bool, error)
}

type StatsRepository interface {
	GetStats(ctx context.Context) (map[string]interface{}, error)
}

type Repository interface {
	UserRepository
	TeamRepository
	PullRequestRepository
	StatsRepository
}

type Service struct {
	repo         Repository
	maxReviewers int
	logger       *logger.Logger
}

func NewService(repo Repository, maxReviewers int, log *logger.Logger) *Service {
	return &Service{
		repo:         repo,
		maxReviewers: maxReviewers,
		logger:       log,
	}
}

func (s *Service) CreateTeam(ctx context.Context, team *models.Team) (*models.Team, error) {
	exists, err := s.repo.TeamExists(ctx, team.TeamName)
	if err != nil {
		s.logger.Error("Failed to check team existence during team creation", "error", err, "team_name", team.TeamName)
		return nil, fmt.Errorf("failed to check team existence: %w", err)
	}
	if exists {
		return nil, appErrors.ErrTeamExists
	}

	if err := s.repo.CreateTeam(ctx, team); err != nil {
		s.logger.Error("Failed to create team", "error", err, "team_name", team.TeamName)
		return nil, err
	}

	createdTeam, err := s.repo.GetTeam(ctx, team.TeamName)
	if err != nil {
		s.logger.Error("Failed to get created team", "error", err, "team_name", team.TeamName)
		return nil, fmt.Errorf("team created but failed to retrieve: %w", err)
	}

	s.logger.Info("Team created", "team_name", team.TeamName, "members_count", len(team.Members))
	return createdTeam, nil
}

func (s *Service) GetTeam(ctx context.Context, teamName string) (*models.Team, error) {
	return s.repo.GetTeam(ctx, teamName)
}

func (s *Service) SetUserActive(ctx context.Context, userID string, isActive bool) (*models.User, error) {
	if err := s.repo.SetUserActive(ctx, userID, isActive); err != nil {
		return nil, err
	}
	return s.repo.GetUserByID(ctx, userID)
}

func (s *Service) CreatePR(ctx context.Context, prID, prName, authorID string) (*models.PullRequest, error) {
	exists, err := s.repo.PRExists(ctx, prID)
	if err != nil {
		s.logger.Error("Failed to check PR existence during PR creation", "error", err, "pull_request_id", prID)
		return nil, fmt.Errorf("failed to check PR existence: %w", err)
	}
	if exists {
		return nil, appErrors.ErrPRExists
	}

	author, err := s.repo.GetUserByID(ctx, authorID)
	if err != nil {
		if errors.Is(err, appErrors.ErrUserNotFound) {
			return nil, fmt.Errorf("author not found: %w", err)
		}
		s.logger.Error("Failed to get author during PR creation", "error", err, "author_id", authorID)
		return nil, fmt.Errorf("failed to get author: %w", err)
	}

	candidates, err := s.repo.GetActiveUsersByTeam(ctx, author.TeamName, authorID)
	if err != nil {
		s.logger.Error("Failed to get active users by team during PR creation", "error", err, "team_name", author.TeamName, "author_id", authorID)
		return nil, err
	}

	reviewers := s.selectRandomReviewers(candidates, s.maxReviewers)

	if len(reviewers) == 0 {
		s.logger.Warn("No reviewers assigned to PR - no active candidates in team",
			"pull_request_id", prID,
			"author_id", authorID,
			"team_name", author.TeamName)
	}

	s.logger.Info("PR created with reviewers assigned",
		"pull_request_id", prID,
		"author_id", authorID,
		"team_name", author.TeamName,
		"reviewers_count", len(reviewers),
		"reviewers", reviewers,
	)

	now := time.Now()
	pr := &models.PullRequest{
		PullRequestID:     prID,
		PullRequestName:   prName,
		AuthorID:          authorID,
		Status:            models.PRStatusOpen,
		AssignedReviewers: reviewers,
		CreatedAt:         &now,
	}

	if createErr := s.repo.CreatePR(ctx, pr); createErr != nil {
		s.logger.Error("Failed to create PR in repository", "error", createErr, "pull_request_id", prID, "author_id", authorID)
		return nil, createErr
	}

	createdPR, err := s.repo.GetPRByID(ctx, prID)
	if err != nil {
		s.logger.Error("Failed to get created PR", "error", err, "pull_request_id", prID, "author_id", authorID)
		return nil, err
	}
	return createdPR, nil
}

func (s *Service) MergePR(ctx context.Context, prID string) (*models.PullRequest, error) {
	pr, err := s.repo.GetPRByID(ctx, prID)
	if err != nil {
		return nil, err
	}

	if pr.Status == models.PRStatusMerged {
		return pr, nil
	}

	if pr.Status != models.PRStatusOpen {
		s.logger.Warn("Attempt to merge PR with invalid status",
			"pull_request_id", prID,
			"current_status", pr.Status,
			"expected_status", models.PRStatusOpen)
		return nil, appErrors.ErrPRInvalidStatus
	}

	if err := s.repo.UpdatePRStatusWithMergedAt(ctx, prID, models.PRStatusMerged); err != nil {
		s.logger.Error("Failed to merge PR", "error", err, "pull_request_id", prID)
		return nil, err
	}

	s.logger.Info("PR merged", "pull_request_id", prID, "author_id", pr.AuthorID)

	now := time.Now()
	pr.Status = models.PRStatusMerged
	pr.MergedAt = &now
	return pr, nil
}

func (s *Service) ReassignReviewer(ctx context.Context, prID, oldReviewerID string) (*models.PullRequest, string, error) {
	pr, err := s.repo.GetPRByID(ctx, prID)
	if err != nil {
		return nil, "", err
	}

	if pr.Status == models.PRStatusMerged {
		return nil, "", appErrors.ErrPRMerged
	}

	found := false
	for _, reviewerID := range pr.AssignedReviewers {
		if reviewerID == oldReviewerID {
			found = true
			break
		}
	}
	if !found {
		return nil, "", appErrors.ErrReviewerNotAssigned
	}

	oldReviewer, err := s.repo.GetUserByID(ctx, oldReviewerID)
	if err != nil {
		return nil, "", fmt.Errorf("old reviewer not found: %w", err)
	}

	candidates, err := s.repo.GetActiveUsersByTeam(ctx, oldReviewer.TeamName, oldReviewerID)
	if err != nil {
		s.logger.Error("Failed to get active users by team during reviewer reassignment", "error", err, "team_name", oldReviewer.TeamName, "old_reviewer_id", oldReviewerID)
		return nil, "", err
	}

	excludedIDs := make(map[string]bool, len(pr.AssignedReviewers)+1)
	excludedIDs[pr.AuthorID] = true
	for _, reviewerID := range pr.AssignedReviewers {
		excludedIDs[reviewerID] = true
	}

	filteredCandidates := make([]*models.User, 0, len(candidates))
	for _, candidate := range candidates {
		if !excludedIDs[candidate.UserID] {
			filteredCandidates = append(filteredCandidates, candidate)
		}
	}

	if len(filteredCandidates) == 0 {
		return nil, "", appErrors.ErrNoCandidate
	}

	newReviewer, err := s.selectRandomCandidate(filteredCandidates)
	if err != nil {
		return nil, "", fmt.Errorf("failed to select random candidate: %w", err)
	}

	s.logger.Info("Reassigning reviewer",
		"pull_request_id", prID,
		"old_reviewer_id", oldReviewerID,
		"new_reviewer_id", newReviewer.UserID,
		"team_name", oldReviewer.TeamName,
	)

	newReviewers := make([]string, len(pr.AssignedReviewers))
	copy(newReviewers, pr.AssignedReviewers)
	for i, reviewerID := range newReviewers {
		if reviewerID == oldReviewerID {
			newReviewers[i] = newReviewer.UserID
			break
		}
	}

	if err := s.repo.UpdatePRReviewers(ctx, prID, newReviewers); err != nil {
		s.logger.Error("Failed to update PR reviewers during reassignment", "error", err, "pull_request_id", prID)
		return nil, "", err
	}

	pr.AssignedReviewers = newReviewers
	return pr, newReviewer.UserID, nil
}

func (s *Service) GetPRsByReviewer(ctx context.Context, reviewerID string) ([]*models.PullRequestShort, error) {
	_, err := s.repo.GetUserByID(ctx, reviewerID)
	if err != nil {
		return nil, err
	}

	prs, err := s.repo.GetPRsByReviewer(ctx, reviewerID)
	if err != nil {
		s.logger.Error("Failed to get PRs by reviewer", "error", err, "reviewer_id", reviewerID)
		return nil, err
	}

	shortPRs := make([]*models.PullRequestShort, len(prs))
	for i, pr := range prs {
		shortPRs[i] = &models.PullRequestShort{
			PullRequestID:   pr.PullRequestID,
			PullRequestName: pr.PullRequestName,
			AuthorID:        pr.AuthorID,
			Status:          pr.Status,
		}
	}

	return shortPRs, nil
}

func (s *Service) selectRandomReviewers(candidates []*models.User, maxCount int) []string {
	if len(candidates) == 0 {
		return []string{}
	}

	count := maxCount
	if len(candidates) < maxCount {
		count = len(candidates)
	}

	selected := make(map[string]bool, count)
	reviewers := make([]string, 0, count)

	shuffled := make([]*models.User, len(candidates))
	copy(shuffled, candidates)

	if err := s.shuffleUsers(shuffled); err != nil {
		s.logger.Warn("Failed to shuffle users, using deterministic order", "error", err)
	}

	for i := 0; i < len(shuffled) && len(reviewers) < count; i++ {
		userID := shuffled[i].UserID
		if !selected[userID] {
			selected[userID] = true
			reviewers = append(reviewers, userID)
		}
	}

	return reviewers
}

func (s *Service) shuffleUsers(users []*models.User) error {
	for i := len(users) - 1; i > 0; i-- {
		jBig, err := rand.Int(rand.Reader, big.NewInt(int64(i+1)))
		if err != nil {
			return fmt.Errorf("failed to generate random number: %w", err)
		}
		j := jBig.Int64()
		users[i], users[j] = users[j], users[i]
	}
	return nil
}

func (s *Service) selectRandomCandidate(candidates []*models.User) (*models.User, error) {
	if len(candidates) == 0 {
		return nil, fmt.Errorf("no candidates available")
	}
	if len(candidates) == 1 {
		return candidates[0], nil
	}

	idxBig, err := rand.Int(rand.Reader, big.NewInt(int64(len(candidates))))
	if err != nil {
		return nil, fmt.Errorf("failed to generate random index: %w", err)
	}
	idx := int(idxBig.Int64())
	return candidates[idx], nil
}

func (s *Service) DeactivateTeamUsers(ctx context.Context, teamName string, userIDs []string) (map[string]interface{}, error) {
	teamExists, err := s.repo.TeamExists(ctx, teamName)
	if err != nil {
		s.logger.Error("Failed to check team existence during team deactivation", "error", err, "team_name", teamName)
		return nil, fmt.Errorf("failed to check team existence: %w", err)
	}
	if !teamExists {
		return nil, appErrors.ErrTeamNotFound
	}

	deactivatedIDs, err := s.repo.DeactivateUsersByTeam(ctx, teamName, userIDs)
	if err != nil {
		s.logger.Error("Failed to deactivate users by team", "error", err, "team_name", teamName, "user_ids", userIDs)
		return nil, fmt.Errorf("failed to deactivate users: %w", err)
	}

	if len(deactivatedIDs) == 0 {
		s.logger.Info("No users deactivated", "team_name", teamName)
		return map[string]interface{}{
			"deactivated_users": []string{},
			"reassigned_prs":    []string{},
		}, nil
	}

	openPRs, err := s.repo.GetOpenPRsByReviewers(ctx, deactivatedIDs)
	if err != nil {
		s.logger.Error("Failed to get open PRs by reviewers during team deactivation", "error", err, "deactivated_ids", deactivatedIDs)
		return nil, fmt.Errorf("failed to get open PRs: %w", err)
	}

	if len(openPRs) == 0 {
		s.logger.Info("Team users deactivated, no PRs to reassign",
			"team_name", teamName,
			"deactivated_count", len(deactivatedIDs))
		return map[string]interface{}{
			"deactivated_users": deactivatedIDs,
			"reassigned_prs":    []string{},
		}, nil
	}

	deactivatedMap := make(map[string]bool, len(deactivatedIDs))
	for _, id := range deactivatedIDs {
		deactivatedMap[id] = true
	}

	teamCandidatesCache := make(map[string][]*models.User)
	prReviewersUpdates := make(map[string][]string)
	reassignedPRs := make([]string, 0, len(openPRs))

	for _, pr := range openPRs {
		hasDeactivatedReviewer := false
		for _, reviewerID := range pr.AssignedReviewers {
			if deactivatedMap[reviewerID] {
				hasDeactivatedReviewer = true
				break
			}
		}

		if !hasDeactivatedReviewer {
			continue
		}

		excludedIDs := make(map[string]bool, len(pr.AssignedReviewers)+1)
		excludedIDs[pr.AuthorID] = true
		for _, reviewerID := range pr.AssignedReviewers {
			excludedIDs[reviewerID] = true
		}

		newReviewers := make([]string, 0, len(pr.AssignedReviewers))

		for _, reviewerID := range pr.AssignedReviewers {
			if deactivatedMap[reviewerID] {
				oldReviewer, err := s.repo.GetUserByID(ctx, reviewerID)
				if err != nil {
					s.logger.Warn("Failed to get deactivated reviewer during reassignment, skipping reviewer",
						"pull_request_id", pr.PullRequestID,
						"reviewer_id", reviewerID,
						"error", err)
					continue
				}

				cacheKey := oldReviewer.TeamName
				candidates, ok := teamCandidatesCache[cacheKey]
				if !ok {
					candidates, err = s.repo.GetActiveUsersByTeam(ctx, oldReviewer.TeamName, "")
					if err != nil {
						s.logger.Warn("Failed to get active users by team during reassignment, skipping reviewer",
							"pull_request_id", pr.PullRequestID,
							"team_name", oldReviewer.TeamName,
							"reviewer_id", reviewerID,
							"error", err)
						continue
					}
					teamCandidatesCache[cacheKey] = candidates
				}

				filteredCandidates := make([]*models.User, 0, len(candidates))
				for _, candidate := range candidates {
					if !excludedIDs[candidate.UserID] {
						filteredCandidates = append(filteredCandidates, candidate)
					}
				}

				if len(filteredCandidates) > 0 {
					newReviewer, err := s.selectRandomCandidate(filteredCandidates)
					if err != nil {
						s.logger.Warn("Failed to select random candidate during team deactivation, skipping reviewer",
							"pull_request_id", pr.PullRequestID,
							"reviewer_id", reviewerID,
							"error", err)
						continue
					}
					newReviewers = append(newReviewers, newReviewer.UserID)
					excludedIDs[newReviewer.UserID] = true
				}
			} else {
				newReviewers = append(newReviewers, reviewerID)
			}
		}

		if len(newReviewers) > s.maxReviewers {
			newReviewers = newReviewers[:s.maxReviewers]
		}

		hasChanges := false
		if len(newReviewers) != len(pr.AssignedReviewers) {
			hasChanges = true
		} else {
			for i, newReviewer := range newReviewers {
				if i >= len(pr.AssignedReviewers) || newReviewer != pr.AssignedReviewers[i] {
					hasChanges = true
					break
				}
			}
		}

		if hasChanges {
			prReviewersUpdates[pr.PullRequestID] = newReviewers
			reassignedPRs = append(reassignedPRs, pr.PullRequestID)

			s.logger.Info("Reassigning reviewers for PR during team deactivation",
				"pull_request_id", pr.PullRequestID,
				"old_reviewers", pr.AssignedReviewers,
				"new_reviewers", newReviewers)
		}
	}

	if len(prReviewersUpdates) > 0 {
		if err := s.repo.BulkUpdatePRReviewers(ctx, prReviewersUpdates); err != nil {
			s.logger.Error("Failed to bulk update PR reviewers during team deactivation", "error", err)
			return nil, fmt.Errorf("failed to update PR reviewers: %w", err)
		}
	}

	s.logger.Info("Team users deactivated with PR reassignments",
		"team_name", teamName,
		"deactivated_count", len(deactivatedIDs),
		"reassigned_prs_count", len(reassignedPRs))

	return map[string]interface{}{
		"deactivated_users": deactivatedIDs,
		"reassigned_prs":    reassignedPRs,
	}, nil
}

func (s *Service) GetStats(ctx context.Context) (map[string]interface{}, error) {
	stats, err := s.repo.GetStats(ctx)
	if err != nil {
		s.logger.Error("Failed to get stats", "error", err)
		return nil, err
	}
	return stats, nil
}

func (s *Service) Ping(ctx context.Context) error {
	_, err := s.repo.GetStats(ctx)
	if err != nil {
		s.logger.Error("Health check failed: repository unavailable", "error", err)
		return fmt.Errorf("repository ping failed: %w", err)
	}
	return nil
}

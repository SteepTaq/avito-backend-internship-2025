package service

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"

	appErrors "github.com/SteepTaq/avito-backend-internship-2025/internal/errors"
	"github.com/SteepTaq/avito-backend-internship-2025/internal/models"
	"github.com/SteepTaq/avito-backend-internship-2025/mocks"
	"github.com/SteepTaq/avito-backend-internship-2025/pkg/logger"
)

func newMockLogger() *logger.Logger {
	return logger.New("debug")
}

func setupService(t *testing.T) (*Service, *mocks.MockRepository) {
	mockRepo := mocks.NewMockRepository(t)
	log := newMockLogger()
	svc := NewService(mockRepo, 2, log)
	return svc, mockRepo
}

func TestNewService(t *testing.T) {
	mockRepo := mocks.NewMockRepository(t)
	log := newMockLogger()
	maxReviewers := 2

	svc := NewService(mockRepo, maxReviewers, log)

	assert.NotNil(t, svc)
	assert.Equal(t, mockRepo, svc.repo)
	assert.Equal(t, maxReviewers, svc.maxReviewers)
	assert.Equal(t, log, svc.logger)
}

func TestCreateTeam_Success(t *testing.T) {
	svc, mockRepo := setupService(t)
	ctx := context.Background()

	team := &models.Team{
		TeamName: "backend",
		Members: []models.TeamMember{
			{UserID: "u1", Username: "Alice", IsActive: true},
			{UserID: "u2", Username: "Bob", IsActive: true},
		},
	}

	expectedTeam := &models.Team{
		TeamName: "backend",
		Members: []models.TeamMember{
			{UserID: "u1", Username: "Alice", IsActive: true},
			{UserID: "u2", Username: "Bob", IsActive: true},
		},
	}

	mockRepo.EXPECT().TeamExists(ctx, "backend").Return(false, nil)
	mockRepo.EXPECT().CreateTeam(ctx, team).Return(nil)
	mockRepo.EXPECT().GetTeam(ctx, "backend").Return(expectedTeam, nil)

	createdTeam, err := svc.CreateTeam(ctx, team)
	assert.NoError(t, err)
	assert.NotNil(t, createdTeam)
	assert.Equal(t, team.TeamName, createdTeam.TeamName)
	assert.Len(t, createdTeam.Members, 2)
	assert.Equal(t, "u1", createdTeam.Members[0].UserID)
	assert.Equal(t, "u2", createdTeam.Members[1].UserID)
}

func TestCreateTeam_TeamExists(t *testing.T) {
	svc, mockRepo := setupService(t)
	ctx := context.Background()

	team := &models.Team{
		TeamName: "backend",
		Members: []models.TeamMember{
			{UserID: "u1", Username: "Alice", IsActive: true},
		},
	}

	mockRepo.EXPECT().TeamExists(ctx, "backend").Return(true, nil)

	_, err := svc.CreateTeam(ctx, team)
	assert.Error(t, err)
	assert.True(t, errors.Is(err, appErrors.ErrTeamExists))
}

func TestCreateTeam_TeamExistsError(t *testing.T) {
	svc, mockRepo := setupService(t)
	ctx := context.Background()

	team := &models.Team{
		TeamName: "backend",
		Members: []models.TeamMember{
			{UserID: "u1", Username: "Alice", IsActive: true},
		},
	}

	mockRepo.EXPECT().TeamExists(ctx, "backend").Return(false, errors.New("database error"))

	_, err := svc.CreateTeam(ctx, team)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "failed to check team existence")
}

func TestCreateTeam_CreateTeamError(t *testing.T) {
	svc, mockRepo := setupService(t)
	ctx := context.Background()

	team := &models.Team{
		TeamName: "backend",
		Members: []models.TeamMember{
			{UserID: "u1", Username: "Alice", IsActive: true},
		},
	}

	mockRepo.EXPECT().TeamExists(ctx, "backend").Return(false, nil)
	mockRepo.EXPECT().CreateTeam(ctx, team).Return(errors.New("database error"))

	_, err := svc.CreateTeam(ctx, team)
	assert.Error(t, err)
	assert.Equal(t, "database error", err.Error())
}

func TestGetTeam_Success(t *testing.T) {
	svc, mockRepo := setupService(t)
	ctx := context.Background()

	expectedTeam := &models.Team{
		TeamName: "backend",
		Members: []models.TeamMember{
			{UserID: "u1", Username: "Alice", IsActive: true},
		},
	}

	mockRepo.EXPECT().GetTeam(ctx, "backend").Return(expectedTeam, nil)

	team, err := svc.GetTeam(ctx, "backend")
	assert.NoError(t, err)
	assert.Equal(t, expectedTeam, team)
}

func TestGetTeam_Error(t *testing.T) {
	svc, mockRepo := setupService(t)
	ctx := context.Background()

	mockRepo.EXPECT().GetTeam(ctx, "backend").Return(nil, errors.New("database error"))

	team, err := svc.GetTeam(ctx, "backend")
	assert.Error(t, err)
	assert.Nil(t, team)
}

func TestSetUserActive_Success(t *testing.T) {
	svc, mockRepo := setupService(t)
	ctx := context.Background()

	user := &models.User{
		UserID:   "u1",
		Username: "Alice",
		IsActive: true,
	}

	mockRepo.EXPECT().SetUserActive(ctx, "u1", true).Return(nil)
	mockRepo.EXPECT().GetUserByID(ctx, "u1").Return(user, nil)

	result, err := svc.SetUserActive(ctx, "u1", true)
	assert.NoError(t, err)
	assert.Equal(t, user, result)
}

func TestSetUserActive_SetUserActiveError(t *testing.T) {
	svc, mockRepo := setupService(t)
	ctx := context.Background()

	mockRepo.EXPECT().SetUserActive(ctx, "u1", true).Return(errors.New("database error"))

	result, err := svc.SetUserActive(ctx, "u1", true)
	assert.Error(t, err)
	assert.Nil(t, result)
}

func TestSetUserActive_GetUserByIDError(t *testing.T) {
	svc, mockRepo := setupService(t)
	ctx := context.Background()

	mockRepo.EXPECT().SetUserActive(ctx, "u1", true).Return(nil)
	mockRepo.EXPECT().GetUserByID(ctx, "u1").Return(nil, errors.New("user not found"))

	result, err := svc.SetUserActive(ctx, "u1", true)
	assert.Error(t, err)
	assert.Nil(t, result)
}

func TestCreatePR_Success(t *testing.T) {
	svc, mockRepo := setupService(t)
	ctx := context.Background()

	author := &models.User{
		UserID:   "u1",
		Username: "Alice",
		TeamName: "backend",
		IsActive: true,
	}

	candidates := []*models.User{
		{UserID: "u2", Username: "Bob", TeamName: "backend", IsActive: true},
		{UserID: "u3", Username: "Charlie", TeamName: "backend", IsActive: true},
	}

	now := time.Now()
	expectedPR := &models.PullRequest{
		PullRequestID:     "pr-1",
		PullRequestName:   "Test PR",
		AuthorID:          "u1",
		Status:            models.PRStatusOpen,
		AssignedReviewers: []string{"u2", "u3"},
		CreatedAt:         &now,
	}

	mockRepo.EXPECT().PRExists(ctx, "pr-1").Return(false, nil)
	mockRepo.EXPECT().GetUserByID(ctx, "u1").Return(author, nil)
	mockRepo.EXPECT().GetActiveUsersByTeam(ctx, "backend", "u1").Return(candidates, nil)
	mockRepo.EXPECT().CreatePR(ctx, mock.AnythingOfType("*models.PullRequest")).Return(nil)
	mockRepo.EXPECT().GetPRByID(ctx, "pr-1").Return(expectedPR, nil)

	pr, err := svc.CreatePR(ctx, "pr-1", "Test PR", "u1")
	assert.NoError(t, err)
	assert.NotNil(t, pr)
	assert.Equal(t, expectedPR.PullRequestID, pr.PullRequestID)
	assert.Equal(t, expectedPR.PullRequestName, pr.PullRequestName)
	assert.Equal(t, expectedPR.AuthorID, pr.AuthorID)
	assert.Equal(t, models.PRStatusOpen, pr.Status)
	assert.NotNil(t, pr.CreatedAt)
	assert.Len(t, pr.AssignedReviewers, 2)
	assert.Contains(t, pr.AssignedReviewers, "u2")
	assert.Contains(t, pr.AssignedReviewers, "u3")
}

func TestCreatePR_PRExists(t *testing.T) {
	svc, mockRepo := setupService(t)
	ctx := context.Background()

	mockRepo.EXPECT().PRExists(ctx, "pr-1").Return(true, nil)

	pr, err := svc.CreatePR(ctx, "pr-1", "Test PR", "u1")
	assert.Error(t, err)
	assert.Nil(t, pr)
	assert.True(t, errors.Is(err, appErrors.ErrPRExists))
}

func TestCreatePR_AuthorNotFound(t *testing.T) {
	svc, mockRepo := setupService(t)
	ctx := context.Background()

	mockRepo.EXPECT().PRExists(ctx, "pr-1").Return(false, nil)
	mockRepo.EXPECT().GetUserByID(ctx, "u1").Return(nil, appErrors.ErrUserNotFound)

	pr, err := svc.CreatePR(ctx, "pr-1", "Test PR", "u1")
	assert.Error(t, err)
	assert.Nil(t, pr)
	assert.Contains(t, err.Error(), "author not found")
}

func TestCreatePR_NoReviewers(t *testing.T) {
	svc, mockRepo := setupService(t)
	ctx := context.Background()

	author := &models.User{
		UserID:   "u1",
		Username: "Alice",
		TeamName: "backend",
		IsActive: true,
	}

	now := time.Now()
	expectedPR := &models.PullRequest{
		PullRequestID:     "pr-1",
		PullRequestName:   "Test PR",
		AuthorID:          "u1",
		Status:            models.PRStatusOpen,
		AssignedReviewers: []string{},
		CreatedAt:         &now,
	}

	mockRepo.EXPECT().PRExists(ctx, "pr-1").Return(false, nil)
	mockRepo.EXPECT().GetUserByID(ctx, "u1").Return(author, nil)
	mockRepo.EXPECT().GetActiveUsersByTeam(ctx, "backend", "u1").Return([]*models.User{}, nil)
	mockRepo.EXPECT().CreatePR(ctx, mock.AnythingOfType("*models.PullRequest")).Return(nil)
	mockRepo.EXPECT().GetPRByID(ctx, "pr-1").Return(expectedPR, nil)

	pr, err := svc.CreatePR(ctx, "pr-1", "Test PR", "u1")
	assert.NoError(t, err)
	assert.NotNil(t, pr)
	assert.Equal(t, "pr-1", pr.PullRequestID)
	assert.Equal(t, "Test PR", pr.PullRequestName)
	assert.Equal(t, "u1", pr.AuthorID)
	assert.Equal(t, models.PRStatusOpen, pr.Status)
	assert.Empty(t, pr.AssignedReviewers)
	assert.NotNil(t, pr.CreatedAt)
}

func TestCreatePR_AuthorGetError(t *testing.T) {
	svc, mockRepo := setupService(t)
	ctx := context.Background()

	mockRepo.EXPECT().PRExists(ctx, "pr-1").Return(false, nil)
	mockRepo.EXPECT().GetUserByID(ctx, "u1").Return(nil, errors.New("database error"))

	pr, err := svc.CreatePR(ctx, "pr-1", "Test PR", "u1")
	assert.Error(t, err)
	assert.Nil(t, pr)
	assert.Contains(t, err.Error(), "failed to get author")
}

func TestCreatePR_GetActiveUsersError(t *testing.T) {
	svc, mockRepo := setupService(t)
	ctx := context.Background()

	author := &models.User{
		UserID:   "u1",
		Username: "Alice",
		TeamName: "backend",
		IsActive: true,
	}

	mockRepo.EXPECT().PRExists(ctx, "pr-1").Return(false, nil)
	mockRepo.EXPECT().GetUserByID(ctx, "u1").Return(author, nil)
	mockRepo.EXPECT().GetActiveUsersByTeam(ctx, "backend", "u1").Return(nil, errors.New("database error"))

	pr, err := svc.CreatePR(ctx, "pr-1", "Test PR", "u1")
	assert.Error(t, err)
	assert.Nil(t, pr)
	assert.Equal(t, "database error", err.Error())
}

func TestCreatePR_CreatePRError(t *testing.T) {
	svc, mockRepo := setupService(t)
	ctx := context.Background()

	author := &models.User{
		UserID:   "u1",
		Username: "Alice",
		TeamName: "backend",
		IsActive: true,
	}

	candidates := []*models.User{
		{UserID: "u2", Username: "Bob", TeamName: "backend", IsActive: true},
	}

	mockRepo.EXPECT().PRExists(ctx, "pr-1").Return(false, nil)
	mockRepo.EXPECT().GetUserByID(ctx, "u1").Return(author, nil)
	mockRepo.EXPECT().GetActiveUsersByTeam(ctx, "backend", "u1").Return(candidates, nil)
	mockRepo.EXPECT().CreatePR(ctx, mock.AnythingOfType("*models.PullRequest")).Return(errors.New("database error"))

	pr, err := svc.CreatePR(ctx, "pr-1", "Test PR", "u1")
	assert.Error(t, err)
	assert.Nil(t, pr)
	assert.Equal(t, "database error", err.Error())
}

func TestCreatePR_GetPRByIDError(t *testing.T) {
	svc, mockRepo := setupService(t)
	ctx := context.Background()

	author := &models.User{
		UserID:   "u1",
		Username: "Alice",
		TeamName: "backend",
		IsActive: true,
	}

	candidates := []*models.User{
		{UserID: "u2", Username: "Bob", TeamName: "backend", IsActive: true},
	}

	mockRepo.EXPECT().PRExists(ctx, "pr-1").Return(false, nil)
	mockRepo.EXPECT().GetUserByID(ctx, "u1").Return(author, nil)
	mockRepo.EXPECT().GetActiveUsersByTeam(ctx, "backend", "u1").Return(candidates, nil)
	mockRepo.EXPECT().CreatePR(ctx, mock.AnythingOfType("*models.PullRequest")).Return(nil)
	mockRepo.EXPECT().GetPRByID(ctx, "pr-1").Return(nil, errors.New("database error"))

	pr, err := svc.CreatePR(ctx, "pr-1", "Test PR", "u1")
	assert.Error(t, err)
	assert.Nil(t, pr)
	assert.Equal(t, "database error", err.Error())
}

func TestMergePR_Success(t *testing.T) {
	svc, mockRepo := setupService(t)
	ctx := context.Background()

	now := time.Now()
	pr := &models.PullRequest{
		PullRequestID: "pr-1",
		Status:        models.PRStatusOpen,
		AuthorID:      "u1",
		CreatedAt:     &now,
	}

	mockRepo.EXPECT().GetPRByID(ctx, "pr-1").Return(pr, nil)
	mockRepo.EXPECT().UpdatePRStatusWithMergedAt(ctx, "pr-1", models.PRStatusMerged).Return(nil)

	result, err := svc.MergePR(ctx, "pr-1")
	assert.NoError(t, err)
	assert.NotNil(t, result)
	assert.Equal(t, "pr-1", result.PullRequestID)
	assert.Equal(t, models.PRStatusMerged, result.Status)
	assert.Equal(t, "u1", result.AuthorID)
	assert.NotNil(t, result.MergedAt)
	assert.NotNil(t, result.CreatedAt)
}

func TestMergePR_AlreadyMerged(t *testing.T) {
	svc, mockRepo := setupService(t)
	ctx := context.Background()

	now := time.Now()
	mergedAt := now.Add(time.Hour)
	pr := &models.PullRequest{
		PullRequestID: "pr-1",
		Status:        models.PRStatusMerged,
		AuthorID:      "u1",
		CreatedAt:     &now,
		MergedAt:      &mergedAt,
	}

	mockRepo.EXPECT().GetPRByID(ctx, "pr-1").Return(pr, nil)

	result, err := svc.MergePR(ctx, "pr-1")
	assert.NoError(t, err)
	assert.Equal(t, pr, result)
	assert.Equal(t, models.PRStatusMerged, result.Status)
}

func TestMergePR_InvalidStatus(t *testing.T) {
	svc, mockRepo := setupService(t)
	ctx := context.Background()

	now := time.Now()
	pr := &models.PullRequest{
		PullRequestID: "pr-1",
		Status:        models.PRStatus("CLOSED"),
		AuthorID:      "u1",
		CreatedAt:     &now,
	}

	mockRepo.EXPECT().GetPRByID(ctx, "pr-1").Return(pr, nil)

	result, err := svc.MergePR(ctx, "pr-1")
	assert.Error(t, err)
	assert.Nil(t, result)
	assert.True(t, errors.Is(err, appErrors.ErrPRInvalidStatus))
}

func TestMergePR_PRNotFound(t *testing.T) {
	svc, mockRepo := setupService(t)
	ctx := context.Background()

	mockRepo.EXPECT().GetPRByID(ctx, "pr-1").Return(nil, appErrors.ErrPRNotFound)

	result, err := svc.MergePR(ctx, "pr-1")
	assert.Error(t, err)
	assert.Nil(t, result)
}

func TestReassignReviewer_Success(t *testing.T) {
	svc, mockRepo := setupService(t)
	ctx := context.Background()

	now := time.Now()
	pr := &models.PullRequest{
		PullRequestID:     "pr-1",
		AuthorID:          "u1",
		Status:            models.PRStatusOpen,
		AssignedReviewers: []string{"u2", "u3"},
		CreatedAt:         &now,
	}

	oldReviewer := &models.User{
		UserID:   "u2",
		Username: "Bob",
		TeamName: "backend",
		IsActive: true,
	}

	candidates := []*models.User{
		{UserID: "u4", Username: "David", TeamName: "backend", IsActive: true},
		{UserID: "u5", Username: "Eve", TeamName: "backend", IsActive: true},
	}

	mockRepo.EXPECT().GetPRByID(ctx, "pr-1").Return(pr, nil)
	mockRepo.EXPECT().GetUserByID(ctx, "u2").Return(oldReviewer, nil)
	mockRepo.EXPECT().GetActiveUsersByTeam(ctx, "backend", "u2").Return(candidates, nil)
	mockRepo.EXPECT().UpdatePRReviewers(ctx, "pr-1", mock.AnythingOfType("[]string")).Return(nil)

	resultPR, newReviewerID, err := svc.ReassignReviewer(ctx, "pr-1", "u2")
	assert.NoError(t, err)
	assert.NotNil(t, resultPR)
	assert.NotEmpty(t, newReviewerID)
	assert.NotEqual(t, "u2", newReviewerID)
	assert.Contains(t, resultPR.AssignedReviewers, newReviewerID)
	assert.NotContains(t, resultPR.AssignedReviewers, "u2")
}

func TestReassignReviewer_PRMerged(t *testing.T) {
	svc, mockRepo := setupService(t)
	ctx := context.Background()

	now := time.Now()
	mergedAt := now.Add(time.Hour)
	pr := &models.PullRequest{
		PullRequestID:     "pr-1",
		AuthorID:          "u1",
		Status:            models.PRStatusMerged,
		AssignedReviewers: []string{"u2"},
		CreatedAt:         &now,
		MergedAt:          &mergedAt,
	}

	mockRepo.EXPECT().GetPRByID(ctx, "pr-1").Return(pr, nil)

	resultPR, newReviewerID, err := svc.ReassignReviewer(ctx, "pr-1", "u2")
	assert.Error(t, err)
	assert.Nil(t, resultPR)
	assert.Empty(t, newReviewerID)
	assert.True(t, errors.Is(err, appErrors.ErrPRMerged))
}

func TestReassignReviewer_ReviewerNotAssigned(t *testing.T) {
	svc, mockRepo := setupService(t)
	ctx := context.Background()

	now := time.Now()
	pr := &models.PullRequest{
		PullRequestID:     "pr-1",
		AuthorID:          "u1",
		Status:            models.PRStatusOpen,
		AssignedReviewers: []string{"u2"},
		CreatedAt:         &now,
	}

	mockRepo.EXPECT().GetPRByID(ctx, "pr-1").Return(pr, nil)

	resultPR, newReviewerID, err := svc.ReassignReviewer(ctx, "pr-1", "u3")
	assert.Error(t, err)
	assert.Nil(t, resultPR)
	assert.Empty(t, newReviewerID)
	assert.True(t, errors.Is(err, appErrors.ErrReviewerNotAssigned))
}

func TestReassignReviewer_NoCandidates(t *testing.T) {
	svc, mockRepo := setupService(t)
	ctx := context.Background()

	now := time.Now()
	pr := &models.PullRequest{
		PullRequestID:     "pr-1",
		AuthorID:          "u1",
		Status:            models.PRStatusOpen,
		AssignedReviewers: []string{"u2"},
		CreatedAt:         &now,
	}

	oldReviewer := &models.User{
		UserID:   "u2",
		Username: "Bob",
		TeamName: "backend",
		IsActive: true,
	}

	candidates := []*models.User{
		{UserID: "u1", Username: "Alice", TeamName: "backend", IsActive: true},
	}

	mockRepo.EXPECT().GetPRByID(ctx, "pr-1").Return(pr, nil)
	mockRepo.EXPECT().GetUserByID(ctx, "u2").Return(oldReviewer, nil)
	mockRepo.EXPECT().GetActiveUsersByTeam(ctx, "backend", "u2").Return(candidates, nil)

	resultPR, newReviewerID, err := svc.ReassignReviewer(ctx, "pr-1", "u2")
	assert.Error(t, err)
	assert.Nil(t, resultPR)
	assert.Empty(t, newReviewerID)
	assert.True(t, errors.Is(err, appErrors.ErrNoCandidate))
}

func TestReassignReviewer_PRNotFound(t *testing.T) {
	svc, mockRepo := setupService(t)
	ctx := context.Background()

	mockRepo.EXPECT().GetPRByID(ctx, "pr-1").Return(nil, appErrors.ErrPRNotFound)

	resultPR, newReviewerID, err := svc.ReassignReviewer(ctx, "pr-1", "u2")
	assert.Error(t, err)
	assert.Nil(t, resultPR)
	assert.Empty(t, newReviewerID)
}

func TestReassignReviewer_OldReviewerNotFound(t *testing.T) {
	svc, mockRepo := setupService(t)
	ctx := context.Background()

	now := time.Now()
	pr := &models.PullRequest{
		PullRequestID:     "pr-1",
		AuthorID:          "u1",
		Status:            models.PRStatusOpen,
		AssignedReviewers: []string{"u2"},
		CreatedAt:         &now,
	}

	mockRepo.EXPECT().GetPRByID(ctx, "pr-1").Return(pr, nil)
	mockRepo.EXPECT().GetUserByID(ctx, "u2").Return(nil, appErrors.ErrUserNotFound)

	resultPR, newReviewerID, err := svc.ReassignReviewer(ctx, "pr-1", "u2")
	assert.Error(t, err)
	assert.Nil(t, resultPR)
	assert.Empty(t, newReviewerID)
	assert.Contains(t, err.Error(), "old reviewer not found")
}

func TestReassignReviewer_GetActiveUsersError(t *testing.T) {
	svc, mockRepo := setupService(t)
	ctx := context.Background()

	now := time.Now()
	pr := &models.PullRequest{
		PullRequestID:     "pr-1",
		AuthorID:          "u1",
		Status:            models.PRStatusOpen,
		AssignedReviewers: []string{"u2"},
		CreatedAt:         &now,
	}

	oldReviewer := &models.User{
		UserID:   "u2",
		Username: "Bob",
		TeamName: "backend",
		IsActive: true,
	}

	mockRepo.EXPECT().GetPRByID(ctx, "pr-1").Return(pr, nil)
	mockRepo.EXPECT().GetUserByID(ctx, "u2").Return(oldReviewer, nil)
	mockRepo.EXPECT().GetActiveUsersByTeam(ctx, "backend", "u2").Return(nil, errors.New("database error"))

	resultPR, newReviewerID, err := svc.ReassignReviewer(ctx, "pr-1", "u2")
	assert.Error(t, err)
	assert.Nil(t, resultPR)
	assert.Empty(t, newReviewerID)
	assert.Equal(t, "database error", err.Error())
}

func TestReassignReviewer_UpdateReviewersError(t *testing.T) {
	svc, mockRepo := setupService(t)
	ctx := context.Background()

	now := time.Now()
	pr := &models.PullRequest{
		PullRequestID:     "pr-1",
		AuthorID:          "u1",
		Status:            models.PRStatusOpen,
		AssignedReviewers: []string{"u2"},
		CreatedAt:         &now,
	}

	oldReviewer := &models.User{
		UserID:   "u2",
		Username: "Bob",
		TeamName: "backend",
		IsActive: true,
	}

	candidates := []*models.User{
		{UserID: "u4", Username: "David", TeamName: "backend", IsActive: true},
	}

	mockRepo.EXPECT().GetPRByID(ctx, "pr-1").Return(pr, nil)
	mockRepo.EXPECT().GetUserByID(ctx, "u2").Return(oldReviewer, nil)
	mockRepo.EXPECT().GetActiveUsersByTeam(ctx, "backend", "u2").Return(candidates, nil)
	mockRepo.EXPECT().UpdatePRReviewers(ctx, "pr-1", mock.AnythingOfType("[]string")).Return(errors.New("database error"))

	resultPR, newReviewerID, err := svc.ReassignReviewer(ctx, "pr-1", "u2")
	assert.Error(t, err)
	assert.Nil(t, resultPR)
	assert.Empty(t, newReviewerID)
	assert.Equal(t, "database error", err.Error())
}

func TestGetPRsByReviewer_GetPRsError(t *testing.T) {
	svc, mockRepo := setupService(t)
	ctx := context.Background()

	user := &models.User{
		UserID:   "u3",
		Username: "Charlie",
		IsActive: true,
	}

	mockRepo.EXPECT().GetUserByID(ctx, "u3").Return(user, nil)
	mockRepo.EXPECT().GetPRsByReviewer(ctx, "u3").Return(nil, errors.New("database error"))

	result, err := svc.GetPRsByReviewer(ctx, "u3")
	assert.Error(t, err)
	assert.Nil(t, result)
	assert.Equal(t, "database error", err.Error())
}

func TestMergePR_UpdateStatusError(t *testing.T) {
	svc, mockRepo := setupService(t)
	ctx := context.Background()

	now := time.Now()
	pr := &models.PullRequest{
		PullRequestID: "pr-1",
		Status:        models.PRStatusOpen,
		AuthorID:      "u1",
		CreatedAt:     &now,
	}

	mockRepo.EXPECT().GetPRByID(ctx, "pr-1").Return(pr, nil)
	mockRepo.EXPECT().UpdatePRStatusWithMergedAt(ctx, "pr-1", models.PRStatusMerged).Return(errors.New("database error"))

	result, err := svc.MergePR(ctx, "pr-1")
	assert.Error(t, err)
	assert.Nil(t, result)
	assert.Equal(t, "database error", err.Error())
}

func TestGetPRsByReviewer_Success(t *testing.T) {
	svc, mockRepo := setupService(t)
	ctx := context.Background()

	now := time.Now()
	prs := []*models.PullRequest{
		{
			PullRequestID:   "pr-1",
			PullRequestName: "PR 1",
			AuthorID:        "u1",
			Status:          models.PRStatusOpen,
			CreatedAt:       &now,
		},
		{
			PullRequestID:   "pr-2",
			PullRequestName: "PR 2",
			AuthorID:        "u2",
			Status:          models.PRStatusMerged,
			CreatedAt:       &now,
		},
	}

	user := &models.User{
		UserID:   "u3",
		Username: "Charlie",
		IsActive: true,
	}

	mockRepo.EXPECT().GetUserByID(ctx, "u3").Return(user, nil)
	mockRepo.EXPECT().GetPRsByReviewer(ctx, "u3").Return(prs, nil)

	result, err := svc.GetPRsByReviewer(ctx, "u3")
	assert.NoError(t, err)
	assert.NotNil(t, result)
	assert.Len(t, result, 2)

	assert.Equal(t, "pr-1", result[0].PullRequestID)
	assert.Equal(t, "PR 1", result[0].PullRequestName)
	assert.Equal(t, "u1", result[0].AuthorID)
	assert.Equal(t, models.PRStatusOpen, result[0].Status)

	assert.Equal(t, "pr-2", result[1].PullRequestID)
	assert.Equal(t, "PR 2", result[1].PullRequestName)
	assert.Equal(t, "u2", result[1].AuthorID)
	assert.Equal(t, models.PRStatusMerged, result[1].Status)
}

func TestGetPRsByReviewer_UserNotFound(t *testing.T) {
	svc, mockRepo := setupService(t)
	ctx := context.Background()

	mockRepo.EXPECT().GetUserByID(ctx, "u3").Return(nil, appErrors.ErrUserNotFound)

	result, err := svc.GetPRsByReviewer(ctx, "u3")
	assert.Error(t, err)
	assert.Nil(t, result)
}

func TestGetStats_Success(t *testing.T) {
	svc, mockRepo := setupService(t)
	ctx := context.Background()

	expectedStats := map[string]interface{}{
		"total_prs":   10,
		"total_users": 5,
		"total_teams": 2,
	}

	mockRepo.EXPECT().GetStats(ctx).Return(expectedStats, nil)

	stats, err := svc.GetStats(ctx)
	assert.NoError(t, err)
	assert.Equal(t, expectedStats, stats)
}

func TestGetStats_Error(t *testing.T) {
	svc, mockRepo := setupService(t)
	ctx := context.Background()

	mockRepo.EXPECT().GetStats(ctx).Return(nil, errors.New("database error"))

	stats, err := svc.GetStats(ctx)
	assert.Error(t, err)
	assert.Nil(t, stats)
}

func TestPing_Success(t *testing.T) {
	svc, mockRepo := setupService(t)
	ctx := context.Background()

	stats := map[string]interface{}{"test": "ok"}
	mockRepo.EXPECT().GetStats(ctx).Return(stats, nil)

	err := svc.Ping(ctx)
	assert.NoError(t, err)
}

func TestPing_Error(t *testing.T) {
	svc, mockRepo := setupService(t)
	ctx := context.Background()

	mockRepo.EXPECT().GetStats(ctx).Return(nil, errors.New("database error"))

	err := svc.Ping(ctx)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "repository ping failed")
}

func TestDeactivateTeamUsers_TeamNotFound(t *testing.T) {
	svc, mockRepo := setupService(t)
	ctx := context.Background()

	mockRepo.EXPECT().TeamExists(ctx, "nonexistent").Return(false, nil)

	result, err := svc.DeactivateTeamUsers(ctx, "nonexistent", []string{"u1"})
	assert.Error(t, err)
	assert.Nil(t, result)
	assert.True(t, errors.Is(err, appErrors.ErrTeamNotFound))
}

func TestDeactivateTeamUsers_TeamExistsError(t *testing.T) {
	svc, mockRepo := setupService(t)
	ctx := context.Background()

	mockRepo.EXPECT().TeamExists(ctx, "backend").Return(false, errors.New("database error"))

	result, err := svc.DeactivateTeamUsers(ctx, "backend", []string{"u1"})
	assert.Error(t, err)
	assert.Nil(t, result)
	assert.Contains(t, err.Error(), "failed to check team existence")
}

func TestDeactivateTeamUsers_NoUsersDeactivated(t *testing.T) {
	svc, mockRepo := setupService(t)
	ctx := context.Background()

	mockRepo.EXPECT().TeamExists(ctx, "backend").Return(true, nil)
	mockRepo.EXPECT().DeactivateUsersByTeam(ctx, "backend", []string{"u1"}).Return([]string{}, nil)

	result, err := svc.DeactivateTeamUsers(ctx, "backend", []string{"u1"})
	assert.NoError(t, err)
	assert.NotNil(t, result)
	assert.Empty(t, result["deactivated_users"])
	assert.Empty(t, result["reassigned_prs"])
}

func TestDeactivateTeamUsers_NoPRsToReassign(t *testing.T) {
	svc, mockRepo := setupService(t)
	ctx := context.Background()

	deactivatedIDs := []string{"u1", "u2"}

	mockRepo.EXPECT().TeamExists(ctx, "backend").Return(true, nil)
	mockRepo.EXPECT().DeactivateUsersByTeam(ctx, "backend", []string{"u1", "u2"}).Return(deactivatedIDs, nil)
	mockRepo.EXPECT().GetOpenPRsByReviewers(ctx, deactivatedIDs).Return([]*models.PullRequest{}, nil)

	result, err := svc.DeactivateTeamUsers(ctx, "backend", []string{"u1", "u2"})
	assert.NoError(t, err)
	assert.NotNil(t, result)
	assert.Equal(t, deactivatedIDs, result["deactivated_users"])
	assert.Empty(t, result["reassigned_prs"])
}

func TestDeactivateTeamUsers_SuccessWithReassignment(t *testing.T) {
	svc, mockRepo := setupService(t)
	ctx := context.Background()

	deactivatedIDs := []string{"u2"}
	now := time.Now()

	openPRs := []*models.PullRequest{
		{
			PullRequestID:     "pr-1",
			PullRequestName:   "Test PR 1",
			AuthorID:          "u1",
			Status:            models.PRStatusOpen,
			AssignedReviewers: []string{"u2", "u3"},
			CreatedAt:         &now,
		},
	}

	oldReviewer := &models.User{
		UserID:   "u2",
		Username: "Bob",
		TeamName: "backend",
		IsActive: false,
	}

	candidates := []*models.User{
		{
			UserID:   "u4",
			Username: "Dave",
			TeamName: "backend",
			IsActive: true,
		},
		{
			UserID:   "u5",
			Username: "Eve",
			TeamName: "backend",
			IsActive: true,
		},
	}

	mockRepo.EXPECT().TeamExists(ctx, "backend").Return(true, nil)
	mockRepo.EXPECT().DeactivateUsersByTeam(ctx, "backend", []string{"u2"}).Return(deactivatedIDs, nil)
	mockRepo.EXPECT().GetOpenPRsByReviewers(ctx, deactivatedIDs).Return(openPRs, nil)
	mockRepo.EXPECT().GetUserByID(ctx, "u2").Return(oldReviewer, nil)
	mockRepo.EXPECT().GetActiveUsersByTeam(ctx, "backend", "").Return(candidates, nil)
	mockRepo.EXPECT().BulkUpdatePRReviewers(ctx, mock.MatchedBy(func(updates map[string][]string) bool {
		if reviewers, ok := updates["pr-1"]; ok {
			if len(reviewers) != 2 {
				return false
			}
			hasU3 := false
			hasNewCandidate := false
			for _, r := range reviewers {
				if r == "u3" {
					hasU3 = true
				}
				if r == "u4" || r == "u5" {
					hasNewCandidate = true
				}
			}
			return hasU3 && hasNewCandidate
		}
		return false
	})).Return(nil)

	result, err := svc.DeactivateTeamUsers(ctx, "backend", []string{"u2"})
	assert.NoError(t, err)
	assert.NotNil(t, result)
	assert.Equal(t, deactivatedIDs, result["deactivated_users"])
	reassignedPRs := result["reassigned_prs"].([]string)
	assert.Len(t, reassignedPRs, 1)
	assert.Equal(t, "pr-1", reassignedPRs[0])
}

func TestDeactivateTeamUsers_DeactivateAllUsers(t *testing.T) {
	svc, mockRepo := setupService(t)
	ctx := context.Background()

	deactivatedIDs := []string{"u1", "u2", "u3"}

	mockRepo.EXPECT().TeamExists(ctx, "backend").Return(true, nil)
	mockRepo.EXPECT().DeactivateUsersByTeam(ctx, "backend", []string{}).Return(deactivatedIDs, nil)
	mockRepo.EXPECT().GetOpenPRsByReviewers(ctx, deactivatedIDs).Return([]*models.PullRequest{}, nil)

	result, err := svc.DeactivateTeamUsers(ctx, "backend", []string{})
	assert.NoError(t, err)
	assert.NotNil(t, result)
	assert.Equal(t, deactivatedIDs, result["deactivated_users"])
	assert.Empty(t, result["reassigned_prs"])
}

func TestDeactivateTeamUsers_GetOpenPRsError(t *testing.T) {
	svc, mockRepo := setupService(t)
	ctx := context.Background()

	deactivatedIDs := []string{"u1", "u2"}

	mockRepo.EXPECT().TeamExists(ctx, "backend").Return(true, nil)
	mockRepo.EXPECT().DeactivateUsersByTeam(ctx, "backend", []string{"u1", "u2"}).Return(deactivatedIDs, nil)
	mockRepo.EXPECT().GetOpenPRsByReviewers(ctx, deactivatedIDs).Return(nil, errors.New("database error"))

	result, err := svc.DeactivateTeamUsers(ctx, "backend", []string{"u1", "u2"})
	assert.Error(t, err)
	assert.Nil(t, result)
	assert.Contains(t, err.Error(), "failed to get open PRs")
}

func TestDeactivateTeamUsers_GetUserByIDError(t *testing.T) {
	svc, mockRepo := setupService(t)
	ctx := context.Background()

	deactivatedIDs := []string{"u2"}
	now := time.Now()

	openPRs := []*models.PullRequest{
		{
			PullRequestID:     "pr-1",
			PullRequestName:   "Test PR 1",
			AuthorID:          "u1",
			Status:            models.PRStatusOpen,
			AssignedReviewers: []string{"u2"},
			CreatedAt:         &now,
		},
	}

	mockRepo.EXPECT().TeamExists(ctx, "backend").Return(true, nil)
	mockRepo.EXPECT().DeactivateUsersByTeam(ctx, "backend", []string{"u2"}).Return(deactivatedIDs, nil)
	mockRepo.EXPECT().GetOpenPRsByReviewers(ctx, deactivatedIDs).Return(openPRs, nil)
	mockRepo.EXPECT().GetUserByID(ctx, "u2").Return(nil, errors.New("user not found"))
	mockRepo.EXPECT().BulkUpdatePRReviewers(ctx, map[string][]string{"pr-1": {}}).Return(nil)

	result, err := svc.DeactivateTeamUsers(ctx, "backend", []string{"u2"})
	assert.NoError(t, err)
	assert.NotNil(t, result)
	assert.Equal(t, deactivatedIDs, result["deactivated_users"])
	assert.NotEmpty(t, result["reassigned_prs"])
}

func TestDeactivateTeamUsers_BulkUpdateError(t *testing.T) {
	svc, mockRepo := setupService(t)
	ctx := context.Background()

	deactivatedIDs := []string{"u2"}
	now := time.Now()

	openPRs := []*models.PullRequest{
		{
			PullRequestID:     "pr-1",
			PullRequestName:   "Test PR 1",
			AuthorID:          "u1",
			Status:            models.PRStatusOpen,
			AssignedReviewers: []string{"u2"},
			CreatedAt:         &now,
		},
	}

	oldReviewer := &models.User{
		UserID:   "u2",
		Username: "Bob",
		TeamName: "backend",
		IsActive: false,
	}

	candidates := []*models.User{
		{
			UserID:   "u4",
			Username: "Dave",
			TeamName: "backend",
			IsActive: true,
		},
	}

	mockRepo.EXPECT().TeamExists(ctx, "backend").Return(true, nil)
	mockRepo.EXPECT().DeactivateUsersByTeam(ctx, "backend", []string{"u2"}).Return(deactivatedIDs, nil)
	mockRepo.EXPECT().GetOpenPRsByReviewers(ctx, deactivatedIDs).Return(openPRs, nil)
	mockRepo.EXPECT().GetUserByID(ctx, "u2").Return(oldReviewer, nil)
	mockRepo.EXPECT().GetActiveUsersByTeam(ctx, "backend", "").Return(candidates, nil)
	mockRepo.EXPECT().BulkUpdatePRReviewers(ctx, mock.Anything).Return(errors.New("database error"))

	result, err := svc.DeactivateTeamUsers(ctx, "backend", []string{"u2"})
	assert.Error(t, err)
	assert.Nil(t, result)
	assert.Contains(t, err.Error(), "failed to update PR reviewers")
}

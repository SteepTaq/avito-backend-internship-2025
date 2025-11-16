//go:build e2e

package e2e

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"os"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/SteepTaq/avito-backend-internship-2025/internal/models"
	"github.com/SteepTaq/avito-backend-internship-2025/pkg/logger"
)

var (
	testClient *http.Client
	baseURL    string
)

func TestMain(m *testing.M) {
	log := logger.New("debug")

	appURL := os.Getenv("E2E_APP_URL")
	if appURL == "" {
		appURL = "http://localhost:8081"
	}

	log.Info("Using Docker application for E2E tests", "url", appURL)
	baseURL = appURL
	testClient = &http.Client{Timeout: 10 * time.Second}

	for i := 0; i < 30; i++ {
		resp, err := testClient.Get(appURL + "/health")
		if err == nil && resp.StatusCode == http.StatusOK {
			resp.Body.Close()
			log.Info("Docker application is ready")
			break
		}
		if i == 29 {
			log.Error("Docker application is not available", "url", appURL)
			os.Exit(1)
		}
		time.Sleep(1 * time.Second)
	}

	code := m.Run()
	os.Exit(code)
}

func cleanupTestData(t *testing.T) {
	t.Helper()
	ctx := context.Background()

	dsn := os.Getenv("TEST_DATABASE_URL")
	if dsn == "" {
		dsn = "postgres://postgres:postgres@localhost:5433/test_db?sslmode=disable"
	}

	pool, err := pgxpool.New(ctx, dsn)
	require.NoError(t, err, "Failed to connect to database for cleanup")
	defer pool.Close()

	_, err = pool.Exec(ctx, "TRUNCATE TABLE pull_requests, users CASCADE")
	require.NoError(t, err, "Failed to cleanup test data")
}

func TestE2E_TeamFlow(t *testing.T) {
	cleanupTestData(t)

	t.Run("Create Team", func(t *testing.T) {
		team := models.Team{
			TeamName: "backend-team",
			Members: []models.TeamMember{
				{UserID: "user1", Username: "Alice", IsActive: true},
				{UserID: "user2", Username: "Bob", IsActive: true},
				{UserID: "user3", Username: "Charlie", IsActive: false},
			},
		}

		body, err := json.Marshal(team)
		require.NoError(t, err, "Failed to marshal team")

		resp, err := testClient.Post(baseURL+"/team/add", "application/json", bytes.NewBuffer(body))
		require.NoError(t, err, "Failed to create team")
		defer func() { _ = resp.Body.Close() }()

		assert.Equal(t, http.StatusCreated, resp.StatusCode, "should return 201 Created")
		assert.Equal(t, "application/json", resp.Header.Get("Content-Type"), "should set Content-Type header")

		var result struct {
			Team models.Team `json:"team"`
		}
		err = json.NewDecoder(resp.Body).Decode(&result)
		require.NoError(t, err, "Failed to decode response")

		assert.Equal(t, "backend-team", result.Team.TeamName, "team name should match")
		assert.Len(t, result.Team.Members, 3, "should have 3 members")
		assert.Equal(t, "user1", result.Team.Members[0].UserID)
		assert.Equal(t, "user2", result.Team.Members[1].UserID)
		assert.Equal(t, "user3", result.Team.Members[2].UserID)
	})

	t.Run("Get Team", func(t *testing.T) {
		resp, err := testClient.Get(baseURL + "/team/get?team_name=backend-team")
		require.NoError(t, err, "Failed to get team")
		defer func() { _ = resp.Body.Close() }()

		assert.Equal(t, http.StatusOK, resp.StatusCode, "should return 200 OK")
		assert.Equal(t, "application/json", resp.Header.Get("Content-Type"), "should set Content-Type header")

		var team models.Team
		err = json.NewDecoder(resp.Body).Decode(&team)
		require.NoError(t, err, "Failed to decode response")

		assert.Equal(t, "backend-team", team.TeamName, "team name should match")
		assert.Len(t, team.Members, 3, "should have 3 members")
	})

	t.Run("Create Duplicate Team", func(t *testing.T) {
		team := models.Team{
			TeamName: "backend-team",
			Members:  []models.TeamMember{},
		}

		body, err := json.Marshal(team)
		require.NoError(t, err, "Failed to marshal team")

		resp, err := testClient.Post(baseURL+"/team/add", "application/json", bytes.NewBuffer(body))
		require.NoError(t, err, "Failed to make request")
		defer func() { _ = resp.Body.Close() }()

		assert.Equal(t, http.StatusBadRequest, resp.StatusCode, "should return 400 Bad Request")
		assert.Equal(t, "application/json", resp.Header.Get("Content-Type"), "should set Content-Type header")

		var errorResp models.ErrorResponse
		err = json.NewDecoder(resp.Body).Decode(&errorResp)
		require.NoError(t, err, "Failed to decode error response")
		assert.NotEmpty(t, errorResp.Error.Message, "error message should not be empty")
	})
}

func TestE2E_PullRequestFlow(t *testing.T) {
	cleanupTestData(t)

	team := models.Team{
		TeamName: "dev-team",
		Members: []models.TeamMember{
			{UserID: "author", Username: "Author", IsActive: true},
			{UserID: "reviewer1", Username: "Reviewer1", IsActive: true},
			{UserID: "reviewer2", Username: "Reviewer2", IsActive: true},
			{UserID: "reviewer3", Username: "Reviewer3", IsActive: true},
		},
	}

	body, err := json.Marshal(team)
	require.NoError(t, err, "Failed to marshal team")

	resp, err := testClient.Post(baseURL+"/team/add", "application/json", bytes.NewBuffer(body))
	require.NoError(t, err, "Failed to create team")
	require.Equal(t, http.StatusCreated, resp.StatusCode, "team should be created")
	_ = resp.Body.Close()

	t.Run("Create Pull Request", func(t *testing.T) {
		prInput := models.CreatePRRequest{
			PullRequestID:   "pr-1",
			PullRequestName: "Add feature X",
			AuthorID:        "author",
		}

		body, err := json.Marshal(prInput)
		require.NoError(t, err, "Failed to marshal PR request")

		resp, err := testClient.Post(baseURL+"/pullRequest/create", "application/json", bytes.NewBuffer(body))
		require.NoError(t, err, "Failed to create PR")
		defer func() { _ = resp.Body.Close() }()

		assert.Equal(t, http.StatusCreated, resp.StatusCode, "should return 201 Created")
		assert.Equal(t, "application/json", resp.Header.Get("Content-Type"), "should set Content-Type header")

		var result struct {
			Pr models.PullRequest `json:"pr"`
		}
		err = json.NewDecoder(resp.Body).Decode(&result)
		require.NoError(t, err, "Failed to decode response")

		assert.Equal(t, "pr-1", result.Pr.PullRequestID, "PR ID should match")
		assert.Equal(t, "Add feature X", result.Pr.PullRequestName, "PR name should match")
		assert.Equal(t, "author", result.Pr.AuthorID, "author ID should match")
		assert.Equal(t, models.PRStatusOpen, result.Pr.Status, "PR status should be OPEN")
		assert.NotNil(t, result.Pr.CreatedAt, "CreatedAt should be set")
		assert.NotEmpty(t, result.Pr.AssignedReviewers, "PR should have assigned reviewers")

		for _, reviewer := range result.Pr.AssignedReviewers {
			assert.NotEqual(t, "author", reviewer, "author should not be assigned as reviewer")
		}
	})

	t.Run("Get Reviewer Assignments", func(t *testing.T) {
		time.Sleep(100 * time.Millisecond)

		resp, err := testClient.Get(baseURL + "/users/getReview?user_id=reviewer1")
		require.NoError(t, err, "Failed to get assignments")
		defer func() { _ = resp.Body.Close() }()

		assert.Equal(t, http.StatusOK, resp.StatusCode, "should return 200 OK")
		assert.Equal(t, "application/json", resp.Header.Get("Content-Type"), "should set Content-Type header")

		var result struct {
			UserId       string                    `json:"user_id"`
			PullRequests []models.PullRequestShort `json:"pull_requests"`
		}
		err = json.NewDecoder(resp.Body).Decode(&result)
		require.NoError(t, err, "Failed to decode response")

		assert.Equal(t, "reviewer1", result.UserId, "user ID should match")
		assert.GreaterOrEqual(t, len(result.PullRequests), 0, "pull requests list should exist")
	})

	t.Run("Merge Pull Request", func(t *testing.T) {
		mergeInput := models.MergePRRequest{
			PullRequestID: "pr-1",
		}

		body, err := json.Marshal(mergeInput)
		require.NoError(t, err, "Failed to marshal merge request")

		resp, err := testClient.Post(baseURL+"/pullRequest/merge", "application/json", bytes.NewBuffer(body))
		require.NoError(t, err, "Failed to merge PR")
		defer func() { _ = resp.Body.Close() }()

		assert.Equal(t, http.StatusOK, resp.StatusCode, "should return 200 OK")
		assert.Equal(t, "application/json", resp.Header.Get("Content-Type"), "should set Content-Type header")

		var result struct {
			Pr models.PullRequest `json:"pr"`
		}
		err = json.NewDecoder(resp.Body).Decode(&result)
		require.NoError(t, err, "Failed to decode response")

		assert.Equal(t, "pr-1", result.Pr.PullRequestID, "PR ID should match")
		assert.Equal(t, models.PRStatusMerged, result.Pr.Status, "PR status should be MERGED")
		assert.NotNil(t, result.Pr.MergedAt, "MergedAt should be set")
		assert.NotNil(t, result.Pr.CreatedAt, "CreatedAt should be set")
	})

	t.Run("Merge Idempotency", func(t *testing.T) {
		mergeInput := models.MergePRRequest{
			PullRequestID: "pr-1",
		}

		body, err := json.Marshal(mergeInput)
		require.NoError(t, err, "Failed to marshal merge request")

		resp, err := testClient.Post(baseURL+"/pullRequest/merge", "application/json", bytes.NewBuffer(body))
		require.NoError(t, err, "Failed to merge PR")
		defer func() { _ = resp.Body.Close() }()

		assert.Equal(t, http.StatusOK, resp.StatusCode, "should return 200 OK (idempotent)")

		var result struct {
			Pr models.PullRequest `json:"pr"`
		}
		err = json.NewDecoder(resp.Body).Decode(&result)
		require.NoError(t, err, "Failed to decode response")
		assert.Equal(t, models.PRStatusMerged, result.Pr.Status, "PR status should still be MERGED")
	})

	t.Run("Reassign on Merged PR Should Fail", func(t *testing.T) {
		reassignInput := models.ReassignReviewerRequest{
			PullRequestID: "pr-1",
			OldUserID:     "reviewer1",
		}

		body, err := json.Marshal(reassignInput)
		require.NoError(t, err, "Failed to marshal reassign request")

		resp, err := testClient.Post(baseURL+"/pullRequest/reassign", "application/json", bytes.NewBuffer(body))
		require.NoError(t, err, "Failed to reassign PR")
		defer func() { _ = resp.Body.Close() }()

		assert.Equal(t, http.StatusConflict, resp.StatusCode, "should return 409 Conflict")
		assert.Equal(t, "application/json", resp.Header.Get("Content-Type"), "should set Content-Type header")

		var errorResp models.ErrorResponse
		err = json.NewDecoder(resp.Body).Decode(&errorResp)
		require.NoError(t, err, "Failed to decode error response")
		assert.NotEmpty(t, errorResp.Error.Message, "error message should not be empty")
	})
}

func TestE2E_ReassignFlow(t *testing.T) {
	cleanupTestData(t)

	team := models.Team{
		TeamName: "reassign-team",
		Members: []models.TeamMember{
			{UserID: "author", Username: "Author", IsActive: true},
			{UserID: "rev1", Username: "Reviewer1", IsActive: true},
			{UserID: "rev2", Username: "Reviewer2", IsActive: true},
			{UserID: "rev3", Username: "Reviewer3", IsActive: true},
		},
	}

	body, err := json.Marshal(team)
	require.NoError(t, err, "Failed to marshal team")

	resp, err := testClient.Post(baseURL+"/team/add", "application/json", bytes.NewBuffer(body))
	require.NoError(t, err, "Failed to create team")
	require.Equal(t, http.StatusCreated, resp.StatusCode, "team should be created")
	_ = resp.Body.Close()

	prInput := models.CreatePRRequest{
		PullRequestID:   "pr-reassign",
		PullRequestName: "Test Reassign",
		AuthorID:        "author",
	}

	body, err = json.Marshal(prInput)
	require.NoError(t, err, "Failed to marshal PR request")

	resp, err = testClient.Post(baseURL+"/pullRequest/create", "application/json", bytes.NewBuffer(body))
	require.NoError(t, err, "Failed to create PR")
	require.Equal(t, http.StatusCreated, resp.StatusCode, "PR should be created")

	var createResult struct {
		Pr models.PullRequest `json:"pr"`
	}
	err = json.NewDecoder(resp.Body).Decode(&createResult)
	require.NoError(t, err, "Failed to decode PR creation response")
	_ = resp.Body.Close()

	if len(createResult.Pr.AssignedReviewers) == 0 {
		t.Skip("No reviewers assigned, cannot test reassignment")
	}

	oldReviewer := createResult.Pr.AssignedReviewers[0]

	t.Run("Reassign Reviewer", func(t *testing.T) {
		reassignInput := models.ReassignReviewerRequest{
			PullRequestID: "pr-reassign",
			OldUserID:     oldReviewer,
		}

		body, err := json.Marshal(reassignInput)
		require.NoError(t, err, "Failed to marshal reassign request")

		resp, err := testClient.Post(baseURL+"/pullRequest/reassign", "application/json", bytes.NewBuffer(body))
		require.NoError(t, err, "Failed to reassign")
		defer func() { _ = resp.Body.Close() }()

		assert.Equal(t, http.StatusOK, resp.StatusCode, "should return 200 OK")
		assert.Equal(t, "application/json", resp.Header.Get("Content-Type"), "should set Content-Type header")

		var result struct {
			Pr         models.PullRequest `json:"pr"`
			ReplacedBy string             `json:"replaced_by"`
		}
		err = json.NewDecoder(resp.Body).Decode(&result)
		require.NoError(t, err, "Failed to decode response")

		assert.NotEmpty(t, result.ReplacedBy, "ReplacedBy should not be empty")
		assert.NotEqual(t, oldReviewer, result.ReplacedBy, "new reviewer should not be the same as old reviewer")
		assert.Equal(t, "pr-reassign", result.Pr.PullRequestID, "PR ID should match")

		found := false
		for _, reviewer := range result.Pr.AssignedReviewers {
			if reviewer == oldReviewer {
				found = true
				break
			}
		}
		assert.False(t, found, "old reviewer should not be in assigned reviewers")

		found = false
		for _, reviewer := range result.Pr.AssignedReviewers {
			if reviewer == result.ReplacedBy {
				found = true
				break
			}
		}
		assert.True(t, found, "new reviewer should be in assigned reviewers")
	})
}

func TestE2E_SetUserIsActive(t *testing.T) {
	cleanupTestData(t)

	team := models.Team{
		TeamName: "active-team",
		Members: []models.TeamMember{
			{UserID: "test-user", Username: "TestUser", IsActive: true},
		},
	}

	body, err := json.Marshal(team)
	require.NoError(t, err, "Failed to marshal team")

	resp, err := testClient.Post(baseURL+"/team/add", "application/json", bytes.NewBuffer(body))
	require.NoError(t, err, "Failed to create team")
	require.Equal(t, http.StatusCreated, resp.StatusCode, "team should be created")
	_ = resp.Body.Close()

	t.Run("Deactivate User", func(t *testing.T) {
		input := models.SetUserActiveRequest{
			UserID:   "test-user",
			IsActive: false,
		}

		body, err := json.Marshal(input)
		require.NoError(t, err, "Failed to marshal request")

		resp, err := testClient.Post(baseURL+"/users/setIsActive", "application/json", bytes.NewBuffer(body))
		require.NoError(t, err, "Failed to set user active")
		defer func() { _ = resp.Body.Close() }()

		assert.Equal(t, http.StatusOK, resp.StatusCode, "should return 200 OK")
		assert.Equal(t, "application/json", resp.Header.Get("Content-Type"), "should set Content-Type header")

		var result struct {
			User models.User `json:"user"`
		}
		err = json.NewDecoder(resp.Body).Decode(&result)
		require.NoError(t, err, "Failed to decode response")

		assert.Equal(t, "test-user", result.User.UserID, "user ID should match")
		assert.False(t, result.User.IsActive, "user should be inactive")
	})

	t.Run("Reactivate User", func(t *testing.T) {
		input := models.SetUserActiveRequest{
			UserID:   "test-user",
			IsActive: true,
		}

		body, err := json.Marshal(input)
		require.NoError(t, err, "Failed to marshal request")

		resp, err := testClient.Post(baseURL+"/users/setIsActive", "application/json", bytes.NewBuffer(body))
		require.NoError(t, err, "Failed to set user active")
		defer func() { _ = resp.Body.Close() }()

		assert.Equal(t, http.StatusOK, resp.StatusCode, "should return 200 OK")
		assert.Equal(t, "application/json", resp.Header.Get("Content-Type"), "should set Content-Type header")

		var result struct {
			User models.User `json:"user"`
		}
		err = json.NewDecoder(resp.Body).Decode(&result)
		require.NoError(t, err, "Failed to decode response")

		assert.Equal(t, "test-user", result.User.UserID, "user ID should match")
		assert.True(t, result.User.IsActive, "user should be active")
	})
}

func TestE2E_ErrorCases(t *testing.T) {
	cleanupTestData(t)

	t.Run("Get Non-Existing Team", func(t *testing.T) {
		resp, err := testClient.Get(baseURL + "/team/get?team_name=non-existing")
		require.NoError(t, err, "Failed to get team")
		defer func() { _ = resp.Body.Close() }()

		assert.Equal(t, http.StatusNotFound, resp.StatusCode, "should return 404 Not Found")
		assert.Equal(t, "application/json", resp.Header.Get("Content-Type"), "should set Content-Type header")

		var errorResp models.ErrorResponse
		err = json.NewDecoder(resp.Body).Decode(&errorResp)
		require.NoError(t, err, "Failed to decode error response")
		assert.NotEmpty(t, errorResp.Error.Message, "error message should not be empty")
	})

	t.Run("Create PR with Non-Existing Author", func(t *testing.T) {
		prInput := models.CreatePRRequest{
			PullRequestID:   "pr-bad",
			PullRequestName: "Bad PR",
			AuthorID:        "non-existing-author",
		}

		body, err := json.Marshal(prInput)
		require.NoError(t, err, "Failed to marshal PR request")

		resp, err := testClient.Post(baseURL+"/pullRequest/create", "application/json", bytes.NewBuffer(body))
		require.NoError(t, err, "Failed to create PR")
		defer func() { _ = resp.Body.Close() }()

		assert.Equal(t, http.StatusNotFound, resp.StatusCode, "should return 404 Not Found")
		assert.Equal(t, "application/json", resp.Header.Get("Content-Type"), "should set Content-Type header")

		var errorResp models.ErrorResponse
		err = json.NewDecoder(resp.Body).Decode(&errorResp)
		require.NoError(t, err, "Failed to decode error response")
		assert.NotEmpty(t, errorResp.Error.Message, "error message should not be empty")
	})

	t.Run("Set Active for Non-Existing User", func(t *testing.T) {
		input := models.SetUserActiveRequest{
			UserID:   "non-existing-user",
			IsActive: false,
		}

		body, err := json.Marshal(input)
		require.NoError(t, err, "Failed to marshal request")

		resp, err := testClient.Post(baseURL+"/users/setIsActive", "application/json", bytes.NewBuffer(body))
		require.NoError(t, err, "Failed to set user active")
		defer func() { _ = resp.Body.Close() }()

		assert.Equal(t, http.StatusNotFound, resp.StatusCode, "should return 404 Not Found")
		assert.Equal(t, "application/json", resp.Header.Get("Content-Type"), "should set Content-Type header")

		var errorResp models.ErrorResponse
		err = json.NewDecoder(resp.Body).Decode(&errorResp)
		require.NoError(t, err, "Failed to decode error response")
		assert.NotEmpty(t, errorResp.Error.Message, "error message should not be empty")
	})
}

func TestE2E_DeactivateTeamUsers(t *testing.T) {
	cleanupTestData(t)

	team := models.Team{
		TeamName: "deactivate-team",
		Members: []models.TeamMember{
			{UserID: "author", Username: "Author", IsActive: true},
			{UserID: "rev1", Username: "Reviewer1", IsActive: true},
			{UserID: "rev2", Username: "Reviewer2", IsActive: true},
			{UserID: "rev3", Username: "Reviewer3", IsActive: true},
		},
	}

	body, err := json.Marshal(team)
	require.NoError(t, err, "Failed to marshal team")

	resp, err := testClient.Post(baseURL+"/team/add", "application/json", bytes.NewBuffer(body))
	require.NoError(t, err, "Failed to create team")
	require.Equal(t, http.StatusCreated, resp.StatusCode, "team should be created")
	_ = resp.Body.Close()

	prInput := models.CreatePRRequest{
		PullRequestID:   "pr-deactivate",
		PullRequestName: "Test Deactivate",
		AuthorID:        "author",
	}

	body, err = json.Marshal(prInput)
	require.NoError(t, err, "Failed to marshal PR request")

	resp, err = testClient.Post(baseURL+"/pullRequest/create", "application/json", bytes.NewBuffer(body))
	require.NoError(t, err, "Failed to create PR")
	require.Equal(t, http.StatusCreated, resp.StatusCode, "PR should be created")
	_ = resp.Body.Close()

	t.Run("Deactivate Team Users", func(t *testing.T) {
		input := models.DeactivateTeamUsersRequest{
			TeamName: "deactivate-team",
			UserIDs:  []string{"rev1", "rev2"},
		}

		body, err := json.Marshal(input)
		require.NoError(t, err, "Failed to marshal request")

		resp, err := testClient.Post(baseURL+"/users/deactivateTeam", "application/json", bytes.NewBuffer(body))
		require.NoError(t, err, "Failed to deactivate team users")
		defer func() { _ = resp.Body.Close() }()

		assert.Equal(t, http.StatusOK, resp.StatusCode, "should return 200 OK")
		assert.Equal(t, "application/json", resp.Header.Get("Content-Type"), "should set Content-Type header")

		var result map[string]interface{}
		err = json.NewDecoder(resp.Body).Decode(&result)
		require.NoError(t, err, "Failed to decode response")

		deactivatedUsers, ok := result["deactivated_users"].([]interface{})
		require.True(t, ok, "deactivated_users should be present in response")
		assert.Greater(t, len(deactivatedUsers), 0, "at least some users should be deactivated")

		_, hasReassigned := result["reassigned_prs"]
		assert.True(t, hasReassigned, "reassigned_prs should be present in response")
	})
}

func TestE2E_Stats(t *testing.T) {
	cleanupTestData(t)

	team := models.Team{
		TeamName: "stats-team",
		Members: []models.TeamMember{
			{UserID: "author", Username: "Author", IsActive: true},
			{UserID: "reviewer", Username: "Reviewer", IsActive: true},
		},
	}

	body, err := json.Marshal(team)
	require.NoError(t, err, "Failed to marshal team")

	resp, err := testClient.Post(baseURL+"/team/add", "application/json", bytes.NewBuffer(body))
	require.NoError(t, err, "Failed to create team")
	require.Equal(t, http.StatusCreated, resp.StatusCode, "team should be created")
	_ = resp.Body.Close()

	prInput := models.CreatePRRequest{
		PullRequestID:   "pr-stats",
		PullRequestName: "Test Stats",
		AuthorID:        "author",
	}

	body, err = json.Marshal(prInput)
	require.NoError(t, err, "Failed to marshal PR request")

	resp, err = testClient.Post(baseURL+"/pullRequest/create", "application/json", bytes.NewBuffer(body))
	require.NoError(t, err, "Failed to create PR")
	require.Equal(t, http.StatusCreated, resp.StatusCode, "PR should be created")
	_ = resp.Body.Close()

	t.Run("Get Stats", func(t *testing.T) {
		resp, err := testClient.Get(baseURL + "/stats")
		require.NoError(t, err, "Failed to get stats")
		defer func() { _ = resp.Body.Close() }()

		assert.Equal(t, http.StatusOK, resp.StatusCode, "should return 200 OK")
		assert.Equal(t, "application/json", resp.Header.Get("Content-Type"), "should set Content-Type header")

		var stats map[string]interface{}
		err = json.NewDecoder(resp.Body).Decode(&stats)
		require.NoError(t, err, "Failed to decode response")

		assert.Contains(t, stats, "total_prs", "stats should contain total_prs")
		assert.Contains(t, stats, "total_users", "stats should contain total_users")
		assert.Contains(t, stats, "total_teams", "stats should contain total_teams")
		assert.Contains(t, stats, "active_users", "stats should contain active_users")

		totalPRs, ok := stats["total_prs"].(float64)
		assert.True(t, ok, "total_prs should be a number")
		assert.GreaterOrEqual(t, totalPRs, float64(1), "should have at least 1 PR")

		totalUsers, ok := stats["total_users"].(float64)
		assert.True(t, ok, "total_users should be a number")
		assert.GreaterOrEqual(t, totalUsers, float64(2), "should have at least 2 users")
	})
}

func TestE2E_Health(t *testing.T) {
	t.Run("Health Check", func(t *testing.T) {
		resp, err := testClient.Get(baseURL + "/health")
		require.NoError(t, err, "Failed to get health")
		defer func() { _ = resp.Body.Close() }()

		assert.Equal(t, http.StatusOK, resp.StatusCode, "should return 200 OK")
		assert.Equal(t, "application/json", resp.Header.Get("Content-Type"), "should set Content-Type header")

		var healthResp map[string]string
		err = json.NewDecoder(resp.Body).Decode(&healthResp)
		require.NoError(t, err, "Failed to decode response")
		assert.Equal(t, "ok", healthResp["status"], "status should be 'ok'")
	})
}

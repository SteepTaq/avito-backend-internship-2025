package handler

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"

	"github.com/SteepTaq/avito-backend-internship-2025/internal/config"
	appErrors "github.com/SteepTaq/avito-backend-internship-2025/internal/errors"
	"github.com/SteepTaq/avito-backend-internship-2025/internal/models"
	"github.com/SteepTaq/avito-backend-internship-2025/mocks"
	"github.com/SteepTaq/avito-backend-internship-2025/pkg/logger"
)

func setupHandler(t *testing.T) (*Handler, *mocks.MockService) {
	mockService := mocks.NewMockService(t)
	cfg := &config.Config{
		ServerPort:   "8080",
		DatabaseURL:  "postgres://test",
		MaxReviewers: 2,
		LogLevel:     "debug",
	}
	log := logger.New("debug")
	handler := NewHandler(mockService, cfg, log)
	return handler, mockService
}

func setupRequest(method, url string, body interface{}) *http.Request {
	var reqBody []byte
	if body != nil {
		reqBody, _ = json.Marshal(body)
	}

	req := httptest.NewRequest(method, url, bytes.NewBuffer(reqBody))
	req.Header.Set("Content-Type", "application/json")

	ctx := chi.NewRouteContext()
	ctx.URLParams.Add("request_id", "test-request-id")
	req = req.WithContext(context.WithValue(req.Context(), chi.RouteCtxKey, ctx))
	req = req.WithContext(context.WithValue(req.Context(), middleware.RequestIDKey, "test-request-id"))

	return req
}

func TestHandler_Health(t *testing.T) {
	t.Run("success", func(t *testing.T) {
		handler, mockService := setupHandler(t)
		mockService.EXPECT().Ping(mock.Anything).Return(nil)

		req := setupRequest("GET", "/health", nil)
		w := httptest.NewRecorder()

		handler.Health(w, req)

		assert.Equal(t, http.StatusOK, w.Code, "should return 200 OK")
		assert.Equal(t, "application/json", w.Header().Get("Content-Type"), "should set Content-Type header")

		var response map[string]string
		err := json.Unmarshal(w.Body.Bytes(), &response)
		assert.NoError(t, err, "response should be valid JSON")
		assert.Equal(t, "ok", response["status"], "should return status 'ok'")
		assert.Len(t, response, 1, "response should contain only status field")
	})

	t.Run("service unavailable", func(t *testing.T) {
		handler, mockService := setupHandler(t)
		mockService.EXPECT().Ping(mock.Anything).Return(errors.New("database error"))

		req := setupRequest("GET", "/health", nil)
		w := httptest.NewRecorder()

		handler.Health(w, req)

		assert.Equal(t, http.StatusServiceUnavailable, w.Code, "should return 503 Service Unavailable")
		assert.Equal(t, "application/json", w.Header().Get("Content-Type"), "should set Content-Type header")

		var response map[string]string
		err := json.Unmarshal(w.Body.Bytes(), &response)
		assert.NoError(t, err, "response should be valid JSON")
		assert.Equal(t, "unhealthy", response["status"], "should return status 'unhealthy'")
		assert.Contains(t, response, "error", "should contain error field")
	})
}

func TestHandler_GetStats(t *testing.T) {
	t.Run("success", func(t *testing.T) {
		handler, mockService := setupHandler(t)
		expectedStats := map[string]interface{}{
			"total_prs":   10,
			"total_users": 5,
			"total_teams": 2,
		}

		mockService.EXPECT().GetStats(mock.Anything).Return(expectedStats, nil)

		req := setupRequest("GET", "/stats", nil)
		w := httptest.NewRecorder()

		handler.GetStats(w, req)

		assert.Equal(t, http.StatusOK, w.Code, "should return 200 OK")
		assert.Equal(t, "application/json", w.Header().Get("Content-Type"), "should set Content-Type header")

		var response map[string]interface{}
		err := json.Unmarshal(w.Body.Bytes(), &response)
		assert.NoError(t, err, "response should be valid JSON")
		assert.Equal(t, float64(10), response["total_prs"])
		assert.Equal(t, float64(5), response["total_users"])
		assert.Equal(t, float64(2), response["total_teams"])
	})

	t.Run("service error", func(t *testing.T) {
		handler, mockService := setupHandler(t)
		mockService.EXPECT().GetStats(mock.Anything).Return(nil, errors.New("database error"))

		req := setupRequest("GET", "/stats", nil)
		w := httptest.NewRecorder()

		handler.GetStats(w, req)

		assert.Equal(t, http.StatusInternalServerError, w.Code, "should return 500 Internal Server Error")
		assert.Equal(t, "application/json", w.Header().Get("Content-Type"), "should set Content-Type header")

		var errorResp models.ErrorResponse
		err := json.Unmarshal(w.Body.Bytes(), &errorResp)
		assert.NoError(t, err, "error response should be valid JSON")
		assert.Equal(t, models.ErrorCodeInternalServerError, errorResp.Error.Code)
		assert.Equal(t, models.ErrMsgInternalServerError, errorResp.Error.Message)
	})
}

func TestHandler_CreateTeam(t *testing.T) {
	t.Run("success", func(t *testing.T) {
		handler, mockService := setupHandler(t)
		team := &models.Team{
			TeamName: "backend",
			Members: []models.TeamMember{
				{UserID: "u1", Username: "Alice", IsActive: true},
				{UserID: "u2", Username: "Bob", IsActive: true},
			},
		}

		createdTeam := &models.Team{
			TeamName: "backend",
			Members: []models.TeamMember{
				{UserID: "u1", Username: "Alice", IsActive: true},
				{UserID: "u2", Username: "Bob", IsActive: true},
			},
		}

		mockService.EXPECT().CreateTeam(mock.Anything, team).Return(createdTeam, nil)

		req := setupRequest("POST", "/team/add", team)
		w := httptest.NewRecorder()

		handler.CreateTeam(w, req)

		assert.Equal(t, http.StatusCreated, w.Code, "should return 201 Created")
		assert.Equal(t, "application/json", w.Header().Get("Content-Type"), "should set Content-Type header")

		var response map[string]*models.Team
		err := json.Unmarshal(w.Body.Bytes(), &response)
		assert.NoError(t, err, "response should be valid JSON")
		assert.NotNil(t, response["team"], "response should contain 'team' field")
		assert.Equal(t, "backend", response["team"].TeamName, "should parse team_name correctly from request body")
		assert.Len(t, response["team"].Members, 2, "should parse team members correctly")
	})

	t.Run("invalid JSON", func(t *testing.T) {
		handler, _ := setupHandler(t)
		req := httptest.NewRequest("POST", "/team/add", bytes.NewBufferString("invalid json"))
		req.Header.Set("Content-Type", "application/json")
		req = req.WithContext(context.WithValue(req.Context(), middleware.RequestIDKey, "test-request-id"))
		w := httptest.NewRecorder()

		handler.CreateTeam(w, req)

		assert.Equal(t, http.StatusBadRequest, w.Code, "should return 400 Bad Request for invalid JSON")
		assert.Equal(t, "application/json", w.Header().Get("Content-Type"), "should set Content-Type header")

		var errorResp models.ErrorResponse
		err := json.Unmarshal(w.Body.Bytes(), &errorResp)
		assert.NoError(t, err, "error response should be valid JSON")
		assert.Equal(t, models.ErrorCodeValidationError, errorResp.Error.Code, "should return validation error code")
		assert.Equal(t, models.ErrMsgInvalidRequestBody, errorResp.Error.Message)
	})

	t.Run("validation error - empty team name", func(t *testing.T) {
		handler, _ := setupHandler(t)
		team := &models.Team{
			TeamName: "",
			Members: []models.TeamMember{
				{UserID: "u1", Username: "Alice", IsActive: true},
			},
		}

		req := setupRequest("POST", "/team/add", team)
		w := httptest.NewRecorder()

		handler.CreateTeam(w, req)

		assert.Equal(t, http.StatusBadRequest, w.Code, "should return 400 Bad Request for validation error")
		assert.Equal(t, "application/json", w.Header().Get("Content-Type"), "should set Content-Type header")

		var errorResp models.ErrorResponse
		err := json.Unmarshal(w.Body.Bytes(), &errorResp)
		assert.NoError(t, err, "error response should be valid JSON")
		assert.Equal(t, models.ErrorCodeValidationError, errorResp.Error.Code, "should return validation error code")
		assert.NotEmpty(t, errorResp.Error.Message, "should return validation error message")
	})

	t.Run("team exists", func(t *testing.T) {
		handler, mockService := setupHandler(t)
		team := &models.Team{
			TeamName: "backend",
			Members: []models.TeamMember{
				{UserID: "u1", Username: "Alice", IsActive: true},
			},
		}

		mockService.EXPECT().CreateTeam(mock.Anything, team).Return(nil, appErrors.ErrTeamExists)

		req := setupRequest("POST", "/team/add", team)
		w := httptest.NewRecorder()

		handler.CreateTeam(w, req)

		assert.Equal(t, http.StatusBadRequest, w.Code, "should return 400 Bad Request for existing team")
		assert.Equal(t, "application/json", w.Header().Get("Content-Type"), "should set Content-Type header")

		var errorResp models.ErrorResponse
		err := json.Unmarshal(w.Body.Bytes(), &errorResp)
		assert.NoError(t, err, "error response should be valid JSON")
		assert.Equal(t, models.ErrorCodeTeamExists, errorResp.Error.Code)
		assert.Equal(t, models.ErrMsgTeamNameAlreadyExists, errorResp.Error.Message)
	})

	t.Run("get team error after creation", func(t *testing.T) {
		handler, mockService := setupHandler(t)
		team := &models.Team{
			TeamName: "backend",
			Members: []models.TeamMember{
				{UserID: "u1", Username: "Alice", IsActive: true},
			},
		}

		mockService.EXPECT().CreateTeam(mock.Anything, team).Return(nil, errors.New("team created but failed to retrieve: database error"))

		req := setupRequest("POST", "/team/add", team)
		w := httptest.NewRecorder()

		handler.CreateTeam(w, req)

		assert.Equal(t, http.StatusInternalServerError, w.Code, "should return 500 Internal Server Error for service errors")
		assert.Equal(t, "application/json", w.Header().Get("Content-Type"), "should set Content-Type header")

		var errorResp models.ErrorResponse
		err := json.Unmarshal(w.Body.Bytes(), &errorResp)
		assert.NoError(t, err, "error response should be valid JSON")
		assert.Equal(t, models.ErrorCodeInternalServerError, errorResp.Error.Code)
		assert.Equal(t, models.ErrMsgInternalServerError, errorResp.Error.Message)
	})
}

func TestHandler_GetTeam(t *testing.T) {
	t.Run("success", func(t *testing.T) {
		handler, mockService := setupHandler(t)
		expectedTeam := &models.Team{
			TeamName: "backend",
			Members: []models.TeamMember{
				{UserID: "u1", Username: "Alice", IsActive: true},
			},
		}

		mockService.EXPECT().GetTeam(mock.Anything, "backend").Return(expectedTeam, nil)

		req := httptest.NewRequest("GET", "/team/get?team_name=backend", http.NoBody)
		req = req.WithContext(context.WithValue(req.Context(), middleware.RequestIDKey, "test-request-id"))
		w := httptest.NewRecorder()

		handler.GetTeam(w, req)

		assert.Equal(t, http.StatusOK, w.Code, "should return 200 OK")
		assert.Equal(t, "application/json", w.Header().Get("Content-Type"), "should set Content-Type header")

		var team models.Team
		err := json.Unmarshal(w.Body.Bytes(), &team)
		assert.NoError(t, err, "response should be valid JSON")
		assert.Equal(t, "backend", team.TeamName, "should parse team_name from query params correctly")
		assert.Len(t, team.Members, 1, "should return team with members")
	})

	t.Run("missing team_name", func(t *testing.T) {
		handler, _ := setupHandler(t)
		req := httptest.NewRequest("GET", "/team/get", http.NoBody)
		req = req.WithContext(context.WithValue(req.Context(), middleware.RequestIDKey, "test-request-id"))
		w := httptest.NewRecorder()

		handler.GetTeam(w, req)

		assert.Equal(t, http.StatusBadRequest, w.Code, "should return 400 Bad Request for missing required query param")
		assert.Equal(t, "application/json", w.Header().Get("Content-Type"), "should set Content-Type header")

		var errorResp models.ErrorResponse
		err := json.Unmarshal(w.Body.Bytes(), &errorResp)
		assert.NoError(t, err, "error response should be valid JSON")
		assert.Equal(t, models.ErrorCodeValidationError, errorResp.Error.Code, "should return error code for missing param")
		assert.Equal(t, models.ErrMsgTeamNameRequired, errorResp.Error.Message, "should return team name required message")
	})

	t.Run("team not found", func(t *testing.T) {
		handler, mockService := setupHandler(t)
		mockService.EXPECT().GetTeam(mock.Anything, "nonexistent").Return(nil, appErrors.ErrTeamNotFound)

		req := httptest.NewRequest("GET", "/team/get?team_name=nonexistent", http.NoBody)
		req = req.WithContext(context.WithValue(req.Context(), middleware.RequestIDKey, "test-request-id"))
		w := httptest.NewRecorder()

		handler.GetTeam(w, req)

		assert.Equal(t, http.StatusNotFound, w.Code, "should return 404 Not Found")
		assert.Equal(t, "application/json", w.Header().Get("Content-Type"), "should set Content-Type header")

		var errorResp models.ErrorResponse
		err := json.Unmarshal(w.Body.Bytes(), &errorResp)
		assert.NoError(t, err, "error response should be valid JSON")
		assert.Equal(t, models.ErrorCodeNotFound, errorResp.Error.Code)
		assert.Equal(t, models.ErrMsgResourceNotFound, errorResp.Error.Message)
	})
}

func TestHandler_SetUserActive(t *testing.T) {
	t.Run("success", func(t *testing.T) {
		handler, mockService := setupHandler(t)
		reqBody := map[string]interface{}{
			"user_id":   "u1",
			"is_active": true,
		}

		user := &models.User{
			UserID:   "u1",
			Username: "Alice",
			TeamName: "backend",
			IsActive: true,
		}

		mockService.EXPECT().SetUserActive(mock.Anything, "u1", true).Return(user, nil)

		req := setupRequest("POST", "/users/setIsActive", reqBody)
		w := httptest.NewRecorder()

		handler.SetUserActive(w, req)

		assert.Equal(t, http.StatusOK, w.Code, "should return 200 OK")
		assert.Equal(t, "application/json", w.Header().Get("Content-Type"), "should set Content-Type header")

		var response map[string]*models.User
		err := json.Unmarshal(w.Body.Bytes(), &response)
		assert.NoError(t, err, "response should be valid JSON")
		assert.NotNil(t, response["user"], "response should contain 'user' field")
		assert.Equal(t, "u1", response["user"].UserID, "should parse user_id correctly from request body")
		assert.True(t, response["user"].IsActive, "should parse is_active correctly from request body")
	})

	t.Run("invalid JSON", func(t *testing.T) {
		handler, _ := setupHandler(t)

		req := httptest.NewRequest("POST", "/users/setIsActive", bytes.NewBufferString("invalid json"))
		req.Header.Set("Content-Type", "application/json")
		req = req.WithContext(context.WithValue(req.Context(), middleware.RequestIDKey, "test-request-id"))
		w := httptest.NewRecorder()

		handler.SetUserActive(w, req)

		assert.Equal(t, http.StatusBadRequest, w.Code, "should return 400 Bad Request for invalid JSON")
		assert.Equal(t, "application/json", w.Header().Get("Content-Type"), "should set Content-Type header")

		var errorResp models.ErrorResponse
		err := json.Unmarshal(w.Body.Bytes(), &errorResp)
		assert.NoError(t, err, "error response should be valid JSON")
		assert.Equal(t, models.ErrorCodeValidationError, errorResp.Error.Code)
		assert.Equal(t, models.ErrMsgInvalidRequestBody, errorResp.Error.Message)
	})

	t.Run("user not found", func(t *testing.T) {
		handler, mockService := setupHandler(t)
		reqBody := map[string]interface{}{
			"user_id":   "nonexistent",
			"is_active": true,
		}

		mockService.EXPECT().SetUserActive(mock.Anything, "nonexistent", true).Return(nil, appErrors.ErrUserNotFound)

		req := setupRequest("POST", "/users/setIsActive", reqBody)
		w := httptest.NewRecorder()

		handler.SetUserActive(w, req)

		assert.Equal(t, http.StatusNotFound, w.Code, "should return 404 Not Found")
		assert.Equal(t, "application/json", w.Header().Get("Content-Type"), "should set Content-Type header")

		var errorResp models.ErrorResponse
		err := json.Unmarshal(w.Body.Bytes(), &errorResp)
		assert.NoError(t, err, "error response should be valid JSON")
		assert.Equal(t, models.ErrorCodeNotFound, errorResp.Error.Code)
		assert.Equal(t, models.ErrMsgResourceNotFound, errorResp.Error.Message)
	})
}

func TestHandler_GetPRsByReviewer(t *testing.T) {
	t.Run("success", func(t *testing.T) {
		handler, mockService := setupHandler(t)
		prs := []*models.PullRequestShort{
			{
				PullRequestID:   "pr-1",
				PullRequestName: "PR 1",
				AuthorID:        "u1",
				Status:          models.PRStatusOpen,
			},
			{
				PullRequestID:   "pr-2",
				PullRequestName: "PR 2",
				AuthorID:        "u2",
				Status:          models.PRStatusMerged,
			},
		}

		mockService.EXPECT().GetPRsByReviewer(mock.Anything, "u3").Return(prs, nil)

		req := httptest.NewRequest("GET", "/users/getReview?user_id=u3", http.NoBody)
		req = req.WithContext(context.WithValue(req.Context(), middleware.RequestIDKey, "test-request-id"))
		w := httptest.NewRecorder()

		handler.GetPRsByReviewer(w, req)

		assert.Equal(t, http.StatusOK, w.Code, "should return 200 OK")
		assert.Equal(t, "application/json", w.Header().Get("Content-Type"), "should set Content-Type header")

		var response map[string]interface{}
		err := json.Unmarshal(w.Body.Bytes(), &response)
		assert.NoError(t, err, "response should be valid JSON")
		assert.Equal(t, "u3", response["user_id"], "should parse user_id from query params correctly")
		assert.Contains(t, response, "pull_requests", "response should contain 'pull_requests' field")
		assert.IsType(t, []interface{}{}, response["pull_requests"], "pull_requests should be an array")
	})

	t.Run("missing user_id", func(t *testing.T) {
		handler, _ := setupHandler(t)

		req := httptest.NewRequest("GET", "/users/getReview", http.NoBody)
		req = req.WithContext(context.WithValue(req.Context(), middleware.RequestIDKey, "test-request-id"))
		w := httptest.NewRecorder()

		handler.GetPRsByReviewer(w, req)

		assert.Equal(t, http.StatusBadRequest, w.Code, "should return 400 Bad Request for missing required query param")
		assert.Equal(t, "application/json", w.Header().Get("Content-Type"), "should set Content-Type header")

		var errorResp models.ErrorResponse
		err := json.Unmarshal(w.Body.Bytes(), &errorResp)
		assert.NoError(t, err, "error response should be valid JSON")
		assert.Equal(t, models.ErrorCodeValidationError, errorResp.Error.Code, "should return error code for missing param")
		assert.Equal(t, models.ErrMsgUserIDRequired, errorResp.Error.Message, "should return user ID required message")
	})

	t.Run("user not found", func(t *testing.T) {
		handler, mockService := setupHandler(t)

		mockService.EXPECT().GetPRsByReviewer(mock.Anything, "nonexistent").Return(nil, appErrors.ErrUserNotFound)

		req := httptest.NewRequest("GET", "/users/getReview?user_id=nonexistent", http.NoBody)
		req = req.WithContext(context.WithValue(req.Context(), middleware.RequestIDKey, "test-request-id"))
		w := httptest.NewRecorder()

		handler.GetPRsByReviewer(w, req)

		assert.Equal(t, http.StatusNotFound, w.Code)
	})
}

func TestHandler_CreatePR(t *testing.T) {
	t.Run("success", func(t *testing.T) {
		handler, mockService := setupHandler(t)
		reqBody := map[string]string{
			"pull_request_id":   "pr-1",
			"pull_request_name": "Test PR",
			"author_id":         "u1",
		}

		now := time.Now()
		pr := &models.PullRequest{
			PullRequestID:     "pr-1",
			PullRequestName:   "Test PR",
			AuthorID:          "u1",
			Status:            models.PRStatusOpen,
			AssignedReviewers: []string{"u2", "u3"},
			CreatedAt:         &now,
		}

		mockService.EXPECT().CreatePR(mock.Anything, "pr-1", "Test PR", "u1").Return(pr, nil)

		req := setupRequest("POST", "/pullRequest/create", reqBody)
		w := httptest.NewRecorder()

		handler.CreatePR(w, req)

		assert.Equal(t, http.StatusCreated, w.Code, "should return 201 Created")
		assert.Equal(t, "application/json", w.Header().Get("Content-Type"), "should set Content-Type header")

		var response map[string]*models.PullRequest
		err := json.Unmarshal(w.Body.Bytes(), &response)
		assert.NoError(t, err, "response should be valid JSON")
		assert.NotNil(t, response["pr"], "response should contain 'pr' field")
		assert.Equal(t, "pr-1", response["pr"].PullRequestID, "should parse pull_request_id correctly from request body")
		assert.Equal(t, "Test PR", response["pr"].PullRequestName, "should parse pull_request_name correctly")
		assert.Equal(t, "u1", response["pr"].AuthorID, "should parse author_id correctly")
	})

	t.Run("invalid JSON", func(t *testing.T) {
		handler, _ := setupHandler(t)

		req := httptest.NewRequest("POST", "/pullRequest/create", bytes.NewBufferString("invalid json"))
		req.Header.Set("Content-Type", "application/json")
		req = req.WithContext(context.WithValue(req.Context(), middleware.RequestIDKey, "test-request-id"))
		w := httptest.NewRecorder()

		handler.CreatePR(w, req)

		assert.Equal(t, http.StatusBadRequest, w.Code)
	})

	t.Run("validation error - empty PR ID", func(t *testing.T) {
		handler, _ := setupHandler(t)
		reqBody := map[string]string{
			"pull_request_id":   "",
			"pull_request_name": "Test PR",
			"author_id":         "u1",
		}

		req := setupRequest("POST", "/pullRequest/create", reqBody)
		w := httptest.NewRecorder()

		handler.CreatePR(w, req)

		assert.Equal(t, http.StatusBadRequest, w.Code, "should return 400 Bad Request for validation error")
		assert.Equal(t, "application/json", w.Header().Get("Content-Type"), "should set Content-Type header")

		var errorResp models.ErrorResponse
		err := json.Unmarshal(w.Body.Bytes(), &errorResp)
		assert.NoError(t, err, "error response should be valid JSON")
		assert.Equal(t, models.ErrorCodeValidationError, errorResp.Error.Code, "should return validation error code")
		assert.NotEmpty(t, errorResp.Error.Message, "should return validation error message")
	})

	t.Run("PR exists", func(t *testing.T) {
		handler, mockService := setupHandler(t)
		reqBody := map[string]string{
			"pull_request_id":   "pr-1",
			"pull_request_name": "Test PR",
			"author_id":         "u1",
		}

		mockService.EXPECT().CreatePR(mock.Anything, "pr-1", "Test PR", "u1").Return(nil, appErrors.ErrPRExists)

		req := setupRequest("POST", "/pullRequest/create", reqBody)
		w := httptest.NewRecorder()

		handler.CreatePR(w, req)

		assert.Equal(t, http.StatusConflict, w.Code, "should return 409 Conflict for existing PR")
		assert.Equal(t, "application/json", w.Header().Get("Content-Type"), "should set Content-Type header")

		var errorResp models.ErrorResponse
		err := json.Unmarshal(w.Body.Bytes(), &errorResp)
		assert.NoError(t, err, "error response should be valid JSON")
		assert.Equal(t, models.ErrorCodePRExists, errorResp.Error.Code)
		assert.Equal(t, models.ErrMsgPRIDAlreadyExists, errorResp.Error.Message)
	})
}

func TestHandler_MergePR(t *testing.T) {
	t.Run("success", func(t *testing.T) {
		handler, mockService := setupHandler(t)
		reqBody := map[string]string{
			"pull_request_id": "pr-1",
		}

		now := time.Now()
		mergedAt := now.Add(time.Hour)
		pr := &models.PullRequest{
			PullRequestID: "pr-1",
			Status:        models.PRStatusMerged,
			AuthorID:      "u1",
			CreatedAt:     &now,
			MergedAt:      &mergedAt,
		}

		mockService.EXPECT().MergePR(mock.Anything, "pr-1").Return(pr, nil)

		req := setupRequest("POST", "/pullRequest/merge", reqBody)
		w := httptest.NewRecorder()

		handler.MergePR(w, req)

		assert.Equal(t, http.StatusOK, w.Code, "should return 200 OK")
		assert.Equal(t, "application/json", w.Header().Get("Content-Type"), "should set Content-Type header")

		var response map[string]*models.PullRequest
		err := json.Unmarshal(w.Body.Bytes(), &response)
		assert.NoError(t, err, "response should be valid JSON")
		assert.NotNil(t, response["pr"], "response should contain 'pr' field")
		assert.Equal(t, models.PRStatusMerged, response["pr"].Status, "should return merged status")
		assert.NotNil(t, response["pr"].MergedAt, "should set merged_at timestamp")
	})

	t.Run("invalid JSON", func(t *testing.T) {
		handler, _ := setupHandler(t)

		req := httptest.NewRequest("POST", "/pullRequest/merge", bytes.NewBufferString("invalid json"))
		req.Header.Set("Content-Type", "application/json")
		req = req.WithContext(context.WithValue(req.Context(), middleware.RequestIDKey, "test-request-id"))
		w := httptest.NewRecorder()

		handler.MergePR(w, req)

		assert.Equal(t, http.StatusBadRequest, w.Code)
	})

	t.Run("PR not found", func(t *testing.T) {
		handler, mockService := setupHandler(t)
		reqBody := map[string]string{
			"pull_request_id": "nonexistent",
		}

		mockService.EXPECT().MergePR(mock.Anything, "nonexistent").Return(nil, appErrors.ErrPRNotFound)

		req := setupRequest("POST", "/pullRequest/merge", reqBody)
		w := httptest.NewRecorder()

		handler.MergePR(w, req)

		assert.Equal(t, http.StatusNotFound, w.Code)
	})
}

func TestHandler_ReassignReviewer(t *testing.T) {
	t.Run("success", func(t *testing.T) {
		handler, mockService := setupHandler(t)
		reqBody := map[string]string{
			"pull_request_id": "pr-1",
			"old_user_id":     "u2",
		}

		now := time.Now()
		pr := &models.PullRequest{
			PullRequestID:     "pr-1",
			Status:            models.PRStatusOpen,
			AssignedReviewers: []string{"u3"},
			CreatedAt:         &now,
		}

		mockService.EXPECT().ReassignReviewer(mock.Anything, "pr-1", "u2").Return(pr, "u3", nil)

		req := setupRequest("POST", "/pullRequest/reassign", reqBody)
		w := httptest.NewRecorder()

		handler.ReassignReviewer(w, req)

		assert.Equal(t, http.StatusOK, w.Code, "should return 200 OK")
		assert.Equal(t, "application/json", w.Header().Get("Content-Type"), "should set Content-Type header")

		var response map[string]interface{}
		err := json.Unmarshal(w.Body.Bytes(), &response)
		assert.NoError(t, err, "response should be valid JSON")
		assert.Equal(t, "u3", response["replaced_by"], "should return new reviewer ID")
		assert.Contains(t, response, "pr", "response should contain 'pr' field")
	})

	t.Run("invalid JSON", func(t *testing.T) {
		handler, _ := setupHandler(t)

		req := httptest.NewRequest("POST", "/pullRequest/reassign", bytes.NewBufferString("invalid json"))
		req.Header.Set("Content-Type", "application/json")
		req = req.WithContext(context.WithValue(req.Context(), middleware.RequestIDKey, "test-request-id"))
		w := httptest.NewRecorder()

		handler.ReassignReviewer(w, req)

		assert.Equal(t, http.StatusBadRequest, w.Code)
	})

	t.Run("PR merged", func(t *testing.T) {
		handler, mockService := setupHandler(t)
		reqBody := map[string]string{
			"pull_request_id": "pr-1",
			"old_user_id":     "u2",
		}

		mockService.EXPECT().ReassignReviewer(mock.Anything, "pr-1", "u2").Return(nil, "", appErrors.ErrPRMerged)

		req := setupRequest("POST", "/pullRequest/reassign", reqBody)
		w := httptest.NewRecorder()

		handler.ReassignReviewer(w, req)

		assert.Equal(t, http.StatusConflict, w.Code, "should return 409 Conflict for merged PR")
		assert.Equal(t, "application/json", w.Header().Get("Content-Type"), "should set Content-Type header")

		var errorResp models.ErrorResponse
		err := json.Unmarshal(w.Body.Bytes(), &errorResp)
		assert.NoError(t, err, "error response should be valid JSON")
		assert.Equal(t, models.ErrorCodePRMerged, errorResp.Error.Code)
		assert.Equal(t, models.ErrMsgCannotReassignMerged, errorResp.Error.Message)
	})

	t.Run("reviewer not assigned", func(t *testing.T) {
		handler, mockService := setupHandler(t)
		reqBody := map[string]string{
			"pull_request_id": "pr-1",
			"old_user_id":     "u2",
		}

		mockService.EXPECT().ReassignReviewer(mock.Anything, "pr-1", "u2").Return(nil, "", appErrors.ErrReviewerNotAssigned)

		req := setupRequest("POST", "/pullRequest/reassign", reqBody)
		w := httptest.NewRecorder()

		handler.ReassignReviewer(w, req)

		assert.Equal(t, http.StatusConflict, w.Code)
	})

	t.Run("no candidate", func(t *testing.T) {
		handler, mockService := setupHandler(t)
		reqBody := map[string]string{
			"pull_request_id": "pr-1",
			"old_user_id":     "u2",
		}

		mockService.EXPECT().ReassignReviewer(mock.Anything, "pr-1", "u2").Return(nil, "", appErrors.ErrNoCandidate)

		req := setupRequest("POST", "/pullRequest/reassign", reqBody)
		w := httptest.NewRecorder()

		handler.ReassignReviewer(w, req)

		assert.Equal(t, http.StatusConflict, w.Code, "should return 409 Conflict for no candidate")
		assert.Equal(t, "application/json", w.Header().Get("Content-Type"), "should set Content-Type header")

		var errorResp models.ErrorResponse
		err := json.Unmarshal(w.Body.Bytes(), &errorResp)
		assert.NoError(t, err, "error response should be valid JSON")
		assert.Equal(t, models.ErrorCodeNoCandidate, errorResp.Error.Code)
		assert.Equal(t, models.ErrMsgNoCandidate, errorResp.Error.Message)
	})
}

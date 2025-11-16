package handler

import (
	"context"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"

	"github.com/SteepTaq/avito-backend-internship-2025/internal/config"
	"github.com/SteepTaq/avito-backend-internship-2025/internal/models"
	"github.com/SteepTaq/avito-backend-internship-2025/pkg/logger"
)

type TeamService interface {
	CreateTeam(ctx context.Context, team *models.Team) (*models.Team, error)
	GetTeam(ctx context.Context, teamName string) (*models.Team, error)
}

type UserService interface {
	SetUserActive(ctx context.Context, userID string, isActive bool) (*models.User, error)
	GetPRsByReviewer(ctx context.Context, reviewerID string) ([]*models.PullRequestShort, error)
	DeactivateTeamUsers(ctx context.Context, teamName string, userIDs []string) (map[string]interface{}, error)
}

type PullRequestService interface {
	CreatePR(ctx context.Context, prID, prName, authorID string) (*models.PullRequest, error)
	MergePR(ctx context.Context, prID string) (*models.PullRequest, error)
	ReassignReviewer(ctx context.Context, prID, oldReviewerID string) (*models.PullRequest, string, error)
}

type StatsService interface {
	GetStats(ctx context.Context) (map[string]interface{}, error)
}

type HealthService interface {
	Ping(ctx context.Context) error
}

type Service interface {
	TeamService
	UserService
	PullRequestService
	StatsService
	HealthService
}

type Handler struct {
	service Service
	config  *config.Config
	logger  *logger.Logger
}

func NewHandler(svc Service, cfg *config.Config, log *logger.Logger) *Handler {
	return &Handler{
		service: svc,
		config:  cfg,
		logger:  log,
	}
}

func (h *Handler) SetupRoutes() *chi.Mux {
	router := chi.NewRouter()

	// middleware
	router.Use(middleware.RequestID)
	router.Use(middleware.RealIP)
	router.Use(logger.RequestLogger(h.logger))
	router.Use(logger.Recoverer(h.logger))

	// health check
	router.Get("/health", h.Health)

	// statistics
	router.Get("/stats", h.GetStats)

	// teams
	router.Post("/team/add", h.CreateTeam)
	router.Get("/team/get", h.GetTeam)

	// users
	router.Post("/users/setIsActive", h.SetUserActive)
	router.Post("/users/deactivateTeam", h.DeactivateTeamUsers)
	router.Get("/users/getReview", h.GetPRsByReviewer)

	// pull requests
	router.Post("/pullRequest/create", h.CreatePR)
	router.Post("/pullRequest/merge", h.MergePR)
	router.Post("/pullRequest/reassign", h.ReassignReviewer)

	return router
}

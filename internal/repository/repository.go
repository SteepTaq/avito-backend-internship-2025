package repository

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	appErrors "github.com/SteepTaq/avito-backend-internship-2025/internal/errors"
	"github.com/SteepTaq/avito-backend-internship-2025/internal/models"
	"github.com/SteepTaq/avito-backend-internship-2025/pkg/logger"
)

type Repository struct {
	db     *pgxpool.Pool
	logger *logger.Logger
}

func NewRepository(db *pgxpool.Pool, log *logger.Logger) *Repository {
	return &Repository{
		db:     db,
		logger: log,
	}
}

func (r *Repository) Close() {
	r.db.Close()
}

func (r *Repository) Ping(ctx context.Context) error {
	return r.db.Ping(ctx)
}

func InitDB(databaseURL string) (*pgxpool.Pool, error) {
	config, err := pgxpool.ParseConfig(databaseURL)
	if err != nil {
		return nil, fmt.Errorf("failed to parse database URL: %w", err)
	}

	config.MaxConns = 25
	config.MinConns = 5
	config.MaxConnLifetime = 5 * time.Minute
	config.MaxConnIdleTime = 1 * time.Minute

	pool, err := pgxpool.NewWithConfig(context.Background(), config)
	if err != nil {
		return nil, fmt.Errorf("failed to create connection pool: %w", err)
	}

	if err := pool.Ping(context.Background()); err != nil {
		pool.Close()
		return nil, fmt.Errorf("failed to ping database: %w", err)
	}

	return pool, nil
}

func (r *Repository) GetUserByID(ctx context.Context, userID string) (*models.User, error) {
	query := `SELECT user_id, username, team_name, is_active FROM users WHERE user_id = $1`
	user := &models.User{}
	err := r.db.QueryRow(ctx, query, userID).Scan(
		&user.UserID,
		&user.Username,
		&user.TeamName,
		&user.IsActive,
	)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, appErrors.ErrUserNotFound
		}
		r.logger.Debug("Database query failed: get user", "error", err, "user_id", userID)
		return nil, fmt.Errorf("failed to get user: %w", err)
	}
	return user, nil
}

func (r *Repository) SetUserActive(ctx context.Context, userID string, isActive bool) error {
	query := `UPDATE users SET is_active = $1 WHERE user_id = $2`
	result, err := r.db.Exec(ctx, query, isActive, userID)
	if err != nil {
		r.logger.Debug("Database query failed: set user active", "error", err, "user_id", userID, "is_active", isActive)
		return fmt.Errorf("failed to set user active: %w", err)
	}
	if result.RowsAffected() == 0 {
		return appErrors.ErrUserNotFound
	}
	return nil
}

func (r *Repository) DeactivateUsersByTeam(ctx context.Context, teamName string, userIDs []string) ([]string, error) {
	var query string
	var args []interface{}
	if len(userIDs) == 0 {
		query = `UPDATE users SET is_active = false WHERE team_name = $1 RETURNING user_id`
		args = []interface{}{teamName}
	} else {
		query = `UPDATE users SET is_active = false WHERE team_name = $1 AND user_id = ANY($2::text[]) RETURNING user_id`
		args = []interface{}{teamName, userIDs}
	}

	rows, err := r.db.Query(ctx, query, args...)
	if err != nil {
		r.logger.Debug("Database query failed: deactivate users by team", "error", err, "team_name", teamName)
		return nil, fmt.Errorf("failed to deactivate users: %w", err)
	}
	defer rows.Close()

	deactivatedIDs := make([]string, 0)
	for rows.Next() {
		var userID string
		if err := rows.Scan(&userID); err != nil {
			r.logger.Debug("Database scan failed: deactivate users by team", "error", err)
			return nil, fmt.Errorf("failed to scan deactivated user ID: %w", err)
		}
		deactivatedIDs = append(deactivatedIDs, userID)
	}
	if err := rows.Err(); err != nil {
		r.logger.Debug("Database rows iteration failed: deactivate users by team", "error", err)
		return nil, fmt.Errorf("failed to iterate deactivated users: %w", err)
	}

	return deactivatedIDs, nil
}

func (r *Repository) GetOpenPRsByReviewers(ctx context.Context, reviewerIDs []string) ([]*models.PullRequest, error) {
	if len(reviewerIDs) == 0 {
		return []*models.PullRequest{}, nil
	}

	query := `
		SELECT DISTINCT pull_request_id, pull_request_name, author_id, status, assigned_reviewers, created_at, merged_at
		FROM pull_requests
		WHERE status = 'OPEN' AND assigned_reviewers && $1::text[]
		ORDER BY created_at DESC
	`
	rows, err := r.db.Query(ctx, query, reviewerIDs)
	if err != nil {
		r.logger.Debug("Database query failed: get open PRs by reviewers", "error", err, "reviewer_ids", reviewerIDs)
		return nil, fmt.Errorf("failed to get open PRs by reviewers: %w", err)
	}
	defer rows.Close()

	var prs []*models.PullRequest
	for rows.Next() {
		pr := &models.PullRequest{}
		var reviewers []string
		if err := rows.Scan(
			&pr.PullRequestID,
			&pr.PullRequestName,
			&pr.AuthorID,
			&pr.Status,
			&reviewers,
			&pr.CreatedAt,
			&pr.MergedAt,
		); err != nil {
			r.logger.Debug("Database scan failed: get open PRs by reviewers", "error", err)
			return nil, fmt.Errorf("failed to scan PR: %w", err)
		}
		if reviewers == nil {
			pr.AssignedReviewers = []string{}
		} else {
			pr.AssignedReviewers = reviewers
		}
		prs = append(prs, pr)
	}
	if err := rows.Err(); err != nil {
		r.logger.Debug("Database rows iteration failed: get open PRs by reviewers", "error", err)
		return nil, fmt.Errorf("failed to iterate PRs: %w", err)
	}

	return prs, nil
}

func (r *Repository) BulkUpdatePRReviewers(ctx context.Context, prReviewers map[string][]string) error {
	if len(prReviewers) == 0 {
		return nil
	}

	tx, err := r.db.Begin(ctx)
	if err != nil {
		r.logger.Debug("Database transaction begin failed: bulk update PR reviewers", "error", err)
		return fmt.Errorf("failed to begin transaction: %w", err)
	}
	defer tx.Rollback(ctx)

	query := `UPDATE pull_requests SET assigned_reviewers = $1 WHERE pull_request_id = $2`
	for prID, reviewers := range prReviewers {
		if _, err := tx.Exec(ctx, query, reviewers, prID); err != nil {
			r.logger.Debug("Database exec failed: bulk update PR reviewers", "error", err, "pull_request_id", prID)
			return fmt.Errorf("failed to update PR reviewers: %w", err)
		}
	}

	if err := tx.Commit(ctx); err != nil {
		r.logger.Debug("Database transaction commit failed: bulk update PR reviewers", "error", err)
		return fmt.Errorf("failed to commit transaction: %w", err)
	}

	return nil
}

func (r *Repository) GetActiveUsersByTeam(ctx context.Context, teamName, excludeUserID string) ([]*models.User, error) {
	query := `
		SELECT user_id, username, team_name, is_active 
		FROM users 
		WHERE team_name = $1 AND is_active = true AND user_id != $2
		ORDER BY user_id
	`
	rows, err := r.db.Query(ctx, query, teamName, excludeUserID)
	if err != nil {
		r.logger.Debug("Database query failed: get active users by team", "error", err, "team_name", teamName, "exclude_user_id", excludeUserID)
		return nil, fmt.Errorf("failed to get active users by team: %w", err)
	}
	defer rows.Close()

	var users []*models.User
	for rows.Next() {
		user := &models.User{}
		if err := rows.Scan(&user.UserID, &user.Username, &user.TeamName, &user.IsActive); err != nil {
			r.logger.Debug("Database scan failed: get active users by team", "error", err, "team_name", teamName)
			return nil, fmt.Errorf("failed to scan active user: %w", err)
		}
		users = append(users, user)
	}
	if err := rows.Err(); err != nil {
		r.logger.Debug("Database rows iteration failed: get active users by team", "error", err, "team_name", teamName)
		return nil, fmt.Errorf("failed to iterate active users: %w", err)
	}
	return users, nil
}

func (r *Repository) GetUsersByTeam(ctx context.Context, teamName string) ([]*models.User, error) {
	query := `
		SELECT user_id, username, team_name, is_active 
		FROM users 
		WHERE team_name = $1
		ORDER BY user_id
	`
	rows, err := r.db.Query(ctx, query, teamName)
	if err != nil {
		r.logger.Debug("Database query failed: get users by team", "error", err, "team_name", teamName)
		return nil, fmt.Errorf("failed to get users by team: %w", err)
	}
	defer rows.Close()

	var users []*models.User
	for rows.Next() {
		user := &models.User{}
		if err := rows.Scan(&user.UserID, &user.Username, &user.TeamName, &user.IsActive); err != nil {
			r.logger.Debug("Database scan failed: get users by team", "error", err, "team_name", teamName)
			return nil, fmt.Errorf("failed to scan user: %w", err)
		}
		users = append(users, user)
	}
	if err := rows.Err(); err != nil {
		r.logger.Debug("Database rows iteration failed: get users by team", "error", err, "team_name", teamName)
		return nil, fmt.Errorf("failed to iterate users: %w", err)
	}
	return users, nil
}

func (r *Repository) CreateTeam(ctx context.Context, team *models.Team) error {
	tx, err := r.db.Begin(ctx)
	if err != nil {
		r.logger.Debug("Database transaction begin failed: create team", "error", err, "team_name", team.TeamName)
		return fmt.Errorf("failed to begin transaction: %w", err)
	}
	defer tx.Rollback(ctx)

	for _, member := range team.Members {
		user := &models.User{
			UserID:   member.UserID,
			Username: member.Username,
			TeamName: team.TeamName,
			IsActive: member.IsActive,
		}
		query := `
			INSERT INTO users (user_id, username, team_name, is_active)
			VALUES ($1, $2, $3, $4)
			ON CONFLICT (user_id) 
			DO UPDATE SET 
				username = EXCLUDED.username,
				team_name = EXCLUDED.team_name,
				is_active = EXCLUDED.is_active
		`
		if _, err := tx.Exec(ctx, query, user.UserID, user.Username, user.TeamName, user.IsActive); err != nil {
			r.logger.Debug("Database query failed: create/update user in team", "error", err, "user_id", member.UserID, "team_name", team.TeamName)
			return fmt.Errorf("failed to create user %s: %w", member.UserID, err)
		}
	}

	if err := tx.Commit(ctx); err != nil {
		r.logger.Debug("Database transaction commit failed: create team", "error", err)
		return fmt.Errorf("failed to commit transaction: %w", err)
	}

	return nil
}

func (r *Repository) GetTeam(ctx context.Context, teamName string) (*models.Team, error) {
	users, err := r.GetUsersByTeam(ctx, teamName)
	if err != nil {
		return nil, fmt.Errorf("failed to get users by team: %w", err)
	}
	if len(users) == 0 {
		return nil, appErrors.ErrTeamNotFound
	}

	team := &models.Team{
		TeamName: teamName,
		Members:  make([]models.TeamMember, 0, len(users)),
	}

	for _, user := range users {
		team.Members = append(team.Members, models.TeamMember{
			UserID:   user.UserID,
			Username: user.Username,
			IsActive: user.IsActive,
		})
	}

	return team, nil
}

func (r *Repository) TeamExists(ctx context.Context, teamName string) (bool, error) {
	query := `SELECT EXISTS(SELECT 1 FROM users WHERE team_name = $1 LIMIT 1)`
	var exists bool
	err := r.db.QueryRow(ctx, query, teamName).Scan(&exists)
	if err != nil {
		r.logger.Debug("Database query failed: check team existence", "error", err, "team_name", teamName)
		return false, fmt.Errorf("failed to check team existence: %w", err)
	}
	return exists, nil
}

func (r *Repository) CreatePR(ctx context.Context, pr *models.PullRequest) error {
	query := `
		INSERT INTO pull_requests (pull_request_id, pull_request_name, author_id, status, assigned_reviewers, created_at)
		VALUES ($1, $2, $3, $4, $5, NOW())
		ON CONFLICT (pull_request_id) DO NOTHING
	`
	result, err := r.db.Exec(ctx, query, pr.PullRequestID, pr.PullRequestName, pr.AuthorID, pr.Status, pr.AssignedReviewers)
	if err != nil {
		r.logger.Debug("Database query failed: create PR", "error", err, "pull_request_id", pr.PullRequestID)
		return fmt.Errorf("failed to create PR: %w", err)
	}
	if result.RowsAffected() == 0 {
		return appErrors.ErrPRExists
	}
	return nil
}

func (r *Repository) GetPRByID(ctx context.Context, prID string) (*models.PullRequest, error) {
	query := `
		SELECT pull_request_id, pull_request_name, author_id, status, assigned_reviewers, created_at, merged_at
		FROM pull_requests
		WHERE pull_request_id = $1
	`
	pr := &models.PullRequest{}
	var reviewers []string
	err := r.db.QueryRow(ctx, query, prID).Scan(
		&pr.PullRequestID,
		&pr.PullRequestName,
		&pr.AuthorID,
		&pr.Status,
		&reviewers,
		&pr.CreatedAt,
		&pr.MergedAt,
	)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, appErrors.ErrPRNotFound
		}
		r.logger.Debug("Database query failed: get PR by ID", "error", err, "pull_request_id", prID)
		return nil, fmt.Errorf("failed to get pull request: %w", err)
	}
	if reviewers == nil {
		pr.AssignedReviewers = []string{}
	} else {
		pr.AssignedReviewers = reviewers
	}
	return pr, nil
}

func (r *Repository) UpdatePRStatus(ctx context.Context, prID string, status models.PRStatus) error {
	query := `UPDATE pull_requests SET status = $1 WHERE pull_request_id = $2`
	_, err := r.db.Exec(ctx, query, status, prID)
	if err != nil {
		r.logger.Debug("Database query failed: update PR status", "error", err, "pull_request_id", prID, "status", status)
		return err
	}
	return nil
}

func (r *Repository) UpdatePRStatusWithMergedAt(ctx context.Context, prID string, status models.PRStatus) error {
	query := `UPDATE pull_requests SET status = $1, merged_at = NOW() WHERE pull_request_id = $2`
	_, err := r.db.Exec(ctx, query, status, prID)
	if err != nil {
		r.logger.Debug("Database query failed: update PR status with merged_at", "error", err, "pull_request_id", prID, "status", status)
		return fmt.Errorf("failed to update PR status with merged_at: %w", err)
	}
	return nil
}

func (r *Repository) UpdatePRReviewers(ctx context.Context, prID string, reviewers []string) error {
	query := `UPDATE pull_requests SET assigned_reviewers = $1 WHERE pull_request_id = $2`
	_, err := r.db.Exec(ctx, query, reviewers, prID)
	if err != nil {
		r.logger.Debug("Database query failed: update PR reviewers", "error", err, "pull_request_id", prID)
		return fmt.Errorf("failed to update PR reviewers: %w", err)
	}
	return nil
}

func (r *Repository) GetPRsByReviewer(ctx context.Context, reviewerID string) ([]*models.PullRequest, error) {
	query := `
		SELECT pull_request_id, pull_request_name, author_id, status, assigned_reviewers, created_at, merged_at
		FROM pull_requests
		WHERE $1 = ANY(assigned_reviewers)
		ORDER BY created_at DESC
	`
	rows, err := r.db.Query(ctx, query, reviewerID)
	if err != nil {
		r.logger.Debug("Database query failed: get PRs by reviewer", "error", err, "reviewer_id", reviewerID)
		return nil, fmt.Errorf("failed to get PRs by reviewer: %w", err)
	}
	defer rows.Close()

	var prs []*models.PullRequest
	for rows.Next() {
		pr := &models.PullRequest{}
		var reviewers []string
		if err := rows.Scan(
			&pr.PullRequestID,
			&pr.PullRequestName,
			&pr.AuthorID,
			&pr.Status,
			&reviewers,
			&pr.CreatedAt,
			&pr.MergedAt,
		); err != nil {
			r.logger.Debug("Database scan failed: get PRs by reviewer", "error", err, "reviewer_id", reviewerID)
			return nil, fmt.Errorf("failed to scan PR: %w", err)
		}
		if reviewers == nil {
			pr.AssignedReviewers = []string{}
		} else {
			pr.AssignedReviewers = reviewers
		}
		prs = append(prs, pr)
	}
	if err := rows.Err(); err != nil {
		r.logger.Debug("Database rows iteration failed: get PRs by reviewer", "error", err, "reviewer_id", reviewerID)
		return nil, fmt.Errorf("failed to iterate PRs: %w", err)
	}
	return prs, nil
}

func (r *Repository) PRExists(ctx context.Context, prID string) (bool, error) {
	query := `SELECT EXISTS(SELECT 1 FROM pull_requests WHERE pull_request_id = $1)`
	var exists bool
	err := r.db.QueryRow(ctx, query, prID).Scan(&exists)
	if err != nil {
		r.logger.Debug("Database query failed: check PR existence", "error", err, "pull_request_id", prID)
		return false, fmt.Errorf("failed to check PR existence: %w", err)
	}
	return exists, nil
}

func (r *Repository) GetStats(ctx context.Context) (map[string]interface{}, error) {
	stats := make(map[string]interface{})

	var totalPRs, openPRs, mergedPRs int
	query := `
		SELECT 
			COUNT(*) as total_prs,
			COUNT(*) FILTER (WHERE status = 'OPEN') as open_prs,
			COUNT(*) FILTER (WHERE status = 'MERGED') as merged_prs
		FROM pull_requests
	`
	if err := r.db.QueryRow(ctx, query).Scan(&totalPRs, &openPRs, &mergedPRs); err != nil {
		r.logger.Debug("Database query failed: get PR stats", "error", err)
		return nil, fmt.Errorf("failed to get PR stats: %w", err)
	}
	stats["total_prs"] = totalPRs
	stats["open_prs"] = openPRs
	stats["merged_prs"] = mergedPRs

	assignmentQuery := `
		SELECT user_id, COUNT(*) as assignment_count
		FROM pull_requests, unnest(assigned_reviewers) AS user_id
		GROUP BY user_id
		ORDER BY assignment_count DESC
	`
	rows, err := r.db.Query(ctx, assignmentQuery)
	if err != nil {
		r.logger.Debug("Database query failed: get assignments by user", "error", err)
		return nil, fmt.Errorf("failed to get assignments by user: %w", err)
	}
	defer rows.Close()

	assignmentsByUser := make(map[string]int)
	for rows.Next() {
		var userID string
		var count int
		if err := rows.Scan(&userID, &count); err != nil {
			r.logger.Debug("Database scan failed: get assignments by user", "error", err)
			return nil, fmt.Errorf("failed to scan assignment: %w", err)
		}
		assignmentsByUser[userID] = count
	}
	if err := rows.Err(); err != nil {
		r.logger.Debug("Database rows iteration failed: get assignments by user", "error", err)
		return nil, fmt.Errorf("failed to iterate assignments: %w", err)
	}
	stats["assignments_by_user"] = assignmentsByUser

	var totalUsers, activeUsers int
	userQuery := `
		SELECT 
			COUNT(*) as total_users,
			COUNT(*) FILTER (WHERE is_active = true) as active_users
		FROM users
	`
	if err := r.db.QueryRow(ctx, userQuery).Scan(&totalUsers, &activeUsers); err != nil {
		r.logger.Debug("Database query failed: get user stats", "error", err)
		return nil, fmt.Errorf("failed to get user stats: %w", err)
	}
	stats["total_users"] = totalUsers
	stats["active_users"] = activeUsers

	var totalTeams int
	if err := r.db.QueryRow(ctx, `SELECT COUNT(DISTINCT team_name) FROM users`).Scan(&totalTeams); err != nil {
		r.logger.Debug("Database query failed: get total teams", "error", err)
		return nil, fmt.Errorf("failed to get total teams: %w", err)
	}
	stats["total_teams"] = totalTeams

	return stats, nil
}

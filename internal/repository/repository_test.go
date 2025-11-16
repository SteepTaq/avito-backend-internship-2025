//go:build integration

package repository

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/golang-migrate/migrate/v4"
	_ "github.com/golang-migrate/migrate/v4/database/postgres"
	_ "github.com/golang-migrate/migrate/v4/source/file"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/testcontainers/testcontainers-go"
	"github.com/testcontainers/testcontainers-go/modules/postgres"
	"github.com/testcontainers/testcontainers-go/wait"

	"github.com/SteepTaq/avito-backend-internship-2025/internal/models"
	"github.com/SteepTaq/avito-backend-internship-2025/pkg/logger"
)

func setupTestDB(t *testing.T) (*pgxpool.Pool, func()) {
	t.Helper()
	ctx := context.Background()

	postgresContainer, err := postgres.RunContainer(ctx,
		testcontainers.WithImage("postgres:16-alpine"),
		postgres.WithDatabase("testdb"),
		postgres.WithUsername("testuser"),
		postgres.WithPassword("testpass"),
		testcontainers.WithWaitStrategy(
			wait.ForLog("database system is ready to accept connections").
				WithOccurrence(2).
				WithStartupTimeout(30*time.Second),
		),
	)
	require.NoError(t, err, "Failed to start postgres container")

	connStr, err := postgresContainer.ConnectionString(ctx, "sslmode=disable")
	require.NoError(t, err, "Failed to get connection string")

	err = runMigrations(connStr)
	require.NoError(t, err, "Failed to run migrations")

	pool, err := pgxpool.New(ctx, connStr)
	require.NoError(t, err, "Failed to create connection pool")

	err = pool.Ping(ctx)
	require.NoError(t, err, "Failed to ping database")

	cleanup := func() {
		pool.Close()
		if err := postgresContainer.Terminate(ctx); err != nil {
			t.Logf("Failed to terminate container: %v", err)
		}
	}

	return pool, cleanup
}

func runMigrations(databaseURL string) error {
	wd, err := os.Getwd()
	if err != nil {
		return fmt.Errorf("failed to get working directory: %w", err)
	}

	migrationsPath := filepath.Join(wd, "..", "..", "migrations")
	migrationsPath, err = filepath.Abs(migrationsPath)
	if err != nil {
		return fmt.Errorf("failed to get absolute path: %w", err)
	}

	m, err := migrate.New(
		fmt.Sprintf("file://%s", migrationsPath),
		databaseURL,
	)
	if err != nil {
		return fmt.Errorf("failed to create migrate instance: %w", err)
	}
	defer m.Close()

	if err := m.Up(); err != nil && err != migrate.ErrNoChange {
		return fmt.Errorf("failed to apply migrations: %w", err)
	}

	return nil
}

func setupRepository(t *testing.T) (*Repository, func()) {
	t.Helper()
	pool, cleanup := setupTestDB(t)
	log := logger.New("debug")
	repo := NewRepository(pool, log)
	return repo, cleanup
}

func createTestUser(t *testing.T, repo *Repository, ctx context.Context, userID, username, teamName string, isActive bool) {
	t.Helper()
	_, err := repo.db.Exec(ctx,
		"INSERT INTO users (user_id, username, team_name, is_active) VALUES ($1, $2, $3, $4) ON CONFLICT DO NOTHING",
		userID, username, teamName, isActive,
	)
	require.NoError(t, err)
}

func TestRepository_GetUserByID(t *testing.T) {
	repo, cleanup := setupRepository(t)
	defer cleanup()

	ctx := context.Background()

	t.Run("user not found", func(t *testing.T) {
		user, err := repo.GetUserByID(ctx, "nonexistent")
		assert.Error(t, err)
		assert.Nil(t, user)
	})

	t.Run("get existing user", func(t *testing.T) {
		_, err := repo.db.Exec(ctx,
			"INSERT INTO users (user_id, username, team_name, is_active) VALUES ($1, $2, $3, $4)",
			"u1", "Alice", "backend", true,
		)
		require.NoError(t, err)

		user, err := repo.GetUserByID(ctx, "u1")
		assert.NoError(t, err)
		assert.NotNil(t, user)
		assert.Equal(t, "u1", user.UserID)
		assert.Equal(t, "Alice", user.Username)
		assert.Equal(t, "backend", user.TeamName)
		assert.True(t, user.IsActive)
	})
}

func TestRepository_SetUserActive(t *testing.T) {
	repo, cleanup := setupRepository(t)
	defer cleanup()

	ctx := context.Background()

	t.Run("user not found", func(t *testing.T) {
		err := repo.SetUserActive(ctx, "nonexistent", true)
		assert.Error(t, err)
	})

	t.Run("update user active status", func(t *testing.T) {
		_, err := repo.db.Exec(ctx,
			"INSERT INTO users (user_id, username, team_name, is_active) VALUES ($1, $2, $3, $4)",
			"u2", "Bob", "frontend", true,
		)
		require.NoError(t, err)

		err = repo.SetUserActive(ctx, "u2", false)
		assert.NoError(t, err)

		user, err := repo.GetUserByID(ctx, "u2")
		assert.NoError(t, err)
		assert.False(t, user.IsActive)
	})
}

func TestRepository_CreateTeam(t *testing.T) {
	repo, cleanup := setupRepository(t)
	defer cleanup()

	ctx := context.Background()

	t.Run("create team with members", func(t *testing.T) {
		team := &models.Team{
			TeamName: "backend",
			Members: []models.TeamMember{
				{UserID: "u1", Username: "Alice", IsActive: true},
				{UserID: "u2", Username: "Bob", IsActive: true},
			},
		}

		err := repo.CreateTeam(ctx, team)
		assert.NoError(t, err)

		createdTeam, err := repo.GetTeam(ctx, "backend")
		assert.NoError(t, err)
		assert.NotNil(t, createdTeam)
		assert.Equal(t, "backend", createdTeam.TeamName)
		assert.Len(t, createdTeam.Members, 2)
	})

	t.Run("update existing user on conflict", func(t *testing.T) {
		_, err := repo.db.Exec(ctx,
			"INSERT INTO users (user_id, username, team_name, is_active) VALUES ($1, $2, $3, $4)",
			"u3", "Charlie", "old_team", true,
		)
		require.NoError(t, err)

		team := &models.Team{
			TeamName: "new_team",
			Members: []models.TeamMember{
				{UserID: "u3", Username: "Charlie Updated", IsActive: false},
			},
		}

		err = repo.CreateTeam(ctx, team)
		assert.NoError(t, err)

		user, err := repo.GetUserByID(ctx, "u3")
		assert.NoError(t, err)
		assert.Equal(t, "new_team", user.TeamName)
		assert.Equal(t, "Charlie Updated", user.Username)
		assert.False(t, user.IsActive)
	})
}

func TestRepository_GetTeam(t *testing.T) {
	repo, cleanup := setupRepository(t)
	defer cleanup()

	ctx := context.Background()

	t.Run("team not found", func(t *testing.T) {
		team, err := repo.GetTeam(ctx, "nonexistent")
		assert.Error(t, err)
		assert.Nil(t, team)
	})

	t.Run("get team with members", func(t *testing.T) {
		team := &models.Team{
			TeamName: "frontend",
			Members: []models.TeamMember{
				{UserID: "u4", Username: "David", IsActive: true},
				{UserID: "u5", Username: "Eve", IsActive: false},
			},
		}

		err := repo.CreateTeam(ctx, team)
		require.NoError(t, err)

		gotTeam, err := repo.GetTeam(ctx, "frontend")
		assert.NoError(t, err)
		assert.NotNil(t, gotTeam)
		assert.Equal(t, "frontend", gotTeam.TeamName)
		assert.Len(t, gotTeam.Members, 2)
	})
}

func TestRepository_TeamExists(t *testing.T) {
	repo, cleanup := setupRepository(t)
	defer cleanup()

	ctx := context.Background()

	t.Run("team does not exist", func(t *testing.T) {
		exists, err := repo.TeamExists(ctx, "nonexistent")
		assert.NoError(t, err)
		assert.False(t, exists)
	})

	t.Run("team exists", func(t *testing.T) {
		team := &models.Team{
			TeamName: "devops",
			Members: []models.TeamMember{
				{UserID: "u6", Username: "Frank", IsActive: true},
			},
		}

		err := repo.CreateTeam(ctx, team)
		require.NoError(t, err)

		exists, err := repo.TeamExists(ctx, "devops")
		assert.NoError(t, err)
		assert.True(t, exists)
	})
}

func TestRepository_GetActiveUsersByTeam(t *testing.T) {
	repo, cleanup := setupRepository(t)
	defer cleanup()

	ctx := context.Background()

	t.Run("get active users excluding self", func(t *testing.T) {
		team := &models.Team{
			TeamName: "qa",
			Members: []models.TeamMember{
				{UserID: "u7", Username: "Grace", IsActive: true},
				{UserID: "u8", Username: "Henry", IsActive: true},
				{UserID: "u9", Username: "Ivy", IsActive: false},
			},
		}

		err := repo.CreateTeam(ctx, team)
		require.NoError(t, err)

		users, err := repo.GetActiveUsersByTeam(ctx, "qa", "u7")
		assert.NoError(t, err)
		assert.Len(t, users, 1)
		assert.Equal(t, "u8", users[0].UserID)
		assert.True(t, users[0].IsActive)
	})
}

func TestRepository_CreatePR(t *testing.T) {
	repo, cleanup := setupRepository(t)
	defer cleanup()

	ctx := context.Background()

	t.Run("create PR", func(t *testing.T) {
		createTestUser(t, repo, ctx, "u1", "Alice", "backend", true)

		now := time.Now()
		pr := &models.PullRequest{
			PullRequestID:     "pr-1",
			PullRequestName:   "Test PR",
			AuthorID:          "u1",
			Status:            models.PRStatusOpen,
			AssignedReviewers: []string{"u2", "u3"},
		}

		err := repo.CreatePR(ctx, pr)
		assert.NoError(t, err)

		gotPR, err := repo.GetPRByID(ctx, "pr-1")
		assert.NoError(t, err)
		assert.NotNil(t, gotPR)
		assert.Equal(t, "pr-1", gotPR.PullRequestID)
		assert.Equal(t, "Test PR", gotPR.PullRequestName)
		assert.Equal(t, "u1", gotPR.AuthorID)
		assert.Equal(t, models.PRStatusOpen, gotPR.Status)
		assert.Len(t, gotPR.AssignedReviewers, 2)
		assert.NotNil(t, gotPR.CreatedAt)
		assert.True(t, gotPR.CreatedAt.After(now.Add(-time.Second)))
	})
}

func TestRepository_GetPRByID(t *testing.T) {
	repo, cleanup := setupRepository(t)
	defer cleanup()

	ctx := context.Background()

	t.Run("PR not found", func(t *testing.T) {
		pr, err := repo.GetPRByID(ctx, "nonexistent")
		assert.Error(t, err)
		assert.Nil(t, pr)
	})

	t.Run("get PR with empty reviewers", func(t *testing.T) {
		createTestUser(t, repo, ctx, "u1", "Alice", "backend", true)

		pr := &models.PullRequest{
			PullRequestID:     "pr-2",
			PullRequestName:   "Empty Reviewers PR",
			AuthorID:          "u1",
			Status:            models.PRStatusOpen,
			AssignedReviewers: []string{},
		}

		err := repo.CreatePR(ctx, pr)
		require.NoError(t, err)

		gotPR, err := repo.GetPRByID(ctx, "pr-2")
		assert.NoError(t, err)
		assert.Empty(t, gotPR.AssignedReviewers)
	})
}

func TestRepository_UpdatePRStatus(t *testing.T) {
	repo, cleanup := setupRepository(t)
	defer cleanup()

	ctx := context.Background()

	t.Run("update PR status", func(t *testing.T) {
		createTestUser(t, repo, ctx, "u1", "Alice", "backend", true)

		pr := &models.PullRequest{
			PullRequestID:     "pr-3",
			PullRequestName:   "Status Test PR",
			AuthorID:          "u1",
			Status:            models.PRStatusOpen,
			AssignedReviewers: []string{},
		}

		err := repo.CreatePR(ctx, pr)
		require.NoError(t, err)

		err = repo.UpdatePRStatusWithMergedAt(ctx, "pr-3", models.PRStatusMerged)
		assert.NoError(t, err)

		gotPR, err := repo.GetPRByID(ctx, "pr-3")
		assert.NoError(t, err)
		assert.Equal(t, models.PRStatusMerged, gotPR.Status)
	})
}

func TestRepository_UpdatePRStatusWithMergedAt(t *testing.T) {
	repo, cleanup := setupRepository(t)
	defer cleanup()

	ctx := context.Background()

	t.Run("update PR status with merged_at", func(t *testing.T) {
		createTestUser(t, repo, ctx, "u1", "Alice", "backend", true)

		pr := &models.PullRequest{
			PullRequestID:     "pr-4",
			PullRequestName:   "Merge Test PR",
			AuthorID:          "u1",
			Status:            models.PRStatusOpen,
			AssignedReviewers: []string{},
		}

		err := repo.CreatePR(ctx, pr)
		require.NoError(t, err)

		err = repo.UpdatePRStatusWithMergedAt(ctx, "pr-4", models.PRStatusMerged)
		assert.NoError(t, err)

		gotPR, err := repo.GetPRByID(ctx, "pr-4")
		assert.NoError(t, err)
		assert.Equal(t, models.PRStatusMerged, gotPR.Status)
		assert.NotNil(t, gotPR.MergedAt)
	})
}

func TestRepository_UpdatePRReviewers(t *testing.T) {
	repo, cleanup := setupRepository(t)
	defer cleanup()

	ctx := context.Background()

	t.Run("update PR reviewers", func(t *testing.T) {
		createTestUser(t, repo, ctx, "u1", "Alice", "backend", true)

		pr := &models.PullRequest{
			PullRequestID:     "pr-5",
			PullRequestName:   "Reviewers Test PR",
			AuthorID:          "u1",
			Status:            models.PRStatusOpen,
			AssignedReviewers: []string{"u2"},
		}

		err := repo.CreatePR(ctx, pr)
		require.NoError(t, err)

		newReviewers := []string{"u3", "u4"}
		err = repo.UpdatePRReviewers(ctx, "pr-5", newReviewers)
		assert.NoError(t, err)

		gotPR, err := repo.GetPRByID(ctx, "pr-5")
		assert.NoError(t, err)
		assert.Len(t, gotPR.AssignedReviewers, 2)
	})
}

func TestRepository_GetPRsByReviewer(t *testing.T) {
	repo, cleanup := setupRepository(t)
	defer cleanup()

	ctx := context.Background()

	t.Run("get PRs by reviewer", func(t *testing.T) {
		createTestUser(t, repo, ctx, "u1", "Alice", "backend", true)

		pr1 := &models.PullRequest{
			PullRequestID:     "pr-6",
			PullRequestName:   "PR 1",
			AuthorID:          "u1",
			Status:            models.PRStatusOpen,
			AssignedReviewers: []string{"u2", "u3"},
		}

		pr2 := &models.PullRequest{
			PullRequestID:     "pr-7",
			PullRequestName:   "PR 2",
			AuthorID:          "u1",
			Status:            models.PRStatusOpen,
			AssignedReviewers: []string{"u2"},
		}

		pr3 := &models.PullRequest{
			PullRequestID:     "pr-8",
			PullRequestName:   "PR 3",
			AuthorID:          "u1",
			Status:            models.PRStatusOpen,
			AssignedReviewers: []string{"u4"},
		}

		err := repo.CreatePR(ctx, pr1)
		require.NoError(t, err)
		err = repo.CreatePR(ctx, pr2)
		require.NoError(t, err)
		err = repo.UpdatePRStatusWithMergedAt(ctx, "pr-7", models.PRStatusMerged)
		require.NoError(t, err)
		err = repo.CreatePR(ctx, pr3)
		require.NoError(t, err)

		prs, err := repo.GetPRsByReviewer(ctx, "u2")
		assert.NoError(t, err)
		assert.Len(t, prs, 2)
	})
}

func TestRepository_PRExists(t *testing.T) {
	repo, cleanup := setupRepository(t)
	defer cleanup()

	ctx := context.Background()

	t.Run("PR does not exist", func(t *testing.T) {
		exists, err := repo.PRExists(ctx, "nonexistent")
		assert.NoError(t, err)
		assert.False(t, exists)
	})

	t.Run("PR exists", func(t *testing.T) {
		createTestUser(t, repo, ctx, "u1", "Alice", "backend", true)

		pr := &models.PullRequest{
			PullRequestID:     "pr-9",
			PullRequestName:   "Exists Test PR",
			AuthorID:          "u1",
			Status:            models.PRStatusOpen,
			AssignedReviewers: []string{},
		}

		err := repo.CreatePR(ctx, pr)
		require.NoError(t, err)

		exists, err := repo.PRExists(ctx, "pr-9")
		assert.NoError(t, err)
		assert.True(t, exists)
	})
}

func TestRepository_GetStats(t *testing.T) {
	repo, cleanup := setupRepository(t)
	defer cleanup()

	ctx := context.Background()

	t.Run("get stats with data", func(t *testing.T) {
		team := &models.Team{
			TeamName: "stats_team",
			Members: []models.TeamMember{
				{UserID: "u10", Username: "User1", IsActive: true},
				{UserID: "u11", Username: "User2", IsActive: true},
				{UserID: "u12", Username: "User3", IsActive: false},
			},
		}

		err := repo.CreateTeam(ctx, team)
		require.NoError(t, err)

		pr1 := &models.PullRequest{
			PullRequestID:     "pr-10",
			PullRequestName:   "PR 1",
			AuthorID:          "u10",
			Status:            models.PRStatusOpen,
			AssignedReviewers: []string{"u11"},
		}

		pr2 := &models.PullRequest{
			PullRequestID:     "pr-11",
			PullRequestName:   "PR 2",
			AuthorID:          "u10",
			Status:            models.PRStatusOpen,
			AssignedReviewers: []string{"u11", "u12"},
		}

		err = repo.CreatePR(ctx, pr1)
		require.NoError(t, err)
		err = repo.CreatePR(ctx, pr2)
		require.NoError(t, err)
		err = repo.UpdatePRStatusWithMergedAt(ctx, "pr-11", models.PRStatusMerged)
		require.NoError(t, err)

		stats, err := repo.GetStats(ctx)
		assert.NoError(t, err)
		assert.NotNil(t, stats)

		assert.Equal(t, 2, stats["total_prs"])
		assert.Equal(t, 1, stats["open_prs"])
		assert.Equal(t, 1, stats["merged_prs"])
		assert.Equal(t, 3, stats["total_users"])
		assert.Equal(t, 2, stats["active_users"])
		assert.Equal(t, 1, stats["total_teams"])

		assignments, ok := stats["assignments_by_user"].(map[string]int)
		assert.True(t, ok)
		assert.Equal(t, 2, assignments["u11"])
		assert.Equal(t, 1, assignments["u12"])
	})
}

func TestRepository_Ping(t *testing.T) {
	repo, cleanup := setupRepository(t)
	defer cleanup()

	ctx := context.Background()

	err := repo.Ping(ctx)
	assert.NoError(t, err)
}

func TestRepository_DeactivateUsersByTeam(t *testing.T) {
	repo, cleanup := setupRepository(t)
	defer cleanup()

	ctx := context.Background()

	t.Run("deactivate all users in team", func(t *testing.T) {
		team := &models.Team{
			TeamName: "deactivate_team",
			Members: []models.TeamMember{
				{UserID: "u13", Username: "User1", IsActive: true},
				{UserID: "u14", Username: "User2", IsActive: true},
				{UserID: "u15", Username: "User3", IsActive: true},
			},
		}

		err := repo.CreateTeam(ctx, team)
		require.NoError(t, err)

		deactivatedIDs, err := repo.DeactivateUsersByTeam(ctx, "deactivate_team", []string{})
		assert.NoError(t, err)
		assert.Len(t, deactivatedIDs, 3)
		assert.Contains(t, deactivatedIDs, "u13")
		assert.Contains(t, deactivatedIDs, "u14")
		assert.Contains(t, deactivatedIDs, "u15")

		user1, err := repo.GetUserByID(ctx, "u13")
		assert.NoError(t, err)
		assert.False(t, user1.IsActive)

		user2, err := repo.GetUserByID(ctx, "u14")
		assert.NoError(t, err)
		assert.False(t, user2.IsActive)
	})

	t.Run("deactivate specific users in team", func(t *testing.T) {
		team := &models.Team{
			TeamName: "deactivate_specific_team",
			Members: []models.TeamMember{
				{UserID: "u16", Username: "User1", IsActive: true},
				{UserID: "u17", Username: "User2", IsActive: true},
				{UserID: "u18", Username: "User3", IsActive: true},
			},
		}

		err := repo.CreateTeam(ctx, team)
		require.NoError(t, err)

		deactivatedIDs, err := repo.DeactivateUsersByTeam(ctx, "deactivate_specific_team", []string{"u16", "u17"})
		assert.NoError(t, err)
		assert.Len(t, deactivatedIDs, 2)
		assert.Contains(t, deactivatedIDs, "u16")
		assert.Contains(t, deactivatedIDs, "u17")

		user1, err := repo.GetUserByID(ctx, "u16")
		assert.NoError(t, err)
		assert.False(t, user1.IsActive)

		user2, err := repo.GetUserByID(ctx, "u17")
		assert.NoError(t, err)
		assert.False(t, user2.IsActive)

		user3, err := repo.GetUserByID(ctx, "u18")
		assert.NoError(t, err)
		assert.True(t, user3.IsActive)
	})

	t.Run("deactivate users from non-existent team", func(t *testing.T) {
		deactivatedIDs, err := repo.DeactivateUsersByTeam(ctx, "nonexistent_team", []string{})
		assert.NoError(t, err)
		assert.Empty(t, deactivatedIDs)
	})
}

func TestRepository_GetOpenPRsByReviewers(t *testing.T) {
	repo, cleanup := setupRepository(t)
	defer cleanup()

	ctx := context.Background()

	t.Run("get open PRs by reviewers", func(t *testing.T) {
		createTestUser(t, repo, ctx, "u1", "Alice", "backend", true)

		pr1 := &models.PullRequest{
			PullRequestID:     "pr-12",
			PullRequestName:   "Open PR 1",
			AuthorID:          "u1",
			Status:            models.PRStatusOpen,
			AssignedReviewers: []string{"u2", "u3"},
		}

		pr2 := &models.PullRequest{
			PullRequestID:     "pr-13",
			PullRequestName:   "Open PR 2",
			AuthorID:          "u1",
			Status:            models.PRStatusOpen,
			AssignedReviewers: []string{"u2"},
		}

		pr3 := &models.PullRequest{
			PullRequestID:     "pr-14",
			PullRequestName:   "Merged PR",
			AuthorID:          "u1",
			Status:            models.PRStatusOpen,
			AssignedReviewers: []string{"u2"},
		}

		err := repo.CreatePR(ctx, pr1)
		require.NoError(t, err)
		err = repo.CreatePR(ctx, pr2)
		require.NoError(t, err)
		err = repo.CreatePR(ctx, pr3)
		require.NoError(t, err)
		err = repo.UpdatePRStatusWithMergedAt(ctx, "pr-14", models.PRStatusMerged)
		require.NoError(t, err)

		prs, err := repo.GetOpenPRsByReviewers(ctx, []string{"u2"})
		assert.NoError(t, err)
		assert.Len(t, prs, 2)

		for _, pr := range prs {
			assert.Equal(t, models.PRStatusOpen, pr.Status)
			assert.Contains(t, pr.AssignedReviewers, "u2")
		}
	})

	t.Run("get open PRs by multiple reviewers", func(t *testing.T) {
		createTestUser(t, repo, ctx, "u1", "Alice", "backend", true)

		pr1 := &models.PullRequest{
			PullRequestID:     "pr-multi-1",
			PullRequestName:   "PR with reviewer20",
			AuthorID:          "u1",
			Status:            models.PRStatusOpen,
			AssignedReviewers: []string{"reviewer20"},
		}

		pr2 := &models.PullRequest{
			PullRequestID:     "pr-multi-2",
			PullRequestName:   "PR with reviewer21",
			AuthorID:          "u1",
			Status:            models.PRStatusOpen,
			AssignedReviewers: []string{"reviewer21"},
		}

		err := repo.CreatePR(ctx, pr1)
		require.NoError(t, err)
		err = repo.CreatePR(ctx, pr2)
		require.NoError(t, err)

		prs, err := repo.GetOpenPRsByReviewers(ctx, []string{"reviewer20", "reviewer21"})
		assert.NoError(t, err)
		assert.GreaterOrEqual(t, len(prs), 2, "should return at least 2 PRs")
		prIDs := make(map[string]bool)
		for _, pr := range prs {
			prIDs[pr.PullRequestID] = true
		}
		assert.True(t, prIDs["pr-multi-1"], "should contain pr-multi-1")
		assert.True(t, prIDs["pr-multi-2"], "should contain pr-multi-2")
	})

	t.Run("get open PRs with empty reviewer list", func(t *testing.T) {
		prs, err := repo.GetOpenPRsByReviewers(ctx, []string{})
		assert.NoError(t, err)
		assert.Empty(t, prs)
	})

	t.Run("get open PRs for reviewer with no PRs", func(t *testing.T) {
		prs, err := repo.GetOpenPRsByReviewers(ctx, []string{"nonexistent_reviewer"})
		assert.NoError(t, err)
		assert.Empty(t, prs)
	})
}

func TestRepository_BulkUpdatePRReviewers(t *testing.T) {
	repo, cleanup := setupRepository(t)
	defer cleanup()

	ctx := context.Background()

	t.Run("bulk update PR reviewers", func(t *testing.T) {
		createTestUser(t, repo, ctx, "u1", "Alice", "backend", true)

		pr1 := &models.PullRequest{
			PullRequestID:     "pr-17",
			PullRequestName:   "PR 1",
			AuthorID:          "u1",
			Status:            models.PRStatusOpen,
			AssignedReviewers: []string{"u2"},
		}

		pr2 := &models.PullRequest{
			PullRequestID:     "pr-18",
			PullRequestName:   "PR 2",
			AuthorID:          "u1",
			Status:            models.PRStatusOpen,
			AssignedReviewers: []string{"u3"},
		}

		err := repo.CreatePR(ctx, pr1)
		require.NoError(t, err)
		err = repo.CreatePR(ctx, pr2)
		require.NoError(t, err)

		prReviewers := map[string][]string{
			"pr-17": {"u4", "u5"},
			"pr-18": {"u6"},
		}

		err = repo.BulkUpdatePRReviewers(ctx, prReviewers)
		assert.NoError(t, err)

		gotPR1, err := repo.GetPRByID(ctx, "pr-17")
		assert.NoError(t, err)
		assert.Len(t, gotPR1.AssignedReviewers, 2)
		assert.Contains(t, gotPR1.AssignedReviewers, "u4")
		assert.Contains(t, gotPR1.AssignedReviewers, "u5")

		gotPR2, err := repo.GetPRByID(ctx, "pr-18")
		assert.NoError(t, err)
		assert.Len(t, gotPR2.AssignedReviewers, 1)
		assert.Contains(t, gotPR2.AssignedReviewers, "u6")
	})

	t.Run("bulk update with empty map", func(t *testing.T) {
		err := repo.BulkUpdatePRReviewers(ctx, map[string][]string{})
		assert.NoError(t, err)
	})

	t.Run("bulk update with empty reviewers list", func(t *testing.T) {
		createTestUser(t, repo, ctx, "u1", "Alice", "backend", true)

		pr := &models.PullRequest{
			PullRequestID:     "pr-19",
			PullRequestName:   "PR with empty reviewers",
			AuthorID:          "u1",
			Status:            models.PRStatusOpen,
			AssignedReviewers: []string{"u2"},
		}

		err := repo.CreatePR(ctx, pr)
		require.NoError(t, err)

		prReviewers := map[string][]string{
			"pr-19": {},
		}

		err = repo.BulkUpdatePRReviewers(ctx, prReviewers)
		assert.NoError(t, err)

		gotPR, err := repo.GetPRByID(ctx, "pr-19")
		assert.NoError(t, err)
		assert.Empty(t, gotPR.AssignedReviewers)
	})
}

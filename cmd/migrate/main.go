package main

import (
	"errors"
	"flag"
	"fmt"
	"os"

	"github.com/golang-migrate/migrate/v4"
	_ "github.com/golang-migrate/migrate/v4/database/postgres"
	_ "github.com/golang-migrate/migrate/v4/source/file"

	"github.com/SteepTaq/avito-backend-internship-2025/pkg/logger"
)

func main() {
	var (
		databaseURL    = flag.String("database-url", "", "Database connection URL")
		migrationsPath = flag.String("migrations-path", "./migrations", "Path to migrations directory")
		command        = flag.String("command", "up", "Migration command: up, down, force, version")
		version        = flag.Int("version", 0, "Version for force command")
		logLevel       = flag.String("log-level", "info", "Log level: debug, info, warn, error")
	)
	flag.Parse()

	log := logger.New(*logLevel)

	if *databaseURL == "" {
		*databaseURL = os.Getenv("DATABASE_URL")
		if *databaseURL == "" {
			log.Error("DATABASE_URL is required", "hint", "use -database-url flag or DATABASE_URL env var")
			os.Exit(1)
		}
	}

	log.Info("Initializing migrations",
		"migrations_path", *migrationsPath,
		"command", *command,
	)

	m, err := migrate.New(
		fmt.Sprintf("file://%s", *migrationsPath),
		*databaseURL,
	)
	if err != nil {
		log.Error("Failed to create migrate instance", "error", err)
		os.Exit(1)
	}
	defer m.Close()

	switch *command {
	case "up":
		log.Info("Applying migrations")
		if err := m.Up(); err != nil {
			if errors.Is(err, migrate.ErrNoChange) {
				log.Info("No migrations to apply")
				return
			}
			log.Error("Failed to apply migrations", "error", err)
			os.Exit(1)
		}
		log.Info("Migrations applied successfully")

	case "down":
		log.Info("Rolling back migrations")
		if err := m.Down(); err != nil {
			if errors.Is(err, migrate.ErrNoChange) {
				log.Info("No migrations to rollback")
				return
			}
			log.Error("Failed to rollback migrations", "error", err)
			os.Exit(1)
		}
		log.Info("Migrations rolled back successfully")

	case "force":
		if *version == 0 {
			log.Error("Version is required for force command")
			os.Exit(1)
		}
		log.Info("Forcing migration version", "version", *version)
		if err := m.Force(*version); err != nil {
			log.Error("Failed to force migration version", "error", err, "version", *version)
			os.Exit(1)
		}
		log.Info("Migration version forced", "version", *version)

	case "version":
		version, dirty, err := m.Version()
		if err != nil {
			if errors.Is(err, migrate.ErrNilVersion) {
				log.Info("No migrations applied")
				return
			}
			log.Error("Failed to get migration version", "error", err)
			os.Exit(1)
		}
		if dirty {
			log.Warn("Current migration version", "version", version, "dirty", true)
		} else {
			log.Info("Current migration version", "version", version)
		}

	default:
		log.Error("Unknown command", "command", *command, "valid_commands", []string{"up", "down", "force", "version"})
		os.Exit(1)
	}
}

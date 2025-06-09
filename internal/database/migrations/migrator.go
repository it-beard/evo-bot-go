package migrations

import (
	"database/sql"
	"evo-bot-go/internal/database/migrations/implementations"
	"fmt"
	"log"
)

const (
	createMigrationsTableSQL = `
		CREATE TABLE IF NOT EXISTS migrations (
			id SERIAL PRIMARY KEY,
			name TEXT NOT NULL UNIQUE,
			timestamp TEXT NOT NULL,
			created_at TIMESTAMP WITH TIME ZONE DEFAULT NOW()
		)
	`
)

// initMigrationsSchema initializes the migrations table schema
func initMigrationsSchema(db *sql.DB) error {
	if _, err := db.Exec(createMigrationsTableSQL); err != nil {
		return fmt.Errorf("failed to create migrations table: %w", err)
	}

	return nil
}

// Registry returns all available migrations in order
func Registry() []implementations.Migration {
	return []implementations.Migration{
		implementations.NewInitialMigration(),
		implementations.NewChangeUserIdToNullableString(),
		implementations.NewRenameUserIdToUserNickname(),
		implementations.NewRenameContentsToEvents(),
		implementations.NewAddNewEventTypes(),
		implementations.NewUpdateEventsConstraints(),
		implementations.NewAddUsersAndProfilesTables(),
		implementations.NewAddPublishedMessageIDToProfiles(),
		implementations.NewRenameWebsiteToFreelink(),
		implementations.NewRemoveSocialLinksFromProfiles(),
		implementations.NewAddRandomCoffeePollTables(),
		implementations.NewRemoveChatIdFromRandomCoffeePolls(),
		implementations.NewAddIsClubMemberToUsers(),
		// Add new migrations here
	}
}

// RunMigrations checks and runs all pending migrations
func RunMigrations(db *sql.DB) error {
	// Initialize the migrations table
	if err := initMigrationsSchema(db); err != nil {
		return fmt.Errorf("failed to initialize migrations schema: %w", err)
	}

	// Get all registered migrations
	allMigrations := Registry()

	// If no migrations, return early
	if len(allMigrations) == 0 {
		log.Println("No migrations to apply")
		return nil
	}

	// Get already applied migrations
	appliedMigrations, err := getAppliedMigrations(db)
	if err != nil {
		return fmt.Errorf("failed to get applied migrations: %w", err)
	}

	// Run each pending migration
	for _, migration := range allMigrations {
		migrationName := migration.Name()

		// Skip already applied migrations
		if _, exists := appliedMigrations[migrationName]; exists {
			continue
		}

		// Apply the migration
		log.Printf("Applying migration: %s", migrationName)
		if err := migration.Apply(db); err != nil {
			// If migration fails, roll it back
			log.Printf("Migration %s failed, rolling back: %v", migrationName, err)
			rollbackErr := migration.Rollback(db)
			if rollbackErr != nil {
				log.Printf("Warning: Rollback of migration %s also failed: %v", migrationName, rollbackErr)
			} else {
				// Remove the migration from the database if rollback succeeds
				if err := removeMigrationRecord(db, migrationName); err != nil {
					log.Printf("Warning: Failed to remove migration record for %s: %v", migrationName, err)
				}
			}
			log.Printf("Failed to apply migration %s: %v", migrationName, err)
			break // Stop the loop after the first failed migration
		}

		// Record the migration to the database
		if err := recordMigration(db, migrationName, migration.Timestamp()); err != nil {
			return fmt.Errorf("failed to record migration %s: %w", migrationName, err)
		}

		log.Printf("Successfully applied migration: %s", migrationName)
	}

	return nil
}

func recordMigration(db *sql.DB, name string, timestamp string) error {
	insertSQL := `INSERT INTO migrations (name, timestamp) VALUES ($1, $2)`
	_, err := db.Exec(insertSQL, name, timestamp)
	return err
}

func removeMigrationRecord(db *sql.DB, name string) error {
	deleteSQL := `DELETE FROM migrations WHERE name = $1`
	_, err := db.Exec(deleteSQL, name)
	return err
}

func getAppliedMigrations(db *sql.DB) (map[string]bool, error) {
	appliedMigrations := make(map[string]bool)

	rows, err := db.Query("SELECT name FROM migrations")
	if err != nil {
		return nil, fmt.Errorf("failed to query migrations: %w", err)
	}
	defer rows.Close()

	for rows.Next() {
		var name string
		if err := rows.Scan(&name); err != nil {
			return nil, fmt.Errorf("failed to scan migration name: %w", err)
		}
		appliedMigrations[name] = true
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("error iterating migration rows: %w", err)
	}

	return appliedMigrations, nil
}

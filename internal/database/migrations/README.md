# Database Migrations

This package contains the database migration system for the evocoders-bot-go project.

## Migration System Features

- Automatic tracking of applied migrations in a `migrations` table
- Automatic rollback if a migration fails
- Migrations are applied in order based on their timestamp
- Idempotent migrations that can be safely run multiple times
- Migrations run automatically when the application starts

## How to Create a New Migration

1. Create a new file in the `migrations/implementations` package with a name in the format `YYYYMMDD_description.go`
2. Define a struct that embeds `BaseMigration` and implements the `Migration` interface
3. Implement `Apply` and `Rollback` methods
4. Add the migration to the registry in `migrator.go`

### Example Migration

```go
package implementations

import (
	"database/sql"
)

type MyNewMigration struct {
	BaseMigration
}

func NewMyNewMigration() *MyNewMigration {
	return &MyNewMigration{
		BaseMigration: BaseMigration{
			name:      "my_new_migration",
			timestamp: "20240403", // Today's date in YYYYMMDD format
		},
	}
}

func (m *MyNewMigration) Apply(db *sql.DB) error {
	sql := `ALTER TABLE my_table ADD COLUMN new_column TEXT`
	_, err := db.Exec(sql)
	return err
}

func (m *MyNewMigration) Rollback(db *sql.DB) error {
	sql := `ALTER TABLE my_table DROP COLUMN IF EXISTS new_column`
	_, err := db.Exec(sql)
	return err
}
```

Then add it to the registry in `migrator.go`:

```go
// Registry returns all available migrations in order
func Registry() []implementations.Migration {
	return []implementations.Migration{
		// old migrations
		NewMyNewMigration(),
		// Add new migrations here
	}
}
```

## How Migrations Work

Migrations are run automatically when the application starts. The system:

1. Creates the migrations table in the database initialization phase
2. Gets a list of all registered migrations from the Registry
3. Checks which migrations have already been applied
4. Applies any pending migrations in order
5. If a migration fails, it automatically rolls it back and stops processing further migrations

No manual execution is required. Simply add a new migration to the registry and restart the application.

## Migration Files Organization

- `migrator.go` - Contains the Migration interface, Registry, and RunMigrations function
- `implementations/` - Directory containing individual migration implementations

## Rollback Behavior

If a migration fails during application, the system will:

1. Automatically run the rollback function for that migration
2. Remove the migration record from the database if the rollback succeeds
3. Stop processing any further migrations
4. Log detailed error information 
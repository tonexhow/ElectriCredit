package database

import (
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"strings"
)

type priceSeed struct {
	BaseRate            string `json:"base_rate"`
	TenantRate          string `json:"tenant_rate"`
	DebtLimit           string `json:"debt_limit"`
	PricePerKWhCentavos *int64 `json:"price_per_kwh_centavos,omitempty"`
	DebtLimitCentavos   *int64 `json:"debt_limit_centavos,omitempty"`
}

// CreateDatabase creates and safely updates the ElectriCredit database.
// It is idempotent and can be run repeatedly without clearing user data.
func CreateDatabase(db *sql.DB, priceJSONPath string) error {
	if db == nil {
		return errors.New("database connection is nil")
	}

	// Some older builds used a separate roles table referenced by superusers.
	// Temporarily disable FK enforcement while that table is rebuilt in place;
	// foreign keys are re-enabled and verified before this initializer returns.
	if _, err := db.Exec(`PRAGMA foreign_keys = OFF;`); err != nil {
		return fmt.Errorf("disable foreign keys for migration: %w", err)
	}
	defer func() { _, _ = db.Exec(`PRAGMA foreign_keys = ON;`) }()

	tx, err := db.Begin()
	if err != nil {
		return fmt.Errorf("begin database initialization: %w", err)
	}
	defer tx.Rollback()

	// Older ElectriCredit builds stored staff ranks in a separate roles table.
	// Copy that value onto superusers before the current schema/indexes are applied.
	if err := prepareLegacyStaffRanks(tx); err != nil {
		return err
	}

	schema := []string{
		`CREATE TABLE IF NOT EXISTS users (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			first_name TEXT NOT NULL,
			last_name TEXT NOT NULL,
			email TEXT UNIQUE,
			phone TEXT,
			room TEXT,
			profile_image TEXT,
			profile_image_blob BLOB,
			profile_image_mime TEXT,
			birth_date TEXT,
			gender TEXT CHECK(gender IS NULL OR gender IN ('female','male','other','prefer_not_to_say')),
			links_json TEXT,
			debt_limit_units INTEGER,
			status TEXT NOT NULL DEFAULT 'active'
				CHECK(status IN ('active','inactive','disabled')),
			created_at INTEGER NOT NULL,
			updated_at INTEGER NOT NULL
		);`,

		`CREATE TABLE IF NOT EXISTS superusers (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			first_name TEXT NOT NULL,
			last_name TEXT NOT NULL,
			email TEXT NOT NULL UNIQUE,
			phone TEXT,
			profile_image TEXT,
			profile_image_blob BLOB,
			profile_image_mime TEXT,
			birth_date TEXT,
			links_json TEXT,
			rank TEXT NOT NULL CHECK(rank IN ('admin','owner','developer')),
			password_hash TEXT NOT NULL,
			is_head_developer INTEGER NOT NULL DEFAULT 0 CHECK(is_head_developer IN (0,1)),
			status TEXT NOT NULL DEFAULT 'active'
				CHECK(status IN ('active','inactive','disabled')),
			created_at INTEGER NOT NULL,
			updated_at INTEGER NOT NULL
		);`,

		`CREATE TABLE IF NOT EXISTS cards (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			uid TEXT NOT NULL UNIQUE,
			user_id INTEGER NOT NULL,
			balance_units INTEGER NOT NULL DEFAULT 0,
			status TEXT NOT NULL DEFAULT 'active'
				CHECK(status IN ('active','inactive','blocked','lost')),
			created_at INTEGER NOT NULL,
			updated_at INTEGER NOT NULL,
			FOREIGN KEY(user_id) REFERENCES users(id) ON DELETE CASCADE
		);`,

		`CREATE TABLE IF NOT EXISTS hubs (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			mac_address TEXT NOT NULL UNIQUE,
			name TEXT NOT NULL UNIQUE,
			location TEXT,
			auth_token_hash TEXT,
			health_status TEXT NOT NULL DEFAULT 'offline'
				CHECK(health_status IN ('ok','warning','offline')),
			last_seen_at INTEGER,
			last_warning TEXT,
			status TEXT NOT NULL DEFAULT 'active'
				CHECK(status IN ('active','inactive','disabled')),
			created_at INTEGER NOT NULL,
			updated_at INTEGER NOT NULL
		);`,

		`CREATE TABLE IF NOT EXISTS registries (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			mac_address TEXT NOT NULL UNIQUE,
			name TEXT NOT NULL UNIQUE,
			location TEXT,
			auth_token_hash TEXT,
			health_status TEXT NOT NULL DEFAULT 'offline'
				CHECK(health_status IN ('ok','warning','offline')),
			last_seen_at INTEGER,
			last_warning TEXT,
			status TEXT NOT NULL DEFAULT 'active'
				CHECK(status IN ('active','inactive','disabled')),
			created_at INTEGER NOT NULL,
			updated_at INTEGER NOT NULL
		);`,

		`CREATE TABLE IF NOT EXISTS module_pairing_sessions (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			session_code TEXT NOT NULL UNIQUE,
			module_type TEXT NOT NULL CHECK(module_type IN ('hub','registry')),
			module_id INTEGER NOT NULL,
			desired_status TEXT NOT NULL DEFAULT 'active' CHECK(desired_status IN ('active','inactive','disabled')),
			server_url TEXT NOT NULL,
			created_by INTEGER NOT NULL,
			status TEXT NOT NULL DEFAULT 'waiting' CHECK(status IN ('waiting','connected','expired','cancelled')),
			expires_at INTEGER NOT NULL,
			confirmed_at INTEGER,
			created_at INTEGER NOT NULL,
			updated_at INTEGER NOT NULL,
			FOREIGN KEY(created_by) REFERENCES superusers(id) ON DELETE CASCADE
		);`,

		`CREATE TABLE IF NOT EXISTS registry_sessions (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			registry_id INTEGER,
			card_id INTEGER,
			started_at INTEGER NOT NULL,
			ended_at INTEGER,
			status TEXT NOT NULL DEFAULT 'active'
				CHECK(status IN ('active','completed','cancelled','fault')),
			FOREIGN KEY(registry_id) REFERENCES registries(id) ON DELETE SET NULL,
			FOREIGN KEY(card_id) REFERENCES cards(id) ON DELETE SET NULL
		);`,

		`CREATE TABLE IF NOT EXISTS registry_topups (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			operation_id TEXT NOT NULL UNIQUE,
			registry_id INTEGER,
			card_id INTEGER,
			amount_units INTEGER NOT NULL CHECK(amount_units > 0),
			created_at INTEGER NOT NULL,
			FOREIGN KEY(registry_id) REFERENCES registries(id) ON DELETE SET NULL,
			FOREIGN KEY(card_id) REFERENCES cards(id) ON DELETE SET NULL
		);`,

		`CREATE TABLE IF NOT EXISTS power_sessions (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			card_id INTEGER,
			hub_id INTEGER,
			started_at INTEGER NOT NULL,
			ended_at INTEGER,
			energy_wh INTEGER NOT NULL DEFAULT 0 CHECK(energy_wh >= 0),
			cost_units INTEGER NOT NULL DEFAULT 0 CHECK(cost_units >= 0),
			rate_units INTEGER,
			status TEXT NOT NULL DEFAULT 'active'
				CHECK(status IN ('active','completed','stopped','fault')),
			FOREIGN KEY(card_id) REFERENCES cards(id) ON DELETE SET NULL,
			FOREIGN KEY(hub_id) REFERENCES hubs(id) ON DELETE SET NULL
		);`,

		`CREATE TABLE IF NOT EXISTS price_configuration (
			id INTEGER PRIMARY KEY CHECK(id = 1),
			base_rate_units INTEGER NOT NULL CHECK(base_rate_units >= 0),
			tenant_rate_units INTEGER NOT NULL CHECK(tenant_rate_units >= 0),
			debt_limit_units INTEGER NOT NULL CHECK(debt_limit_units <= 0),
			change_note TEXT,
			updated_by INTEGER,
			created_at INTEGER NOT NULL,
			updated_at INTEGER NOT NULL,
			FOREIGN KEY(updated_by) REFERENCES superusers(id) ON DELETE SET NULL
		);`,

		`CREATE TABLE IF NOT EXISTS announcements (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			title TEXT NOT NULL,
			message TEXT NOT NULL,
			created_by INTEGER,
			created_at INTEGER NOT NULL,
			updated_at INTEGER NOT NULL,
			FOREIGN KEY(created_by) REFERENCES superusers(id) ON DELETE SET NULL
		);`,

		`CREATE TABLE IF NOT EXISTS logs (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			actor_id INTEGER,
			level TEXT NOT NULL DEFAULT 'info'
				CHECK(level IN ('debug','info','warning','error','security')),
			action TEXT NOT NULL,
			entity_type TEXT,
			entity_id INTEGER,
			message TEXT NOT NULL,
			created_at INTEGER NOT NULL,
			FOREIGN KEY(actor_id) REFERENCES superusers(id) ON DELETE SET NULL
		);`,

		`CREATE TABLE IF NOT EXISTS login_sessions (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			superuser_id INTEGER NOT NULL,
			token_hash TEXT NOT NULL UNIQUE,
			expires_at INTEGER NOT NULL,
			created_at INTEGER NOT NULL,
			last_seen_at INTEGER NOT NULL DEFAULT 0,
			FOREIGN KEY(superuser_id) REFERENCES superusers(id) ON DELETE CASCADE
		);`,

		`CREATE TABLE IF NOT EXISTS password_recovery_otps (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			superuser_id INTEGER NOT NULL,
			otp_hash TEXT NOT NULL,
			reset_token_hash TEXT,
			expires_at INTEGER NOT NULL,
			verified_at INTEGER,
			consumed_at INTEGER,
			attempts INTEGER NOT NULL DEFAULT 0 CHECK(attempts >= 0),
			created_at INTEGER NOT NULL,
			updated_at INTEGER NOT NULL,
			FOREIGN KEY(superuser_id) REFERENCES superusers(id) ON DELETE CASCADE
		);`,

		`CREATE TABLE IF NOT EXISTS profile_email_otps (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			superuser_id INTEGER NOT NULL,
			new_email TEXT NOT NULL,
			otp_hash TEXT NOT NULL,
			expires_at INTEGER NOT NULL,
			verified_at INTEGER,
			consumed_at INTEGER,
			attempts INTEGER NOT NULL DEFAULT 0 CHECK(attempts >= 0),
			created_at INTEGER NOT NULL,
			updated_at INTEGER NOT NULL,
			FOREIGN KEY(superuser_id) REFERENCES superusers(id) ON DELETE CASCADE
		);`,

		`CREATE TABLE IF NOT EXISTS staff_group_messages (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			superuser_id INTEGER NOT NULL,
			message TEXT NOT NULL,
			created_at INTEGER NOT NULL,
			updated_at INTEGER NOT NULL,
			FOREIGN KEY(superuser_id) REFERENCES superusers(id) ON DELETE CASCADE
		);`,

		`CREATE UNIQUE INDEX IF NOT EXISTS idx_users_email_nocase
			ON users(email COLLATE NOCASE)
			WHERE email IS NOT NULL;`,

		`CREATE INDEX IF NOT EXISTS idx_superusers_rank
			ON superusers(rank);`,

		`CREATE INDEX IF NOT EXISTS idx_hubs_status
			ON hubs(status);`,

		`CREATE INDEX IF NOT EXISTS idx_registries_status
			ON registries(status);`,

		`CREATE INDEX IF NOT EXISTS idx_module_pairing_active
			ON module_pairing_sessions(status, expires_at);`,

		`CREATE INDEX IF NOT EXISTS idx_cards_user_id
			ON cards(user_id);`,

		`CREATE INDEX IF NOT EXISTS idx_registry_sessions_registry_id
			ON registry_sessions(registry_id);`,

		`CREATE UNIQUE INDEX IF NOT EXISTS idx_registry_one_active_session
			ON registry_sessions(registry_id)
			WHERE status = 'active';`,

		`CREATE INDEX IF NOT EXISTS idx_power_sessions_card_id
			ON power_sessions(card_id);`,

		`CREATE INDEX IF NOT EXISTS idx_power_sessions_hub_id
			ON power_sessions(hub_id);`,

		`CREATE INDEX IF NOT EXISTS idx_logs_created_at
			ON logs(created_at);`,

		`CREATE INDEX IF NOT EXISTS idx_login_sessions_presence
			ON login_sessions(expires_at, last_seen_at);`,

		`CREATE INDEX IF NOT EXISTS idx_password_recovery_superuser_created
			ON password_recovery_otps(superuser_id, created_at);`,

		`CREATE INDEX IF NOT EXISTS idx_password_recovery_active
			ON password_recovery_otps(superuser_id, expires_at, consumed_at);`,

		`CREATE INDEX IF NOT EXISTS idx_password_recovery_reset_token
			ON password_recovery_otps(reset_token_hash)
			WHERE reset_token_hash IS NOT NULL;`,

		`CREATE INDEX IF NOT EXISTS idx_profile_email_otps_superuser_created
			ON profile_email_otps(superuser_id, created_at);`,

		`CREATE INDEX IF NOT EXISTS idx_staff_group_messages_created
			ON staff_group_messages(id, created_at);`,
	}

	for _, statement := range schema {
		if _, err := tx.Exec(statement); err != nil {
			return fmt.Errorf("create schema: %w", err)
		}
	}

	if err := migrateLegacyStaffSchema(tx); err != nil {
		return err
	}
	if err := migrateSeparatedSuperusers(tx); err != nil {
		return err
	}
	if _, err := tx.Exec(`CREATE UNIQUE INDEX IF NOT EXISTS idx_superusers_email_nocase ON superusers(email COLLATE NOCASE);`); err != nil {
		return fmt.Errorf("create superuser email index: %w", err)
	}
	if err := migrateModulePairingCreatorFK(tx); err != nil {
		return err
	}
	if err := migrateActivityReferenceFKs(tx); err != nil {
		return err
	}

	// These tables belonged to older architecture and are intentionally removed.
	// Electricity usage is durable in power_sessions; staff rank is stored directly
	// on superusers; initialization no longer relies on a generic settings table.
	for _, legacyTable := range []string{"balance_transactions", "roles", "system_settings"} {
		if _, err := tx.Exec(`DROP TABLE IF EXISTS ` + legacyTable + `;`); err != nil {
			return fmt.Errorf("remove legacy table %s: %w", legacyTable, err)
		}
	}

	// Remove a stray development-only scratch table from earlier builds.
	if _, err := tx.Exec(`DROP TABLE IF EXISTS _x;`); err != nil {
		return fmt.Errorf("remove legacy scratch table: %w", err)
	}

	if err := migrateMoneyPrecision(tx); err != nil {
		return err
	}

	if err := ensureColumn(
		tx,
		"users",
		"profile_image",
		"ALTER TABLE users ADD COLUMN profile_image TEXT;",
	); err != nil {
		return err
	}

	for _, column := range []struct {
		name string
		sql  string
	}{
		{"profile_image_blob", "ALTER TABLE users ADD COLUMN profile_image_blob BLOB;"},
		{"profile_image_mime", "ALTER TABLE users ADD COLUMN profile_image_mime TEXT;"},
		{"birth_date", "ALTER TABLE users ADD COLUMN birth_date TEXT;"},
		{"gender", "ALTER TABLE users ADD COLUMN gender TEXT;"},
		{"links_json", "ALTER TABLE users ADD COLUMN links_json TEXT;"},
		{"debt_limit_units", "ALTER TABLE users ADD COLUMN debt_limit_units INTEGER;"},
	} {
		if err := ensureColumn(tx, "users", column.name, column.sql); err != nil {
			return err
		}
	}

	if err := ensureColumn(tx, "power_sessions", "rate_units", "ALTER TABLE power_sessions ADD COLUMN rate_units INTEGER;"); err != nil {
		return err
	}

	for _, table := range []string{"hubs", "registries"} {
		for _, column := range []struct {
			name string
			sql  string
		}{
			{"auth_token_hash", fmt.Sprintf("ALTER TABLE %s ADD COLUMN auth_token_hash TEXT;", table)},
			{"health_status", fmt.Sprintf("ALTER TABLE %s ADD COLUMN health_status TEXT NOT NULL DEFAULT 'offline';", table)},
			{"last_seen_at", fmt.Sprintf("ALTER TABLE %s ADD COLUMN last_seen_at INTEGER;", table)},
			{"last_warning", fmt.Sprintf("ALTER TABLE %s ADD COLUMN last_warning TEXT;", table)},
		} {
			if err := ensureColumn(tx, table, column.name, column.sql); err != nil {
				return err
			}
		}
	}

	if err := dropLegacyCardCode(tx); err != nil {
		return err
	}

	if err := ensureColumn(
		tx,
		"superusers",
		"is_head_developer",
		"ALTER TABLE superusers ADD COLUMN is_head_developer INTEGER NOT NULL DEFAULT 0;",
	); err != nil {
		return err
	}

	if err := ensureColumn(
		tx,
		"login_sessions",
		"last_seen_at",
		"ALTER TABLE login_sessions ADD COLUMN last_seen_at INTEGER NOT NULL DEFAULT 0;",
	); err != nil {
		return err
	}

	if err := ensurePriceConfigurationColumns(tx); err != nil {
		return err
	}

	if err := seedPriceConfiguration(tx, priceJSONPath); err != nil {
		return err
	}

	// Demo records are created only for a genuinely empty staff database.
	// A database reset preserves Developer accounts, so demo tenants/modules do
	// not silently return on the next server start and no system_settings table
	// is necessary.
	var staffCount int64
	if err := tx.QueryRow(`SELECT COUNT(*) FROM superusers;`).Scan(&staffCount); err != nil {
		return fmt.Errorf("inspect staff records: %w", err)
	}
	if staffCount == 0 {
		if err := seedSampleData(tx); err != nil {
			return err
		}
		if err := seedSampleTenants(tx); err != nil {
			return err
		}
		if err := seedDemoUsage(tx); err != nil {
			return err
		}
	}

	// Remove the obsolete local Owner seed when the requested Gmail Owner exists.
	if _, err := tx.Exec(`
		DELETE FROM superusers
		WHERE LOWER(email)='owner@electricredit.local'
		  AND EXISTS (SELECT 1 FROM superusers WHERE LOWER(email)='jacpano@gmail.com');
	`); err != nil {
		return fmt.Errorf("remove legacy owner account: %w", err)
	}

	// Keep one explicit Head Developer. The requested primary Developer account
	// is preferred when present; otherwise the earliest Developer becomes head.
	if _, err := tx.Exec(`
		UPDATE superusers SET is_head_developer=1
		WHERE id=(
			SELECT s.id FROM superusers s
			WHERE s.rank='developer'
			ORDER BY CASE WHEN LOWER(s.email)='jpano487023@gmail.com' THEN 0 ELSE 1 END, s.id
			LIMIT 1
		)
		AND NOT EXISTS(
			SELECT 1 FROM superusers sx
			WHERE sx.rank='developer' AND sx.is_head_developer=1
		);
	`); err != nil {
		return fmt.Errorf("ensure Head Developer: %w", err)
	}

	if err := tx.Commit(); err != nil {
		return fmt.Errorf("commit database initialization: %w", err)
	}

	if _, err := db.Exec(`PRAGMA foreign_keys = ON;`); err != nil {
		return fmt.Errorf("re-enable foreign keys: %w", err)
	}
	rows, err := db.Query(`PRAGMA foreign_key_check;`)
	if err != nil {
		return fmt.Errorf("verify foreign keys: %w", err)
	}
	defer rows.Close()
	if rows.Next() {
		return errors.New("database migration left a foreign-key violation")
	}
	if err := rows.Err(); err != nil {
		return fmt.Errorf("verify foreign keys: %w", err)
	}

	return nil
}

func tableExists(tx *sql.Tx, table string) (bool, error) {
	var count int
	if err := tx.QueryRow(`SELECT COUNT(*) FROM sqlite_master WHERE type='table' AND name=?;`, table).Scan(&count); err != nil {
		return false, err
	}
	return count > 0, nil
}

// prepareLegacyStaffRanks makes the old role_id schema readable by the new
// rank-based schema before indexes are created. It does not remove anything.
func prepareLegacyStaffRanks(tx *sql.Tx) error {
	exists, err := tableExists(tx, "superusers")
	if err != nil || !exists {
		return err
	}
	roleIDExists, err := columnExists(tx, "superusers", "role_id")
	if err != nil || !roleIDExists {
		return err
	}
	rankExists, err := columnExists(tx, "superusers", "rank")
	if err != nil {
		return err
	}
	if !rankExists {
		if _, err := tx.Exec(`ALTER TABLE superusers ADD COLUMN rank TEXT;`); err != nil {
			return fmt.Errorf("add superusers.rank: %w", err)
		}
	}
	rolesExist, err := tableExists(tx, "roles")
	if err != nil {
		return err
	}
	if rolesExist {
		if _, err := tx.Exec(`
			UPDATE superusers
			SET rank=COALESCE(NULLIF(rank,''),(SELECT name FROM roles WHERE id=superusers.role_id),'admin');
		`); err != nil {
			return fmt.Errorf("migrate staff ranks: %w", err)
		}
	} else if _, err := tx.Exec(`UPDATE superusers SET rank=COALESCE(NULLIF(rank,''),'admin');`); err != nil {
		return fmt.Errorf("initialize staff ranks: %w", err)
	}
	return nil
}

// migrateLegacyStaffSchema removes role_id without changing staff/user IDs.
// Child tables keep referencing the same superusers table name and IDs.
func migrateLegacyStaffSchema(tx *sql.Tx) error {
	exists, err := tableExists(tx, "superusers")
	if err != nil || !exists {
		return err
	}
	roleIDExists, err := columnExists(tx, "superusers", "role_id")
	if err != nil || !roleIDExists {
		return err
	}
	headExists, err := columnExists(tx, "superusers", "is_head_developer")
	if err != nil {
		return err
	}
	headExpr := "0"
	if headExists {
		headExpr = "COALESCE(is_head_developer,0)"
	}
	if _, err := tx.Exec(`DROP INDEX IF EXISTS idx_superusers_role_id;`); err != nil {
		return fmt.Errorf("drop legacy staff role index: %w", err)
	}
	if _, err := tx.Exec(`DROP INDEX IF EXISTS idx_superusers_rank;`); err != nil {
		return fmt.Errorf("prepare staff rank index: %w", err)
	}
	create := `CREATE TABLE superusers_rebuild (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		user_id INTEGER NOT NULL UNIQUE,
		rank TEXT NOT NULL CHECK(rank IN ('admin','owner','developer')),
		password_hash TEXT NOT NULL,
		is_head_developer INTEGER NOT NULL DEFAULT 0 CHECK(is_head_developer IN (0,1)),
		status TEXT NOT NULL DEFAULT 'active' CHECK(status IN ('active','inactive','disabled')),
		created_at INTEGER NOT NULL,
		updated_at INTEGER NOT NULL,
		FOREIGN KEY(user_id) REFERENCES users(id) ON DELETE CASCADE
	);`
	if _, err := tx.Exec(`DROP TABLE IF EXISTS superusers_rebuild;`); err != nil {
		return err
	}
	if _, err := tx.Exec(create); err != nil {
		return fmt.Errorf("create rank-based staff table: %w", err)
	}
	copySQL := fmt.Sprintf(`
		INSERT INTO superusers_rebuild(id,user_id,rank,password_hash,is_head_developer,status,created_at,updated_at)
		SELECT id,user_id,
		       CASE LOWER(COALESCE(NULLIF(rank,''),'admin')) WHEN 'owner' THEN 'owner' WHEN 'developer' THEN 'developer' ELSE 'admin' END,
		       password_hash,%s,status,created_at,updated_at
		FROM superusers
		ORDER BY id;`, headExpr)
	if _, err := tx.Exec(copySQL); err != nil {
		return fmt.Errorf("copy staff ranks: %w", err)
	}
	if _, err := tx.Exec(`DROP TABLE superusers;`); err != nil {
		return fmt.Errorf("replace legacy staff table: %w", err)
	}
	if _, err := tx.Exec(`ALTER TABLE superusers_rebuild RENAME TO superusers;`); err != nil {
		return fmt.Errorf("activate rank-based staff table: %w", err)
	}
	if _, err := tx.Exec(`CREATE INDEX IF NOT EXISTS idx_superusers_rank ON superusers(rank);`); err != nil {
		return fmt.Errorf("create staff rank index: %w", err)
	}
	return nil
}

// migrateSeparatedSuperusers moves staff profile/account data out of users.
// After this migration, users contains tenants only and superusers is fully
// self-contained for Administrator, Owner, and Developer accounts.
func migrateSeparatedSuperusers(tx *sql.Tx) error {
	exists, err := tableExists(tx, "superusers")
	if err != nil || !exists {
		return err
	}
	userIDExists, err := columnExists(tx, "superusers", "user_id")
	if err != nil || !userIDExists {
		return err
	}

	if _, err := tx.Exec(`DROP INDEX IF EXISTS idx_superusers_rank;`); err != nil {
		return err
	}
	if _, err := tx.Exec(`DROP INDEX IF EXISTS idx_superusers_email_nocase;`); err != nil {
		return err
	}
	if _, err := tx.Exec(`DROP TABLE IF EXISTS superusers_separated;`); err != nil {
		return err
	}
	if _, err := tx.Exec(`CREATE TABLE superusers_separated (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		first_name TEXT NOT NULL,
		last_name TEXT NOT NULL,
		email TEXT NOT NULL UNIQUE,
		phone TEXT,
		profile_image TEXT,
		profile_image_blob BLOB,
		profile_image_mime TEXT,
		birth_date TEXT,
		links_json TEXT,
		rank TEXT NOT NULL CHECK(rank IN ('admin','owner','developer')),
		password_hash TEXT NOT NULL,
		is_head_developer INTEGER NOT NULL DEFAULT 0 CHECK(is_head_developer IN (0,1)),
		status TEXT NOT NULL DEFAULT 'active' CHECK(status IN ('active','inactive','disabled')),
		created_at INTEGER NOT NULL,
		updated_at INTEGER NOT NULL
	);`); err != nil {
		return fmt.Errorf("create separated staff table: %w", err)
	}

	if _, err := tx.Exec(`INSERT INTO superusers_separated(
		id,first_name,last_name,email,phone,profile_image,profile_image_blob,profile_image_mime,birth_date,links_json,
		rank,password_hash,is_head_developer,status,created_at,updated_at)
		SELECT s.id,u.first_name,u.last_name,COALESCE(NULLIF(TRIM(u.email),''),'staff-'||s.id||'@electricredit.local'),u.phone,u.profile_image,u.profile_image_blob,u.profile_image_mime,u.birth_date,u.links_json,
		       s.rank,s.password_hash,COALESCE(s.is_head_developer,0),s.status,s.created_at,s.updated_at
		FROM superusers s JOIN users u ON u.id=s.user_id ORDER BY s.id;`); err != nil {
		return fmt.Errorf("copy separated staff profiles: %w", err)
	}

	// Pairing records historically stored users.id. Staff IDs were separately
	// auto-incremented, so explicitly map them before staff rows are removed.
	pairingExists, err := tableExists(tx, "module_pairing_sessions")
	if err != nil {
		return err
	}
	if pairingExists {
		if _, err := tx.Exec(`UPDATE module_pairing_sessions
			SET created_by=COALESCE((SELECT s.id FROM superusers s WHERE s.user_id=module_pairing_sessions.created_by),created_by);`); err != nil {
			return fmt.Errorf("map module pairing creators: %w", err)
		}
	}

	// Remove only the profile rows that belonged to the old staff accounts.
	// Tenants without RFID cards must remain valid tenant records.
	if _, err := tx.Exec(`DELETE FROM users WHERE id IN (SELECT user_id FROM superusers);`); err != nil {
		return fmt.Errorf("remove migrated staff profiles from tenants: %w", err)
	}

	if _, err := tx.Exec(`DROP TABLE superusers;`); err != nil {
		return fmt.Errorf("replace joined staff table: %w", err)
	}
	if _, err := tx.Exec(`ALTER TABLE superusers_separated RENAME TO superusers;`); err != nil {
		return err
	}
	if _, err := tx.Exec(`CREATE INDEX IF NOT EXISTS idx_superusers_rank ON superusers(rank);`); err != nil {
		return err
	}
	if _, err := tx.Exec(`CREATE UNIQUE INDEX IF NOT EXISTS idx_superusers_email_nocase ON superusers(email COLLATE NOCASE);`); err != nil {
		return err
	}

	return nil
}

// migrateModulePairingCreatorFK rebuilds the small temporary pairing table so
// created_by correctly references the now-independent superusers table.
func migrateModulePairingCreatorFK(tx *sql.Tx) error {
	exists, err := tableExists(tx, "module_pairing_sessions")
	if err != nil || !exists {
		return err
	}
	rows, err := tx.Query(`PRAGMA foreign_key_list(module_pairing_sessions);`)
	if err != nil {
		return err
	}
	needs := false
	for rows.Next() {
		var id, seq int
		var table, from, to, onUpdate, onDelete, match string
		if err := rows.Scan(&id, &seq, &table, &from, &to, &onUpdate, &onDelete, &match); err != nil {
			rows.Close()
			return err
		}
		if from == "created_by" && table != "superusers" {
			needs = true
		}
	}
	if err := rows.Err(); err != nil {
		rows.Close()
		return err
	}
	rows.Close()
	if !needs {
		return nil
	}
	if _, err := tx.Exec(`DROP TABLE IF EXISTS module_pairing_sessions_rebuild;`); err != nil {
		return err
	}
	if _, err := tx.Exec(`CREATE TABLE module_pairing_sessions_rebuild (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		session_code TEXT NOT NULL UNIQUE,
		module_type TEXT NOT NULL CHECK(module_type IN ('hub','registry')),
		module_id INTEGER NOT NULL,
		desired_status TEXT NOT NULL DEFAULT 'active' CHECK(desired_status IN ('active','inactive','disabled')),
		server_url TEXT NOT NULL,
		created_by INTEGER NOT NULL,
		status TEXT NOT NULL DEFAULT 'waiting' CHECK(status IN ('waiting','connected','expired','cancelled')),
		expires_at INTEGER NOT NULL,
		confirmed_at INTEGER,
		created_at INTEGER NOT NULL,
		updated_at INTEGER NOT NULL,
		FOREIGN KEY(created_by) REFERENCES superusers(id) ON DELETE CASCADE
	);`); err != nil {
		return err
	}
	if _, err := tx.Exec(`INSERT INTO module_pairing_sessions_rebuild SELECT * FROM module_pairing_sessions;`); err != nil {
		return err
	}
	if _, err := tx.Exec(`DROP TABLE module_pairing_sessions;`); err != nil {
		return err
	}
	if _, err := tx.Exec(`ALTER TABLE module_pairing_sessions_rebuild RENAME TO module_pairing_sessions;`); err != nil {
		return err
	}
	if _, err := tx.Exec(`CREATE INDEX IF NOT EXISTS idx_module_pairing_active ON module_pairing_sessions(status, expires_at);`); err != nil {
		return err
	}
	return nil
}

func migrateActivityReferenceFKs(tx *sql.Tx) error {
	// Usage/top-up history must survive management deletion. Older schemas used
	// restrictive foreign keys, which caused raw SQLite 787 errors when a card
	// or module had historical rows. Rebuild the three small activity tables so
	// completed history keeps its amounts/timestamps while deleted live records
	// are represented by NULL references. Active sessions are still guarded by
	// the management handlers before deletion.
	var topupSQL string
	if err := tx.QueryRow(`SELECT COALESCE(sql,'') FROM sqlite_master WHERE type='table' AND name='registry_topups';`).Scan(&topupSQL); err != nil {
		return err
	}
	var powerSQL, registrySQL string
	if err := tx.QueryRow(`SELECT COALESCE(sql,'') FROM sqlite_master WHERE type='table' AND name='power_sessions';`).Scan(&powerSQL); err != nil {
		return err
	}
	if err := tx.QueryRow(`SELECT COALESCE(sql,'') FROM sqlite_master WHERE type='table' AND name='registry_sessions';`).Scan(&registrySQL); err != nil {
		return err
	}
	ready := strings.Count(strings.ToUpper(topupSQL), "ON DELETE SET NULL") >= 2 &&
		strings.Count(strings.ToUpper(powerSQL), "ON DELETE SET NULL") >= 2 &&
		strings.Count(strings.ToUpper(registrySQL), "ON DELETE SET NULL") >= 2
	if ready {
		return nil
	}

	for _, stmt := range []string{
		`DROP TABLE IF EXISTS registry_sessions_rebuild;`,
		`CREATE TABLE registry_sessions_rebuild (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			registry_id INTEGER,
			card_id INTEGER,
			started_at INTEGER NOT NULL,
			ended_at INTEGER,
			status TEXT NOT NULL DEFAULT 'active' CHECK(status IN ('active','completed','cancelled','fault')),
			FOREIGN KEY(registry_id) REFERENCES registries(id) ON DELETE SET NULL,
			FOREIGN KEY(card_id) REFERENCES cards(id) ON DELETE SET NULL
		);`,
		`INSERT INTO registry_sessions_rebuild(id,registry_id,card_id,started_at,ended_at,status)
		 SELECT id,registry_id,card_id,started_at,ended_at,status FROM registry_sessions;`,
		`DROP TABLE registry_sessions;`,
		`ALTER TABLE registry_sessions_rebuild RENAME TO registry_sessions;`,

		`DROP TABLE IF EXISTS registry_topups_rebuild;`,
		`CREATE TABLE registry_topups_rebuild (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			operation_id TEXT NOT NULL UNIQUE,
			registry_id INTEGER,
			card_id INTEGER,
			amount_units INTEGER NOT NULL CHECK(amount_units > 0),
			created_at INTEGER NOT NULL,
			FOREIGN KEY(registry_id) REFERENCES registries(id) ON DELETE SET NULL,
			FOREIGN KEY(card_id) REFERENCES cards(id) ON DELETE SET NULL
		);`,
		`INSERT INTO registry_topups_rebuild(id,operation_id,registry_id,card_id,amount_units,created_at)
		 SELECT id,operation_id,registry_id,card_id,amount_units,created_at FROM registry_topups;`,
		`DROP TABLE registry_topups;`,
		`ALTER TABLE registry_topups_rebuild RENAME TO registry_topups;`,

		`DROP TABLE IF EXISTS power_sessions_rebuild;`,
		`CREATE TABLE power_sessions_rebuild (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			card_id INTEGER,
			hub_id INTEGER,
			started_at INTEGER NOT NULL,
			ended_at INTEGER,
			energy_wh INTEGER NOT NULL DEFAULT 0 CHECK(energy_wh >= 0),
			cost_units INTEGER NOT NULL DEFAULT 0 CHECK(cost_units >= 0),
			rate_units INTEGER,
			status TEXT NOT NULL DEFAULT 'active' CHECK(status IN ('active','completed','stopped','fault')),
			FOREIGN KEY(card_id) REFERENCES cards(id) ON DELETE SET NULL,
			FOREIGN KEY(hub_id) REFERENCES hubs(id) ON DELETE SET NULL
		);`,
		`INSERT INTO power_sessions_rebuild(id,card_id,hub_id,started_at,ended_at,energy_wh,cost_units,rate_units,status)
		 SELECT id,card_id,hub_id,started_at,ended_at,energy_wh,cost_units,rate_units,status FROM power_sessions;`,
		`DROP TABLE power_sessions;`,
		`ALTER TABLE power_sessions_rebuild RENAME TO power_sessions;`,

		`CREATE INDEX IF NOT EXISTS idx_registry_sessions_registry_id ON registry_sessions(registry_id);`,
		`CREATE UNIQUE INDEX IF NOT EXISTS idx_registry_one_active_session ON registry_sessions(registry_id) WHERE status='active' AND registry_id IS NOT NULL;`,
		`CREATE INDEX IF NOT EXISTS idx_power_sessions_card_id ON power_sessions(card_id);`,
		`CREATE INDEX IF NOT EXISTS idx_power_sessions_hub_id ON power_sessions(hub_id);`,
	} {
		if _, err := tx.Exec(stmt); err != nil {
			return fmt.Errorf("migrate activity references: %w", err)
		}
	}
	return nil
}

func ensurePriceConfigurationColumns(tx *sql.Tx) error {
	exists, err := tableExists(tx, "price_configuration")
	if err != nil || !exists {
		return err
	}
	for _, column := range []struct {
		name string
		sql  string
	}{
		{"change_note", "ALTER TABLE price_configuration ADD COLUMN change_note TEXT;"},
		{"updated_by", "ALTER TABLE price_configuration ADD COLUMN updated_by INTEGER;"},
		{"created_at", "ALTER TABLE price_configuration ADD COLUMN created_at INTEGER;"},
	} {
		if err := ensureColumn(tx, "price_configuration", column.name, column.sql); err != nil {
			return err
		}
	}
	if _, err := tx.Exec(`
		UPDATE price_configuration
		SET created_at=COALESCE(created_at,updated_at,?),
		    change_note=COALESCE(NULLIF(change_note,''),'Existing electricity rate');
	`, UnixTime()); err != nil {
		return fmt.Errorf("initialize rate metadata: %w", err)
	}
	return nil
}

func ensureColumn(
	tx *sql.Tx,
	table string,
	column string,
	alterStatement string,
) error {
	rows, err := tx.Query(fmt.Sprintf("PRAGMA table_info(%s);", table))
	if err != nil {
		return fmt.Errorf("inspect table %s: %w", table, err)
	}
	defer rows.Close()

	found := false

	for rows.Next() {
		var (
			cid        int
			name       string
			dataType   string
			notNull    int
			defaultV   any
			primaryKey int
		)

		if err := rows.Scan(
			&cid,
			&name,
			&dataType,
			&notNull,
			&defaultV,
			&primaryKey,
		); err != nil {
			return fmt.Errorf("inspect column on %s: %w", table, err)
		}

		if name == column {
			found = true
			break
		}
	}

	if err := rows.Err(); err != nil {
		return fmt.Errorf("inspect table %s: %w", table, err)
	}

	if found {
		return nil
	}

	if _, err := tx.Exec(alterStatement); err != nil {
		return fmt.Errorf("add column %s.%s: %w", table, column, err)
	}

	return nil
}

func dropLegacyCardCode(tx *sql.Tx) error {
	exists, err := columnExists(tx, "cards", "card_code")
	if err != nil || !exists {
		return err
	}
	if _, err := tx.Exec(`DROP INDEX IF EXISTS idx_cards_card_code;`); err != nil {
		return fmt.Errorf("drop legacy card identifier index: %w", err)
	}
	if _, err := tx.Exec(`ALTER TABLE cards DROP COLUMN card_code;`); err != nil {
		return fmt.Errorf("remove legacy card identifier column: %w", err)
	}
	return nil
}

func seedPriceConfiguration(tx *sql.Tx, path string) error {
	seed := priceSeed{
		BaseRate:   "13.50000",
		TenantRate: "13.50000",
		DebtLimit:  "-100.00000",
	}

	if path != "" {
		data, err := os.ReadFile(path)
		if err != nil && !errors.Is(err, os.ErrNotExist) {
			return fmt.Errorf("read price configuration: %w", err)
		}

		if err == nil {
			if err := json.Unmarshal(data, &seed); err != nil {
				return fmt.Errorf("decode price configuration: %w", err)
			}
		}
	}

	// Backward compatibility for older price.json files that used centavos.
	if seed.BaseRate == "" && seed.PricePerKWhCentavos != nil {
		seed.BaseRate = FormatMoneyUnits(*seed.PricePerKWhCentavos * 1000)
	}
	if seed.TenantRate == "" {
		seed.TenantRate = seed.BaseRate
	}
	if seed.DebtLimit == "" && seed.DebtLimitCentavos != nil {
		seed.DebtLimit = FormatMoneyUnits(*seed.DebtLimitCentavos * 1000)
	}

	baseRate, err := ParseMoneyUnits(seed.BaseRate)
	if err != nil || baseRate < 0 {
		return errors.New("base_rate must be a non-negative peso amount with at most 5 decimals")
	}
	tenantRate, err := ParseMoneyUnits(seed.TenantRate)
	if err != nil || tenantRate < 0 {
		return errors.New("tenant_rate must be a non-negative peso amount with at most 5 decimals")
	}
	debtLimit, err := ParseMoneyUnits(seed.DebtLimit)
	if err != nil || debtLimit > 0 {
		return errors.New("debt_limit must be zero or negative with at most 5 decimals")
	}

	_, err = tx.Exec(
		`INSERT INTO price_configuration(
			id,
			base_rate_units,
			tenant_rate_units,
			debt_limit_units,
			change_note,
			created_at,
			updated_at
		)
		VALUES(1, ?, ?, ?, 'Initial electricity rate', ?, ?)
		ON CONFLICT(id) DO NOTHING;`,
		baseRate,
		tenantRate,
		debtLimit,
		UnixTime(),
		UnixTime(),
	)
	if err != nil {
		return fmt.Errorf("seed price configuration: %w", err)
	}

	return nil
}

func migrateMoneyPrecision(tx *sql.Tx) error {
	migrations := []struct {
		table     string
		oldColumn string
		newColumn string
	}{
		{"cards", "balance_centavos", "balance_units"},
		{"power_sessions", "cost_centavos", "cost_units"},
		{"price_configuration", "price_per_kwh_centavos", "base_rate_units"},
		{"price_configuration", "debt_limit_centavos", "debt_limit_units"},
	}

	for _, migration := range migrations {
		oldExists, err := columnExists(tx, migration.table, migration.oldColumn)
		if err != nil {
			return err
		}
		if !oldExists {
			continue
		}

		newExists, err := columnExists(tx, migration.table, migration.newColumn)
		if err != nil {
			return err
		}
		if newExists {
			return fmt.Errorf("money migration found both %s.%s and %s.%s", migration.table, migration.oldColumn, migration.table, migration.newColumn)
		}

		if _, err := tx.Exec(fmt.Sprintf(
			"ALTER TABLE %s RENAME COLUMN %s TO %s;",
			migration.table,
			migration.oldColumn,
			migration.newColumn,
		)); err != nil {
			return fmt.Errorf("rename money column %s.%s: %w", migration.table, migration.oldColumn, err)
		}

		if _, err := tx.Exec(fmt.Sprintf(
			"UPDATE %s SET %s = %s * 1000;",
			migration.table,
			migration.newColumn,
			migration.newColumn,
		)); err != nil {
			return fmt.Errorf("upgrade money precision for %s.%s: %w", migration.table, migration.newColumn, err)
		}
	}

	tenantRateExists, err := columnExists(tx, "price_configuration", "tenant_rate_units")
	if err != nil {
		return err
	}
	if !tenantRateExists {
		if _, err := tx.Exec(`ALTER TABLE price_configuration ADD COLUMN tenant_rate_units INTEGER NOT NULL DEFAULT 0;`); err != nil {
			return fmt.Errorf("add price_configuration.tenant_rate_units: %w", err)
		}
		if _, err := tx.Exec(`UPDATE price_configuration SET tenant_rate_units = base_rate_units;`); err != nil {
			return fmt.Errorf("initialize tenant rate: %w", err)
		}
	}

	return nil
}

func columnExists(tx *sql.Tx, table string, column string) (bool, error) {
	rows, err := tx.Query(fmt.Sprintf("PRAGMA table_info(%s);", table))
	if err != nil {
		return false, fmt.Errorf("inspect table %s: %w", table, err)
	}
	defer rows.Close()

	for rows.Next() {
		var (
			cid        int
			name       string
			dataType   string
			notNull    int
			defaultV   any
			primaryKey int
		)
		if err := rows.Scan(&cid, &name, &dataType, &notNull, &defaultV, &primaryKey); err != nil {
			return false, fmt.Errorf("inspect column on %s: %w", table, err)
		}
		if name == column {
			return true, nil
		}
	}
	if err := rows.Err(); err != nil {
		return false, fmt.Errorf("inspect table %s: %w", table, err)
	}
	return false, nil
}

type sampleSuperuser struct {
	Rank         string
	FirstName    string
	LastName     string
	Email        string
	PasswordHash string
}

func seedSampleData(tx *sql.Tx) error {
	now := UnixTime()

	registries := []struct {
		MAC      string
		Name     string
		Location string
		Status   string
	}{
		{"02:EC:10:00:00:01", "Registry Station 01", "Main Registry Area", "active"},
		{"02:EC:10:00:00:02", "Registry Station 02", "Secondary Registry Area", "active"},
	}

	for _, registry := range registries {
		if _, err := tx.Exec(`
			INSERT INTO registries(
				mac_address, name, location, status, created_at, updated_at
			)
			VALUES(?, ?, ?, ?, ?, ?)
			ON CONFLICT(mac_address) DO UPDATE SET
				name = excluded.name,
				location = excluded.location;
		`, registry.MAC, registry.Name, registry.Location, registry.Status, now, now); err != nil {
			return fmt.Errorf("seed registry %s: %w", registry.Name, err)
		}
	}

	// One initial Registry session makes the demo show the derived "In Use"
	// state. It is inserted only when there has never been a Registry session.
	if _, err := tx.Exec(`
		INSERT INTO registry_sessions(registry_id, started_at, status)
		SELECT id, ?, 'active'
		FROM registries
		WHERE mac_address = '02:EC:10:00:00:01'
		  AND NOT EXISTS (SELECT 1 FROM registry_sessions);
	`, now); err != nil {
		return fmt.Errorf("seed Registry demo session: %w", err)
	}

	hubs := []struct {
		MAC      string
		Name     string
		Location string
		Status   string
	}{
		{"02:EC:20:00:00:01", "Hub 01", "Room A-01", "active"},
		{"02:EC:20:00:00:02", "Hub 02", "Room A-02", "active"},
		{"02:EC:20:00:00:03", "Hub 03", "Room B-01", "active"},
		{"02:EC:20:00:00:04", "Hub 04", "Room B-02", "active"},
		{"02:EC:20:00:00:05", "Hub 05", "Room C-01", "disabled"},
	}

	for _, hub := range hubs {
		if _, err := tx.Exec(`
			INSERT INTO hubs(
				mac_address, name, location, status, created_at, updated_at
			)
			VALUES(?, ?, ?, ?, ?, ?)
			ON CONFLICT(mac_address) DO UPDATE SET
				name = excluded.name,
				location = excluded.location;
		`, hub.MAC, hub.Name, hub.Location, hub.Status, now, now); err != nil {
			return fmt.Errorf("seed hub %s: %w", hub.Name, err)
		}
	}

	// Owner accounts are seeded in the explicit development account list below.

	accounts := []sampleSuperuser{
		{
			Rank: "developer", FirstName: "Joshane Rhea", LastName: "Paquibot",
			Email:        "joshane@electricredit.local",
			PasswordHash: "pbkdf2_sha256$180000$VwsG27dChxv+iWyMVE6c6A$BKE3BJtXGXwsmdw+4pEt//rt+/TzmcuJ71DbGf5weWk",
		},
		{
			Rank: "developer", FirstName: "Jellen", LastName: "Anos",
			Email:        "jellen@electricredit.local",
			PasswordHash: "pbkdf2_sha256$180000$yBrKYN1A2af5O72NsI8ZWw$I9KIY4mTrNOwvkgw3FVYk9p2zlRATAkCFrwGZOdFWlw",
		},
		{
			Rank: "developer", FirstName: "Joselito Jr.", LastName: "Tambacan",
			Email:        "joselito@electricredit.local",
			PasswordHash: "pbkdf2_sha256$180000$Zkv2rRJlGE8dpSARMRY77w$yZQOf2TpyWt/Bbg8ex95KQKH1lI94i8yRiKoAjP3axw",
		},
		{
			Rank: "developer", FirstName: "Jaycob", LastName: "Lumayag",
			Email:        "jaycob@electricredit.local",
			PasswordHash: "pbkdf2_sha256$180000$aZz3Y0QglS3eoOdzgzZ/yg$Ag7e8lxZgGQEdXVKfeE24HA5yc54c9KwHILg9CGl2jk",
		},
		{
			Rank: "developer", FirstName: "Group45", LastName: "Developer",
			Email:        "developer5@electricredit.local",
			PasswordHash: "pbkdf2_sha256$180000$WZlX6ZfhHQQZdi99xEin7Q$ADfjbBj+YChN8nEnUryrXk70m/wrkvayEEdYR2iTXfk",
		},
		{
			Rank: "admin", FirstName: "ElectriCredit", LastName: "Admin One",
			Email:        "admin1@electricredit.local",
			PasswordHash: "pbkdf2_sha256$180000$SKS7QEc5Uq7ciMJbEpbatw$l8BeMyUG9aX71hnNnQ6BV4hHK/7pWnSG7O8q+CRPQ9w",
		},
		{
			Rank: "admin", FirstName: "ElectriCredit", LastName: "Admin Two",
			Email:        "admin2@electricredit.local",
			PasswordHash: "pbkdf2_sha256$180000$7acaJwsBbDjkcFc5zkTQTA$Kod3Ky2miMEBMNE+/sjOeJW9crmhzaSVnwvA5YG6ujs",
		},
		// Temporary testing accounts requested for local development.
		// They are inserted only when missing, so changing their passwords later
		// through account recovery will not be undone on server restart.
		{
			Rank: "developer", FirstName: "Jhon Anthony", LastName: "Catamco Pano",
			Email:        "jpano487023@gmail.com",
			PasswordHash: "pbkdf2_sha256$180000$SNL+SKL8V6a6rEdOjKq+0A$u8qhjO7OcWTHbIX22AFv3JI6a949Qwm3n4oHGj0zIEM",
		},
		{
			Rank: "owner", FirstName: "Jhon Anthony", LastName: "Catamco Pano",
			Email:        "jacpano@gmail.com",
			PasswordHash: "pbkdf2_sha256$180000$/G2Styi+syAyXcEukY8F/A$6KkjScWruTODRQHvwoWTKHpnpGLZ6uNM9zBBZ8ZOU5A",
		},
		{
			Rank: "admin", FirstName: "ElectriCredit", LastName: "Test Admin",
			Email:        "chesternovocha@gmail.com",
			PasswordHash: "pbkdf2_sha256$180000$q56VoHwCkJUXAjj3V57h/w$v1rwcYgr2yLvYHjtlNZYlHU5G3BrVdcRAg06U3Exu7E",
		},
	}

	for _, account := range accounts {
		if err := seedSampleSuperuser(tx, account); err != nil {
			return err
		}
	}

	return nil
}

func seedSampleTenants(tx *sql.Tx) error {
	type cardSeed struct {
		UID     string
		Balance string
		Status  string
	}
	type tenantSeed struct {
		FirstName string
		LastName  string
		Email     string
		Phone     string
		BirthDate string
		Gender    string
		DebtLimit string
		Cards     []cardSeed
	}

	tenants := []tenantSeed{
		{"Maria", "Santos", "tenant01@electricredit.local", "+639982104001", "1999-04-15", "female", "-100.00000", []cardSeed{{"04:EC:01:00:00:01", "125.45000", "active"}, {"04:EC:01:00:00:11", "42.10550", "active"}}},
		{"Carlo", "Reyes", "tenant02@electricredit.local", "+639982104002", "2000-08-21", "male", "-50.00000", []cardSeed{{"04:EC:01:00:00:02", "-3.00012", "active"}}},
		{"Andrea", "Cruz", "tenant03@electricredit.local", "+639982104003", "2001-11-03", "female", "-75.00000", []cardSeed{{"04:EC:01:00:00:03", "50.00000", "active"}, {"04:EC:01:00:00:13", "18.75000", "inactive"}}},
		{"Miguel", "Dela Pena", "tenant04@electricredit.local", "+639982104004", "1998-02-10", "male", "-120.00000", []cardSeed{{"04:EC:01:00:00:04", "0.00012", "active"}}},
		{"Lea", "Navarro", "tenant05@electricredit.local", "+639982104005", "2002-06-28", "female", "-25.00000", []cardSeed{{"04:EC:01:00:00:05", "300.25050", "active"}, {"04:EC:01:00:00:15", "88.12500", "active"}, {"04:EC:01:00:00:25", "12.00000", "blocked"}}},
	}

	now := UnixTime()
	for _, tenant := range tenants {
		debtLimit, err := ParseMoneyUnits(tenant.DebtLimit)
		if err != nil {
			return fmt.Errorf("parse demo tenant debt limit: %w", err)
		}

		if _, err := tx.Exec(`
			INSERT OR IGNORE INTO users(
				first_name,last_name,email,phone,room,profile_image,birth_date,gender,debt_limit_units,status,created_at,updated_at
			) VALUES(?,?,?,?,NULL,'assets/default-image.png',?,?,?,'active',?,?);
		`, tenant.FirstName, tenant.LastName, tenant.Email, tenant.Phone, tenant.BirthDate, tenant.Gender, debtLimit, now, now); err != nil {
			return fmt.Errorf("seed demo tenant %s: %w", tenant.Email, err)
		}

		if _, err := tx.Exec(`
			UPDATE users SET phone=?,room=NULL,
				birth_date=COALESCE(NULLIF(birth_date,''),?),
				gender=COALESCE(NULLIF(gender,''),?),
				debt_limit_units=COALESCE(debt_limit_units,?),updated_at=?
			WHERE LOWER(email)=LOWER(?);
		`, tenant.Phone, tenant.BirthDate, tenant.Gender, debtLimit, now, tenant.Email); err != nil {
			return fmt.Errorf("refresh demo tenant %s: %w", tenant.Email, err)
		}

		var userID int64
		if err := tx.QueryRow(`SELECT id FROM users WHERE LOWER(email)=LOWER(?) LIMIT 1;`, tenant.Email).Scan(&userID); err != nil {
			return fmt.Errorf("find demo tenant %s: %w", tenant.Email, err)
		}

		for _, card := range tenant.Cards {
			balance, err := ParseMoneyUnits(card.Balance)
			if err != nil {
				return fmt.Errorf("parse demo card balance: %w", err)
			}
			if _, err := tx.Exec(`
				INSERT OR IGNORE INTO cards(uid,user_id,balance_units,status,created_at,updated_at)
				VALUES(?,?,?,?,?,?);
			`, card.UID, userID, balance, card.Status, now, now); err != nil {
				return fmt.Errorf("seed demo tenant card %s: %w", card.UID, err)
			}
		}
	}

	return nil
}

func seedDemoUsage(tx *sql.Tx) error {
	var existing int64
	if err := tx.QueryRow(`SELECT COUNT(*) FROM power_sessions;`).Scan(&existing); err != nil {
		return fmt.Errorf("inspect demo usage: %w", err)
	}
	if existing > 0 {
		return nil
	}

	var tenantRate int64
	if err := tx.QueryRow(`SELECT tenant_rate_units FROM price_configuration WHERE id=1;`).Scan(&tenantRate); err != nil {
		return fmt.Errorf("load tenant rate for demo usage: %w", err)
	}

	type usageSeed struct {
		UID      string
		HubMAC   string
		HoursAgo int64
		Minutes  int64
		EnergyWh int64
		Status   string
	}
	usage := []usageSeed{
		{"04:EC:01:00:00:01", "02:EC:20:00:00:01", 318, 62, 810, "completed"},
		{"04:EC:01:00:00:02", "02:EC:20:00:00:02", 291, 48, 530, "completed"},
		{"04:EC:01:00:00:03", "02:EC:20:00:00:03", 266, 85, 1240, "completed"},
		{"04:EC:01:00:00:11", "02:EC:20:00:00:01", 244, 31, 280, "completed"},
		{"04:EC:01:00:00:05", "02:EC:20:00:00:04", 219, 92, 1510, "completed"},
		{"04:EC:01:00:00:04", "02:EC:20:00:00:03", 198, 44, 390, "completed"},
		{"04:EC:01:00:00:13", "02:EC:20:00:00:02", 171, 55, 670, "completed"},
		{"04:EC:01:00:00:15", "02:EC:20:00:00:04", 148, 73, 920, "completed"},
		{"04:EC:01:00:00:01", "02:EC:20:00:00:01", 123, 57, 760, "completed"},
		{"04:EC:01:00:00:02", "02:EC:20:00:00:02", 101, 34, 310, "completed"},
		{"04:EC:01:00:00:03", "02:EC:20:00:00:03", 76, 69, 1100, "completed"},
		{"04:EC:01:00:00:05", "02:EC:20:00:00:04", 52, 88, 1430, "completed"},
		{"04:EC:01:00:00:11", "02:EC:20:00:00:01", 27, 40, 470, "completed"},
		{"04:EC:01:00:00:15", "02:EC:20:00:00:04", 8, 51, 690, "completed"},
		{"04:EC:01:00:00:01", "02:EC:20:00:00:01", 1, 0, 145, "active"},
		{"04:EC:01:00:00:03", "02:EC:20:00:00:03", 0, 0, 92, "active"},
	}

	now := UnixTime()
	for index, item := range usage {
		var cardID, hubID int64
		if err := tx.QueryRow(`SELECT id FROM cards WHERE uid=?;`, item.UID).Scan(&cardID); err != nil {
			return fmt.Errorf("find demo usage card %s: %w", item.UID, err)
		}
		if err := tx.QueryRow(`SELECT id FROM hubs WHERE mac_address=?;`, item.HubMAC).Scan(&hubID); err != nil {
			return fmt.Errorf("find demo usage hub %s: %w", item.HubMAC, err)
		}
		started := now - item.HoursAgo*3600
		var ended any
		if item.Status == "active" {
			ended = nil
		} else {
			ended = started + item.Minutes*60
		}
		cost := item.EnergyWh * tenantRate / 1000
		result, err := tx.Exec(`
			INSERT INTO power_sessions(card_id,hub_id,started_at,ended_at,energy_wh,cost_units,status)
			VALUES(?,?,?,?,?,?,?);
		`, cardID, hubID, started, ended, item.EnergyWh, cost, item.Status)
		if err != nil {
			return fmt.Errorf("seed demo power session %d: %w", index+1, err)
		}
		_, _ = result.LastInsertId()
	}
	return nil
}

func seedSampleSuperuser(tx *sql.Tx, account sampleSuperuser) error {
	account.Email = strings.ToLower(strings.TrimSpace(account.Email))
	now := UnixTime()
	image := sampleDeveloperImage(account.FirstName, account.LastName)
	if _, err := tx.Exec(`
		INSERT INTO superusers(
			first_name,last_name,email,profile_image,rank,password_hash,status,created_at,updated_at
		) VALUES(?,?,?,?,?,?,'active',?,?)
		ON CONFLICT(email) DO NOTHING;
	`, account.FirstName, account.LastName, account.Email, image, account.Rank, account.PasswordHash, now, now); err != nil {
		return fmt.Errorf("seed sample superuser %s: %w", account.Email, err)
	}
	// Apply bundled Developer photos only while an account still uses the generic fallback.
	if account.Rank == "developer" && image != "assets/default-image.png" {
		_, _ = tx.Exec(`UPDATE superusers SET profile_image=? WHERE LOWER(email)=? AND (profile_image IS NULL OR profile_image='' OR profile_image='assets/default-image.png') AND (profile_image_blob IS NULL OR length(profile_image_blob)=0);`, image, account.Email)
	}
	return nil
}

func sampleDeveloperImage(first, last string) string {
	switch strings.ToLower(strings.TrimSpace(first + " " + last)) {
	case "joshane rhea paquibot":
		return "assets/developers/Joshane Rhea Paquibot.jpg"
	case "jellen anos":
		return "assets/developers/Jellen Anos.jpg"
	case "joselito jr. tambacan":
		return "assets/developers/Joselito Jr. Tambacan.jpg"
	case "jaycob lumayag":
		return "assets/developers/Jaycob Lumayag.jpg"
	case "jhon anthony catamco pano":
		return "assets/developers/Jhon Anthony Pano.jpg"
	default:
		return "assets/default-image.png"
	}
}

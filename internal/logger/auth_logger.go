package logger

import (
	"database/sql"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"time"

	_ "modernc.org/sqlite"
	tele "gopkg.in/telebot.v3"
)

var authDB *sql.DB

// InitAuthLogger initializes the SQLite database for authentication logs.
// It creates the logs dir, access.db file, and the required table.
func InitAuthLogger() error {
	if err := os.MkdirAll(logDir, 0755); err != nil {
		return fmt.Errorf("failed to create log dir for auth DB: %w", err)
	}

	dbPath := filepath.Join(logDir, "access.db")
	db, err := sql.Open("sqlite", dbPath)
	if err != nil {
		return fmt.Errorf("failed to open auth sqlite db: %w", err)
	}

	// Optimize for mobile storage I/O
	if _, err := db.Exec("PRAGMA journal_mode=WAL;"); err != nil {
		slog.Warn("Failed to set WAL mode on auth DB", "error", err)
	}

	// Create table if not exists
	createTableQuery := `
	CREATE TABLE IF NOT EXISTS access_logs (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		user_id INTEGER,
		username TEXT,
		command_text TEXT,
		attempt_at DATETIME
	);
	`
	if _, err := db.Exec(createTableQuery); err != nil {
		return fmt.Errorf("failed to create access_logs table: %w", err)
	}

	authDB = db
	slog.Info("Auth logger (SQLite) initialized successfully", "path", dbPath)
	return nil
}

// CloseAuthLogger cleanly closes the SQLite database connection.
func CloseAuthLogger() {
	if authDB != nil {
		if err := authDB.Close(); err != nil {
			slog.Error("Error closing Auth DB", "error", err)
		} else {
			slog.Info("Auth DB closed successfully.")
		}
	}
}

// LogUnauthorized records an unauthorized access attempt to the database.
// As per Architect's Note, we return no errors to ensure a "Silent Drop" strategy, 
// just logging the failure internally.
func LogUnauthorized(u *tele.User, msg string) {
	if authDB == nil {
		slog.Error("Auth DB is not initialized. Cannot log unauthorized try.")
		return
	}

	if u == nil {
		slog.Warn("LogUnauthorized called with nil User")
		return
	}

	query := `INSERT INTO access_logs (user_id, username, command_text, attempt_at) VALUES (?, ?, ?, ?)`
	
	username := u.Username
	if username == "" {
		username = fmt.Sprintf("%s %s", u.FirstName, u.LastName)
	}
	
	// Ensure Attempt_At uses UTC+9 (KST)
	kst := time.FixedZone("KST", 9*3600)
	now := time.Now().In(kst)

	_, err := authDB.Exec(query, u.ID, username, msg, now)
	if err != nil {
		slog.Error("Failed to inert unauthorized log to DB", "error", err, "user_id", u.ID)
	}
}

// DailyAuthStats represents the daily aggregation of unauthorized attempts.
type DailyAuthStats struct {
	TotalAttempts int
	TopIntruders  []string
}

// GetDailyStats queries the total attempts and top intruders for a given date.
func GetDailyStats(targetDate time.Time) (DailyAuthStats, error) {
	stats := DailyAuthStats{TotalAttempts: 0, TopIntruders: []string{}}
	
	if authDB == nil {
		return stats, fmt.Errorf("Auth DB is not initialized")
	}

	kst := time.FixedZone("KST", 9*3600)
	target := targetDate.In(kst)
	
	// SQLite date formatting: attempt_at is stored natively via go-sqlite datetime binder.
	// We'll filter by string prefixes of the YYYY-MM-DD if string storage was used, 
	// but DATETIME types in SQLite are easiest to query using date()
	dateStr := target.Format("2006-01-02")
	
	// Query total attempts
	totalQuery := `SELECT COUNT(*) FROM access_logs WHERE date(attempt_at) = ?`
	if err := authDB.QueryRow(totalQuery, dateStr).Scan(&stats.TotalAttempts); err != nil {
		return stats, fmt.Errorf("failed counting total attempts: %w", err)
	}

	// Query top 3 intruders
	topQuery := `
		SELECT user_id, COUNT(*) as cnt 
		FROM access_logs 
		WHERE date(attempt_at) = ? 
		GROUP BY user_id 
		ORDER BY cnt DESC 
		LIMIT 3
	`
	rows, err := authDB.Query(topQuery, dateStr)
	if err != nil {
		return stats, fmt.Errorf("failed fetching top intruders: %w", err)
	}
	defer rows.Close()

	for rows.Next() {
		var uid int64
		var count int
		if err := rows.Scan(&uid, &count); err == nil {
			stats.TopIntruders = append(stats.TopIntruders, fmt.Sprintf("ID: %d (%d times)", uid, count))
		}
	}

	return stats, nil
}

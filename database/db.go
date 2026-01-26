package database

import (
	"database/sql"
	"os"
	"path/filepath"

	_ "github.com/mattn/go-sqlite3"
)

func dbPath() string {
	if p := os.Getenv("DB_PATH"); p != "" {
		return p
	}
	return filepath.Join("data", "app.db")
}

func Open() (*sql.DB, error) {
	path := dbPath()
	_ = os.MkdirAll(filepath.Dir(path), 0755)

	db, err := sql.Open("sqlite3", path)
	if err != nil {
		return nil, err
	}

	_, _ = db.Exec("PRAGMA journal_mode=WAL;")
	_, _ = db.Exec("PRAGMA synchronous=NORMAL;")

	// students
	_, err = db.Exec(`
CREATE TABLE IF NOT EXISTS students (
	chat_id INTEGER PRIMARY KEY,
	name TEXT NOT NULL
);`)
	if err != nil {
		_ = db.Close()
		return nil, err
	}

	// teachers
	_, err = db.Exec(`
CREATE TABLE IF NOT EXISTS teachers (
	id INTEGER PRIMARY KEY AUTOINCREMENT,
	login TEXT NOT NULL UNIQUE,
	password_hash TEXT NOT NULL,
	chat_id INTEGER,
	is_primary INTEGER NOT NULL DEFAULT 0
);`)
	if err != nil {
		_ = db.Close()
		return nil, err
	}

	// appointments (записи)
	_, err = db.Exec(`
CREATE TABLE IF NOT EXISTS appointments (
	id INTEGER PRIMARY KEY AUTOINCREMENT,
	student_chat_id INTEGER NOT NULL,
	student_name TEXT NOT NULL,
	start_ts INTEGER NOT NULL,
	end_ts INTEGER NOT NULL,
	duration_min INTEGER NOT NULL,
	created_ts INTEGER NOT NULL
);`)
	if err != nil {
		_ = db.Close()
		return nil, err
	}

	// индексы на appointments
	_, err = db.Exec(`CREATE INDEX IF NOT EXISTS idx_appointments_start ON appointments(start_ts);`)
	if err != nil {
		_ = db.Close()
		return nil, err
	}
	_, err = db.Exec(`CREATE INDEX IF NOT EXISTS idx_appointments_student ON appointments(student_chat_id, start_ts);`)
	if err != nil {
		_ = db.Close()
		return nil, err
	}

	// user_settings (настройки напоминаний)
	_, err = db.Exec(`
CREATE TABLE IF NOT EXISTS user_settings (
	chat_id INTEGER PRIMARY KEY,
	reminders_enabled INTEGER NOT NULL DEFAULT 1,
	remind_before_min INTEGER NOT NULL DEFAULT 60
);`)
	if err != nil {
		_ = db.Close()
		return nil, err
	}

	// reminders (очередь напоминаний)
	_, err = db.Exec(`
CREATE TABLE IF NOT EXISTS reminders (
	id INTEGER PRIMARY KEY AUTOINCREMENT,
	appointment_id INTEGER NOT NULL,
	recipient_chat_id INTEGER NOT NULL,
	send_at_ts INTEGER NOT NULL,
	sent_ts INTEGER,
	kind TEXT NOT NULL
);`)
	if err != nil {
		_ = db.Close()
		return nil, err
	}

	_, err = db.Exec(`CREATE INDEX IF NOT EXISTS idx_reminders_due ON reminders(sent_ts, send_at_ts);`)
	if err != nil {
		_ = db.Close()
		return nil, err
	}

	return db, nil
}

package database

import "database/sql"

func GetStudentName(db *sql.DB, chatID int64) (string, bool, error) {
	var name string
	err := db.QueryRow(`SELECT name FROM students WHERE chat_id = ?`, chatID).Scan(&name)
	if err == sql.ErrNoRows {
		return "", false, nil
	}
	if err != nil {
		return "", false, err
	}
	return name, true, nil
}

func UpsertStudentName(db *sql.DB, chatID int64, name string) error {
	_, err := db.Exec(`
	INSERT INTO students(chat_id, name) VALUES(?, ?)
	ON CONFLICT(chat_id) DO UPDATE SET name = excluded.name
	`, chatID, name)
	return err
}

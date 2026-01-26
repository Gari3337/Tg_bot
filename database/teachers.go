package database

import (
	"database/sql"
)

type Teacher struct {
	ID           int64
	Login        string
	PasswordHash string
}

func GetTeacherByLogin(db *sql.DB, login string) (Teacher, bool, error) {
	var t Teacher
	err := db.QueryRow(
		`SELECT id, login, password_hash FROM teachers WHERE login = ?`,
		login,
	).Scan(&t.ID, &t.Login, &t.PasswordHash)

	if err == sql.ErrNoRows {
		return Teacher{}, false, nil
	}
	if err != nil {
		return Teacher{}, false, err
	}
	return t, true, nil
}

func UpsertTeacher(db *sql.DB, login, passwordHash string) error {
	_, err := db.Exec(`
		INSERT INTO teachers(login, password_hash) VALUES(?, ?)
		ON CONFLICT(login) DO UPDATE SET password_hash = excluded.password_hash
	`, login, passwordHash)
	return err
}

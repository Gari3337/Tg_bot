package database

import "database/sql"

func ClearStudentsAndAppointments(db *sql.DB) error {
	_, err := db.Exec(`
		DELETE FROM students;
		DELETE FROM appointments;
		DELETE FROM reminders;
		DELETE FROM sqlite_sequence WHERE name IN ('appointments','reminders');
	`)
	return err
}

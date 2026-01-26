package database

import (
	"context"
	"database/sql"
	"errors"
	"time"
)

var ErrSlotBusy = errors.New("slot busy")

type Appointment struct {
	ID            int64
	StudentChatID int64
	StudentName   string
	StartTS       int64
	EndTS         int64
	DurationMin   int
	CreatedTS     int64
}

// GetAppointmentsByDay возвращает все записи на конкретный день (по локальному времени)
func GetAppointmentsByDay(db *sql.DB, dayStartTS int64, dayEndTS int64) ([]Appointment, error) {
	rows, err := db.Query(`
		SELECT id, student_chat_id, student_name, start_ts, end_ts, duration_min, created_ts
		FROM appointments
		WHERE start_ts >= ? AND start_ts < ?
		ORDER BY start_ts
	`, dayStartTS, dayEndTS)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var res []Appointment
	for rows.Next() {
		var a Appointment
		if err := rows.Scan(
			&a.ID,
			&a.StudentChatID,
			&a.StudentName,
			&a.StartTS,
			&a.EndTS,
			&a.DurationMin,
			&a.CreatedTS,
		); err != nil {
			return nil, err
		}
		res = append(res, a)
	}
	return res, nil
}

// DeleteAppointmentByIDTeacher удаляет запись по id (для преподавателя — без проверки владельца)
func DeleteAppointmentByIDTeacher(db *sql.DB, id int64) error {
	res, err := db.Exec(`
		DELETE FROM appointments
		WHERE id = ?
	`, id)
	if err != nil {
		return err
	}
	n, err := res.RowsAffected()
	if err != nil {
		return err
	}
	if n == 0 {
		return errors.New("appointment not found")
	}
	return nil
}

// CreateAppointmentTx атомарно:
// 1) блокирует запись (BEGIN IMMEDIATE)
// 2) проверяет пересечение интервалов
// 3) вставляет запись если свободно
func CreateAppointmentTx(db *sql.DB, studentChatID int64, studentName string, startTS int64, durationMin int) (int64, error) {
	if durationMin != 60 && durationMin != 90 {
		return 0, errors.New("invalid duration")
	}
	endTS := startTS + int64(durationMin)*60
	createdTS := time.Now().Unix()

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	tx, err := db.BeginTx(ctx, &sql.TxOptions{Isolation: sql.LevelSerializable})
	if err != nil {
		return 0, err
	}
	defer func() { _ = tx.Rollback() }()

	// ✅ ВАЖНО: BEGIN IMMEDIATE — чтобы второй конкурент ждал/не проскочил
	//_, err = tx.ExecContext(ctx, "BEGIN IMMEDIATE;")
	//if err != nil {
	//	return 0, err
	//}

	// ✅ Проверяем пересечение интервалов
	var cnt int
	err = tx.QueryRowContext(ctx, `
		SELECT COUNT(1)
		FROM appointments
		WHERE start_ts < ? AND end_ts > ?;
	`, endTS, startTS).Scan(&cnt)
	if err != nil {
		return 0, err
	}

	if cnt > 0 {
		return 0, ErrSlotBusy
	}

	// ✅ Вставляем запись
	res, err := tx.ExecContext(ctx, `
		INSERT INTO appointments (student_chat_id, student_name, start_ts, end_ts, duration_min, created_ts)
		VALUES (?, ?, ?, ?, ?, ?);
	`, studentChatID, studentName, startTS, endTS, durationMin, createdTS)
	if err != nil {
		return 0, err
	}

	id, err := res.LastInsertId()
	if err != nil {
		return 0, err
	}

	// ✅ Фиксируем
	if err := tx.Commit(); err != nil {
		return 0, err
	}
	return id, nil
}

// GetFutureAppointments возвращает будущие записи ученика (для выбора отмены)
func GetFutureAppointments(db *sql.DB, chatID int64) ([]Appointment, error) {
	rows, err := db.Query(`
		SELECT id, student_chat_id, student_name, start_ts, end_ts, duration_min, created_ts
		FROM appointments
		WHERE student_chat_id = ?
		  AND start_ts > ?
		ORDER BY start_ts
	`, chatID, time.Now().Unix())
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var res []Appointment
	for rows.Next() {
		var a Appointment
		if err := rows.Scan(
			&a.ID,
			&a.StudentChatID,
			&a.StudentName,
			&a.StartTS,
			&a.EndTS,
			&a.DurationMin,
			&a.CreatedTS,
		); err != nil {
			return nil, err
		}
		res = append(res, a)
	}
	return res, nil
}

// DeleteAppointmentByID удаляет запись по id, но только если она принадлежит этому ученику
func DeleteAppointmentByID(db *sql.DB, id int64, chatID int64) error {
	res, err := db.Exec(`
		DELETE FROM appointments
		WHERE id = ? AND student_chat_id = ?
	`, id, chatID)
	if err != nil {
		return err
	}

	n, err := res.RowsAffected()
	if err != nil {
		return err
	}
	if n == 0 {
		return errors.New("appointment not found")
	}
	return nil
}

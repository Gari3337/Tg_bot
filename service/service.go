package service

import (
	calendar "bot/calendarwidget"
	"bot/database"
	"bot/telegram"
	"fmt"
	"log/slog"
	"strconv"
	"strings"
	"time"
	"unicode"
)

type BookingState struct {
	Step         string
	Date         string // "YYYY-MM-DD"
	Time         string // "HH:MM"
	DurationMin  int    // 60/90
	RepeatMonths int    // 0/1/3/6
	CalYear      int
	CalMonth     int // 1..12
	Confirmed    bool
}

var booking = make(map[int64]*BookingState)

var teacherstatus = make(map[int64]string)
var teacherlogin = make(map[int64]string)

var studentstatus = make(map[int64]string)
var teacherChatIDs = make(map[int64]bool)
var lastBotMsgID = make(map[int64]int)

func Namevalidation(name string) (string, bool) {
	name = strings.TrimSpace(name)
	if len(name) == 0 || len(name) > 30 {
		return "", false
	}
	parts := strings.Split(name, " ")
	if len(parts) != 2 {
		return "", false
	}
	surname := parts[0]
	initials := parts[1]
	if len(surname) == 0 || !unicode.IsLetter([]rune(surname)[0]) {
		return "", false
	}
	initrune := []rune(initials)
	if len(initrune) == 2 {
		if unicode.IsLetter(initrune[0]) && initrune[1] == '.' {
			return name, true
		}
	}
	if len(initrune) == 4 {
		if unicode.IsLetter(initrune[0]) && initrune[1] == '.' &&
			unicode.IsLetter(initrune[2]) && initrune[3] == '.' {
			return name, true
		}
	}
	return "", false
}

func StartBot(token string) error {
	var last_update int64 = 0
	db, err := database.Open()
	if err != nil {
		slog.Error("DB open error", "err", err)
		return err
	}
	defer db.Close()

	for {
		params := map[string]string{
			"timeout": "10",
		}
		if last_update > 0 {
			params["offset"] = strconv.FormatInt(last_update+1, 10)
		}

		var update_resp telegram.GetUpdatesResponse
		err := telegram.CallTelegramAPIGet(token, "getUpdates", params, &update_resp)
		if err != nil {
			slog.Error("getUpdates error", "err", err)
			continue
		}

		for _, update := range update_resp.Result {
			last_update = update.UpdateID
			var chatID int64
			var text string
			if update.CallbackQuery != nil && update.CallbackQuery.Data != "" {
				if update.CallbackQuery.Message == nil {
					_ = telegram.AnswerCallbackQuery(token, update.CallbackQuery.ID)
					continue
				}

				chatID = update.CallbackQuery.Message.Chat.ID
				data := update.CallbackQuery.Data

				_ = telegram.AnswerCallbackQuery(token, update.CallbackQuery.ID)

				st, ok := booking[chatID]
				if !ok {
					st = &BookingState{}
					booking[chatID] = st
				}

				// 3) разбор data
				switch {
				case strings.HasPrefix(data, "t_cancel_app:"):
					// t_cancel_app:<id>:<YYYY-MM-DD>
					parts := strings.Split(data, ":")
					if len(parts) != 3 {
						continue
					}
					id, err := strconv.ParseInt(parts[1], 10, 64)
					if err != nil {
						continue
					}
					date := parts[2]

					if !teacherChatIDs[chatID] {
						_ = telegram.SendMessage(token, chatID, "Недостаточно прав.")
						continue
					}

					if err := database.DeleteAppointmentByIDTeacher(db, id); err != nil {
						_ = telegram.SendMessage(token, chatID, "Ошибка отмены записи")
						continue
					}

					_ = telegram.SendMessage(token, chatID, "✅ Запись отменена")

					// (опционально) сразу показать список заново на эту дату
					loc := time.FixedZone("Europe/Moscow", 3*3600)
					day, _ := time.ParseInLocation("2006-01-02", date, loc)
					dayStart := time.Date(day.Year(), day.Month(), day.Day(), 0, 0, 0, 0, loc).Unix()
					dayEnd := time.Date(day.Year(), day.Month(), day.Day(), 0, 0, 0, 0, loc).Add(24 * time.Hour).Unix()

					apps, err := database.GetAppointmentsByDay(db, dayStart, dayEnd)
					if err != nil || len(apps) == 0 {
						_ = telegram.SendMessage(token, chatID, "На "+day.Format("02.01.2006")+" больше нет записей.")
						continue
					}

					var rows [][]telegram.InlineKeyboardButton
					for _, a := range apps {
						tm := time.Unix(a.StartTS, 0).In(loc).Format("15:04")
						btnText := "❌ " + tm + " — " + a.StudentName + " (" + strconv.Itoa(a.DurationMin) + " мин)"
						rows = append(rows, []telegram.InlineKeyboardButton{
							{Text: btnText, CallbackData: "t_cancel_app:" + strconv.FormatInt(a.ID, 10) + ":" + date},
						})
					}

					kb := &telegram.InlineKeyboardMarkup{InlineKeyboard: rows}
					_ = telegram.SendMessageInlineKeyboard(token, chatID, "Записи на "+day.Format("02.01.2006")+" (нажмите чтобы отменить):", kb)
					continue

				case strings.HasPrefix(data, "cancel_app:"):
					idStr := strings.TrimPrefix(data, "cancel_app:")
					id, err := strconv.ParseInt(idStr, 10, 64)
					if err != nil {
						break
					}

					if err := database.DeleteAppointmentByID(db, id, chatID); err != nil {
						_ = telegram.SendMessage(token, chatID, "Ошибка отмены записи")
						break
					}

					_ = telegram.SendMessage(token, chatID, "✅ Запись отменена")
					continue

				case data == "booking_cancel":
					delete(booking, chatID)
					_ = telegram.SendMessage(token, chatID, "Ок, отменил текущую запись.")
					continue

				case strings.HasPrefix(data, "dur_pick:"):
					// dur_pick:60 или dur_pick:90
					minStr := strings.TrimPrefix(data, "dur_pick:")
					mins, err := strconv.Atoi(minStr)
					if err != nil || (mins != 60 && mins != 90) {
						break
					}

					st.DurationMin = mins
					st.Step = "pick_repeat"

					_ = telegram.SendMessageInlineKeyboard(
						token,
						chatID,
						"Как записать?",
						RepeatKeyboard(),
					)
					continue

				case strings.HasPrefix(data, "rep_pick:"):
					valStr := strings.TrimPrefix(data, "rep_pick:")
					months, err := strconv.Atoi(valStr)
					if err != nil || (months != 0 && months != 1 && months != 3 && months != 6) {
						break
					}

					st.RepeatMonths = months
					st.Step = "confirm"

					_ = telegram.SendMessageInlineKeyboard(
						token,
						chatID,
						"Подтвердить запись?",
						ConfirmKeyboard(),
					)
					continue

				case strings.HasPrefix(data, "cal:"):
					parts := strings.Split(data, ":")
					if len(parts) < 2 {
						break
					}

					switch parts[1] {

					case "day":
						// cal:day:15
						if len(parts) != 3 {
							break
						}
						dayNum, err := strconv.Atoi(parts[2])
						if err != nil {
							break
						}
						if st.CalYear == 0 || st.CalMonth == 0 {
							break
						}

						// Локация (лучше, чем FixedZone("Europe/Moscow"...))
						loc, err := time.LoadLocation("Europe/Moscow")
						if err != nil {
							loc = time.FixedZone("MSK", 3*3600)
						}

						dayTime := time.Date(
							st.CalYear,
							time.Month(st.CalMonth),
							dayNum,
							0, 0, 0, 0,
							loc,
						)

						date := dayTime.Format("2006-01-02")
						st.Date = date

						// ✅ ЕСЛИ ЭТО ПРОСМОТР УЧИТЕЛЯ — ПОКАЗЫВАЕМ ЗАПИСИ
						if st.Step == "t_view_pick_date" {
							dayStart := time.Date(dayTime.Year(), dayTime.Month(), dayTime.Day(), 0, 0, 0, 0, loc).Unix()
							dayEnd := time.Date(dayTime.Year(), dayTime.Month(), dayTime.Day(), 0, 0, 0, 0, loc).Add(24 * time.Hour).Unix()

							apps, err := database.GetAppointmentsByDay(db, dayStart, dayEnd)
							if err != nil {
								_ = telegram.SendMessage(token, chatID, "Ошибка чтения базы данных")
								continue
							}
							if len(apps) == 0 {
								_ = telegram.SendMessage(token, chatID, "На "+dayTime.Format("02.01.2006")+" записей нет.")
								continue
							}

							var rows [][]telegram.InlineKeyboardButton
							for _, a := range apps {
								tm := time.Unix(a.StartTS, 0).In(loc).Format("15:04")
								btnText := "❌ " + tm + " — " + a.StudentName + " (" + strconv.Itoa(a.DurationMin) + " мин)"
								rows = append(rows, []telegram.InlineKeyboardButton{
									{
										Text:         btnText,
										CallbackData: "t_cancel_app:" + strconv.FormatInt(a.ID, 10) + ":" + date,
									},
								})
							}

							kb := &telegram.InlineKeyboardMarkup{InlineKeyboard: rows}
							_ = telegram.SendMessageInlineKeyboard(token, chatID, "Записи на "+dayTime.Format("02.01.2006")+" (нажмите чтобы отменить):", kb)
							continue
						}

						// ✅ ИНАЧЕ (УЧЕНИК) — стандартный сценарий выбора времени
						st.Step = "pick_time"
						kb := TimeKeyboard(date, 2)
						_ = telegram.SendMessageInlineKeyboard(token, chatID, "Выберите время:", kb)
						continue

					case "nav":
						// cal:nav:prev / cal:nav:next
						if len(parts) != 3 {
							break
						}

						if parts[2] == "prev" {
							st.CalMonth--
							if st.CalMonth < 1 {
								st.CalMonth = 12
								st.CalYear--
							}
						} else if parts[2] == "next" {
							st.CalMonth++
							if st.CalMonth > 12 {
								st.CalMonth = 1
								st.CalYear++
							}
						} else {
							break
						}

						cal := calendar.NewCalendar(calendar.Options{
							Language:     "ru",
							InitialYear:  st.CalYear,
							InitialMonth: time.Month(st.CalMonth),
						})

						kb := &telegram.InlineKeyboardMarkup{
							InlineKeyboard: cal.GetKeyboard(),
						}

						_ = telegram.SendMessageInlineKeyboard(
							token,
							chatID,
							"Выберите дату:",
							kb,
						)
						continue

					case "noop":
						continue
					}

				case strings.HasPrefix(data, "cal_pick:"):
					// cal_pick:YYYY-MM-DD
					date := strings.TrimPrefix(data, "cal_pick:")
					st.Date = date
					st.Date = date

					// --- если преподаватель смотрит записи ---
					if st.Step == "t_view_pick_date" {
						// день в МСК
						loc := time.FixedZone("Europe/Moscow", 3*3600)
						day, _ := time.ParseInLocation("2006-01-02", date, loc)
						dayStart := time.Date(day.Year(), day.Month(), day.Day(), 0, 0, 0, 0, loc).Unix()
						dayEnd := time.Date(day.Year(), day.Month(), day.Day(), 0, 0, 0, 0, loc).Add(24 * time.Hour).Unix()

						apps, err := database.GetAppointmentsByDay(db, dayStart, dayEnd)
						if err != nil {
							_ = telegram.SendMessage(token, chatID, "Ошибка чтения базы данных")
							continue
						}

						if len(apps) == 0 {
							_ = telegram.SendMessage(token, chatID, "На "+day.Format("02.01.2006")+" записей нет.")
							continue
						}

						// сообщение + кнопки отмены
						var rows [][]telegram.InlineKeyboardButton
						for _, a := range apps {
							tm := time.Unix(a.StartTS, 0).In(loc).Format("15:04")
							btnText := "❌ " + tm + " — " + a.StudentName + " (" + strconv.Itoa(a.DurationMin) + " мин)"
							rows = append(rows, []telegram.InlineKeyboardButton{
								{
									Text:         btnText,
									CallbackData: "t_cancel_app:" + strconv.FormatInt(a.ID, 10) + ":" + date,
								},
							})
						}

						kb := &telegram.InlineKeyboardMarkup{InlineKeyboard: rows}
						_ = telegram.SendMessageInlineKeyboard(token, chatID, "Записи на "+day.Format("02.01.2006")+" (нажмите чтобы отменить):", kb)
						continue
					}

					// --- иначе это ученик и стандартный сценарий записи ---
					st.Step = "pick_time"
					kb := TimeKeyboard(date, 2)
					_ = telegram.SendMessageInlineKeyboard(token, chatID, "Выберите время:", kb)
					continue

				case strings.HasPrefix(data, "time_page:"):
					// time_page:YYYY-MM-DD:2
					parts := strings.Split(data, ":")
					if len(parts) == 3 {
						date := parts[1]
						page, err := strconv.Atoi(parts[2])
						if err == nil {
							st.Date = date
							st.Step = "pick_time"

							kb := TimeKeyboard(date, page)
							_ = telegram.SendMessageInlineKeyboard(token, chatID, "Выберите время:", kb)
						}
					}

				case strings.HasPrefix(data, "time_pick:"):
					// time_pick:YYYY-MM-DD:15:30
					parts := strings.Split(data, ":")
					if len(parts) == 4 {
						st.Date = parts[1]
						st.Time = parts[2] + ":" + parts[3]
						st.Step = "pick_duration"

						_ = telegram.SendMessageInlineKeyboard(
							token,
							chatID,
							"Вы выбрали: "+st.Date+" "+st.Time+"\nВыберите длительность:",
							DurationKeyboard(),
						)
						continue
					}
				case data == "confirm_yes":
					//if st.Step != "confirm" {
					//	continue
					//}
					//
					//loc := time.FixedZone("Europe/Moscow", 3*3600)
					//dt, err := time.ParseInLocation("2006-01-02 15:04", st.Date+" "+st.Time, loc)
					// ✅ не блокируем подтверждение по Step, проверяем по данным
					if st.Date == "" || st.Time == "" || (st.DurationMin != 60 && st.DurationMin != 90) {
						_ = telegram.SendMessage(token, chatID, "Сессия записи устарела или не заполнена. Нажмите «Записаться» ещё раз.")
						delete(booking, chatID)
						continue
					}

					loc := time.FixedZone("Europe/Moscow", 3*3600)
					dt, err := time.ParseInLocation("2006-01-02 15:04", st.Date+" "+st.Time, loc)
					if err != nil {
						_ = telegram.SendMessage(token, chatID, "Ошибка даты/времени. Попробуйте заново.")
						delete(booking, chatID)
						continue
					}
					if dt.Before(time.Now().In(loc)) {
						_ = telegram.SendMessage(token, chatID, "Нельзя записаться в прошлое")
						delete(booking, chatID)
						continue
					}
					if err != nil {
						_ = telegram.SendMessage(token, chatID, "Ошибка даты/времени. Попробуйте заново.")
						delete(booking, chatID)
						continue
					}
					startTS := dt.Unix()                   // ✅ ВОТ ОН
					start := time.Unix(startTS, 0).In(loc) // ✅ и start тоже

					studentName, okName, err := database.GetStudentName(db, chatID)
					if err != nil || !okName {
						delete(booking, chatID)
						_ = telegram.SendMessage(token, chatID, "Не найдено имя ученика. Нажмите /start и выберите Ученик.")
						continue
					}

					loc = time.FixedZone("Europe/Moscow", 3*3600)
					// попытка создать одну запись
					tryCreate := func(t time.Time) (created bool, busy bool, e error) {
						_, e = database.CreateAppointmentTx(db, chatID, studentName, t.Unix(), st.DurationMin)
						if e == nil {
							return true, false, nil
						}
						if e == database.ErrSlotBusy {
							return false, true, nil
						}
						return false, false, e
					}

					createdCount := 0
					var busyList []string

					if st.RepeatMonths == 0 {
						created, busy, e := tryCreate(start)
						if e != nil {
							slog.Error("create appointment error", "err", e)
							_ = telegram.SendMessage(token, chatID, "Ошибка записи в базу данных")
							delete(booking, chatID)
							continue
						}
						if busy {
							_ = telegram.SendMessage(token, chatID, "❌ Нельзя записаться на это время")
							delete(booking, chatID)
							continue
						}
						if created {
							createdCount = 1
						}

						_ = telegram.SendMessage(token, chatID, "✅ Вы записаны!")
						notify := "📌 Новая запись\n" +
							"Ученик: " + studentName + "\n" +
							"Дата/время: " + start.Format("02.01.2006 15:04") + "\n" +
							"Длительность: " + strconv.Itoa(st.DurationMin) + " мин"

						slog.Info("notify teachers", "count", len(teacherChatIDs), "teachers", fmt.Sprintf("%v", teacherChatIDs))
						for tid := range teacherChatIDs {
							if err := telegram.SendMessage(token, tid, notify); err != nil {
								slog.Error("notify teacher send failed", "teacher_chat_id", tid, "err", err)
							}
						}
						delete(booking, chatID)
						continue
					}

					until := start.AddDate(0, st.RepeatMonths, 0) // по календарю
					for t := start; !t.After(until); t = t.AddDate(0, 0, 7) {
						created, busy, e := tryCreate(t)
						if e != nil {
							slog.Error("create appointment error", "err", e)
							_ = telegram.SendMessage(token, chatID, "Ошибка записи в базу данных")
							delete(booking, chatID)
							continue
						}
						if created {
							createdCount++
						}
						if busy {
							busyList = append(busyList, t.Format("02.01.2006 15:04"))
						}
					}

					msg := "✅ Создано записей: " + strconv.Itoa(createdCount)
					if len(busyList) > 0 {
						msg += "\n\n❌ Не удалось (занято):\n- " + strings.Join(busyList, "\n- ")
					}
					_ = telegram.SendMessage(token, chatID, msg)
					notify := "📌 Новая серия записей\n" +
						"Ученик: " + studentName + "\n" +
						"Старт: " + start.Format("02.01.2006 15:04") + "\n" +
						"Длительность: " + strconv.Itoa(st.DurationMin) + " мин\n" +
						"Создано: " + strconv.Itoa(createdCount)

					for tid := range teacherChatIDs {
						_ = telegram.SendMessage(token, tid, notify)
					}

					delete(booking, chatID)
					continue

				case data == "confirm_no":
					delete(booking, chatID)
					_ = telegram.SendMessage(token, chatID, "Запись отменена")
					continue

				case strings.HasPrefix(data, "time_manual:"):
					// time_manual:YYYY-MM-DD
					parts := strings.Split(data, ":")
					if len(parts) == 2 {
						st.Date = parts[1]
						st.Step = "pick_time_manual"

						_ = telegram.SendMessage(
							token,
							chatID,
							"Введите время для "+st.Date+" (15:30 / 9:30 / 15.30)",
						)
					}

				default:
					// неизвестный callback — ничего не делаем
				}

				continue
			}

			if update.Message == nil || update.Message.Text == "" {
				continue
			}

			chatID = update.Message.Chat.ID
			text = update.Message.Text

			//!!!!!!!!!!!!!!!Очистка БД!!!!!!!!!!! Держать закомичнным!
			//if text == "/clear_db" {
			//	if err := database.ClearStudentsAndAppointments(db); err != nil {
			//		slog.Error("clear db error", "err", err)
			//		_ = telegram.SendMessage(token, chatID, "Ошибка очистки базы данных")
			//	} else {
			//		_ = telegram.SendMessage(token, chatID, "✅ База данных очищена")
			//	}
			//	continue
			//}

			t := strings.ToLower(strings.TrimSpace(text))

			if t == "нет" || t == "отмена" || t == "cancel" {
				delete(booking, chatID)
				_ = telegram.SendMessage(token, chatID, "Ок, отменил текущую запись.")
				continue
			}

			if text == "/start" || text == "Назад" {
				teacherstatus[chatID] = ""
				teacherlogin[chatID] = ""
				studentstatus[chatID] = ""
				delete(booking, chatID)

				keyboard := Rolekeyboard()
				message := "Доброго времени суток!\nПожалуйста, выберите вашу роль для продолжения работы с ботом."
				_ = telegram.SendMessageKeyboard(token, chatID, message, keyboard)
				continue
			}

			if st, ok := booking[chatID]; ok && st.Step == "pick_time_manual" {
				timeStr, ok := normalizeTime(text)
				if !ok {
					_ = telegram.SendMessage(token, chatID, "Неверное время. Пример: 15:30 / 9:30 / 15.30 (только минуты 00 или 30)")
					continue
				}

				st.Time = timeStr
				st.Step = "pick_duration"

				_ = telegram.SendMessageInlineKeyboard(
					token,
					chatID,
					"Вы выбрали: "+st.Date+" "+st.Time+"\nВыберите длительность:",
					DurationKeyboard(),
				)
				continue
			}
			// ===== ЗАПИСЬ: ВВОД ДАТЫ И ВРЕМЕНИ (НЕ ЗАВИСИТ ОТ wait_name) =====
			if st, ok := booking[chatID]; ok && st.Step == "pick_time" {
				loc := time.FixedZone("Europe/Moscow", 3*3600)

				dt, err := time.ParseInLocation("02.01.2006 15:04", strings.TrimSpace(text), loc)
				if err != nil {
					_ = telegram.SendMessage(token, chatID, "Неверный формат. Пример: 25.06.2025 15:30")
					continue
				}
				if dt.Before(time.Now().In(loc)) {
					_ = telegram.SendMessage(token, chatID, "Нельзя записаться в прошлое")
					continue
				}
				st.Date = dt.Format("2006-01-02")
				st.Time = dt.Format("15:04")

				st.Step = "pick_duration"
				_ = telegram.SendMessage(token, chatID, "Выберите длительность:\n1 — 1 час\n2 — 1.5 часа")
				continue
			}

			// ===== ОЖИДАНИЕ ИМЕНИ УЧЕНИКА =====
			if studentstatus[chatID] == "wait_name" {
				name, ok := Namevalidation(text)
				if !ok {
					_ = telegram.SendMessage(token, chatID, "Неверный формат. Пример: Иванов И.И. или Иванов И.")
					continue
				}

				// ✅ сохраняем в БД
				if err := database.UpsertStudentName(db, chatID, name); err != nil {
					slog.Error("save student name error", "chat_id", chatID, "err", err)
					_ = telegram.SendMessage(token, chatID, "Ошибка сохранения в базе данных. Попробуйте ещё раз.")
					continue
				}

				studentstatus[chatID] = ""
				keyboard := Studkeyboard()
				_ = telegram.SendMessageKeyboard(token, chatID, "Готово! Вы записаны как: "+name, keyboard)
				continue
			}

			// ===== ЗАПИСЬ: ВЫБОР ДЛИТЕЛЬНОСТИ =====
			//if st, ok := booking[chatID]; ok && st.Step == "pick_duration" {
			//	if text == "1" {
			//		st.DurationMin = 60
			//	} else if text == "2" {
			//		st.DurationMin = 90
			//	} else {
			//		_ = telegram.SendMessage(token, chatID, "Введите 1 или 2")
			//		continue
			//	}
			if st, ok := booking[chatID]; ok && st.Step == "pick_duration" {
				_ = telegram.SendMessageInlineKeyboard(
					token,
					chatID,
					"Выберите длительность:",
					DurationKeyboard(),
				)
				continue
			}

			// ЗАПИСЬ: ВЫБОР ПОВТОРОВ
			if st, ok := booking[chatID]; ok && st.Step == "pick_repeat" {
				switch strings.TrimSpace(text) {
				case "0":
					st.RepeatMonths = 0
				case "1":
					st.RepeatMonths = 1
				case "3":
					st.RepeatMonths = 3
				case "6":
					st.RepeatMonths = 6
				default:
					_ = telegram.SendMessage(token, chatID, "Введите 0, 1, 3 или 6")
					continue
				}

				st.Step = "confirm"
				_ = telegram.SendMessageInlineKeyboard(
					token,
					chatID,
					"Подтвердить запись?",
					ConfirmKeyboard(),
				)
				continue
			}

			//ЗАПИСЬ:
			//	ПОДТВЕРЖДЕНИЕ + СОЗДАНИЕ
			if st, ok := booking[chatID]; ok && st.Step == "confirm" {

				// ✅ если подтверждение пришло кнопкой — пропускаем проверку "да"
				if !st.Confirmed {
					if strings.ToLower(strings.TrimSpace(text)) != "да" {
						delete(booking, chatID)
						_ = telegram.SendMessage(token, chatID, "Запись отменена")
						continue
					}
				}
				st.Confirmed = false // сброс

				loc := time.FixedZone("Europe/Moscow", 3*3600)
				dt, err := time.ParseInLocation("2006-01-02 15:04", st.Date+" "+st.Time, loc)
				if err != nil {
					_ = telegram.SendMessage(token, chatID, "Ошибка даты/времени. Попробуйте заново.")
					delete(booking, chatID)
					continue
				}
				startTS := dt.Unix()                   // ✅ ВОТ ОН
				start := time.Unix(startTS, 0).In(loc) // ✅ и start тоже

				studentName, okName, err := database.GetStudentName(db, chatID)
				if err != nil || !okName {
					delete(booking, chatID)
					_ = telegram.SendMessage(token, chatID, "Не найдено имя ученика. Нажмите /start и выберите Ученик.")
					continue
				}

				loc = time.FixedZone("Europe/Moscow", 3*3600)
				// попытка создать одну запись
				tryCreate := func(t time.Time) (created bool, busy bool, e error) {
					_, e = database.CreateAppointmentTx(db, chatID, studentName, t.Unix(), st.DurationMin)
					if e == nil {
						return true, false, nil
					}
					if e == database.ErrSlotBusy {
						return false, true, nil
					}
					return false, false, e
				}

				createdCount := 0
				var busyList []string

				if st.RepeatMonths == 0 {
					created, busy, e := tryCreate(start)
					if e != nil {
						slog.Error("create appointment error", "err", e)
						_ = telegram.SendMessage(token, chatID, "Ошибка записи в базу данных")
						delete(booking, chatID)
						continue
					}
					if busy {
						_ = telegram.SendMessage(token, chatID, "❌ Нельзя записаться на это время")
						delete(booking, chatID)
						continue
					}
					if created {
						createdCount = 1
					}

					_ = telegram.SendMessage(token, chatID, "✅ Вы записаны!")
					notify := "📌 Новая запись\n" +
						"Ученик: " + studentName + "\n" +
						"Дата/время: " + start.Format("02.01.2006 15:04") + "\n" +
						"Длительность: " + strconv.Itoa(st.DurationMin) + " мин"

					slog.Info("notify teachers", "count", len(teacherChatIDs), "teachers", fmt.Sprintf("%v", teacherChatIDs))
					for tid := range teacherChatIDs {
						if err := telegram.SendMessage(token, tid, notify); err != nil {
							slog.Error("notify teacher send failed", "teacher_chat_id", tid, "err", err)
						}
					}
					delete(booking, chatID)
					continue
				}

				until := start.AddDate(0, st.RepeatMonths, 0) // по календарю
				for t := start; !t.After(until); t = t.AddDate(0, 0, 7) {
					created, busy, e := tryCreate(t)
					if e != nil {
						slog.Error("create appointment error", "err", e)
						_ = telegram.SendMessage(token, chatID, "Ошибка записи в базу данных")
						delete(booking, chatID)
						continue
					}
					if created {
						createdCount++
					}
					if busy {
						busyList = append(busyList, t.Format("02.01.2006 15:04"))
					}
				}

				msg := "✅ Создано записей: " + strconv.Itoa(createdCount)
				if len(busyList) > 0 {
					msg += "\n\n❌ Не удалось (занято):\n- " + strings.Join(busyList, "\n- ")
				}
				_ = telegram.SendMessage(token, chatID, msg)
				notify := "📌 Новая серия записей\n" +
					"Ученик: " + studentName + "\n" +
					"Старт: " + start.Format("02.01.2006 15:04") + "\n" +
					"Длительность: " + strconv.Itoa(st.DurationMin) + " мин\n" +
					"Создано: " + strconv.Itoa(createdCount)

				for tid := range teacherChatIDs {
					_ = telegram.SendMessage(token, tid, notify)
				}

				delete(booking, chatID)
				continue
			}

			if text == "/start" {
				teacherstatus[chatID] = ""
				teacherlogin[chatID] = ""
				studentstatus[chatID] = ""
				keyboard := Rolekeyboard()
				message := "Доброго времени суток!\nПожалуйста, выберите вашу роль для продолжения работы с ботом."
				if err := telegram.SendMessageKeyboard(token, chatID, message, keyboard); err != nil {
					slog.Error("send message error", "err", err)
				}
				continue
			}

			if text == "Преподаватель" {
				teacherstatus[chatID] = "login"
				teacherlogin[chatID] = ""
				_ = telegram.SendMessage(token, chatID, "Введите логин:")
				continue
			}

			if teacherstatus[chatID] == "login" {
				teacherlogin[chatID] = text
				teacherstatus[chatID] = "password"
				_ = telegram.SendMessage(token, chatID, "Введите пароль:")
				continue
			}

			if teacherstatus[chatID] == "password" {
				login := teacherlogin[chatID]
				password := text

				t, ok, err := database.GetTeacherByLogin(db, login)
				if err != nil {
					slog.Error("DB read error", "err", err)
					_ = telegram.SendMessage(token, chatID, "Ошибка чтения базы данных")
					teacherstatus[chatID] = ""
					teacherlogin[chatID] = ""
					continue
				}

				if !ok {
					_ = telegram.SendMessage(token, chatID, "Неверный логин или пароль!")
					teacherstatus[chatID] = ""
					teacherlogin[chatID] = ""
					continue
				}

				if CheckPassword(t.PasswordHash, password) {
					teacherChatIDs[chatID] = true // ✅ ВОТ ЭТОГО НЕ ХВАТАЛО
					slog.Info("teacher logged in", "chat_id", chatID, "teachers_count", len(teacherChatIDs))
					_ = telegram.SendMessage(token, chatID, "Авторизация прошла успешно!")
					_ = telegram.SendMessageKeyboard(token, chatID, "Меню преподавателя:", Teachkeyboard())
				} else {
					_ = telegram.SendMessage(token, chatID, "Неверный логин или пароль!")
				}

				teacherstatus[chatID] = ""
				teacherlogin[chatID] = ""
				continue
			}

			if text == "Ученик" {
				if name, ok, err := database.GetStudentName(db, chatID); err != nil {
					slog.Error("DB read error", "err", err)
					_ = telegram.SendMessage(token, chatID, "Ошибка чтения базы данных")
					continue
				} else if ok {
					keyboard := Studkeyboard()
					_ = telegram.SendMessageKeyboard(token, chatID, "Вы записаны как: "+name, keyboard)
					continue
				} else {
					studentstatus[chatID] = "wait_name"
					_ = telegram.SendMessage(token, chatID, "Введите Ваши инициалы...\n(Например: Иванов И.И./ Иванов И. , если нет отчества)")
					continue
				}
			}

			if text == "Записаться" {
				st, ok := booking[chatID]
				if !ok {
					st = &BookingState{}
					booking[chatID] = st
				}

				st.Step = "pick_date"
				st.Date = ""
				st.Time = ""
				st.DurationMin = 0
				st.RepeatMonths = 0

				// текущий месяц/год (если не задано — ставим "сейчас")
				now := time.Now()
				if st.CalYear == 0 {
					st.CalYear = now.Year()
				}
				if st.CalMonth == 0 {
					st.CalMonth = int(now.Month())
				}

				cal := calendar.NewCalendar(calendar.Options{
					Language:     "ru",
					InitialYear:  st.CalYear,
					InitialMonth: time.Month(st.CalMonth),
				})

				kb := &telegram.InlineKeyboardMarkup{
					InlineKeyboard: cal.GetKeyboard(),
				}

				_ = telegram.SendMessageInlineKeyboard(
					token,
					chatID,
					"Выберите дату:",
					kb,
				)
				continue
			}

			if text == "Посмотреть записи" || text == "/day" {
				if !teacherChatIDs[chatID] {
					_ = telegram.SendMessage(token, chatID, "Сначала войдите как преподаватель.")
					continue
				}

				st, ok := booking[chatID]
				if !ok {
					st = &BookingState{}
					booking[chatID] = st
				}

				st.Step = "t_view_pick_date"

				now := time.Now()
				st.CalYear = now.Year()
				st.CalMonth = int(now.Month())

				cal := calendar.NewCalendar(calendar.Options{
					Language:     "ru",
					InitialYear:  st.CalYear,
					InitialMonth: time.Month(st.CalMonth),
				})

				kb := &telegram.InlineKeyboardMarkup{
					InlineKeyboard: cal.GetKeyboard(),
				}

				_ = telegram.SendMessageInlineKeyboard(token, chatID, "Выберите день:", kb)
				continue
			}

			if text == "Записи по дням" {
				if !teacherChatIDs[chatID] {
					_ = telegram.SendMessage(token, chatID, "Сначала войдите как преподаватель.")
					continue
				}

				// важно: сбросить старую "ученическую" запись, если была
				st := &BookingState{}
				booking[chatID] = st

				st.Step = "t_view_pick_date"

				now := time.Now()
				st.CalYear = now.Year()
				st.CalMonth = int(now.Month())

				cal := calendar.NewCalendar(calendar.Options{
					Language:     "ru",
					InitialYear:  st.CalYear,
					InitialMonth: time.Month(st.CalMonth),
				})
				kb := &telegram.InlineKeyboardMarkup{InlineKeyboard: cal.GetKeyboard()}
				_ = telegram.SendMessageInlineKeyboard(token, chatID, "Выберите дату:", kb)
				continue
			}

			if text == "Мои записи" {
				apps, err := database.GetFutureAppointments(db, chatID)
				if err != nil {
					_ = telegram.SendMessage(token, chatID, "Ошибка чтения базы данных")
					continue
				}

				if len(apps) == 0 {
					_ = telegram.SendMessage(token, chatID, "У вас нет будущих записей")
					continue
				}

				var rows [][]telegram.InlineKeyboardButton
				loc := time.FixedZone("Europe/Moscow", 3*3600)

				for _, a := range apps {
					t := time.Unix(a.StartTS, 0).In(loc).Format("02.01.2006 15:04")
					rows = append(rows, []telegram.InlineKeyboardButton{
						{
							Text:         t,
							CallbackData: "noop", // ✅ ничего не делает
						},
					})
				}

				kb := &telegram.InlineKeyboardMarkup{InlineKeyboard: rows}
				_ = telegram.SendMessageInlineKeyboard(token, chatID, "Ваши будущие записи:", kb)
				continue
			}

			if text == "Отменить запись" {
				apps, err := database.GetFutureAppointments(db, chatID)
				if err != nil {
					_ = telegram.SendMessage(token, chatID, "Ошибка чтения базы данных")
					continue
				}

				if len(apps) == 0 {
					_ = telegram.SendMessage(token, chatID, "У вас нет будущих записей")
					continue
				}

				var rows [][]telegram.InlineKeyboardButton
				loc := time.FixedZone("Europe/Moscow", 3*3600)

				for _, a := range apps {
					t := time.Unix(a.StartTS, 0).In(loc).Format("02.01.2006 15:04")
					rows = append(rows, []telegram.InlineKeyboardButton{
						{
							Text:         "❌ " + t,
							CallbackData: "cancel_app:" + strconv.FormatInt(a.ID, 10),
						},
					})
				}

				kb := &telegram.InlineKeyboardMarkup{InlineKeyboard: rows}
				_ = telegram.SendMessageInlineKeyboard(
					token,
					chatID,
					"Выберите запись для отмены:",
					kb,
				)
				continue
			}

			_ = telegram.SendMessage(token, chatID, "Выберите роль или нажмите /start")
		}
	}
}

func sendAndReplace(token string, chatID int64, text string) {
	if mid, ok := lastBotMsgID[chatID]; ok && mid != 0 {
		_ = telegram.DeleteMessage(token, chatID, mid)
	}
	newID, err := telegram.SendMessageReturnID(token, chatID, text)
	if err == nil {
		lastBotMsgID[chatID] = newID
	}
}

func sendAndReplaceInline(token string, chatID int64, text string, kb *telegram.InlineKeyboardMarkup) {
	if mid, ok := lastBotMsgID[chatID]; ok && mid != 0 {
		_ = telegram.DeleteMessage(token, chatID, mid)
	}
	newID, err := telegram.SendMessageInlineKeyboardReturnID(token, chatID, text, kb)
	if err == nil {
		lastBotMsgID[chatID] = newID
	}
}

func normalizeTime(s string) (string, bool) {
	s = strings.TrimSpace(s)
	s = strings.ReplaceAll(s, ".", ":")

	parts := strings.Split(s, ":")
	if len(parts) != 2 {
		return "", false
	}

	h, err1 := strconv.Atoi(parts[0])
	m, err2 := strconv.Atoi(parts[1])
	if err1 != nil || err2 != nil {
		return "", false
	}

	if h < 0 || h > 23 {
		return "", false
	}

	// шаг 30 минут
	if m != 0 && m != 30 {
		return "", false
	}

	return fmt.Sprintf("%d:%02d", h, m), true
}

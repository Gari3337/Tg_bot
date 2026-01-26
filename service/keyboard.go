package service

import "bot/telegram"

func Studkeyboard() *telegram.ReplyKeyboardMarkup {
	return &telegram.ReplyKeyboardMarkup{
		Keyboard: [][]telegram.KeyboardButton{
			{{Text: "Записаться"}},
			{{Text: "Мои записи"}},
			{{Text: "Отменить запись"}},
			{{Text: "Назад"}},
		},
		ResizeKeyboard:  true,
		OneTimeKeyboard: false,
	}
}

func Rolekeyboard() *telegram.ReplyKeyboardMarkup {
	return &telegram.ReplyKeyboardMarkup{
		Keyboard: [][]telegram.KeyboardButton{
			{{Text: "Ученик"}},
			{{Text: "Преподаватель"}},
		},
		ResizeKeyboard:  true,
		OneTimeKeyboard: false,
	}
}

func DurationKeyboard() *telegram.InlineKeyboardMarkup {
	return &telegram.InlineKeyboardMarkup{
		InlineKeyboard: [][]telegram.InlineKeyboardButton{
			{
				{Text: "🕐 1 час", CallbackData: "dur_pick:60"},
				{Text: "🕜 1.5 часа", CallbackData: "dur_pick:90"},
			},
			{
				{Text: "Отмена", CallbackData: "booking_cancel"},
			},
		},
	}
}

func ConfirmKeyboard() *telegram.InlineKeyboardMarkup {
	return &telegram.InlineKeyboardMarkup{
		InlineKeyboard: [][]telegram.InlineKeyboardButton{
			{
				{Text: "✅ Да", CallbackData: "confirm_yes"},
				{Text: "❌ Нет", CallbackData: "confirm_no"},
			},
		},
	}
}

func Teachkeyboard() *telegram.ReplyKeyboardMarkup {
	return &telegram.ReplyKeyboardMarkup{
		Keyboard: [][]telegram.KeyboardButton{
			{{Text: "Записи по дням"}},
			{{Text: "Назад"}},
		},
		ResizeKeyboard:  true,
		OneTimeKeyboard: false,
	}
}

func RepeatKeyboard() *telegram.InlineKeyboardMarkup {
	return &telegram.InlineKeyboardMarkup{
		InlineKeyboard: [][]telegram.InlineKeyboardButton{
			{
				{Text: "разово", CallbackData: "rep_pick:0"},
			},
			{
				{Text: "каждую неделю на 1 месяц", CallbackData: "rep_pick:1"},
			},
			{
				{Text: "каждую неделю на 3 месяца", CallbackData: "rep_pick:3"},
			},
			{
				{Text: "каждую неделю на 6 месяцев", CallbackData: "rep_pick:6"},
			},
			{
				{Text: "Отмена", CallbackData: "booking_cancel"},
			},
		},
	}
}

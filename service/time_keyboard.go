package service

import (
	"bot/telegram"
	"fmt"
	"strconv"
)

// page: 0..3
// 0 = 00:00-05:30
// 1 = 06:00-11:30
// 2 = 12:00-17:30
// 3 = 18:00-23:30
func TimeKeyboard(dateYYYYMMDD string, page int) *telegram.InlineKeyboardMarkup {
	if page < 0 {
		page = 0
	}
	if page > 3 {
		page = 3
	}

	startHour := page * 6 // 0, 6, 12, 18
	var rows [][]telegram.InlineKeyboardButton

	// 12 строк: каждый час -> две кнопки (00 и 30)
	for h := startHour; h < startHour+6; h++ {
		t1 := fmt.Sprintf("%02d:00", h)
		t2 := fmt.Sprintf("%02d:30", h)

		rows = append(rows, []telegram.InlineKeyboardButton{
			{Text: t1, CallbackData: "time_pick:" + dateYYYYMMDD + ":" + t1},
			{Text: t2, CallbackData: "time_pick:" + dateYYYYMMDD + ":" + t2},
		})
	}

	// Навигация (⟵ page/4 ⟶)
	nav := []telegram.InlineKeyboardButton{}
	if page > 0 {
		nav = append(nav, telegram.InlineKeyboardButton{
			Text: "⟵", CallbackData: "time_page:" + dateYYYYMMDD + ":" + strconv.Itoa(page-1),
		})
	}
	nav = append(nav, telegram.InlineKeyboardButton{
		Text: fmt.Sprintf("%d/4", page+1), CallbackData: "noop",
	})
	if page < 3 {
		nav = append(nav, telegram.InlineKeyboardButton{
			Text: "⟶", CallbackData: "time_page:" + dateYYYYMMDD + ":" + strconv.Itoa(page+1),
		})
	}
	rows = append(rows, nav)

	// Ручной ввод
	rows = append(rows, []telegram.InlineKeyboardButton{
		{Text: "⌨️ Ввести вручную", CallbackData: "time_manual:" + dateYYYYMMDD},
	})

	return &telegram.InlineKeyboardMarkup{InlineKeyboard: rows}
}

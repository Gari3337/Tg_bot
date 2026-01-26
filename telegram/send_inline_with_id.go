package telegram

import (
	"encoding/json"
	"strconv"
)

type sendMessageInlineRespID struct {
	Ok     bool `json:"ok"`
	Result struct {
		MessageID int `json:"message_id"`
	} `json:"result"`
}

// Если у тебя уже есть SendMessageInlineKeyboard — можешь оставить его,
// но для удаления нам нужен message_id, поэтому делаем версию с возвратом ID.
func SendMessageInlineKeyboardReturnID(token string, chatID int64, text string, kb *InlineKeyboardMarkup) (int, error) {
	// Telegram требует reply_markup как JSON-строку
	rm, _ := json.Marshal(kb)

	params := map[string]string{
		"chat_id":      strconv.FormatInt(chatID, 10),
		"text":         text,
		"reply_markup": string(rm),
	}

	var resp sendMessageInlineRespID
	// если твой CallTelegramAPIGet принимает map — ок.
	// Если Telegram у тебя отправляется POST — скажи, я подстрою под твой helper.
	if err := CallTelegramAPIGet(token, "sendMessage", params, &resp); err != nil {
		return 0, err
	}
	return resp.Result.MessageID, nil
}

package telegram

import "strconv"

type deleteMessageResp struct {
	Ok     bool `json:"ok"`
	Result bool `json:"result"`
}

func DeleteMessage(token string, chatID int64, messageID int) error {
	params := map[string]string{
		"chat_id":    strconv.FormatInt(chatID, 10),
		"message_id": strconv.Itoa(messageID),
	}
	var resp deleteMessageResp
	return CallTelegramAPIGet(token, "deleteMessage", params, &resp)
}

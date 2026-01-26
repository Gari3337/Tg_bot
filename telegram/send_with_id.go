package telegram

import "strconv"

type sendMessageRespID struct {
	Ok     bool `json:"ok"`
	Result struct {
		MessageID int `json:"message_id"`
	} `json:"result"`
}

func SendMessageReturnID(token string, chatID int64, text string) (int, error) {
	params := map[string]string{
		"chat_id": strconv.FormatInt(chatID, 10),
		"text":    text,
	}
	var resp sendMessageRespID
	if err := CallTelegramAPIGet(token, "sendMessage", params, &resp); err != nil {
		return 0, err
	}
	return resp.Result.MessageID, nil
}

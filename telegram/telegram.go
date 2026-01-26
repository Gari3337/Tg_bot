package telegram

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
)

func CallTelegramAPIGet(token string, method string, params map[string]string, result interface{}) error {
	botToken := fmt.Sprintf("bot%s", token)
	baseURL := &url.URL{
		Scheme: "https",
		Host:   "api.telegram.org",
	}

	baseURL = baseURL.JoinPath(botToken, method)

	q := baseURL.Query()
	for key, value := range params {
		q.Set(key, value)
	}
	baseURL.RawQuery = q.Encode()

	resp, err := http.Get(baseURL.String())
	if err != nil {
		return err //ошибка при запросе
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return errors.New("telegram returned error: " + string(body)) //ошибка от Telegram
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return fmt.Errorf("read body error: %w", err) //ошибка чтения тела ответа
	}

	if err := json.Unmarshal(body, result); err != nil {
		return fmt.Errorf("json unmarshal error: %w", err) //ошибка разбора JSON
	}

	return nil
}

func CallTelegramAPIPostJSON(token string, method string, payload interface{}, result interface{}) error {
	apiURL := fmt.Sprintf("https://api.telegram.org/bot%s/%s", token, method)
	jsonBody, err := json.Marshal(payload)
	if err != nil {
		return fmt.Errorf("error: %w", err)
	}

	req, err := http.NewRequest("POST", apiURL, bytes.NewBuffer(jsonBody))
	if err != nil {
		return fmt.Errorf("error: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")

	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		return fmt.Errorf("error: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("telegram API status %d: %s", resp.StatusCode, string(body))
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return fmt.Errorf("error: %w", err)
	}

	if result != nil {
		if err := json.Unmarshal(body, result); err != nil {
			return fmt.Errorf("error: %w", err)
		}
	}

	return nil
}

func SendMessage(token string, chatID int64, text string) error {
	payload := map[string]interface{}{
		"chat_id": chatID,
		"text":    text,
	}

	var resp SendMessageResponse
	if err := CallTelegramAPIPostJSON(token, "sendMessage", payload, &resp); err != nil {
		return err
	}
	if !resp.Ok {
		return fmt.Errorf("telegram sendMessage not ok")
	}

	return nil
}

func SendMessageKeyboard(token string, chatID int64, text string, keyboard *ReplyKeyboardMarkup) error {
	payload := map[string]interface{}{
		"chat_id": chatID,
		"text":    text,
	}

	if keyboard != nil {
		payload["reply_markup"] = keyboard
	}

	var resp SendMessageResponse
	if err := CallTelegramAPIPostJSON(token, "sendMessage", payload, &resp); err != nil {
		return err
	}
	if !resp.Ok {
		return fmt.Errorf("telegram sendMessage not ok")
	}

	return nil
}

func SendMessageInlineKeyboard(token string, chatID int64, text string, keyboard *InlineKeyboardMarkup) error {
	payload := map[string]interface{}{
		"chat_id": chatID,
		"text":    text,
	}
	if keyboard != nil {
		payload["reply_markup"] = keyboard
	}

	var resp SendMessageResponse
	if err := CallTelegramAPIPostJSON(token, "sendMessage", payload, &resp); err != nil {
		return err
	}
	if !resp.Ok {
		return fmt.Errorf("telegram sendMessage not ok")
	}
	return nil
}

func AnswerCallbackQuery(token, callbackQueryID string) error {
	payload := map[string]interface{}{
		"callback_query_id": callbackQueryID,
	}
	return CallTelegramAPIPostJSON(token, "answerCallbackQuery", payload, nil)
}

//func SendMessageReturnID(token string, chatID int64, text string) (int, error) {
//	params := map[string]string{
//		"chat_id": strconv.FormatInt(chatID, 10),
//		"text":    text,
//	}
//	var resp SendMessageResponse
//	if err := CallTelegramAPIGet(token, "sendMessage", params, &resp); err != nil {
//		return 0, err
//	}
//	return resp.Result.MessageID, nil
//}

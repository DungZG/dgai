package service

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"strconv"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/logger"
	"github.com/QuantumNous/new-api/model"
	"github.com/QuantumNous/new-api/setting/operation_setting"
)

const (
	telegramAPIBaseURL      = "https://api.telegram.org"
	telegramPollTimeout     = 30
	telegramMaxResponseSize = 4 << 20
	telegramMaxSupportRunes = 3500
)

type telegramBot struct {
	token       string
	supportURL  string
	supportChat int64
	apiBaseURL  string
	client      *http.Client
}

type telegramUpdate struct {
	UpdateID      int64                  `json:"update_id"`
	Message       *telegramMessage       `json:"message"`
	CallbackQuery *telegramCallbackQuery `json:"callback_query"`
}

type telegramMessage struct {
	MessageID      int64            `json:"message_id"`
	Text           string           `json:"text"`
	Chat           telegramChat     `json:"chat"`
	From           *telegramUser    `json:"from"`
	ReplyToMessage *telegramMessage `json:"reply_to_message"`
}

type telegramCallbackQuery struct {
	ID      string           `json:"id"`
	From    *telegramUser    `json:"from"`
	Message *telegramMessage `json:"message"`
	Data    string           `json:"data"`
}

type telegramChat struct {
	ID   int64  `json:"id"`
	Type string `json:"type"`
}

type telegramUser struct {
	ID       int64  `json:"id"`
	Username string `json:"username"`
	IsBot    bool   `json:"is_bot"`
}

type telegramChatMember struct {
	Status string `json:"status"`
}

func StartTelegramBot() {
	if !common.IsMasterNode {
		return
	}
	bot := newTelegramBot()
	if bot == nil {
		common.SysLog(common.LogText("Telegram bot is disabled: TELEGRAM_BOT_TOKEN is not configured"))
		return
	}
	common.SysLog(common.LogText("Telegram bot polling started"))
	activeTelegramBot = bot
	go bot.run()
}

func newTelegramBot() *telegramBot {
	token := strings.TrimSpace(common.TelegramBotToken)
	if token == "" {
		return nil
	}
	bot := &telegramBot{
		token:      token,
		supportURL: strings.TrimSpace(os.Getenv("DGAI_TELEGRAM_SUPPORT_URL")),
		apiBaseURL: telegramAPIBaseURL,
		client:     &http.Client{Timeout: 40 * time.Second},
	}
	if rawChatID := strings.TrimSpace(os.Getenv("DGAI_TELEGRAM_SUPPORT_CHAT_ID")); rawChatID != "" {
		chatID, err := strconv.ParseInt(rawChatID, 10, 64)
		if err != nil || chatID >= 0 {
			common.SysLog(common.LogText("Telegram support chat ID is invalid; support tickets are disabled"))
		} else {
			bot.supportChat = chatID
		}
	}
	return bot
}

func (bot *telegramBot) run() {
	var offset int64
	for {
		updates, err := bot.getUpdates(offset)
		if err != nil {
			common.SysLog(common.LogText("Telegram bot polling failed: %s", err.Error()))
			time.Sleep(5 * time.Second)
			continue
		}
		for _, update := range updates {
			if update.UpdateID < offset {
				continue
			}
			if err := bot.handleUpdate(update); err != nil {
				common.SysLog(common.LogText("Telegram bot update %d failed: %s", update.UpdateID, err.Error()))
				time.Sleep(5 * time.Second)
				break
			}
			offset = update.UpdateID + 1
		}
	}
}

func (bot *telegramBot) getUpdates(offset int64) ([]telegramUpdate, error) {
	values := url.Values{"timeout": {strconv.Itoa(telegramPollTimeout)}}
	if offset > 0 {
		values.Set("offset", strconv.FormatInt(offset, 10))
	}
	endpoint := fmt.Sprintf("%s/bot%s/getUpdates?%s", bot.apiBaseURL, bot.token, values.Encode())
	request, err := http.NewRequest(http.MethodGet, endpoint, nil)
	if err != nil {
		return nil, err
	}
	response, err := bot.client.Do(request)
	if err != nil {
		return nil, fmt.Errorf("Telegram getUpdates request failed")
	}
	defer response.Body.Close()
	if response.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("Telegram getUpdates returned HTTP %d", response.StatusCode)
	}
	var result struct {
		OK          bool             `json:"ok"`
		Description string           `json:"description"`
		Result      []telegramUpdate `json:"result"`
	}
	if err := common.DecodeJson(io.LimitReader(response.Body, telegramMaxResponseSize), &result); err != nil {
		return nil, err
	}
	if !result.OK {
		return nil, fmt.Errorf("Telegram getUpdates: %s", result.Description)
	}
	return result.Result, nil
}

func (bot *telegramBot) handleUpdate(update telegramUpdate) error {
	if update.CallbackQuery != nil {
		return bot.handleCallbackQuery(update.CallbackQuery)
	}
	message := update.Message
	if message == nil {
		return nil
	}
	if bot.supportChat != 0 && message.Chat.ID == bot.supportChat && message.Chat.Type != "private" {
		return bot.handleSupportReply(message)
	}
	if message.Chat.Type != "private" {
		command, _ := parseTelegramCommand(message.Text)
		if command != "models" {
			return nil
		}
		return bot.showModelsGroup(message.Chat.ID)
	}
	return bot.handlePrivateMessage(message)
}

func (bot *telegramBot) handlePrivateMessage(message *telegramMessage) error {
	command, args := parseTelegramCommand(message.Text)
	if command == "" {
		return nil
	}

	switch command {
	case "start", "help":
		return bot.showMainMenu(message.Chat.ID, 0, message.From)
	case "topup":
		if args != "" {
			return bot.createTopUpFromCommand(message.Chat.ID, message.From, args)
		}
		return bot.showTopUpMenu(message.Chat.ID, 0, message.From)
	case "balance":
		return bot.showBalance(message.Chat.ID, 0, message.From)
	case "redeem":
		if !operation_setting.IsPaymentComplianceConfirmed() {
			return bot.sendMessage(message.Chat.ID, "Tính năng đổi mã đang tạm tắt. Vui lòng liên hệ quản trị viên.")
		}
		if args == "" {
			return bot.sendMessage(message.Chat.ID, "Cách dùng: /redeem MÃ_QUÀ_TẶNG")
		}
		user, err := bot.linkedUser(message.From)
		if err != nil {
			return bot.sendMessage(message.Chat.ID, err.Error())
		}
		quota, err := model.Redeem(args, user.Id)
		if err != nil {
			return bot.sendMessage(message.Chat.ID, "Đổi mã thất bại. Kiểm tra mã hoặc thử lại sau.")
		}
		return bot.sendMessage(message.Chat.ID, "Đổi mã thành công. Đã cộng "+logger.FormatQuota(quota)+" vào số dư.")
	case "models":
		return bot.showModels(message.Chat.ID, 0, message.From)
	case "plans":
		return bot.showPlans(message.Chat.ID, 0, message.From)
	case "id":
		if message.From == nil {
			return nil
		}
		text := fmt.Sprintf("🆔 <b>Telegram ID</b>\n<code>%d</code>\nDùng ID này để liên kết trong Hồ sơ hoặc Admin → Users.", message.From.ID)
		return bot.sendCard(message.Chat.ID, text, keyboard(
			[]telegramInlineKeyboardButton{callbackButton("🏠 Menu", "menu")},
		))
	case "support":
		return bot.openSupport(message, args)
	case "price":
		return bot.handlePriceCommand(message, args)
	case "prices":
		return bot.handlePriceList(message)
	case "cancel":
		return bot.handlePriceCancel(message, args)
	case "undo":
		return bot.handlePriceUndo(message, args)
	case "pricecheck":
		return bot.handlePriceCheck(message, args)
	default:
		return bot.sendMessage(message.Chat.ID, "Không nhận ra lệnh. Gửi /help để xem danh sách lệnh.")
	}
}

func (bot *telegramBot) linkedUser(from *telegramUser) (*model.User, error) {
	if from == nil || from.ID <= 0 {
		return nil, fmt.Errorf("Không xác định được tài khoản Telegram.")
	}
	user, err := model.GetUserByTelegramID(strconv.FormatInt(from.ID, 10))
	if err != nil || user.Status != common.UserStatusEnabled {
		return nil, fmt.Errorf("Tài khoản Telegram chưa được liên kết hoặc đang bị khóa. Hãy đăng nhập website và liên kết Telegram trước.")
	}
	return user, nil
}

func (bot *telegramBot) openSupport(message *telegramMessage, text string) error {
	if text == "" {
		if bot.supportURL != "" {
			return bot.sendMessage(message.Chat.ID, "Nhóm hỗ trợ: "+bot.supportURL)
		}
		if bot.supportChat != 0 {
			return bot.sendMessage(message.Chat.ID, "Gửi nội dung cần hỗ trợ theo cú pháp: /support nội dung")
		}
		return bot.sendMessage(message.Chat.ID, "Kênh hỗ trợ chưa được cấu hình.")
	}
	if utf8.RuneCountInString(text) > telegramMaxSupportRunes {
		return bot.sendMessage(message.Chat.ID, "Nội dung hỗ trợ tối đa 3500 ký tự.")
	}
	if bot.supportChat == 0 {
		if bot.supportURL != "" {
			return bot.sendMessage(message.Chat.ID, "Nhóm hỗ trợ: "+bot.supportURL)
		}
		return bot.sendMessage(message.Chat.ID, "Kênh hỗ trợ chưa được cấu hình.")
	}
	if message.From == nil || message.From.ID <= 0 {
		return nil
	}
	label := message.From.Username
	if label == "" {
		label = "ID " + strconv.FormatInt(message.From.ID, 10)
	}
	ticketCode := common.GenerateHMAC(strconv.FormatInt(message.From.ID, 10))
	ticket := fmt.Sprintf("[dgai-ticket:%d:%s]\nYêu cầu hỗ trợ từ %s:\n\n%s", message.From.ID, ticketCode, label, text)
	if err := bot.sendMessage(bot.supportChat, ticket); err != nil {
		common.SysLog(common.LogText("Telegram support ticket delivery failed: %s", err.Error()))
		return bot.sendMessage(message.Chat.ID, "Không gửi được yêu cầu hỗ trợ. Vui lòng thử lại sau.")
	}
	return bot.sendMessage(message.Chat.ID, "Đã gửi yêu cầu hỗ trợ. Nhân viên sẽ phản hồi tại đây.")
}

func (bot *telegramBot) handleSupportReply(message *telegramMessage) error {
	if message.From == nil || message.From.ID <= 0 || message.ReplyToMessage == nil || message.ReplyToMessage.From == nil || !message.ReplyToMessage.From.IsBot {
		return nil
	}
	userID, ok := telegramSupportTicketUser(message.ReplyToMessage.Text)
	if !ok || message.Text == "" {
		return nil
	}
	var member telegramChatMember
	if err := bot.callAPI("getChatMember", map[string]any{"chat_id": bot.supportChat, "user_id": message.From.ID}, &member); err != nil {
		return err
	}
	if member.Status != "administrator" && member.Status != "creator" {
		return nil
	}
	return bot.sendMessage(userID, "Phản hồi hỗ trợ:\n"+message.Text)
}

func telegramSupportTicketUser(text string) (int64, bool) {
	line, _, ok := strings.Cut(text, "\n")
	if !ok || !strings.HasPrefix(line, "[dgai-ticket:") || !strings.HasSuffix(line, "]") {
		return 0, false
	}
	parts := strings.Split(strings.TrimSuffix(strings.TrimPrefix(line, "[dgai-ticket:"), "]"), ":")
	if len(parts) != 2 {
		return 0, false
	}
	userID, err := strconv.ParseInt(parts[0], 10, 64)
	return userID, err == nil && userID > 0 && common.GenerateHMAC(parts[0]) == parts[1]
}

func parseTelegramCommand(text string) (string, string) {
	text = strings.TrimSpace(text)
	if !strings.HasPrefix(text, "/") {
		return "", ""
	}
	command, args, _ := strings.Cut(text[1:], " ")
	command, _, _ = strings.Cut(command, "@")
	return strings.ToLower(command), strings.TrimSpace(args)
}

func (bot *telegramBot) sendMessage(chatID int64, text string) error {
	return bot.callAPI("sendMessage", map[string]any{"chat_id": chatID, "text": text}, nil)
}

func (bot *telegramBot) callAPI(method string, params any, result any) error {
	body, err := common.Marshal(params)
	if err != nil {
		return err
	}
	request, err := http.NewRequest(http.MethodPost, fmt.Sprintf("%s/bot%s/%s", bot.apiBaseURL, bot.token, method), bytes.NewReader(body))
	if err != nil {
		return err
	}
	request.Header.Set("Content-Type", "application/json")
	return bot.doRequest(method, request, result)
}

func (bot *telegramBot) doRequest(method string, request *http.Request, result any) error {
	response, err := bot.client.Do(request)
	if err != nil {
		return fmt.Errorf("Telegram %s request failed", method)
	}
	defer response.Body.Close()
	if response.StatusCode != http.StatusOK {
		return fmt.Errorf("Telegram %s returned HTTP %d", method, response.StatusCode)
	}
	var envelope struct {
		OK          bool            `json:"ok"`
		Description string          `json:"description"`
		Result      json.RawMessage `json:"result"`
	}
	if err := common.DecodeJson(io.LimitReader(response.Body, telegramMaxResponseSize), &envelope); err != nil {
		return err
	}
	if !envelope.OK {
		return fmt.Errorf("Telegram %s: %s", method, envelope.Description)
	}
	if result == nil {
		return nil
	}
	return common.Unmarshal(envelope.Result, result)
}

package service

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/oauth"
	"github.com/QuantumNous/new-api/setting/system_setting"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestTelegramEnvOverrides(t *testing.T) {
	previousToken, previousEnabled := common.TelegramBotToken, common.TelegramOAuthEnabled
	previousSettings := *system_setting.GetTelegramSettings()
	t.Cleanup(func() {
		common.TelegramBotToken = previousToken
		common.TelegramOAuthEnabled = previousEnabled
		*system_setting.GetTelegramSettings() = previousSettings
	})
	t.Setenv("TELEGRAM_BOT_TOKEN", "123:test")
	t.Setenv("TELEGRAM_CLIENT_ID", "9252852374")
	t.Setenv("TELEGRAM_CLIENT_SECRET", "test-secret")
	t.Setenv("TELEGRAM_OAUTH_ENABLED", "true")

	system_setting.ApplyTelegramEnvOverrides()

	assert.Equal(t, "123:test", common.TelegramBotToken)
	assert.True(t, common.TelegramOAuthEnabled)
	assert.Equal(t, "9252852374", system_setting.GetTelegramSettings().ClientID)
	assert.Equal(t, "test-secret", system_setting.GetTelegramSettings().ClientSecret)
	assert.NoError(t, oauth.TelegramConfigurationError())

	common.TelegramBotToken = "saved-token"
	t.Setenv("TELEGRAM_BOT_TOKEN", "")
	system_setting.ApplyTelegramEnvOverrides()
	assert.Equal(t, "saved-token", common.TelegramBotToken)
}

func TestNewTelegramBotRequiresTokenAndReadsSupportChat(t *testing.T) {
	previousToken := common.TelegramBotToken
	t.Cleanup(func() { common.TelegramBotToken = previousToken })

	common.TelegramBotToken = ""
	assert.Nil(t, newTelegramBot())

	common.TelegramBotToken = "123:test"
	t.Setenv("DGAI_TELEGRAM_SUPPORT_CHAT_ID", "-1001")
	t.Setenv("DGAI_TELEGRAM_SUPPORT_URL", "https://t.me/dgai_support")
	bot := newTelegramBot()
	require.NotNil(t, bot)
	assert.Equal(t, "123:test", bot.token)
	assert.Equal(t, int64(-1001), bot.supportChat)
	assert.Equal(t, "https://t.me/dgai_support", bot.supportURL)
}

func TestTelegramPrivateCommandsReply(t *testing.T) {
	previousAddress := system_setting.ServerAddress
	system_setting.ServerAddress = "https://dgai.io.vn"
	t.Cleanup(func() { system_setting.ServerAddress = previousAddress })

	var sentText string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		if r.URL.Path != "/bottest-token/sendMessage" {
			http.NotFound(w, r)
			return
		}
		var payload struct {
			Text string `json:"text"`
		}
		require.NoError(t, common.DecodeJson(r.Body, &payload))
		sentText = payload.Text
		_, _ = w.Write([]byte(`{"ok":true,"result":{}}`))
	}))
	defer server.Close()

	bot := &telegramBot{token: "test-token", apiBaseURL: server.URL, client: server.Client()}
	message := &telegramMessage{Chat: telegramChat{ID: 42, Type: "private"}, From: &telegramUser{ID: 42}}

	require.NoError(t, bot.handlePrivateMessage(&telegramMessage{
		Chat: message.Chat, From: message.From, Text: "/start",
	}))
	assert.Contains(t, sentText, "DGai hỗ trợ")

	require.NoError(t, bot.handlePrivateMessage(&telegramMessage{
		Chat: message.Chat, From: message.From, Text: "/topup",
	}))
	assert.Contains(t, sentText, "https://dgai.io.vn/wallet")
}

func TestParseTelegramCommand(t *testing.T) {
	command, args := parseTelegramCommand("  /ReDeEm@dgai_bot  CODE-123  ")
	assert.Equal(t, "redeem", command)
	assert.Equal(t, "CODE-123", args)
	command, args = parseTelegramCommand("hello")
	assert.Empty(t, command)
	assert.Empty(t, args)
}

func TestTelegramBotIgnoresAccountCommandsInGroups(t *testing.T) {
	bot := &telegramBot{supportChat: -1001}
	err := bot.handleUpdate(telegramUpdate{Message: &telegramMessage{
		Text: "/balance", Chat: telegramChat{ID: -1002, Type: "supergroup"},
		From: &telegramUser{ID: 42},
	}})
	require.NoError(t, err)
}

func TestTelegramSupportReplyIgnoresNonAdmin(t *testing.T) {
	sentMessage := false
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		if r.URL.Path == "/bottest-token/getChatMember" {
			_, _ = w.Write([]byte(`{"ok":true,"result":{"status":"member"}}`))
			return
		}
		sentMessage = true
		_, _ = w.Write([]byte(`{"ok":true,"result":{}}`))
	}))
	defer server.Close()

	bot := &telegramBot{token: "test-token", apiBaseURL: server.URL, client: server.Client(), supportChat: -1001}
	err := bot.handleSupportReply(&telegramMessage{
		From: &telegramUser{ID: 7}, Text: "forged reply",
		ReplyToMessage: &telegramMessage{Text: "[dgai-ticket:42]\nrequest", From: &telegramUser{IsBot: true}},
	})
	require.NoError(t, err)
	assert.False(t, sentMessage)
}

func TestTelegramSupportTicketUserAndAdminReply(t *testing.T) {
	validTicket := "[dgai-ticket:42:" + common.GenerateHMAC("42") + "]\nquestion"
	userID, ok := telegramSupportTicketUser(validTicket)
	require.True(t, ok)
	assert.EqualValues(t, 42, userID)
	_, ok = telegramSupportTicketUser("[dgai-ticket:42:forged]\nquestion")
	assert.False(t, ok)
	var sentTo int64
	var sentText string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch r.URL.Path {
		case "/bottest-token/getChatMember":
			_, _ = w.Write([]byte(`{"ok":true,"result":{"status":"administrator"}}`))
		case "/bottest-token/sendMessage":
			var payload struct {
				ChatID int64  `json:"chat_id"`
				Text   string `json:"text"`
			}
			require.NoError(t, common.DecodeJson(r.Body, &payload))
			sentTo, sentText = payload.ChatID, payload.Text
			_, _ = w.Write([]byte(`{"ok":true,"result":{}}`))
		default:
			http.NotFound(w, r)
		}
	}))
	defer server.Close()

	bot := &telegramBot{token: "test-token", apiBaseURL: server.URL, client: server.Client(), supportChat: -1001}
	err := bot.handleSupportReply(&telegramMessage{
		Chat: telegramChat{ID: -1001, Type: "supergroup"},
		From: &telegramUser{ID: 7},
		Text: "Resolved",
		ReplyToMessage: &telegramMessage{
			Text: "[dgai-ticket:42:" + common.GenerateHMAC("42") + "]\nrequest",
			From: &telegramUser{IsBot: true},
		},
	})
	require.NoError(t, err)
	assert.EqualValues(t, 42, sentTo)
	assert.Equal(t, "Phản hồi hỗ trợ:\nResolved", sentText)
}

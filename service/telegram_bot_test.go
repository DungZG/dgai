package service

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/model"
	"github.com/QuantumNous/new-api/oauth"
	"github.com/QuantumNous/new-api/setting/operation_setting"
	"github.com/QuantumNous/new-api/setting/system_setting"
	"github.com/glebarez/sqlite"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
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
	var sentMarkup json.RawMessage
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		if r.URL.Path != "/bottest-token/sendMessage" {
			http.NotFound(w, r)
			return
		}
		var payload struct {
			Text        string          `json:"text"`
			ReplyMarkup json.RawMessage `json:"reply_markup"`
		}
		require.NoError(t, common.DecodeJson(r.Body, &payload))
		sentText = payload.Text
		sentMarkup = payload.ReplyMarkup
		_, _ = w.Write([]byte(`{"ok":true,"result":{}}`))
	}))
	defer server.Close()

	bot := &telegramBot{token: "test-token", apiBaseURL: server.URL, client: server.Client()}
	message := &telegramMessage{Chat: telegramChat{ID: 42, Type: "private"}, From: &telegramUser{ID: 42}}

	require.NoError(t, bot.handlePrivateMessage(&telegramMessage{
		Chat: message.Chat, From: message.From, Text: "/start",
	}))
	assert.Contains(t, sentText, "DGAI API")
	assert.Contains(t, sentText, "Nạp tiền")

	require.NoError(t, bot.handlePrivateMessage(&telegramMessage{
		Chat: message.Chat, From: message.From, Text: "/topup",
	}))
	assert.Contains(t, sentText, "Nạp tiền")
	assert.Contains(t, string(sentMarkup), "$10")
}

func TestTelegramTopUpCallbackSendsQR(t *testing.T) {
	previousDB := model.DB
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	require.NoError(t, err)
	require.NoError(t, db.AutoMigrate(&model.User{}))
	user := &model.User{
		Username:   "tg-topup",
		Password:   "unused-password",
		Status:     common.UserStatusEnabled,
		TelegramId: "42",
		Group:      "default",
	}
	require.NoError(t, db.Create(user).Error)
	model.DB = db
	t.Cleanup(func() {
		model.DB = previousDB
		sqlDB, err := db.DB()
		if err == nil {
			_ = sqlDB.Close()
		}
	})

	previousDisplay := operation_setting.GetGeneralSetting().QuotaDisplayType
	operation_setting.GetGeneralSetting().QuotaDisplayType = operation_setting.QuotaDisplayTypeUSD
	t.Cleanup(func() {
		operation_setting.GetGeneralSetting().QuotaDisplayType = previousDisplay
	})

	previousCreator := createVietQRTopUp
	t.Cleanup(func() { createVietQRTopUp = previousCreator })
	SetVietQRTopUpCreator(func(_ context.Context, userId int, amount int64) (*VietQRTopUpResult, error) {
		require.Equal(t, user.Id, userId)
		require.EqualValues(t, 10, amount)
		return &VietQRTopUpResult{
			TradeNo:      "VQR12345678",
			Amount:       10,
			PayMoney:     10,
			Memo:         "VQR-VIETQR-CONTENT",
			TransferMemo: "DGAIVQR12345678",
			QRImageURL:   "https://img.vietqr.io/image/TPB-100-test.png",
			VNDAmount:    265000,
			BankCode:     "TPB",
			BankAccount:  "10006460824",
			AccountName:  "NGUYEN TRI DUNG",
		}, nil
	})

	var answered bool
	var photo, caption string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch r.URL.Path {
		case "/bottest-token/answerCallbackQuery":
			answered = true
			_, _ = w.Write([]byte(`{"ok":true,"result":true}`))
		case "/bottest-token/sendPhoto":
			var payload struct {
				Photo   string `json:"photo"`
				Caption string `json:"caption"`
			}
			require.NoError(t, common.DecodeJson(r.Body, &payload))
			photo, caption = payload.Photo, payload.Caption
			_, _ = w.Write([]byte(`{"ok":true,"result":{}}`))
		default:
			http.NotFound(w, r)
		}
	}))
	defer server.Close()

	bot := &telegramBot{token: "test-token", apiBaseURL: server.URL, client: server.Client()}
	err = bot.handleCallbackQuery(&telegramCallbackQuery{
		ID:   "cb1",
		From: &telegramUser{ID: 42},
		Message: &telegramMessage{
			MessageID: 1,
			Chat:      telegramChat{ID: 42, Type: "private"},
		},
		Data: "topup:10",
	})
	require.NoError(t, err)
	assert.True(t, answered)
	assert.Equal(t, "https://img.vietqr.io/image/TPB-100-test.png", photo)
	assert.Contains(t, caption, "Nạp $10")
	assert.Contains(t, caption, "265.000 ₫")
	assert.Contains(t, caption, "DGAIVQR12345678")
	assert.NotContains(t, caption, "VQR-VIETQR-CONTENT")
}

func TestModelPriceLine(t *testing.T) {
	tokenModel := model.Pricing{ModelName: "token-model", QuotaType: 0, ModelRatio: 1, CompletionRatio: 3}
	assert.Equal(t, "$2.00 in / $6.00 out / 1M", modelPriceLine(tokenModel, 1))

	requestModel := model.Pricing{ModelName: "request-model", QuotaType: 1, ModelPrice: 0.5}
	assert.Equal(t, "$0.50 / request", modelPriceLine(requestModel, 1))
	assert.Equal(t, "—", modelPriceLine(model.Pricing{}, 1))
}

func TestFormatVND(t *testing.T) {
	assert.Equal(t, "265.000", formatVND(265000))
	assert.Equal(t, "1.000.000", formatVND(1000000))
	assert.Equal(t, "999", formatVND(999))
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

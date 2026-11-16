package system_setting

import (
	"os"
	"strings"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/setting/config"
)

type TelegramSettings struct {
	ClientID     string `json:"client_id"`
	ClientSecret string `json:"client_secret"`
}

var telegramSettings TelegramSettings

func init() {
	config.GlobalConfig.Register("telegram", &telegramSettings)
}

// ApplyTelegramEnvOverrides keeps deployment secrets out of the database and
// lets an operator rotate them by restarting the process. An explicitly set
// environment variable wins over the stored option.
func ApplyTelegramEnvOverrides() {
	// Compose always injects these keys, often with an empty value. Treat blank
	// as "not provided" so a token saved in the console is not erased.
	if value := strings.TrimSpace(os.Getenv("TELEGRAM_CLIENT_ID")); value != "" {
		telegramSettings.ClientID = value
	}
	if value := strings.TrimSpace(os.Getenv("TELEGRAM_CLIENT_SECRET")); value != "" {
		telegramSettings.ClientSecret = value
	}
	if value := strings.TrimSpace(os.Getenv("TELEGRAM_OAUTH_ENABLED")); value != "" {
		common.TelegramOAuthEnabled = strings.EqualFold(value, "true")
	}
	if value := strings.TrimSpace(os.Getenv("TELEGRAM_BOT_TOKEN")); value != "" {
		common.TelegramBotToken = value
	}
}

func GetTelegramSettings() *TelegramSettings {
	return &telegramSettings
}

func (s *TelegramSettings) IsConfigured() bool {
	return strings.TrimSpace(s.ClientID) != "" && strings.TrimSpace(s.ClientSecret) != ""
}

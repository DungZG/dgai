package service

import (
	"context"
	"errors"
	"fmt"
	"html"
	"maps"
	"slices"
	"strconv"
	"strings"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/logger"
	"github.com/QuantumNous/new-api/model"
	perfmetrics "github.com/QuantumNous/new-api/pkg/perf_metrics"
	"github.com/QuantumNous/new-api/setting/operation_setting"
	"github.com/QuantumNous/new-api/setting/ratio_setting"
	"github.com/QuantumNous/new-api/setting/system_setting"
)

type telegramInlineKeyboardButton struct {
	Text         string `json:"text"`
	CallbackData string `json:"callback_data,omitempty"`
	URL          string `json:"url,omitempty"`
}

type telegramInlineKeyboardMarkup struct {
	InlineKeyboard [][]telegramInlineKeyboardButton `json:"inline_keyboard"`
}

func callbackButton(text, data string) telegramInlineKeyboardButton {
	return telegramInlineKeyboardButton{Text: text, CallbackData: data}
}

func urlButton(text, target string) telegramInlineKeyboardButton {
	return telegramInlineKeyboardButton{Text: text, URL: target}
}

func keyboard(rows ...[]telegramInlineKeyboardButton) *telegramInlineKeyboardMarkup {
	return &telegramInlineKeyboardMarkup{InlineKeyboard: rows}
}

func (bot *telegramBot) handleCallbackQuery(query *telegramCallbackQuery) error {
	if query == nil {
		return nil
	}
	from := query.From
	if from == nil && query.Message != nil {
		from = query.Message.From
	}
	if query.Message == nil || query.Message.Chat.Type != "private" {
		return bot.answerCallbackQuery(query.ID, "Mở chat riêng với bot để dùng mục này.")
	}
	chatID := query.Message.Chat.ID
	messageID := query.Message.MessageID

	switch {
	case query.Data == "menu":
		_ = bot.answerCallbackQuery(query.ID, "")
		return bot.showMainMenu(chatID, messageID, from)
	case query.Data == "topup":
		_ = bot.answerCallbackQuery(query.ID, "")
		return bot.showTopUpMenu(chatID, messageID, from)
	case strings.HasPrefix(query.Data, "topup:"):
		amount, err := strconv.ParseInt(strings.TrimPrefix(query.Data, "topup:"), 10, 64)
		if err != nil || amount <= 0 {
			_ = bot.answerCallbackQuery(query.ID, "Số tiền không hợp lệ.")
			return nil
		}
		_ = bot.answerCallbackQuery(query.ID, "Đang tạo QR...")
		return bot.createTopUp(chatID, from, amount)
	case strings.HasPrefix(query.Data, "topupcheck:"):
		_ = bot.answerCallbackQuery(query.ID, "Đang kiểm tra...")
		return bot.checkTopUp(chatID, from, strings.TrimPrefix(query.Data, "topupcheck:"))
	case query.Data == "models":
		_ = bot.answerCallbackQuery(query.ID, "")
		return bot.showModels(chatID, messageID, from)
	case query.Data == "balance":
		_ = bot.answerCallbackQuery(query.ID, "")
		return bot.showBalance(chatID, messageID, from)
	case query.Data == "support":
		_ = bot.answerCallbackQuery(query.ID, "")
		return bot.showSupport(chatID, messageID)
	case query.Data == "redeem":
		_ = bot.answerCallbackQuery(query.ID, "")
		return bot.showRedeem(chatID, messageID)
	default:
		return bot.answerCallbackQuery(query.ID, "")
	}
}

func (bot *telegramBot) showMainMenu(chatID, messageID int64, from *telegramUser) error {
	text, markup := bot.mainMenuContent(from)
	return bot.replaceCard(chatID, messageID, text, markup)
}

func (bot *telegramBot) mainMenuContent(from *telegramUser) (string, *telegramInlineKeyboardMarkup) {
	linked := false
	balance := "—"
	if user, err := bot.linkedUser(from); err == nil {
		linked = true
		if quota, err := model.GetUserQuota(user.Id, true); err == nil {
			balance = formatQuotaDisplay(quota)
		}
	}
	status := "🔒 Chưa liên kết"
	if linked {
		status = "🔓 Đã liên kết"
	}
	text := fmt.Sprintf(`💎 <b>DGAI API</b>
Trạng thái: %s

Nền tảng API AI đa nhà cung cấp, ví USD và nạp tự động.

⚡ <b>Bạn có thể:</b>
• 💳 Nạp tiền bằng QR ngay trong chat
• 📊 Xem tốc độ &amp; giá model
• 🎁 Đổi mã quà tặng
• 🆘 Gửi yêu cầu hỗ trợ

💰 Số dư: <b>%s</b>`, status, balance)

	rows := [][]telegramInlineKeyboardButton{
		{callbackButton("💳 Nạp tiền", "topup"), callbackButton("📊 Model & giá", "models")},
		{callbackButton("💰 Số dư", "balance"), callbackButton("🎁 Đổi mã", "redeem")},
		{callbackButton("🆘 Hỗ trợ", "support")},
	}
	if !linked {
		if profile := bot.profileURL(); profile != "" {
			rows = append(rows, []telegramInlineKeyboardButton{urlButton("🔗 Liên kết Telegram", profile)})
		}
	}
	if docs := bot.docsURL(); docs != "" {
		rows = append(rows, []telegramInlineKeyboardButton{urlButton("🌐 API Docs", docs)})
	}
	return text, keyboard(rows...)
}

func (bot *telegramBot) showTopUpMenu(chatID, messageID int64, from *telegramUser) error {
	linked := false
	balance := "—"
	if user, err := bot.linkedUser(from); err == nil {
		linked = true
		if quota, err := model.GetUserQuota(user.Id, true); err == nil {
			balance = formatQuotaDisplay(quota)
		}
	}
	text := fmt.Sprintf(`💳 <b>Nạp tiền</b>
Chọn gói bên dưới. Bot tạo QR và gửi ngay trong chat.

💰 Số dư: <b>%s</b>
⌨️ Số khác: <code>/topup 25</code>`, balance)
	if !linked {
		text += "\n\n🔒 Liên kết Telegram để nạp tiền."
	}

	rows := make([][]telegramInlineKeyboardButton, 0, 5)
	for i, amount := range topUpPackages() {
		label := formatTopUpAmount(amount)
		if discount, ok := operation_setting.GetPaymentSetting().AmountDiscount[int(amount)]; ok && discount > 0 && discount < 1 {
			label += fmt.Sprintf(" -%d%%", int((1-discount)*100))
		}
		button := callbackButton(label, "topup:"+strconv.FormatInt(amount, 10))
		if i%2 == 0 {
			rows = append(rows, []telegramInlineKeyboardButton{button})
		} else {
			rows[len(rows)-1] = append(rows[len(rows)-1], button)
		}
	}
	if !linked {
		if profile := bot.profileURL(); profile != "" {
			rows = append(rows, []telegramInlineKeyboardButton{urlButton("🔗 Liên kết Telegram", profile)})
		}
	}
	rows = append(rows, []telegramInlineKeyboardButton{callbackButton("🏠 Menu", "menu")})
	return bot.replaceCard(chatID, messageID, text, keyboard(rows...))
}

func (bot *telegramBot) showBalance(chatID, messageID int64, from *telegramUser) error {
	user, err := bot.linkedUser(from)
	if err != nil {
		return bot.sendLinkPrompt(chatID)
	}
	quota, err := model.GetUserQuota(user.Id, true)
	if err != nil {
		return bot.sendMessage(chatID, "Không đọc được số dư. Vui lòng thử lại sau.")
	}
	text := fmt.Sprintf(`💰 <b>Số dư</b>
<b>%s</b>

Tài khoản: <code>%s</code>`, formatQuotaDisplay(quota), html.EscapeString(user.Username))
	return bot.replaceCard(chatID, messageID, text, keyboard(
		[]telegramInlineKeyboardButton{callbackButton("💳 Nạp tiền", "topup")},
		[]telegramInlineKeyboardButton{callbackButton("🏠 Menu", "menu")},
	))
}

func (bot *telegramBot) showModels(chatID, messageID int64, from *telegramUser) error {
	group := "default"
	if user, err := bot.linkedUser(from); err == nil {
		if userGroup, err := model.GetUserGroup(user.Id, true); err == nil && userGroup != "" {
			group = userGroup
		}
	}
	text, err := telegramModelStatus(group)
	if err != nil {
		return bot.sendMessage(chatID, "Không đọc được thống kê model. Vui lòng thử lại sau.")
	}
	return bot.replaceCard(chatID, messageID, text, keyboard(
		[]telegramInlineKeyboardButton{callbackButton("🔄 Làm mới", "models")},
		[]telegramInlineKeyboardButton{callbackButton("🏠 Menu", "menu")},
	))
}

func (bot *telegramBot) showModelsGroup(chatID int64) error {
	text, err := telegramModelStatus("default")
	if err != nil {
		return bot.sendMessage(chatID, "Không đọc được thống kê model. Vui lòng thử lại sau.")
	}
	return bot.sendCard(chatID, text, nil)
}

func (bot *telegramBot) showSupport(chatID, messageID int64) error {
	text := `🆘 <b>Hỗ trợ</b>
Gửi nội dung theo cú pháp:
<code>/support nội dung cần hỗ trợ</code>`
	rows := make([][]telegramInlineKeyboardButton, 0, 2)
	if bot.supportURL != "" {
		rows = append(rows, []telegramInlineKeyboardButton{urlButton("💬 Nhóm hỗ trợ", bot.supportURL)})
	} else if bot.supportChat == 0 {
		text += "\n\nKênh hỗ trợ chưa được cấu hình."
	}
	rows = append(rows, []telegramInlineKeyboardButton{callbackButton("🏠 Menu", "menu")})
	return bot.replaceCard(chatID, messageID, text, keyboard(rows...))
}

func (bot *telegramBot) showRedeem(chatID, messageID int64) error {
	text := `🎁 <b>Đổi mã quà tặng</b>
Gửi: <code>/redeem MÃ_QUÀ_TẶNG</code>`
	return bot.replaceCard(chatID, messageID, text, keyboard(
		[]telegramInlineKeyboardButton{callbackButton("🏠 Menu", "menu")},
	))
}

func (bot *telegramBot) sendLinkPrompt(chatID int64) error {
	text := `🔒 <b>Chưa liên kết Telegram</b>
Đăng nhập website, vào Hồ sơ → Liên kết Telegram, sau đó quay lại đây.`
	rows := make([][]telegramInlineKeyboardButton, 0, 2)
	if profile := bot.profileURL(); profile != "" {
		rows = append(rows, []telegramInlineKeyboardButton{urlButton("🔗 Mở trang cá nhân", profile)})
	}
	rows = append(rows, []telegramInlineKeyboardButton{callbackButton("🏠 Menu", "menu")})
	return bot.sendCard(chatID, text, keyboard(rows...))
}

func (bot *telegramBot) createTopUpFromCommand(chatID int64, from *telegramUser, rawAmount string) error {
	amount, err := strconv.ParseInt(strings.TrimSpace(rawAmount), 10, 64)
	if err != nil || amount <= 0 {
		return bot.sendMessage(chatID, "Cách dùng: /topup hoặc /topup 25")
	}
	return bot.createTopUp(chatID, from, amount)
}

func (bot *telegramBot) createTopUp(chatID int64, from *telegramUser, amount int64) error {
	user, err := bot.linkedUser(from)
	if err != nil {
		return bot.sendLinkPrompt(chatID)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	result, err := CreateVietQRTopUp(ctx, user.Id, amount)
	if err != nil {
		return bot.sendCard(chatID, topUpErrorMessage(err), keyboard(
			[]telegramInlineKeyboardButton{callbackButton("🔄 Thử lại", "topup")},
			[]telegramInlineKeyboardButton{callbackButton("🏠 Menu", "menu")},
		))
	}
	caption := qrCaption(result)
	markup := qrKeyboard(result.TradeNo)
	if err := bot.sendPhotoCard(chatID, result.QRImageURL, caption, markup); err != nil {
		return bot.sendCard(chatID, caption+"\n\n"+html.EscapeString(result.QRImageURL), markup)
	}
	return nil
}

func (bot *telegramBot) checkTopUp(chatID int64, from *telegramUser, tradeNo string) error {
	user, err := bot.linkedUser(from)
	if err != nil {
		return bot.sendLinkPrompt(chatID)
	}
	topUp := model.GetTopUpByTradeNo(tradeNo)
	if topUp == nil || topUp.UserId != user.Id {
		return bot.sendCard(chatID, "Không tìm thấy đơn nạp này.", keyboard(
			[]telegramInlineKeyboardButton{callbackButton("🏠 Menu", "menu")},
		))
	}
	switch topUp.Status {
	case common.TopUpStatusSuccess:
		quota, _ := model.GetUserQuota(user.Id, true)
		text := fmt.Sprintf("✅ <b>Đã thanh toán</b>\nMã đơn: <code>%s</code>\nSố dư: <b>%s</b>",
			html.EscapeString(tradeNo), formatQuotaDisplay(quota))
		return bot.sendCard(chatID, text, keyboard(
			[]telegramInlineKeyboardButton{callbackButton("🏠 Menu", "menu")},
		))
	case common.TopUpStatusPending:
		return bot.sendCard(chatID, "⏳ <b>Chưa thấy thanh toán</b>\nChuyển đúng nội dung và số tiền, sau đó thử lại.", keyboard(
			[]telegramInlineKeyboardButton{callbackButton("🔄 Kiểm tra lại", "topupcheck:"+tradeNo)},
			[]telegramInlineKeyboardButton{callbackButton("🏠 Menu", "menu")},
		))
	case common.TopUpStatusExpired:
		return bot.sendCard(chatID, "⌛ Đơn nạp đã hết hạn. Tạo đơn mới bên dưới.", keyboard(
			[]telegramInlineKeyboardButton{callbackButton("💳 Nạp lại", "topup")},
			[]telegramInlineKeyboardButton{callbackButton("🏠 Menu", "menu")},
		))
	default:
		return bot.sendCard(chatID, "❌ Đơn nạp không thành công. Tạo đơn mới bên dưới.", keyboard(
			[]telegramInlineKeyboardButton{callbackButton("💳 Nạp lại", "topup")},
			[]telegramInlineKeyboardButton{callbackButton("🏠 Menu", "menu")},
		))
	}
}

func (bot *telegramBot) sendCard(chatID int64, text string, markup *telegramInlineKeyboardMarkup) error {
	params := map[string]any{"chat_id": chatID, "text": text, "parse_mode": "HTML"}
	if markup != nil {
		params["reply_markup"] = markup
	}
	return bot.callAPI("sendMessage", params, nil)
}

func (bot *telegramBot) editCard(chatID, messageID int64, text string, markup *telegramInlineKeyboardMarkup) error {
	params := map[string]any{
		"chat_id":    chatID,
		"message_id": messageID,
		"text":       text,
		"parse_mode": "HTML",
	}
	if markup != nil {
		params["reply_markup"] = markup
	}
	return bot.callAPI("editMessageText", params, nil)
}

func (bot *telegramBot) replaceCard(chatID, messageID int64, text string, markup *telegramInlineKeyboardMarkup) error {
	if messageID > 0 {
		if err := bot.editCard(chatID, messageID, text, markup); err == nil || strings.Contains(err.Error(), "message is not modified") {
			return nil
		}
	}
	return bot.sendCard(chatID, text, markup)
}

func (bot *telegramBot) sendPhotoCard(chatID int64, photo, caption string, markup *telegramInlineKeyboardMarkup) error {
	params := map[string]any{
		"chat_id":    chatID,
		"photo":      photo,
		"caption":    caption,
		"parse_mode": "HTML",
	}
	if markup != nil {
		params["reply_markup"] = markup
	}
	return bot.callAPI("sendPhoto", params, nil)
}

func (bot *telegramBot) answerCallbackQuery(id, text string) error {
	params := map[string]any{"callback_query_id": id}
	if text != "" {
		params["text"] = text
	}
	return bot.callAPI("answerCallbackQuery", params, nil)
}

func (bot *telegramBot) serverBaseURL() string {
	base := strings.TrimRight(strings.TrimSpace(system_setting.ServerAddress), "/")
	if base == "" {
		base = "https://dgai.io.vn"
	}
	return base
}

func (bot *telegramBot) profileURL() string {
	return bot.serverBaseURL() + "/profile"
}

func (bot *telegramBot) docsURL() string {
	return bot.serverBaseURL() + "/docs/"
}

func topUpPackages() []int64 {
	options := operation_setting.GetPaymentSetting().AmountOptions
	if len(options) == 0 {
		options = []int{10, 20, 50, 100, 200, 500}
	}
	minAmount := int64(operation_setting.MinTopUp)
	if minAmount <= 0 {
		minAmount = 1
	}
	seen := make(map[int64]bool, len(options))
	result := make([]int64, 0, len(options))
	for _, option := range options {
		amount := int64(option)
		if amount < minAmount || seen[amount] {
			continue
		}
		seen[amount] = true
		result = append(result, amount)
	}
	if len(result) == 0 {
		result = append(result, minAmount)
	}
	if len(result) > 8 {
		result = result[:8]
	}
	return result
}

func formatTopUpAmount(amount int64) string {
	switch operation_setting.GetQuotaDisplayType() {
	case operation_setting.QuotaDisplayTypeTokens:
		return fmt.Sprintf("%d tokens", amount)
	case operation_setting.QuotaDisplayTypeCNY:
		return fmt.Sprintf("¥%d", amount)
	case operation_setting.QuotaDisplayTypeCustom:
		symbol := operation_setting.GetGeneralSetting().CustomCurrencySymbol
		if symbol == "" {
			symbol = "¤"
		}
		return fmt.Sprintf("%s%d", symbol, amount)
	default:
		return fmt.Sprintf("$%d", amount)
	}
}

func formatQuotaDisplay(quota int) string {
	if common.QuotaPerUnit > 0 && operation_setting.GetQuotaDisplayType() == operation_setting.QuotaDisplayTypeUSD {
		return fmt.Sprintf("$%.2f", float64(quota)/common.QuotaPerUnit)
	}
	return logger.FormatQuota(quota)
}

func qrCaption(result *VietQRTopUpResult) string {
	memo := result.TransferMemo
	if memo == "" {
		memo = result.Memo
	}
	return fmt.Sprintf(`💳 <b>Nạp %s</b>
Quét QR để chuyển khoản.

🏦 <b>%s</b> · <code>%s</code>
👤 %s
💵 <b>%s ₫</b>
📝 Nội dung: <code>%s</code>
🧾 Mã đơn: <code>%s</code>

⏳ Hết hạn sau 30 phút. Số dư tự cộng khi SePay xác nhận.`,
		html.EscapeString(formatTopUpAmount(result.Amount)),
		html.EscapeString(result.BankCode),
		html.EscapeString(result.BankAccount),
		html.EscapeString(result.AccountName),
		formatVND(result.VNDAmount),
		html.EscapeString(memo),
		html.EscapeString(result.TradeNo))
}

func qrKeyboard(tradeNo string) *telegramInlineKeyboardMarkup {
	return keyboard(
		[]telegramInlineKeyboardButton{callbackButton("🔄 Kiểm tra thanh toán", "topupcheck:"+tradeNo)},
		[]telegramInlineKeyboardButton{callbackButton("🏠 Menu", "menu")},
	)
}

func topUpErrorMessage(err error) string {
	var topUpErr *VietQRTopUpError
	if errors.As(err, &topUpErr) {
		switch topUpErr.Code {
		case VietQRTopUpErrorNotConfigured:
			return "⚠️ Nạp tiền VietQR chưa được cấu hình. Vui lòng liên hệ quản trị viên."
		case VietQRTopUpErrorAmountTooLow:
			return "⚠️ Số tiền nạp quá nhỏ. Chọn gói khác hoặc dùng /topup <số>."
		case VietQRTopUpErrorQuotaLimit:
			return "⚠️ Ví đã đạt giới hạn số dư. Không thể nạp thêm."
		case VietQRTopUpErrorPaymentLow:
			return "⚠️ Số tiền thanh toán quá nhỏ."
		case VietQRTopUpErrorUser:
			return "⚠️ Không đọc được tài khoản. Vui lòng thử lại sau."
		default:
			return "❌ Không tạo được đơn nạp. Vui lòng thử lại sau."
		}
	}
	return "❌ Không tạo được đơn nạp. Vui lòng thử lại sau."
}

func telegramModelStatus(group string) (string, error) {
	groups := slices.Collect(maps.Keys(ratio_setting.GetGroupRatioCopy()))
	groups = append(groups, "auto")
	slices.Sort(groups)
	summary, err := perfmetrics.QuerySummaryAll(24, groups)
	if err != nil {
		return "", err
	}
	if len(summary.Models) == 0 {
		return "📊 <b>Model &amp; giá</b>\n\nChưa có lượt gọi nào trong 24 giờ qua.", nil
	}

	pricing := make(map[string]model.Pricing, len(summary.Models))
	for _, item := range model.GetPricing() {
		pricing[item.ModelName] = item
	}
	groupRatio := ratio_setting.GetGroupRatio(group)
	if groupRatio <= 0 {
		groupRatio = 1
	}

	var text strings.Builder
	fmt.Fprintf(&text, "📊 <b>Model &amp; giá</b>\n24 giờ gần nhất · nhóm <code>%s</code>\n", html.EscapeString(group))
	limit := min(len(summary.Models), 8)
	for _, item := range summary.Models[:limit] {
		latency := item.P50LatencyMs
		if latency <= 0 {
			latency = item.AvgLatencyMs
		}
		fmt.Fprintf(&text, "\n<b>%s</b>\n", html.EscapeString(item.ModelName))
		fmt.Fprintf(&text, "✅ %.2f%% · ⚡ %s · 🚀 %s\n",
			item.SuccessRate, formatLatency(latency), formatTPS(item.AvgTps))
		fmt.Fprintf(&text, "💰 %s · 📞 %s\n",
			modelPriceLine(pricing[item.ModelName], groupRatio), formatCount(item.RequestCount))
	}
	if len(summary.Models) > limit {
		fmt.Fprintf(&text, "\n… còn %d model khác\n", len(summary.Models)-limit)
	}
	return text.String(), nil
}

func modelPriceLine(pricing model.Pricing, groupRatio float64) string {
	if pricing.ModelName == "" {
		return "—"
	}
	if pricing.QuotaType == 1 {
		return formatUSD(pricing.ModelPrice*groupRatio) + " / request"
	}
	if pricing.ModelRatio > 0 {
		input := pricing.ModelRatio * 2 * groupRatio
		output := input
		if pricing.CompletionRatio > 0 {
			output = input * pricing.CompletionRatio
		}
		return fmt.Sprintf("%s in / %s out / 1M", formatUSD(input), formatUSD(output))
	}
	if pricing.BillingExpr != "" {
		return "theo bậc"
	}
	return "—"
}

func formatUSD(value float64) string {
	switch {
	case value >= 100:
		return fmt.Sprintf("$%.0f", value)
	case value >= 0.1:
		return fmt.Sprintf("$%.2f", value)
	case value >= 0.01:
		return fmt.Sprintf("$%.3f", value)
	case value >= 0.001:
		return fmt.Sprintf("$%.4f", value)
	default:
		return fmt.Sprintf("$%.6f", value)
	}
}

func formatLatency(ms int64) string {
	if ms <= 0 {
		return "—"
	}
	if ms < 1000 {
		return fmt.Sprintf("%dms", ms)
	}
	return fmt.Sprintf("%.1fs", float64(ms)/1000)
}

func formatTPS(tps float64) string {
	if tps <= 0 {
		return "—"
	}
	return fmt.Sprintf("%.1f tok/s", tps)
}

func formatCount(count int64) string {
	switch {
	case count >= 1_000_000:
		return fmt.Sprintf("%.1fM", float64(count)/1_000_000)
	case count >= 1_000:
		return fmt.Sprintf("%.1fk", float64(count)/1_000)
	default:
		return strconv.FormatInt(count, 10)
	}
}

func formatVND(amount int64) string {
	raw := strconv.FormatInt(amount, 10)
	if len(raw) <= 3 {
		return raw
	}
	var text strings.Builder
	for i, r := range raw {
		if i > 0 && (len(raw)-i)%3 == 0 {
			text.WriteByte('.')
		}
		text.WriteRune(r)
	}
	return text.String()
}

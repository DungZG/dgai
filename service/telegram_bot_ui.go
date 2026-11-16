package service

import (
	"bytes"
	"cmp"
	"context"
	"errors"
	"fmt"
	"html"
	"image/png"
	"maps"
	"mime/multipart"
	"net/http"
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
	"github.com/boombuler/barcode"
	"github.com/boombuler/barcode/qr"
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
	case query.Data == "plans":
		_ = bot.answerCallbackQuery(query.ID, "")
		return bot.showPlans(chatID, messageID, from)
	case strings.HasPrefix(query.Data, "plan:"):
		planID, err := strconv.Atoi(strings.TrimPrefix(query.Data, "plan:"))
		if err != nil {
			_ = bot.answerCallbackQuery(query.ID, "Gói không hợp lệ.")
			return nil
		}
		_ = bot.answerCallbackQuery(query.ID, "")
		return bot.showPlanDetail(chatID, messageID, planID)
	case strings.HasPrefix(query.Data, "planbuy:"):
		planID, err := strconv.Atoi(strings.TrimPrefix(query.Data, "planbuy:"))
		if err != nil {
			_ = bot.answerCallbackQuery(query.ID, "Gói không hợp lệ.")
			return nil
		}
		_ = bot.answerCallbackQuery(query.ID, "Đang xử lý...")
		return bot.buyPlan(chatID, from, planID)
	case query.Data == "mysubs":
		_ = bot.answerCallbackQuery(query.ID, "")
		return bot.showMySubscriptions(chatID, messageID, from)
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
• 📦 Mua gói đăng ký bằng số dư
• 📊 Xem tốc độ &amp; giá model
• 🎁 Đổi mã quà tặng
• 🆘 Gửi yêu cầu hỗ trợ

💰 Số dư: <b>%s</b>`, status, balance)

	rows := [][]telegramInlineKeyboardButton{
		{callbackButton("💳 Nạp tiền", "topup"), callbackButton("📦 Gói đăng ký", "plans")},
		{callbackButton("📊 Model & giá", "models"), callbackButton("💰 Số dư", "balance")},
		{callbackButton("🎁 Đổi mã", "redeem"), callbackButton("🆘 Hỗ trợ", "support")},
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
🎁 Ưu đãi nạp được cộng tự động theo bậc.

💰 Số dư: <b>%s</b>
⌨️ Số khác: <code>/topup 25</code>`, balance)
	if !linked {
		text += "\n\n🔒 Liên kết Telegram để nạp tiền."
	}

	rows := make([][]telegramInlineKeyboardButton, 0, 5)
	for i, amount := range topUpPackages() {
		label := formatTopUpAmount(amount)
		if percent := topUpBonusPercent(amount); percent > 0 {
			label += fmt.Sprintf(" +%g%%", percent)
		}
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
	photo, err := qrPNG(result)
	if err != nil {
		common.SysLog(common.LogText("Telegram QR generation failed: %s", err.Error()))
		return bot.sendCard(chatID, caption, markup)
	}
	if err := bot.sendPhotoBytes(chatID, photo, caption, markup); err != nil {
		common.SysLog(common.LogText("Telegram QR upload failed: %s", err.Error()))
		return bot.sendCard(chatID, caption, markup)
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

func qrPNG(result *VietQRTopUpResult) ([]byte, error) {
	content := strings.TrimSpace(result.QRContent)
	if content == "" {
		return nil, errors.New("missing QR content")
	}
	code, err := qr.Encode(content, qr.M, qr.Auto)
	if err != nil {
		return nil, err
	}
	scaled, err := barcode.Scale(code, 512, 512)
	if err != nil {
		return nil, err
	}
	var buf bytes.Buffer
	if err := png.Encode(&buf, scaled); err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}

func (bot *telegramBot) sendPhotoBytes(chatID int64, photo []byte, caption string, markup *telegramInlineKeyboardMarkup) error {
	var body bytes.Buffer
	writer := multipart.NewWriter(&body)
	if err := writer.WriteField("chat_id", strconv.FormatInt(chatID, 10)); err != nil {
		return err
	}
	if err := writer.WriteField("caption", caption); err != nil {
		return err
	}
	if err := writer.WriteField("parse_mode", "HTML"); err != nil {
		return err
	}
	if markup != nil {
		data, err := common.Marshal(markup)
		if err != nil {
			return err
		}
		if err := writer.WriteField("reply_markup", string(data)); err != nil {
			return err
		}
	}
	part, err := writer.CreateFormFile("photo", "qr.png")
	if err != nil {
		return err
	}
	if _, err := part.Write(photo); err != nil {
		return err
	}
	if err := writer.Close(); err != nil {
		return err
	}
	request, err := http.NewRequest(http.MethodPost, fmt.Sprintf("%s/bot%s/sendPhoto", bot.apiBaseURL, bot.token), &body)
	if err != nil {
		return err
	}
	request.Header.Set("Content-Type", writer.FormDataContentType())
	return bot.doRequest("sendPhoto", request, nil)
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

func topUpBonusPercent(amount int64) float64 {
	if operation_setting.GetQuotaDisplayType() != operation_setting.QuotaDisplayTypeUSD {
		return 0
	}
	amountUSD := float64(amount)
	best := 0.0
	for _, tier := range operation_setting.GetPaymentSetting().TopUpBonusTiers {
		if tier.MinAmountUSD > 0 && tier.MinAmountUSD <= amountUSD && tier.Percent > best {
			best = tier.Percent
		}
	}
	return best
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
	bonusLine := ""
	if result.CreditedQuota > 0 {
		if result.BonusQuota > 0 {
			bonusLine = fmt.Sprintf("\n🎁 Nhận: <b>%s</b> (+%s thưởng)",
				formatQuotaDisplay(result.CreditedQuota), formatQuotaDisplay(result.BonusQuota))
		} else {
			bonusLine = fmt.Sprintf("\n🎁 Nhận: <b>%s</b>", formatQuotaDisplay(result.CreditedQuota))
		}
	}
	return fmt.Sprintf(`💳 <b>Nạp %s</b>
Quét QR để chuyển khoản.

🏦 <b>%s</b> · <code>%s</code>
👤 %s
💵 <b>%s ₫</b>%s
📝 Nội dung: <code>%s</code>
🧾 Mã đơn: <code>%s</code>

⏳ Hết hạn sau 30 phút. Số dư tự cộng khi SePay xác nhận.`,
		html.EscapeString(formatTopUpAmount(result.Amount)),
		html.EscapeString(result.BankCode),
		html.EscapeString(result.BankAccount),
		html.EscapeString(result.AccountName),
		formatVND(result.VNDAmount),
		bonusLine,
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
	pricing := filterPricingForGroup(model.GetPricing(), group)
	groupRatio := ratio_setting.GetGroupRatio(group)
	if groupRatio <= 0 {
		groupRatio = 1
	}
	return formatModelStatus(pricing, summary.Models, group, groupRatio), nil
}

func filterPricingForGroup(pricing []model.Pricing, group string) []model.Pricing {
	filtered := make([]model.Pricing, 0, len(pricing))
	for _, item := range pricing {
		if len(item.EnableGroup) == 0 || slices.Contains(item.EnableGroup, "all") || slices.Contains(item.EnableGroup, group) {
			filtered = append(filtered, item)
		}
	}
	if len(filtered) == 0 {
		return pricing
	}
	return filtered
}

func formatModelStatus(pricing []model.Pricing, metrics []perfmetrics.ModelSummary, group string, groupRatio float64) string {
	if len(pricing) == 0 && len(metrics) == 0 {
		return "📊 <b>Model &amp; giá</b>\n\nChưa có model nào đang bán."
	}
	if len(pricing) == 0 {
		for _, item := range metrics {
			pricing = append(pricing, model.Pricing{ModelName: item.ModelName})
		}
	}

	metricByName := make(map[string]perfmetrics.ModelSummary, len(metrics))
	for _, item := range metrics {
		metricByName[item.ModelName] = item
	}
	active := make([]model.Pricing, 0, len(pricing))
	idle := make([]model.Pricing, 0, len(pricing))
	for _, item := range pricing {
		if metric, ok := metricByName[item.ModelName]; ok && metric.RequestCount > 0 {
			active = append(active, item)
		} else {
			idle = append(idle, item)
		}
	}
	slices.SortFunc(active, func(a, b model.Pricing) int {
		return cmp.Compare(metricByName[b.ModelName].RequestCount, metricByName[a.ModelName].RequestCount)
	})
	ordered := make([]model.Pricing, 0, len(active)+len(idle))
	ordered = append(ordered, active...)
	ordered = append(ordered, idle...)

	var text strings.Builder
	fmt.Fprintf(&text, "📊 <b>Model &amp; giá</b>\n24 giờ gần nhất · nhóm <code>%s</code> · <b>%d model</b>\n",
		html.EscapeString(group), len(ordered))
	const limit = 20
	for i, item := range ordered {
		if i >= limit {
			fmt.Fprintf(&text, "\n… còn %d model khác\n", len(ordered)-i)
			break
		}
		fmt.Fprintf(&text, "\n<b>%s</b>\n", html.EscapeString(item.ModelName))
		metric, hasMetric := metricByName[item.ModelName]
		if hasMetric && metric.RequestCount > 0 {
			latency := metric.P50LatencyMs
			if latency <= 0 {
				latency = metric.AvgLatencyMs
			}
			fmt.Fprintf(&text, "✅ %.2f%% · ⚡ %s · 🚀 %s\n",
				metric.SuccessRate, formatLatency(latency), formatTPS(metric.AvgTps))
			fmt.Fprintf(&text, "💰 %s · 📞 %s\n",
				modelPriceLine(item, groupRatio), formatCount(metric.RequestCount))
			continue
		}
		fmt.Fprintf(&text, "⚪ Chưa có traffic 24h · 💰 %s\n", modelPriceLine(item, groupRatio))
	}
	return text.String()
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

func (bot *telegramBot) showPlans(chatID, messageID int64, from *telegramUser) error {
	if !operation_setting.IsPaymentComplianceConfirmed() {
		return bot.replaceCard(chatID, messageID, "📦 <b>Gói đăng ký</b>\n\nTính năng gói đăng ký đang tạm tắt.", keyboard(
			[]telegramInlineKeyboardButton{callbackButton("🏠 Menu", "menu")},
		))
	}
	var plans []model.SubscriptionPlan
	if err := model.DB.Where("enabled = ?", true).Order("sort_order desc, id desc").Find(&plans).Error; err != nil {
		return bot.sendCard(chatID, "Không đọc được danh sách gói. Vui lòng thử lại sau.", keyboard(
			[]telegramInlineKeyboardButton{callbackButton("🏠 Menu", "menu")},
		))
	}
	if len(plans) == 0 {
		return bot.replaceCard(chatID, messageID, "📦 <b>Gói đăng ký</b>\n\nChưa có gói đăng ký nào đang bật.", keyboard(
			[]telegramInlineKeyboardButton{callbackButton("🏠 Menu", "menu")},
		))
	}

	balance := "—"
	linked := false
	if user, err := bot.linkedUser(from); err == nil {
		linked = true
		if quota, err := model.GetUserQuota(user.Id, true); err == nil {
			balance = formatQuotaDisplay(quota)
		}
	}
	var text strings.Builder
	text.WriteString("📦 <b>Gói đăng ký</b>\nChọn gói để xem chi tiết và mua bằng số dư ví.\n")
	for _, plan := range plans {
		plan.NormalizeDefaults()
		fmt.Fprintf(&text, "\n<b>%s</b> — %s / %s\n",
			html.EscapeString(plan.Title), formatPlanPrice(plan), formatPlanDuration(plan))
		if plan.Subtitle != "" {
			fmt.Fprintf(&text, "%s\n", html.EscapeString(plan.Subtitle))
		}
	}
	fmt.Fprintf(&text, "\n💰 Số dư: <b>%s</b>", balance)
	if !linked {
		text.WriteString("\n🔒 Liên kết Telegram để mua gói.")
	}

	rows := make([][]telegramInlineKeyboardButton, 0, len(plans)+3)
	for _, plan := range plans {
		rows = append(rows, []telegramInlineKeyboardButton{
			callbackButton(fmt.Sprintf("%s · %s", plan.Title, formatPlanPrice(plan)), "plan:"+strconv.Itoa(plan.Id)),
		})
	}
	rows = append(rows, []telegramInlineKeyboardButton{callbackButton("📦 Gói của tôi", "mysubs")})
	if !linked {
		if profile := bot.profileURL(); profile != "" {
			rows = append(rows, []telegramInlineKeyboardButton{urlButton("🔗 Liên kết Telegram", profile)})
		}
	}
	rows = append(rows, []telegramInlineKeyboardButton{callbackButton("🏠 Menu", "menu")})
	return bot.replaceCard(chatID, messageID, text.String(), keyboard(rows...))
}

func (bot *telegramBot) showPlanDetail(chatID, messageID int64, planID int) error {
	plan, err := model.GetSubscriptionPlanById(planID)
	if err != nil || plan == nil || !plan.Enabled {
		return bot.sendCard(chatID, "Gói đăng ký không tồn tại hoặc đã tắt.", keyboard(
			[]telegramInlineKeyboardButton{callbackButton("⬅️ Danh sách gói", "plans")},
			[]telegramInlineKeyboardButton{callbackButton("🏠 Menu", "menu")},
		))
	}
	plan.NormalizeDefaults()
	rows := make([][]telegramInlineKeyboardButton, 0, 4)
	if plan.AllowBalancePay == nil || *plan.AllowBalancePay {
		rows = append(rows, []telegramInlineKeyboardButton{
			callbackButton("✅ Mua bằng số dư", "planbuy:"+strconv.Itoa(plan.Id)),
		})
	}
	rows = append(rows,
		[]telegramInlineKeyboardButton{callbackButton("💳 Nạp thêm", "topup"), callbackButton("⬅️ Danh sách", "plans")},
		[]telegramInlineKeyboardButton{callbackButton("📦 Gói của tôi", "mysubs"), callbackButton("🏠 Menu", "menu")},
	)
	return bot.replaceCard(chatID, messageID, formatPlanDetail(*plan), keyboard(rows...))
}

func (bot *telegramBot) buyPlan(chatID int64, from *telegramUser, planID int) error {
	user, err := bot.linkedUser(from)
	if err != nil {
		return bot.sendLinkPrompt(chatID)
	}
	plan, err := model.GetSubscriptionPlanById(planID)
	if err != nil || plan == nil || !plan.Enabled {
		return bot.sendCard(chatID, "Gói đăng ký không tồn tại hoặc đã tắt.", keyboard(
			[]telegramInlineKeyboardButton{callbackButton("⬅️ Danh sách gói", "plans")},
			[]telegramInlineKeyboardButton{callbackButton("🏠 Menu", "menu")},
		))
	}
	plan.NormalizeDefaults()
	if plan.AllowBalancePay != nil && !*plan.AllowBalancePay {
		return bot.sendCard(chatID, "Gói này không hỗ trợ thanh toán bằng số dư.", keyboard(
			[]telegramInlineKeyboardButton{callbackButton("⬅️ Danh sách gói", "plans")},
			[]telegramInlineKeyboardButton{callbackButton("🏠 Menu", "menu")},
		))
	}
	if plan.MaxPurchasePerUser > 0 {
		count, err := model.CountUserSubscriptionsByPlan(user.Id, plan.Id)
		if err != nil {
			return bot.sendCard(chatID, "Không kiểm tra được số lần mua gói. Vui lòng thử lại sau.", nil)
		}
		if count >= int64(plan.MaxPurchasePerUser) {
			return bot.sendCard(chatID, "Bạn đã đạt số lần mua tối đa của gói này.", keyboard(
				[]telegramInlineKeyboardButton{callbackButton("📦 Gói của tôi", "mysubs")},
				[]telegramInlineKeyboardButton{callbackButton("🏠 Menu", "menu")},
			))
		}
	}
	if err := model.PurchaseSubscriptionWithBalance(user.Id, plan.Id); err != nil {
		message := err.Error()
		switch {
		case strings.Contains(message, "Insufficient balance"):
			quota, _ := model.GetUserQuota(user.Id, true)
			return bot.sendCard(chatID, fmt.Sprintf("⚠️ Số dư không đủ để mua gói này.\nSố dư hiện tại: <b>%s</b>", formatQuotaDisplay(quota)), keyboard(
				[]telegramInlineKeyboardButton{callbackButton("💳 Nạp thêm", "topup")},
				[]telegramInlineKeyboardButton{callbackButton("⬅️ Danh sách gói", "plans")},
				[]telegramInlineKeyboardButton{callbackButton("🏠 Menu", "menu")},
			))
		case strings.Contains(message, "does not allow balance"):
			return bot.sendCard(chatID, "Gói này không hỗ trợ thanh toán bằng số dư.", keyboard(
				[]telegramInlineKeyboardButton{callbackButton("⬅️ Danh sách gói", "plans")},
				[]telegramInlineKeyboardButton{callbackButton("🏠 Menu", "menu")},
			))
		default:
			common.SysLog(common.LogText("Telegram subscription purchase failed user_id=%d plan_id=%d error=%q", user.Id, plan.Id, message))
			return bot.sendCard(chatID, "Không mua được gói. Vui lòng thử lại sau.", keyboard(
				[]telegramInlineKeyboardButton{callbackButton("⬅️ Danh sách gói", "plans")},
				[]telegramInlineKeyboardButton{callbackButton("🏠 Menu", "menu")},
			))
		}
	}
	quota, _ := model.GetUserQuota(user.Id, true)
	text := fmt.Sprintf("✅ <b>Đã kích hoạt gói %s</b>\nSố dư còn: <b>%s</b>\n\nXem hạn mức và ngày hết hạn trong Gói của tôi.",
		html.EscapeString(plan.Title), formatQuotaDisplay(quota))
	return bot.sendCard(chatID, text, keyboard(
		[]telegramInlineKeyboardButton{callbackButton("📦 Gói của tôi", "mysubs")},
		[]telegramInlineKeyboardButton{callbackButton("🏠 Menu", "menu")},
	))
}

func (bot *telegramBot) showMySubscriptions(chatID, messageID int64, from *telegramUser) error {
	user, err := bot.linkedUser(from)
	if err != nil {
		return bot.sendLinkPrompt(chatID)
	}
	subscriptions, err := model.GetAllActiveUserSubscriptions(user.Id)
	if err != nil {
		return bot.sendCard(chatID, "Không đọc được gói đăng ký. Vui lòng thử lại sau.", nil)
	}
	if len(subscriptions) == 0 {
		return bot.replaceCard(chatID, messageID, "📦 <b>Gói của tôi</b>\n\nChưa có gói đăng ký đang hoạt động.", keyboard(
			[]telegramInlineKeyboardButton{callbackButton("📦 Xem gói", "plans")},
			[]telegramInlineKeyboardButton{callbackButton("🏠 Menu", "menu")},
		))
	}
	var text strings.Builder
	text.WriteString("📦 <b>Gói của tôi</b>\n")
	for _, summary := range subscriptions {
		sub := summary.Subscription
		if sub == nil {
			continue
		}
		title := fmt.Sprintf("Gói #%d", sub.PlanId)
		if plan, err := model.GetSubscriptionPlanById(sub.PlanId); err == nil && plan != nil {
			title = plan.Title
		}
		fmt.Fprintf(&text, "\n<b>%s</b>\n", html.EscapeString(title))
		fmt.Fprintf(&text, "📊 Đã dùng: %s / %s\n",
			formatQuotaAmount(sub.AmountUsed), formatQuotaInt64(sub.AmountTotal))
		if sub.EndTime > 0 {
			fmt.Fprintf(&text, "⏳ Hết hạn: %s\n", time.Unix(sub.EndTime, 0).Format("02/01/2006 15:04"))
		}
		if sub.NextResetTime > 0 {
			fmt.Fprintf(&text, "🔄 Reset: %s\n", time.Unix(sub.NextResetTime, 0).Format("02/01/2006 15:04"))
		}
	}
	return bot.replaceCard(chatID, messageID, text.String(), keyboard(
		[]telegramInlineKeyboardButton{callbackButton("📦 Xem gói", "plans")},
		[]telegramInlineKeyboardButton{callbackButton("🏠 Menu", "menu")},
	))
}

func formatPlanPrice(plan model.SubscriptionPlan) string {
	switch strings.ToUpper(strings.TrimSpace(plan.Currency)) {
	case "", "USD":
		return fmt.Sprintf("$%.2f", plan.PriceAmount)
	case "EUR":
		return fmt.Sprintf("€%.2f", plan.PriceAmount)
	default:
		return fmt.Sprintf("%s %.2f", strings.ToUpper(strings.TrimSpace(plan.Currency)), plan.PriceAmount)
	}
}

func formatPlanDuration(plan model.SubscriptionPlan) string {
	switch plan.DurationUnit {
	case "year":
		return fmt.Sprintf("%d năm", plan.DurationValue)
	case "month":
		return fmt.Sprintf("%d tháng", plan.DurationValue)
	case "day":
		return fmt.Sprintf("%d ngày", plan.DurationValue)
	case "hour":
		return fmt.Sprintf("%d giờ", plan.DurationValue)
	case "custom":
		seconds := plan.CustomSeconds
		if seconds <= 0 {
			return "tùy chỉnh"
		}
		if seconds%86400 == 0 {
			return fmt.Sprintf("%d ngày", seconds/86400)
		}
		if seconds%3600 == 0 {
			return fmt.Sprintf("%d giờ", seconds/3600)
		}
		return fmt.Sprintf("%d giây", seconds)
	default:
		return fmt.Sprintf("%d %s", plan.DurationValue, plan.DurationUnit)
	}
}

func formatPlanQuotaLabel(plan model.SubscriptionPlan) string {
	value := formatQuotaInt64(plan.TotalAmount)
	switch plan.QuotaResetPeriod {
	case "daily":
		return "Hạn mức/ngày: " + value
	case "weekly":
		return "Hạn mức/tuần: " + value
	case "monthly":
		return "Hạn mức/tháng: " + value
	case "custom":
		return fmt.Sprintf("Hạn mức mỗi %ds: %s", plan.QuotaResetCustomSeconds, value)
	default:
		return "Tổng hạn mức: " + value
	}
}

func formatPlanDetail(plan model.SubscriptionPlan) string {
	var text strings.Builder
	fmt.Fprintf(&text, "📦 <b>%s</b>\n", html.EscapeString(plan.Title))
	if plan.Subtitle != "" {
		fmt.Fprintf(&text, "%s\n", html.EscapeString(plan.Subtitle))
	}
	fmt.Fprintf(&text, "\n💵 <b>%s</b> / %s\n", formatPlanPrice(plan), formatPlanDuration(plan))
	fmt.Fprintf(&text, "📊 %s\n", formatPlanQuotaLabel(plan))
	if plan.UpgradeGroup != "" {
		fmt.Fprintf(&text, "⬆️ Nâng nhóm: <code>%s</code>\n", html.EscapeString(plan.UpgradeGroup))
	}
	if plan.AllowWalletOverflow == nil || *plan.AllowWalletOverflow {
		text.WriteString("✅ Dùng tiếp số dư khi hết hạn mức\n")
	}
	if plan.MaxPurchasePerUser > 0 {
		fmt.Fprintf(&text, "🛒 Tối đa %d lần/người\n", plan.MaxPurchasePerUser)
	}
	if plan.AllowBalancePay != nil && !*plan.AllowBalancePay {
		text.WriteString("⚠️ Không hỗ trợ trả bằng số dư\n")
	}
	return text.String()
}

func formatQuotaInt64(quota int64) string {
	if quota <= 0 {
		return "Không giới hạn"
	}
	return formatQuotaDisplay(int(quota))
}

func formatQuotaAmount(quota int64) string {
	if quota <= 0 {
		return formatQuotaDisplay(0)
	}
	return formatQuotaDisplay(int(quota))
}

package service

import (
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/model"
)

// activeTelegramBot lets background jobs (the price scheduler) report back to
// the chat that created a change.
var activeTelegramBot *telegramBot

type priceCommand struct {
	modelName   string
	inputPrice  float64
	outputPrice float64
	cachePrice  float64
	saleTag     string
	announce    bool
	note        string
	effectiveAt int64
}

const priceCommandUsage = `Cách dùng:
/price <model> <giá vào> <giá ra> [giá cache] [tuỳ chọn]
Ví dụ: /price gpt-6.2-sol 0.11 0.66 --after 2h --sale 90
Tuỳ chọn: --after 90m | --at 22:00 | --sale 90 | --no-announce | --note "ghi chú"
Giá tính theo USD/1M token.`

func (bot *telegramBot) handlePriceCommand(message *telegramMessage, args string) error {
	chatID := message.Chat.ID
	user, ok := bot.priceAdmin(message)
	if !ok {
		return nil
	}
	if strings.TrimSpace(args) == "" {
		return bot.sendMessage(chatID, priceCommandUsage)
	}
	command, err := parsePriceCommand(args, common.GetTimestamp())
	if err != nil {
		return bot.sendMessage(chatID, "Không đọc được lệnh: "+err.Error()+"\n\n"+priceCommandUsage)
	}

	row := &model.PriceSchedule{
		ModelName:     command.modelName,
		NewRatio:      command.inputPrice / 2,
		NewCompletion: command.outputPrice / command.inputPrice,
		NewCache:      command.cachePrice / command.inputPrice,
		SaleTag:       command.saleTag,
		EffectiveAt:   command.effectiveAt,
		Status:        model.PriceScheduleDraft,
		Announce:      command.announce,
		Note:          command.note,
		ChatID:        chatID,
		CreatedBy:     user.Id,
	}
	if err := model.CreatePriceSchedule(row); err != nil {
		return bot.sendMessage(chatID, "Không lưu được lịch đổi giá: "+err.Error())
	}
	return bot.sendCard(chatID, readPricePreview(row), keyboard(
		[]telegramInlineKeyboardButton{
			callbackButton("✅ Xác nhận", fmt.Sprintf("price:confirm:%d", row.Id)),
			callbackButton("❌ Huỷ", fmt.Sprintf("price:cancel:%d", row.Id)),
		},
	))
}

func (bot *telegramBot) handlePriceList(message *telegramMessage) error {
	if _, ok := bot.priceAdmin(message); !ok {
		return nil
	}
	rows, err := model.GetOpenPriceSchedules(20)
	if err != nil {
		return bot.sendMessage(message.Chat.ID, "Không đọc được danh sách lịch.")
	}
	if len(rows) == 0 {
		return bot.sendMessage(message.Chat.ID, "Không có lịch đổi giá nào đang chờ.")
	}
	lines := make([]string, 0, len(rows)+1)
	lines = append(lines, "🗓 <b>Lịch đổi giá đang chờ</b>")
	for _, row := range rows {
		input, output, _ := priceSchedulePrices(&row)
		lines = append(lines, fmt.Sprintf("#%d <b>%s</b> → $%s / $%s · %s · %s",
			row.Id, row.ModelName, formatPrice(input), formatPrice(output),
			time.Unix(row.EffectiveAt, 0).In(vietnamLocation).Format("02/01 15:04"), row.Status))
	}
	return bot.sendMessage(message.Chat.ID, strings.Join(lines, "\n"))
}

func (bot *telegramBot) handlePriceCancel(message *telegramMessage, args string) error {
	if _, ok := bot.priceAdmin(message); !ok {
		return nil
	}
	id, err := strconv.Atoi(strings.TrimSpace(args))
	if err != nil || id <= 0 {
		return bot.sendMessage(message.Chat.ID, "Cách dùng: /cancel <id>")
	}
	cancelled, err := model.CancelPriceSchedule(id)
	if err != nil {
		return bot.sendMessage(message.Chat.ID, "Huỷ thất bại: "+err.Error())
	}
	if !cancelled {
		return bot.sendMessage(message.Chat.ID, "Không tìm thấy lịch đang chờ với id này.")
	}
	return bot.sendMessage(message.Chat.ID, fmt.Sprintf("Đã huỷ lịch #%d.", id))
}

func (bot *telegramBot) handlePriceUndo(message *telegramMessage, args string) error {
	user, ok := bot.priceAdmin(message)
	if !ok {
		return nil
	}
	id, err := strconv.Atoi(strings.TrimSpace(args))
	if err != nil || id <= 0 {
		return bot.sendMessage(message.Chat.ID, "Cách dùng: /undo <id>")
	}
	source, err := model.GetPriceSchedule(id)
	if err != nil {
		return bot.sendMessage(message.Chat.ID, "Không tìm thấy lịch này.")
	}
	if source.Status != model.PriceScheduleApplied {
		return bot.sendMessage(message.Chat.ID, "Chỉ hoàn tác được lịch đã áp dụng.")
	}
	if source.OldRatio <= 0 || source.OldCompletion <= 0 || source.OldCache <= 0 {
		return bot.sendMessage(message.Chat.ID, "Lịch này không lưu đủ giá cũ để hoàn tác.")
	}
	row := &model.PriceSchedule{
		ModelName:     source.ModelName,
		NewRatio:      source.OldRatio,
		NewCompletion: source.OldCompletion,
		NewCache:      source.OldCache,
		EffectiveAt:   common.GetTimestamp(),
		Status:        model.PriceSchedulePending,
		Announce:      true,
		Note:          fmt.Sprintf("undo #%d", source.Id),
		ChatID:        message.Chat.ID,
		CreatedBy:     user.Id,
	}
	if err := model.CreatePriceSchedule(row); err != nil {
		return bot.sendMessage(message.Chat.ID, "Không tạo được lịch hoàn tác: "+err.Error())
	}
	if err := ApplyPriceSchedule(row.Id); err != nil {
		return bot.sendMessage(message.Chat.ID, "Hoàn tác thất bại: "+err.Error())
	}
	return nil
}

func (bot *telegramBot) handlePriceCheck(message *telegramMessage, args string) error {
	if _, ok := bot.priceAdmin(message); !ok {
		return nil
	}
	name := strings.TrimSpace(args)
	if name == "" {
		return bot.sendMessage(message.Chat.ID, "Cách dùng: /pricecheck <model>")
	}
	input, output, cache, ok := currentModelPrice(name)
	if !ok {
		return bot.sendMessage(message.Chat.ID, "Model chưa có giá trong cấu hình.")
	}
	return bot.sendCard(message.Chat.ID, readPriceCheck(name, input, output, cache), nil)
}

func (bot *telegramBot) priceAdmin(message *telegramMessage) (*model.User, bool) {
	if message.From == nil {
		return nil, false
	}
	user, err := bot.linkedUser(message.From)
	if err != nil {
		_ = bot.sendMessage(message.Chat.ID, "Liên kết Telegram với tài khoản quản trị trong Hồ sơ trước.")
		return nil, false
	}
	if user.Role < common.RoleAdminUser {
		_ = bot.sendMessage(message.Chat.ID, "Chỉ quản trị viên dùng được lệnh này.")
		return nil, false
	}
	return user, true
}

func parsePriceCommand(args string, now int64) (*priceCommand, error) {
	fields := strings.Fields(args)
	positional := make([]string, 0, 4)
	command := &priceCommand{announce: true, effectiveAt: now}
	index := 0
	for index < len(fields) {
		field := fields[index]
		if !strings.HasPrefix(field, "--") {
			positional = append(positional, field)
			index++
			continue
		}
		switch field {
		case "--no-announce":
			command.announce = false
			index++
		case "--at", "--after", "--sale", "--note":
			if index+1 >= len(fields) {
				return nil, fmt.Errorf("%s cần một giá trị", field)
			}
			value := fields[index+1]
			switch field {
			case "--at":
				effectiveAt, err := parsePriceAt(value, now)
				if err != nil {
					return nil, err
				}
				command.effectiveAt = effectiveAt
			case "--after":
				delay, err := time.ParseDuration(value)
				if err != nil || delay <= 0 {
					return nil, fmt.Errorf("--after phải như 90m hoặc 2h")
				}
				command.effectiveAt = now + int64(delay.Seconds())
			case "--sale":
				if value != "0" {
					percent, err := strconv.Atoi(value)
					if err != nil || percent < 1 || percent > 99 {
						return nil, fmt.Errorf("--sale phải là 1-99 hoặc 0")
					}
				}
				command.saleTag = value
			case "--note":
				command.note = value
			}
			index += 2
		default:
			return nil, fmt.Errorf("tuỳ chọn lạ %s", field)
		}
	}
	if len(positional) < 3 {
		return nil, fmt.Errorf("cần model, giá vào và giá ra")
	}
	command.modelName = positional[0]
	input, err := parsePriceValue(positional[1])
	if err != nil {
		return nil, fmt.Errorf("giá vào không hợp lệ")
	}
	output, err := parsePriceValue(positional[2])
	if err != nil {
		return nil, fmt.Errorf("giá ra không hợp lệ")
	}
	cache := input * 0.3
	if len(positional) >= 4 {
		cache, err = parsePriceValue(positional[3])
		if err != nil {
			return nil, fmt.Errorf("giá cache không hợp lệ")
		}
	}
	if input <= 0 || output <= 0 || cache <= 0 {
		return nil, fmt.Errorf("giá phải lớn hơn 0")
	}
	if input > 1000 || output > 1000 || cache > 1000 {
		return nil, fmt.Errorf("giá quá lớn, kiểm tra lại đơn vị USD/1M")
	}
	command.inputPrice = input
	command.outputPrice = output
	command.cachePrice = cache
	return command, nil
}

func parsePriceValue(raw string) (float64, error) {
	cleaned := strings.TrimSpace(strings.TrimPrefix(strings.TrimSpace(raw), "$"))
	cleaned = strings.ReplaceAll(cleaned, ",", ".")
	value, err := strconv.ParseFloat(cleaned, 64)
	if err != nil {
		return 0, err
	}
	return value, nil
}

// parsePriceAt accepts HH:MM in Vietnam time; a time that already passed means
// tomorrow at that hour.
func parsePriceAt(raw string, now int64) (int64, error) {
	moment, err := time.ParseInLocation("15:04", strings.TrimSpace(raw), vietnamLocation)
	if err != nil {
		return 0, fmt.Errorf("--at phải như 22:00")
	}
	current := time.Unix(now, 0).In(vietnamLocation)
	target := time.Date(current.Year(), current.Month(), current.Day(), moment.Hour(), moment.Minute(), 0, 0, vietnamLocation)
	if !target.After(current) {
		target = target.Add(24 * time.Hour)
	}
	return target.Unix(), nil
}

func readPricePreview(row *model.PriceSchedule) string {
	input, output, cache := priceSchedulePrices(row)
	oldInput, oldOutput, oldCache, _ := currentModelPrice(row.ModelName)
	lines := []string{
		fmt.Sprintf("💡 <b>Đổi giá %s</b>", row.ModelName),
		fmt.Sprintf("Hiện tại: $%s / $%s / cache $%s", formatPrice(oldInput), formatPrice(oldOutput), formatPrice(oldCache)),
		fmt.Sprintf("Giá mới: $%s / $%s / cache $%s", formatPrice(input), formatPrice(output), formatPrice(cache)),
	}
	if costInput, costOutput, ok := providerCostFor(row.ModelName); ok {
		margin := ""
		if costInput > 0 {
			margin = fmt.Sprintf(" · biên +%.1f%%", (input-costInput)/costInput*100)
		}
		lines = append(lines, fmt.Sprintf("Giá vốn: $%s / $%s%s", formatPrice(costInput), formatPrice(costOutput), margin))
		if input < costInput {
			lines = append(lines, "⚠️ Giá vào thấp hơn giá vốn.")
		}
	} else {
		lines = append(lines, "Giá vốn: chưa có trong provider_costs")
	}
	lines = append(lines, fmt.Sprintf("Áp dụng: %s", time.Unix(row.EffectiveAt, 0).In(vietnamLocation).Format("02/01/2006 15:04")))
	if row.SaleTag != "" {
		lines = append(lines, "Tag sale: "+row.SaleTag)
	}
	if !row.Announce {
		lines = append(lines, "Không đăng thông báo console.")
	}
	if row.Note != "" {
		lines = append(lines, "Ghi chú: "+row.Note)
	}
	return strings.Join(lines, "\n")
}

func readPriceCheck(name string, input, output, cache float64) string {
	lines := []string{
		fmt.Sprintf("🔎 <b>%s</b>", name),
		fmt.Sprintf("Giá bán: $%s / $%s / cache $%s", formatPrice(input), formatPrice(output), formatPrice(cache)),
	}
	if costInput, costOutput, ok := providerCostFor(name); ok {
		line := fmt.Sprintf("Giá vốn: $%s / $%s", formatPrice(costInput), formatPrice(costOutput))
		if costInput > 0 {
			line += fmt.Sprintf(" · biên +%.1f%%", (input-costInput)/costInput*100)
		}
		lines = append(lines, line)
	} else {
		lines = append(lines, "Giá vốn: chưa có trong provider_costs")
	}
	return strings.Join(lines, "\n")
}

// providerCostFor reports the cheapest recorded upstream price for a model, so
// the preview warns even when several channels serve it.
func providerCostFor(name string) (input, output float64, ok bool) {
	costs, err := model.ListProviderCosts()
	if err != nil {
		return 0, 0, false
	}
	for _, cost := range costs {
		if cost.ModelName != name || cost.InputPrice <= 0 {
			continue
		}
		if !ok || cost.InputPrice < input {
			input, output, ok = cost.InputPrice, cost.OutputPrice, true
		}
	}
	return input, output, ok
}

func currentModelPrice(name string) (input, output, cache float64, ok bool) {
	ratios, err := readRatioMap("ModelRatio")
	if err != nil {
		return 0, 0, 0, false
	}
	ratio, ok := ratios[name]
	if !ok {
		return 0, 0, 0, false
	}
	completions, _ := readRatioMap("CompletionRatio")
	caches, _ := readRatioMap("CacheRatio")
	input = ratio * 2
	output = input * completions[name]
	cache = input * caches[name]
	return input, output, cache, true
}

func (bot *telegramBot) handlePriceCallback(query *telegramCallbackQuery) error {
	from := query.From
	if from == nil || query.Message == nil {
		return nil
	}
	chatID := query.Message.Chat.ID
	user, err := bot.linkedUser(from)
	if err != nil || user.Role < common.RoleAdminUser {
		return bot.answerCallbackQuery(query.ID, "Không có quyền.")
	}
	parts := strings.Split(query.Data, ":")
	if len(parts) != 3 {
		return bot.answerCallbackQuery(query.ID, "Dữ liệu không hợp lệ.")
	}
	id, err := strconv.Atoi(parts[2])
	if err != nil {
		return bot.answerCallbackQuery(query.ID, "Dữ liệu không hợp lệ.")
	}
	row, err := model.GetPriceSchedule(id)
	if err != nil || (row.ChatID != 0 && row.ChatID != chatID) {
		return bot.answerCallbackQuery(query.ID, "Không tìm thấy lịch này.")
	}
	switch parts[1] {
	case "cancel":
		cancelled, err := model.CancelPriceSchedule(id)
		if err != nil || !cancelled {
			return bot.answerCallbackQuery(query.ID, "Không huỷ được.")
		}
		_ = bot.answerCallbackQuery(query.ID, "Đã huỷ.")
		return bot.editCard(chatID, query.Message.MessageID, "❌ Đã huỷ lịch đổi giá #"+strconv.Itoa(id)+".", nil)
	case "confirm":
		if row.EffectiveAt > common.GetTimestamp() {
			armed, err := model.ArmPriceSchedule(id)
			if err != nil || !armed {
				return bot.answerCallbackQuery(query.ID, "Lịch đã thay đổi, mở lại bằng /prices.")
			}
			_ = bot.answerCallbackQuery(query.ID, "Đã hẹn giờ.")
			return bot.editCard(chatID, query.Message.MessageID,
				"🕒 Đã hẹn đổi giá "+row.ModelName+" lúc "+
					time.Unix(row.EffectiveAt, 0).In(vietnamLocation).Format("02/01/2006 15:04")+".", nil)
		}
		_ = bot.answerCallbackQuery(query.ID, "Đang áp dụng...")
		if err := ApplyPriceSchedule(id); err != nil {
			return bot.editCard(chatID, query.Message.MessageID, "⚠️ Áp dụng thất bại: "+err.Error(), nil)
		}
		return bot.editCard(chatID, query.Message.MessageID,
			"✅ Đã áp dụng giá mới cho "+row.ModelName+".", nil)
	default:
		return bot.answerCallbackQuery(query.ID, "Hành động không hợp lệ.")
	}
}

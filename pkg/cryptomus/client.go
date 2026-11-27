package cryptomus

import (
	"bytes"
	"context"
	"crypto/md5"
	"crypto/subtle"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/shopspring/decimal"
)

const MaxResponseBytes = 1 << 20

var ErrProtocol = errors.New("cryptomus protocol error")
var defaultHTTPClient = &http.Client{Timeout: 20 * time.Second, CheckRedirect: func(_ *http.Request, _ []*http.Request) error { return http.ErrUseLastResponse }}

type Client struct {
	MerchantID    string
	PaymentAPIKey string
	HTTPClient    *http.Client
}
type Currency struct {
	Currency string `json:"currency"`
	Network  string `json:"network"`
}
type CreateRequest struct {
	Amount                 string     `json:"amount"`
	Currency               string     `json:"currency"`
	OrderID                string     `json:"order_id"`
	Network                string     `json:"network"`
	ToCurrency             string     `json:"to_currency"`
	Currencies             []Currency `json:"currencies"`
	URLCallback            string     `json:"url_callback"`
	URLReturn              string     `json:"url_return"`
	URLSuccess             string     `json:"url_success"`
	AccuracyPaymentPercent int        `json:"accuracy_payment_percent"`
	Subtract               int        `json:"subtract"`
	Lifetime               int        `json:"lifetime"`
	IsPaymentMultiple      bool       `json:"is_payment_multiple"`
}
type Payment struct {
	UUID             string            `json:"uuid"`
	OrderID          string            `json:"order_id"`
	Amount           string            `json:"amount"`
	Currency         string            `json:"currency"`
	PayerCurrency    string            `json:"payer_currency"`
	Network          string            `json:"network"`
	PaymentAmount    string            `json:"payment_amount"`
	PaymentAmountUSD string            `json:"payment_amount_usd"`
	PayerAmount      string            `json:"payer_amount"`
	MerchantAmount   string            `json:"merchant_amount"`
	Commission       string            `json:"commission"`
	Status           string            `json:"status"`
	PaymentStatus    string            `json:"payment_status"`
	URL              string            `json:"url"`
	IsFinal          bool              `json:"is_final"`
	ExpiredAt        int64             `json:"expired_at"`
	ConvertFrom      common.RawMessage `json:"convert_from"`
}

func signature(body []byte, key string) string {
	sum := md5.Sum([]byte(base64.StdEncoding.EncodeToString(body) + key))
	return hex.EncodeToString(sum[:])
}
func ValidCheckoutURL(value string) bool {
	u, err := url.Parse(value)
	return err == nil && u.Scheme == "https" && u.Host == "pay.cryptomus.com" && u.User == nil && u.RawQuery == "" && u.Fragment == "" && strings.HasPrefix(u.Path, "/pay/") && len(u.Path) > len("/pay/")
}
func (c Client) Create(ctx context.Context, request CreateRequest) (Payment, error) {
	return c.post(ctx, "/payment", request)
}
func (c Client) Info(ctx context.Context, orderID string) (Payment, error) {
	return c.post(ctx, "/payment/info", struct {
		OrderID string `json:"order_id"`
	}{orderID})
}
func (c Client) post(ctx context.Context, path string, value any) (Payment, error) {
	var payment Payment
	body, err := common.Marshal(value)
	if err != nil {
		return payment, ErrProtocol
	}
	ctx, cancel := context.WithTimeout(ctx, 20*time.Second)
	defer cancel()
	request, err := http.NewRequestWithContext(ctx, http.MethodPost, "https://api.cryptomus.com/v1"+path, bytes.NewReader(body))
	if err != nil {
		return payment, ErrProtocol
	}
	request.Header.Set("Content-Type", "application/json")
	request.Header.Set("merchant", c.MerchantID)
	request.Header.Set("sign", signature(body, c.PaymentAPIKey))
	client := defaultHTTPClient
	if c.HTTPClient != nil {
		copy := *c.HTTPClient
		copy.CheckRedirect = func(_ *http.Request, _ []*http.Request) error { return http.ErrUseLastResponse }
		client = &copy
	}
	response, err := client.Do(request)
	if err != nil {
		return payment, ErrProtocol
	}
	defer response.Body.Close()
	raw, err := io.ReadAll(io.LimitReader(response.Body, MaxResponseBytes+1))
	if err != nil || len(raw) > MaxResponseBytes || response.StatusCode != http.StatusOK {
		return payment, ErrProtocol
	}
	var envelope struct {
		State  int     `json:"state"`
		Result Payment `json:"result"`
	}
	if common.Unmarshal(raw, &envelope) != nil || envelope.State != 0 || envelope.Result.UUID == "" {
		return payment, ErrProtocol
	}
	if envelope.Result.URL != "" && !ValidCheckoutURL(envelope.Result.URL) {
		return payment, ErrProtocol
	}
	return envelope.Result, nil
}

// PHP signs json_encode(JSON_UNESCAPED_UNICODE), retaining member order and escaping slashes.
func VerifyWebhook(body []byte, key string) (Payment, error) {
	var payment Payment
	decoder := json.NewDecoder(bytes.NewReader(body))
	decoder.UseNumber()
	token, err := decoder.Token()
	if err != nil || token != json.Delim('{') {
		return payment, ErrProtocol
	}
	var unsigned bytes.Buffer
	unsigned.WriteByte('{')
	seen := map[string]bool{}
	sign := ""
	count := 0
	for decoder.More() {
		token, err = decoder.Token()
		if err != nil {
			return payment, ErrProtocol
		}
		name, ok := token.(string)
		if !ok || seen[name] {
			return payment, ErrProtocol
		}
		seen[name] = true
		value, err := orderedValue(decoder)
		if err != nil {
			return payment, err
		}
		if name == "sign" {
			if common.Unmarshal(value, &sign) != nil {
				return payment, ErrProtocol
			}
			continue
		}
		if count > 0 {
			unsigned.WriteByte(',')
		}
		count++
		unsigned.Write(phpString(name))
		unsigned.WriteByte(':')
		unsigned.Write(value)
	}
	if _, err = decoder.Token(); err != nil {
		return payment, ErrProtocol
	}
	if _, err = decoder.Token(); err != io.EOF {
		return payment, ErrProtocol
	}
	unsigned.WriteByte('}')
	expected := signature(unsigned.Bytes(), key)
	if key == "" || len(sign) != len(expected) || subtle.ConstantTimeCompare([]byte(sign), []byte(expected)) != 1 {
		return payment, ErrProtocol
	}
	if common.Unmarshal(body, &payment) != nil {
		return payment, ErrProtocol
	}
	return payment, nil
}
func phpString(value string) []byte {
	var b bytes.Buffer
	encoder := json.NewEncoder(&b)
	encoder.SetEscapeHTML(false)
	_ = encoder.Encode(value)
	out := bytes.TrimSuffix(b.Bytes(), []byte{'\n'})
	out = bytes.ReplaceAll(out, []byte(`\u2028`), []byte("\u2028"))
	out = bytes.ReplaceAll(out, []byte(`\u2029`), []byte("\u2029"))
	return bytes.ReplaceAll(out, []byte{'/'}, []byte{'\\', '/'})
}
func orderedValue(decoder *json.Decoder) ([]byte, error) {
	token, err := decoder.Token()
	if err != nil {
		return nil, ErrProtocol
	}
	switch value := token.(type) {
	case string:
		return phpString(value), nil
	case json.Number:
		return []byte(value.String()), nil
	case nil:
		return []byte("null"), nil
	case bool:
		if value {
			return []byte("true"), nil
		}
		return []byte("false"), nil
	case json.Delim:
		var b bytes.Buffer
		b.WriteRune(rune(value))
		count := 0
		seen := map[string]bool{}
		if value != '{' && value != '[' {
			return nil, ErrProtocol
		}
		for decoder.More() {
			if count > 0 {
				b.WriteByte(',')
			}
			count++
			if value == '{' {
				key, err := decoder.Token()
				if err != nil {
					return nil, ErrProtocol
				}
				name, ok := key.(string)
				if !ok || seen[name] {
					return nil, ErrProtocol
				}
				seen[name] = true
				b.Write(phpString(name))
				b.WriteByte(':')
			}
			child, err := orderedValue(decoder)
			if err != nil {
				return nil, err
			}
			b.Write(child)
		}
		end, err := decoder.Token()
		if err != nil {
			return nil, ErrProtocol
		}
		if value == '{' && end != json.Delim('}') || value == '[' && end != json.Delim(']') {
			return nil, ErrProtocol
		}
		b.WriteRune(rune(end.(json.Delim)))
		return b.Bytes(), nil
	}
	return nil, ErrProtocol
}

// SettlementTolerancePercent is the shortfall a settled invoice may still show
// and count as paid. The provider converts the invoice amount through its own
// rate, so a $100 invoice can settle at $99.99; without this allowance a paid
// order stays pending forever.
const SettlementTolerancePercent = 1

// CoversAmount reports whether a settled amount pays an invoice of expected,
// allowing the provider's conversion shortfall.
func CoversAmount(paid string, expected string) bool {
	got, err := decimal.NewFromString(strings.TrimSpace(paid))
	if err != nil {
		return false
	}
	want, err := decimal.NewFromString(strings.TrimSpace(expected))
	if err != nil || !want.IsPositive() {
		return false
	}
	tolerance := want.Mul(decimal.NewFromInt(SettlementTolerancePercent)).Div(decimal.NewFromInt(100))
	return got.GreaterThanOrEqual(want.Sub(tolerance))
}

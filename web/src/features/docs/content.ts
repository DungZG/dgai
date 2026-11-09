/*
Copyright (C) 2023-2026 QuantumNous

This program is free software: you can redistribute it and/or modify
it under the terms of the GNU Affero General Public License as
published by the Free Software Foundation, either version 3 of the
License, or (at your option) any later version.

This program is distributed in the hope that it will be useful,
but WITHOUT ANY WARRANTY; without even the implied warranty of
MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE. See the
GNU Affero General Public License for more details.

You should have received a copy of the GNU Affero General Public License
along with this program. If not, see <https://www.gnu.org/licenses/>.

For commercial licensing, please contact support@quantumnous.com
*/

export type DocSection = {
  id: string
  title: string
  body: string
}

// Điền domain thật ở đây khi đổi tên miền; mọi snippet trong tài liệu dùng biến này.
export const DOCS_BASE_URL = 'https://dgai.io.vn'

export const DOC_QUICKSTART = `## 1. Lấy API key

1. Đăng nhập vào bảng điều khiển dgai.
2. Mở **Token** ở thanh bên trái, bấm **Tạo token**.
3. Đặt tên, chọn hạn mức (hoặc bật *Không giới hạn*), rồi lưu.
4. Bấm vào token vừa tạo để xem khóa dạng \`sk-...\`.

Mọi request đều dùng khóa này. Không chia sẻ khóa cho người khác — hạn mức
trong ví sẽ bị trừ theo lượng token bạn dùng.

## 2. Gọi request đầu tiên

\`\`\`bash
curl ${DOCS_BASE_URL}/v1/chat/completions \\
  -H "Authorization: Bearer sk-YOUR-KEY" \\
  -H "Content-Type: application/json" \\
  -d '{
    "model": "deepseek-v4.1-flash",
    "messages": [{"role": "user", "content": "Xin chào"}],
    "max_tokens": 512
  }'
\`\`\`

Response trả về theo chuẩn OpenAI, có trường \`usage\` để bạn đối chiếu chi phí.

## 3. Dùng với SDK

\`\`\`python
from openai import OpenAI

client = OpenAI(
    base_url="${DOCS_BASE_URL}/v1",
    api_key="sk-YOUR-KEY",
)

resp = client.chat.completions.create(
    model="deepseek-v4.1-flash",
    messages=[{"role": "user", "content": "Xin chào"}],
)
print(resp.choices[0].message.content)
\`\`\`

## 4. Dùng với công cụ lập trình

| Công cụ | Base URL | Ghi chú |
| --- | --- | --- |
| Cursor / Cline / Continue | \`${DOCS_BASE_URL}/v1\` | Chọn provider *OpenAI Compatible*, dán key |
| Claude Code | \`${DOCS_BASE_URL}\` | Đặt \`ANTHROPIC_BASE_URL\`, \`ANTHROPIC_AUTH_TOKEN\` |
| Codex CLI | \`${DOCS_BASE_URL}/v1\` | Dùng chế độ OpenAI compatible |
| LiteLLM / LangChain | \`${DOCS_BASE_URL}/v1\` | Khai báo như endpoint OpenAI |

> Model DeepSeek V4.1 Flash là model suy luận: nó ghi phần suy nghĩ vào
> \`reasoning_content\` trước khi trả \`content\`. Đặt \`max_tokens\` từ 512 trở lên,
> nếu không phần suy nghĩ sẽ ăn hết hạn mức và \`content\` trả về rỗng.`

export const DOC_API = `## Base URL

\`\`\`
${DOCS_BASE_URL}/v1
\`\`\`

## Xác thực

\`\`\`http
Authorization: Bearer sk-YOUR-KEY
\`\`\`

Với SDK Anthropic, dùng \`x-api-key: sk-YOUR-KEY\` (tương đương).

## Endpoint hỗ trợ

| Phương thức | Đường dẫn | Mô tả |
| --- | --- | --- |
| POST | \`/v1/chat/completions\` | Chat hoàn chỉnh, hỗ trợ stream |
| POST | \`/v1/completions\` | Completion văn bản thuần |
| POST | \`/v1/responses\` | Định dạng Responses API |
| POST | \`/v1/messages\` | Định dạng Anthropic Messages |
| POST | \`/v1/messages/count_tokens\` | Đếm token trước khi gửi |
| POST | \`/v1/embeddings\` | Vector embedding |
| POST | \`/v1/images/generations\` | Tạo ảnh |
| POST | \`/v1/audio/speech\` | Chuyển văn bản thành giọng nói |
| POST | \`/v1/audio/transcriptions\` | Bóc băng ghi âm |
| GET | \`/v1/models\` | Danh sách model bạn được dùng |

## Stream

Thêm \`"stream": true\`. Response là SSE, mỗi dòng bắt đầu bằng \`data: \`,
kết thúc bằng \`data: [DONE]\`.

\`\`\`bash
curl -N ${DOCS_BASE_URL}/v1/chat/completions \\
  -H "Authorization: Bearer sk-YOUR-KEY" \\
  -H "Content-Type: application/json" \\
  -d '{"model":"deepseek-v4.1-flash","stream":true,
       "messages":[{"role":"user","content":"Đếm từ 1 đến 5"}]}'
\`\`\`

## Mã lỗi

| HTTP | Ý nghĩa | Cách xử lý |
| --- | --- | --- |
| 400 | Request sai định dạng | Kiểm tra JSON và tên model |
| 401 | Khóa sai hoặc bị thu hồi | Tạo token mới |
| 403 | Model không nằm trong quyền của token | Bỏ giới hạn model cho token |
| 404 | Model không tồn tại | Gọi \`/v1/models\` để xem danh sách |
| 429 | Gọi quá nhanh | Giảm tốc, thử lại với backoff |
| 500/502 | Lỗi phía upstream | Thử lại sau vài giây |

Lỗi trả về theo cấu trúc:

\`\`\`json
{
  "error": {
    "message": "Mô tả lỗi",
    "type": "invalid_request_error",
    "code": "model_not_found"
  }
}
\`\`\``

export const DOC_MODELS = `## Model đang mở bán

### Tính theo token

| Model | Ngữ cảnh | Vào /1M | Ra /1M | Cache /1M | Dùng cho |
| --- | --- | --- | --- | --- | --- |
| \`deepseek-v4.1-flash\` | 1.0M | $0.08 | $0.28 | $0.004 | Viết code, agent, tài liệu dài |
| \`deepseek-v4.1-flash-1017\` | 1.0M | $0.08 | $0.28 | $0.032 | Cùng model, provider khác |
| \`claude-opus-5-5\` | 200K | $0.30 | $1.50 | $0.15 | Suy luận sâu, refactor lớn |
| \`gpt-6.1-sol\` | 400K | $0.15 | $0.90 | $0.015 | Code, phân tích |
| \`cx/gpt-6-luna\` | 400K | $0.03 | $0.06 | $0.0225 | Tác vụ nhanh, giá rẻ |

### Tính theo request

Model tính giá cố định mỗi lần gọi, không phụ thuộc số token.

| Model | Giá / request | Ghi chú |
| --- | --- | --- |
| \`slow/agy/gemini-3.8-flash-medium\` | $0.002 | Gemini 3.8 Flash, rẻ nhất |
| \`cnb/deepseek-pro-v4\` | $0.003 | DeepSeek Pro V4 |
| \`claude-opus-4-8\` | $0.008 | Opus 4.8 |
| \`claude-opus-5.5\` | $0.02 | Opus thế hệ mới |
| \`claude-fable-5.1-req\` | $0.02 | Fable 5.1 |

Với model tính theo request, prompt càng dài càng lợi — giá không đổi.

## Giá và cách tính## Giá và cách tính

Giá tính theo lượng token thực tế của từng request:

\`\`\`
chi phí = (token_vào_mới / 1.000.000 × giá_vào)
        + (token_đã_cache / 1.000.000 × giá_cache)
        + (token_ra / 1.000.000 × giá_ra)
\`\`\`

Ví dụ: một request dùng 12.000 token vào và 3.000 token ra:

\`\`\`
12.000 / 1.000.000 × 0.08 = $0.00096
 3.000 / 1.000.000 × 0.28 = $0.00084
tổng                        = $0.0018
\`\`\`

Hạn mức được trừ trước khi gọi upstream rồi hoàn lại phần chênh lệch sau khi
có \`usage\` thật, nên bạn không bị tính thừa.

## Cache prompt — giảm tới 88% chi phí

Nhà cung cấp upstream tự cache phần đầu của prompt (system prompt, tài liệu dài,
lịch sử hội thoại). Phần được cache chỉ bị tính **5% giá vào** thay vì 100%.

| Loại token | Giá /1M | Ghi chú |
| --- | --- | --- |
| Token vào mới | $0.08 | Chưa từng xuất hiện trong cache |
| Token vào **đã cache** | **$0.004** | Trùng với request trước đó |
| Token ra | $0.28 | Luôn tính đủ |

Ví dụ với prompt 1.592 token, trong đó 1.536 token đã cache:

\`\`\`
không cache:  1.592 × $0.08/1M + 16 × $0.28/1M  = $0.000132
có cache:       56 × $0.08/1M + 1.536 × $0.004/1M + 16 × $0.28/1M = $0.000015
tiết kiệm: 88%
\`\`\`

### Cách tận dụng

1. **Đặt phần cố định lên đầu prompt** — system prompt, hướng dẫn, tài liệu tham
   khảo. Phần này lặp lại giữa các request nên được cache.
2. **Giữ nguyên phần đầu giữa các lượt** — chỉ thêm tin nhắn mới vào cuối. Sửa
   nội dung phía trước làm cache mất hiệu lực.
3. **Gọi liên tiếp trong thời gian ngắn** — cache upstream có thời hạn (thường
   vài phút đến vài giờ tùy nhà cung cấp).
4. **Dùng cùng một model và cùng API key** — dgai giữ request của bạn trên cùng
   một channel trong 1 giờ, nên cache không bị nguội do đổi nhà cung cấp.

Phần tiết kiệm được phản ánh ngay trong số dư: nhật ký ghi \`cache_tokens\` và
\`cache_ratio\` cho từng request.

## Bảng giá so sánh

| | dgai | Giá chính thức DeepSeek |
| --- | --- | --- |
| Vào /1M | $0.08 | ~$0.27 |
| Ra /1M | $0.28 | ~$1.10 |

## Xem danh sách model bằng API

\`\`\`bash
curl ${DOCS_BASE_URL}/v1/models -H "Authorization: Bearer sk-YOUR-KEY"
\`\`\``

export const DOC_BILLING = `## Ví và hạn mức

Số dư ví tính bằng USD. Mỗi lần gọi API, hệ thống trừ hạn mức theo lượng token
thật của request đó. Khi ví về 0, request bị chặn trước khi tới nhà cung cấp.

## Nạp tiền

Mức nạp tối thiểu: **$1**. Các mức có sẵn: $1, $5, $10, $20, $50, $100.

Hai cách nạp:

### Chuyển khoản ngân hàng

1. Mở **Ví** trong bảng điều khiển, bấm **Nạp tiền**.
2. Quét mã QR, giữ nguyên nội dung chuyển khoản hệ thống sinh ra.
3. Sau khi tiền vào tài khoản, quản trị viên xác nhận và cộng số dư.
   Thời gian xử lý: trong 30 phút, khung 8:00–22:00 hằng ngày.

> Nội dung chuyển khoản là thứ duy nhất dùng để đối chiếu. Chuyển sai nội dung
> sẽ chậm được cộng tiền — hãy giữ nguyên mã hệ thống hiển thị.

### Mã nạp (code nạp)

Nhận mã dạng chuỗi 32 ký tự, mở **Ví → Nạp tiền**, dán mã và xác nhận. Số dư
cộng ngay lập tức. Mỗi mã dùng được một lần.

## Quy tắc

- Số dư không có hạn sử dụng.
- Số dư đã nạp không hoàn lại bằng tiền mặt; chỉ dùng để trả cho lưu lượng API.
- Mã nạp đã dùng không khôi phục được.`

export const DOC_SECTIONS: DocSection[] = [
  { id: 'quickstart', title: 'Bắt đầu', body: DOC_QUICKSTART },
  { id: 'api', title: 'Tham chiếu API', body: DOC_API },
  { id: 'models', title: 'Model và giá', body: DOC_MODELS },
  { id: 'billing', title: 'Ví và nạp tiền', body: DOC_BILLING },
]

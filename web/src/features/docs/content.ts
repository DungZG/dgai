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

const baseUrl = 'https://dgai.vn'

export const DOC_QUICKSTART = `## 1. Lấy API key

1. Đăng nhập vào bảng điều khiển dgai.
2. Mở **Token** ở thanh bên trái, bấm **Tạo token**.
3. Đặt tên, chọn hạn mức (hoặc bật *Không giới hạn*), rồi lưu.
4. Bấm vào token vừa tạo để xem khóa dạng \`sk-...\`.

Mọi request đều dùng khóa này. Không chia sẻ khóa cho người khác — hạn mức
trong ví sẽ bị trừ theo lượng token bạn dùng.

## 2. Gọi request đầu tiên

\`\`\`bash
curl ${baseUrl}/v1/chat/completions \\
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
    base_url="${baseUrl}/v1",
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
| Cursor / Cline / Continue | \`${baseUrl}/v1\` | Chọn provider *OpenAI Compatible*, dán key |
| Claude Code | \`${baseUrl}\` | Đặt \`ANTHROPIC_BASE_URL\`, \`ANTHROPIC_AUTH_TOKEN\` |
| Codex CLI | \`${baseUrl}/v1\` | Dùng chế độ OpenAI compatible |
| LiteLLM / LangChain | \`${baseUrl}/v1\` | Khai báo như endpoint OpenAI |

> Model DeepSeek V4.1 Flash là model suy luận: nó ghi phần suy nghĩ vào
> \`reasoning_content\` trước khi trả \`content\`. Đặt \`max_tokens\` từ 512 trở lên,
> nếu không phần suy nghĩ sẽ ăn hết hạn mức và \`content\` trả về rỗng.`

export const DOC_API = `## Base URL

\`\`\`
${baseUrl}/v1
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
curl -N ${baseUrl}/v1/chat/completions \\
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

| Model | Ngữ cảnh | Vào /1M token | Ra /1M token | Dùng cho |
| --- | --- | --- | --- | --- |
| \`deepseek-v4.1-flash\` | 1.0M | $0.08 | $0.28 | Viết code, phân tích, agent, xử lý tài liệu dài |
| \`deepseek-v4.1-flash-1017\` | 1.0M | $0.08 | $0.28 | Cùng model, định tuyến nhà cung cấp khác — độ trễ thấp hơn |

Hai mã trên là cùng một model, khác nhà cung cấp phía sau. Nếu một mã chậm
hoặc lỗi, đổi sang mã còn lại.

## Giá và cách tính

Giá tính theo lượng token thực tế của từng request:

\`\`\`
chi phí = (token_vào / 1.000.000 × $0.08)
        + (token_ra  / 1.000.000 × $0.28)
\`\`\`

Ví dụ: một request dùng 12.000 token vào và 3.000 token ra:

\`\`\`
12.000 / 1.000.000 × 0.08 = $0.00096
 3.000 / 1.000.000 × 0.28 = $0.00084
tổng                        = $0.0018
\`\`\`

Hạn mức được trừ trước khi gọi upstream rồi hoàn lại phần chênh lệch sau khi
có \`usage\` thật, nên bạn không bị tính thừa.

## Bảng giá so sánh

| | dgai | Giá chính thức DeepSeek |
| --- | --- | --- |
| Vào /1M | $0.08 | ~$0.27 |
| Ra /1M | $0.28 | ~$1.10 |

## Xem danh sách model bằng API

\`\`\`bash
curl ${baseUrl}/v1/models -H "Authorization: Bearer sk-YOUR-KEY"
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

export const DOC_PROVIDERS = `Trang này dành cho quản trị viên: cách nối một nhà cung cấp
upstream vào dgai để bán lại.

## 1. Chuẩn bị phía nhà cung cấp

Mỗi upstream phải cung cấp ba thứ:

| Thứ cần lấy | Ví dụ |
| --- | --- |
| Base URL tương thích OpenAI | \`https://api.vilao.ai\` |
| API key | \`sk-...\` |
| Danh sách model được phép gọi | \`deepseek-v4.1-flash\` |

Kiểm tra key trước khi nhập vào hệ thống:

\`\`\`bash
curl https://api.vilao.ai/v1/models \\
  -H "Authorization: Bearer sk-UPSTREAM-KEY"
\`\`\`

Nếu trả 401, key sai hoặc đã bị thu hồi — dừng lại, xin key mới.

## 2. Thêm channel

Bảng điều khiển → **Kênh** → **Thêm kênh**.

| Trường | Giá trị |
| --- | --- |
| Tên | Tên nhà cung cấp, ví dụ \`Vilao-DeepSeek\` |
| Loại | **OpenAI** |
| Base URL | \`https://api.vilao.ai\` (không kèm \`/v1\`) |
| Khóa | API key của upstream |
| Model | Danh sách phân tách bằng dấu phẩy, khớp chính xác ID upstream |
| Nhóm | \`default\` |
| Tự động cấm | Bật |

Bấm **Kiểm tra** — phải trả về thời gian phản hồi, không phải lỗi giá.

## 3. Đặt giá bán

Bảng điều khiển → **Cài đặt hệ thống → Nhóm & Giá model**.

dgai dùng hai bảng tỉ lệ:

| Khóa | Ý nghĩa | Công thức |
| --- | --- | --- |
| \`ModelRatio\` | Tỉ lệ giá vào | \`tỉ lệ = giá_vào_USD_trên_1M ÷ 2\` |
| \`CompletionRatio\` | Giá ra so với giá vào | \`tỉ lệ = giá_ra ÷ giá_vào\` |

Ví dụ bán với giá $0.08 vào / $0.28 ra:

\`\`\`json
{
  "deepseek-v4.1-flash": 0.04,
  "deepseek-v4.1-flash-1017": 0.04
}
\`\`\`

\`\`\`json
{
  "deepseek-v4.1-flash": 3.5,
  "deepseek-v4.1-flash-1017": 3.5
}
\`\`\`

Chưa khai báo giá thì request trả lỗi \`model_price_error\` và bị chặn — đây là
chốt an toàn để không bán lỗ.

## 4. Nguyên tắc đặt giá

\`\`\`
giá bán  ≥ giá vốn upstream × 1.1     (sàn, tránh lỗ khi upstream đổi giá)
giá bán  ≤ giá chính thức × 0.8       (trần, giữ lợi thế cạnh tranh)
\`\`\`

Khi thêm nhà cung cấp thứ hai cho cùng một model:

- Giá vốn chênh nhau nhiều (5–40 lần) thì **chỉ cho phép failover ở những model
  còn margin dương**. Bật failover cho model có giá vốn cao hơn giá bán sẽ lỗ
  mỗi khi nhà cung cấp chính sập.
- Giữ ít nhất hai channel cho model chủ lực, đặt **trọng số** khác nhau để
  upstream rẻ nhận phần lớn lưu lượng.

## 5. Kiểm tra sau khi cấu hình

1. **Kênh → Kiểm tra** trả về thời gian, không lỗi.
2. Tạo token thử, gọi \`/v1/chat/completions\` với từng model.
3. Mở **Nhật ký** đối chiếu \`prompt_tokens\`, \`completion_tokens\` và hạn mức bị trừ.
4. Đối chiếu hạn mức bị trừ với công thức giá ở trên — lệch nghĩa là tỉ lệ sai.

## 6. Vận hành

| Việc | Cách làm |
| --- | --- |
| Upstream chết | Bật *Tự động cấm* + kiểm tra định kỳ; request tự chuyển channel còn sống |
| Key hết hạn mức | Thêm nhiều khóa vào cùng channel (chế độ nhiều khóa, luân phiên) |
| Đổi giá | Sửa \`ModelRatio\`; áp dụng ngay, không cần khởi động lại |
| Xem chi phí | **Nhật ký** lọc theo model/channel, cột hạn mức là số đã trừ |`

export const DOC_SECTIONS: DocSection[] = [
  { id: 'quickstart', title: 'Bắt đầu', body: DOC_QUICKSTART },
  { id: 'api', title: 'Tham chiếu API', body: DOC_API },
  { id: 'models', title: 'Model và giá', body: DOC_MODELS },
  { id: 'billing', title: 'Ví và nạp tiền', body: DOC_BILLING },
  { id: 'providers', title: 'Cấu hình nhà cung cấp', body: DOC_PROVIDERS },
]

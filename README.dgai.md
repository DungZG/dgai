# dgai

Nền tảng bán API AI: gateway thống nhất trước nhiều nhà cung cấp upstream, tính
phí theo token, ví USD và nạp tiền.

Dự án dựa trên [new-api](https://github.com/QuantumNous/new-api) (AGPL-3.0). Phần
thay đổi của dgai: thương hiệu, tài liệu nội bộ, stack triển khai có HTTPS.

## Kiến trúc

```
khách ──HTTPS──▶ Caddy ──▶ dgai (Go, :3000) ──▶ upstream (Vilao, APIMaster, …)
                             │
                             ├── PostgreSQL  (dữ liệu)
                             └── Redis       (cache, hạn mức)
```

| Thành phần | Vai trò |
| --- | --- |
| `dgai` | Gateway: xác thực token, chọn channel, tính phí, ghi nhật ký |
| `caddy` | TLS, nén, header bảo mật; chuyển tiếp tới `dgai:3000` |
| `postgres` | Người dùng, token, hạn mức, nhật ký |
| `redis` | Cache hạn mức và channel, chia sẻ giữa các tiến trình |

## Triển khai

Yêu cầu: Docker, Bun (build frontend trên máy host).

```bash
# 1. Cấu hình bí mật
cat > .env.dgai <<'EOF'
DGAI_DB_PASSWORD=<mật-khẩu-mạnh>
DGAI_REDIS_PASSWORD=<mật-khẩu-mạnh>
DGAI_SESSION_SECRET=<chuỗi-ngẫu-nhiên-32-byte>
DGAI_PUBLIC_URL=https://dgai.vn
DGAI_SITE_ADDRESS=dgai.vn
EOF

# 2. Build frontend (được nhúng vào binary qua //go:embed)
cd web && bun install --frozen-lockfile && bun run build && cd ..

# 3. Dựng stack
docker compose -f docker-compose.dgai.yml --env-file .env.dgai up -d --build
```

Mở `https://<DGAI_SITE_ADDRESS>` và làm theo trình khởi tạo để tạo tài khoản quản trị.

### HTTPS

`Caddyfile.dgai` mặc định dùng `tls internal` (chứng chỉ tự ký, dùng cho nội bộ).
Với tên miền thật, xoá dòng `tls internal` — Caddy sẽ tự xin chứng chỉ Let's
Encrypt khi `DGAI_SITE_ADDRESS` trỏ đúng tên miền và bản ghi DNS đã hoạt động.

### Biến môi trường quan trọng

| Biến | Ý nghĩa |
| --- | --- |
| `SQL_DSN` | Chuỗi kết nối PostgreSQL |
| `REDIS_CONN_STRING` | Chuỗi kết nối Redis |
| `SESSION_SECRET` | Khoá ký phiên; bắt buộc khi chạy nhiều node |
| `SESSION_COOKIE_SECURE` | `true` khi chạy sau HTTPS |
| `SESSION_COOKIE_TRUSTED_URL` | Origin HTTPS chính xác, không có dấu `/` cuối |
| `TRUSTED_PROXIES` | Dải IP của reverse proxy để lấy đúng IP khách |
| `STREAMING_TIMEOUT` | Giây không có dữ liệu thì cắt stream (mặc định 120) |

## Model đang bán

| Model | Cách tính | Vào /1M | Ra /1M | Cache /1M |
| --- | --- | --- | --- | --- |
| `deepseek-v4.1-flash` | token | $0.08 | $0.28 | $0.004 |
| `deepseek-v4.1-flash-1017` | token | $0.08 | $0.28 | $0.032 |
| `claude-opus-5-5` | token | $0.30 | $1.50 | $0.15 |
| `gpt-6.1-sol` | token | $0.15 | $0.90 | $0.015 |
| `cx/gpt-6-luna` | token | $0.03 | $0.06 | $0.0225 |
| `claude-opus-4-8` | request | $0.008/lần | — | — |
| `claude-opus-5.5` | request | $0.02/lần | — | — |
| `claude-fable-5.1-req` | request | $0.02/lần | — | — |

Model tính theo request khai báo ở `ModelPrice` (USD mỗi lần gọi), model tính
theo token khai báo ở `ModelRatio` + `CompletionRatio` + `CacheRatio`.

## Cấu hình nhà cung cấp

Vào **Kênh → Thêm kênh**:

| Trường | Giá trị |
| --- | --- |
| Loại | `OpenAI` |
| Base URL | Base URL của upstream, **không** kèm `/v1` |
| Khóa | API key của upstream |
| Model | ID model, phân tách bằng dấu phẩy, khớp chính xác upstream |

Bấm **Kiểm tra**. Nếu báo `model_price_error` nghĩa là chưa khai báo giá —
chuyển sang bước dưới.

### Đặt giá bán

**Cài đặt hệ thống → Nhóm & Giá model**:

| Khóa | Công thức |
| --- | --- |
| `ModelRatio` | `giá_vào_USD_trên_1M ÷ 2` |
| `CompletionRatio` | `giá_ra ÷ giá_vào` |

Ví dụ bán $0.08 vào / $0.28 ra cho một model:

```json
{ "ModelRatio": { "deepseek-v4.1-flash": 0.04 },
  "CompletionRatio": { "deepseek-v4.1-flash": 3.5 } }
```

Nguyên tắc: `giá bán ≥ giá vốn × 1.1` và `giá bán ≤ giá chính thức × 0.8`.

## Cache prompt

Upstream tự cache phần đầu prompt lặp lại (system prompt, tài liệu, lịch sử hội
thoại). dgai tính phần đã cache với `CacheRatio` thấp hơn nhiều so với token mới.

| Model | CacheRatio | Giá token cache /1M |
| --- | --- | --- |
| `deepseek-v4.1-flash` | 0.05 | $0.004 |
| `deepseek-v4.1-flash-1017` | 0.4 | $0.032 |

Đặt `CacheRatio` ở **Cài đặt hệ thống → Nhóm & Giá model**. Giá trị phải phản
ánh giá vốn cache của upstream (`cached_input_price` trong `/v1/models`), nếu
không sẽ bán lỗ phần cache.

Rule `channel_affinity_setting` giữ request của cùng người dùng trên một channel
trong 1 giờ (`dgai deepseek sticky`), để cache upstream không bị nguội do đổi
nhà cung cấp giữa các lượt.

## Nạp tiền

| Kênh | Trạng thái |
| --- | --- |
| Mã nạp (redemption code) | Bật sẵn, cộng số dư tức thì |
| Chuyển khoản + QR | `TopUpLink` trỏ tới ảnh VietQR; quản trị viên xác nhận tay |
| EPay / Stripe | Cần cấu hình merchant, xem **Cài đặt → Thanh toán** |

Mức nạp tối thiểu và các mức có sẵn đặt ở `MinTopUp` và
`payment_setting.amount_options`.

## Tài liệu trong ứng dụng

Trang `/docs` phục vụ tài liệu riêng của dgai (nội dung ở
`web/src/features/docs/content.ts`): bắt đầu nhanh, tham chiếu API, model và
giá, ví và nạp tiền, cấu hình nhà cung cấp. Trang không liên kết ra tài liệu
bên ngoài.

## Phát triển

```bash
# Backend + PostgreSQL + Redis
docker compose -f docker-compose.dev.yml up -d --build

# Frontend (proxy /api và /v1 về :3000)
cd web && bun run dev

# Kiểm thử
make test
```

## Giấy phép

AGPL-3.0. Xem `LICENSE` và `NOTICE`. Bản quyền gốc và yêu cầu ghi công thuộc về
những người đóng góp new-api; các thay đổi của dgai được ghi nhận trong lịch sử
commit.

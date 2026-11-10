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

## Triển khai lên server

Ảnh production được build trên GitHub Actions và đẩy lên GHCR, nên server chỉ
cần kéo về — không cài Go/Bun.

### 1. Chuẩn bị DNS

Trỏ bản ghi `A` cho `@` (apex) và bản ghi `CNAME` cho `www` về apex. Mở port 80
và 443. Caddy sẽ tự xin chứng chỉ Let's Encrypt cho cả `DGAI_SITE_ADDRESS` và
`www.<DGAI_SITE_ADDRESS>`, rồi chuyển hướng `www` về apex.

### 2. Cài Docker trên server

```bash
curl -fsSL https://get.docker.com | sh
sudo usermod -aG docker "$USER"   # đăng nhập lại sau bước này
```

### 3. Lấy mã nguồn và cấu hình

```bash
git clone https://github.com/DungZG/dgai.git /opt/dgai
cd /opt/dgai
cp .env.dgai.example .env.dgai
```

Sửa `.env.dgai`:

| Biến | Ghi chú |
| --- | --- |
| `DGAI_IMAGE_REPO` | `<owner>/dgai`, khớp tên repo GHCR |
| `DGAI_SITE_ADDRESS` | Domain, không có `https://` |
| `DGAI_PUBLIC_URL` | `https://<domain>` |
| `DGAI_DB_PASSWORD` | `openssl rand -hex 24` |
| `DGAI_REDIS_PASSWORD` | `openssl rand -hex 24` |
| `DGAI_SESSION_SECRET` | `openssl rand -hex 32` |

Ảnh hiện được publish public tại `ghcr.io/dungzg/dgai:latest`, nên pull không
cần đăng nhập. Nếu sau này chuyển package sang private:

```bash
echo "$GHCR_TOKEN" | docker login ghcr.io -u <github-user> --password-stdin
```

### 4. Khởi động

```bash
docker compose -f docker-compose.prod.yml --env-file .env.dgai pull
docker compose -f docker-compose.prod.yml --env-file .env.dgai up -d
```

Mở `https://<domain>` và làm theo trình khởi tạo để tạo tài khoản quản trị.

### 5. Cập nhật phiên bản

Mỗi lần push lên `main`, GitHub Actions build ảnh mới với tag `latest` và tag
`<sha>`. Trên server:

```bash
git pull
docker compose -f docker-compose.prod.yml --env-file .env.dgai pull dgai
docker compose -f docker-compose.prod.yml --env-file .env.dgai up -d dgai
```

Ghim phiên bản bằng cách đặt `DGAI_IMAGE_TAG=<sha>` thay vì `latest`.

### 6. Sao lưu

```bash
./scripts/backup-dgai.sh
```

Ghi `pg_dump` nén vào `backups/`, giữ 14 ngày. Thêm vào cron:

```
0 3 * * * cd /opt/dgai && ./scripts/backup-dgai.sh >> /var/log/dgai-backup.log 2>&1
```

### 7. Kiểm tra sau khi lên

```bash
curl -s https://<domain>/api/status | head -c 200   # system_name phải là dgai
docker compose -f docker-compose.prod.yml ps        # tất cả healthy
```

Sau đó vào **Cài đặt hệ thống → Site & Branding**, đặt `ServerAddress` =
`https://<domain>` để link OAuth/VietQR sinh đúng.

## Triển khai cục bộ (phát triển)

Yêu cầu: Docker, Bun (build frontend trên máy host).

```bash
# 1. Cấu hình bí mật
cat > .env.dgai <<'EOF'
DGAI_DB_PASSWORD=<mật-khẩu-mạnh>
DGAI_REDIS_PASSWORD=<mật-khẩu-mạnh>
DGAI_SESSION_SECRET=<chuỗi-ngẫu-nhiên-32-byte>
DGAI_PUBLIC_URL=https://localhost
DGAI_SITE_ADDRESS=localhost
EOF

# 2. Build frontend (được nhúng vào binary qua //go:embed)
cd web && bun install --frozen-lockfile && bun run build && cd ..

# 3. Dựng stack
docker compose -f docker-compose.dgai.yml --env-file .env.dgai up -d --build
```

`Caddyfile.dgai` dùng `tls internal` (chứng chỉ tự ký) nên trình duyệt sẽ cảnh
báo. Dùng cho máy dev, không dùng cho khách.

## Cache prompt

Upstream tự cache phần đầu prompt lặp lại (system prompt, tài liệu, lịch sử hội
thoại). dgai tính phần đã cache với `CacheRatio` thấp hơn nhiều so với token mới.

| Model | CacheRatio | Giá token cache /1M |
| --- | --- | --- |
| `deepseek-v4.1-flash-1017` | 0.4 | $0.032 |
| `gemini-3.8-flash` | 0.125 | $0.0075 |
| `gpt-6-luna` | 0.75 | $0.0225 |

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

## VietQR (chuyển khoản ngân hàng)

Bật ở **Cài đặt → Billing & Payment → VietQR**. Tài khoản nhận tiền hiện tại:
**Techcombank · 0395185726**.

| Cài đặt | Ý nghĩa |
| --- | --- |
| `VietQREnabled` | Bật phương thức thanh toán |
| `VietQRBankCode` / `VietQRBankAccount` | `TCB` / `0395185726` |
| `VietQRAccountName` | Tên chủ tài khoản (không dấu), hiện trên QR |
| `VietQRVNDRate` | Tỷ giá USD→VND dùng để quy đổi số tiền trên QR |
| `VietQRMinTopUp` | Mức nạp tối thiểu |

Luồng: khách chọn VietQR → backend tạo đơn `pending` + trả ảnh QR (img.vietqr.io)
với số tiền VND và nội dung `DGAI<userId><mã>` → khách quét và chuyển khoản.

### Đối soát tự động

Hai endpoint cho VietQR Callback API:

| Endpoint | Việc |
| --- | --- |
| `POST /api/user/vietqr/token_generate` | Basic auth → bearer HMAC 5 phút |
| `POST /api/user/vietqr/transaction-sync` | Nhận biến động số dư, cộng hạn mức |

Cấu hình `VietQRSyncUsername/Password/SecretKey` rồi khai báo URL này với VietQR.
Nếu chưa có hợp đồng merchant, để trống — quản trị viên xác nhận tay bằng
`POST /api/user/topup/complete` với `trade_no` của đơn.

Đối soát an toàn: replay cùng giao dịch không cộng hai lần; chuyển thiếu tiền bị
từ chối (`AMOUNT_MISMATCH`); giao dịch ghi nợ (D) bị bỏ qua.

## Kết nối CLI (Claude Code / Codex)

Trong **API Keys**, mở menu của một key → **Connect Claude Code / Codex**.

| Client | Cần chọn | Sinh ra |
| --- | --- | --- |
| Claude Code | model cho Sonnet / Opus / Haiku | bash, PowerShell, `~/.claude/settings.json` |
| Codex | một model GPT | `~/.codex/config.toml`, export `DGAI_API_KEY` |

Claude Code nói Anthropic Messages API → gateway phục vụ tại `POST /v1/messages`.
Codex nói Responses API → `POST /v1/responses` với `wire_api = "responses"`.

Hai biến bắt buộc phía Claude Code:

```bash
export ANTHROPIC_BASE_URL="https://dgai.io.vn"       # origin, không có /v1
export ANTHROPIC_AUTH_TOKEN="sk-..."
```

Phía Codex, key nằm trong biến mà `env_key` trỏ tới:

```toml
model = "gpt-6.1-sol"
model_provider = "dgai"

[model_providers.dgai]
name = "dgai"
base_url = "https://dgai.io.vn/v1"
wire_api = "responses"
env_key = "DGAI_API_KEY"
```

## Tài liệu trong ứng dụng

Trang `/docs` phục vụ tài liệu riêng của dgai (nội dung ở
`web/src/features/docs/content.ts`): bắt đầu nhanh, tham chiếu API, model và
giá, ví và nạp tiền. Trang không liên kết ra tài liệu bên ngoài.

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

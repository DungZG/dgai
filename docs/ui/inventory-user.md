# Inventory — màn hình & popup phía người dùng (dgai web)

Quy ước: `Route` → `file trang` → cấu trúc → mọi dialog/drawer.

## Khung chung
- Public: `PublicLayout` (`web/src/components/layout/components/public-layout.tsx`) + `PublicHeader` (nav: Home, Console, Model Square, Rankings, Docs, About; theme switch, language switch, notifications, Sign in).
- Đã đăng nhập: `AuthenticatedLayout` + `AppSidebar` (`web/src/hooks/use-sidebar-data.ts`) + `AppHeader` (breadcrumb, search `CommandMenu` ⌘K, notification popover, theme toggle, user menu).
- Nhóm sidebar: Chat (Playground, Chat presets) · General (Overview, Dashboard, API Keys, Usage Logs, Audit Logs, Task Logs) · Personal (Wallet, Profile, Security & Access) · Admin (Channels, Models, Users, Redemption Codes, Subscriptions, System Info, Task Plugins, System Settings).
- Bảng dùng `DataTablePage`: toolbar search debounce, filter dropdown, phân trang server-side, card view mobile, bulk actions.
- `CommandMenu` (`web/src/components/command-menu.tsx`): ⌘K, nhóm Navigation / Theme / Language / Account.
- `NotificationPopover` (`web/src/components/notification-popover.tsx`): danh sách thông báo, mark read, empty state.

---

## 1. `/` — Home (public)
- Trang: `web/src/features/home/index.tsx`
- Sections: Hero (badge "AI Application Infrastructure Foundation", H1 "Unified API Gateway for Vast Range of AI Models", 2 CTA Get Started/View Pricing, Docs, **Supported Applications: Claude Code · Codex · Setup guide**), `HeroTerminalDemo` (tabs Chat/Responses/Claude/Gemini, request/response panes, status 200 OK, latency/token/cost footer, stream + SSE badges), `GatewayCard`, feature grid (`FeatureItem`, `IconCard`), `StatItem`, `ScrollingIcons` (marquee logo model), connection line.
- Không có dialog; link `/docs`, `/pricing`, `/sign-up`.

## 2. `/docs` — Documentation (public)
- Trang: `web/src/features/docs/index.tsx`, nội dung `content.ts`
- Layout 2 cột: sidebar mục (Bắt đầu, Tham chiếu API, Model và giá, Ví và nạp tiền, Cấu hình nhà cung cấp) + article Markdown (`Markdown` component: bảng, code block, heading anchor).
- Không dialog.

## 3. `/about` — About (public)
- Trang: `web/src/features/about/index.tsx`
- Render nội dung admin cấu hình: plain text / HTML / URL (iframe). Empty state `Construction` icon + link Documentation + license.

## 4. `/pricing` — Model Square (public)
- Trang: `web/src/features/pricing/` — `index` + `components/`
- Toolbar: search model, Filter (dialog), Sort, Price display mode (Standard/Recharge), Token unit, Grid/Table toggle.
- Grid: `ModelCard` (vendor logo, name, tags, price in/out, cache badge, capability badges, Vendor, latency badge, link chi tiết).
- Table: `pricing-columns.tsx` — Type, Cached, Vendor, Tags, Endpoints, Groups.
- **Dialog Filter** (`pricing-toolbar.tsx` inline): vendor, endpoint type, tags, group, price range.
- Model detail `/pricing/$modelId` — `components/model-details.tsx`: header (logo, name, vendor, badges) + 3 tabs: **Overview** (pricing table theo token/cache, ratio, capabilities, groups, endpoints), **Performance** (`model-details-performance.tsx`: success rate, latency, throughput, uptime sparkline), **API** (`model-details-api.tsx`: curl/SDK snippets theo endpoint, copy).
- States: `loading-skeleton.tsx`, `empty-state.tsx`.

## 5. `/rankings` — Rankings (public)
- Trang: `web/src/features/rankings/index.tsx`
- Sections: `RankingsHero` (period switch), `ModelsSection` (leaderboard bảng + sparkline), `MarketShareSection` (vendor share), `PulseSection` (top movers/droppers).
- States: `RankingsLoading` skeleton, `RankingsError`.

## 6. `/sign-in` — Sign in (public)
- Trang: `web/src/features/auth/sign-in/index.tsx`, form `components/user-auth-form.tsx`
- Fields: username/email, password, captcha (Turnstile), OTP bước 2 (`/otp`), Passkey button, OAuth providers row, legal consent checkbox, links Forgot password / Sign up / Terms / Privacy.
- Dialogs: `telegram-login-dialog.tsx` (QR/mã xác thực), `oauth-callback-screen.tsx` (màn chờ callback).

## 7. `/sign-up` — Sign up (public)
- Trang: `web/src/features/auth/sign-up/index.tsx`, form `sign-up-form.tsx`
- Fields: username, password, confirm, email (+ verification code khi bật), affiliate code, legal consent, captcha.
- Cùng `oauth-providers.tsx` như sign-in.

## 8. `/forgot-password`, `/reset`, `/otp`, `/oauth/$provider`
- `forgot-password/components/forgot-password-form.tsx`: email → gửi link.
- `reset-password-confirm`: mật khẩu mới + confirm.
- `otp/components/otp-form.tsx`: 6 ô nhập mã, resend countdown.
- `oauth/$provider`: `oauth-callback-screen.tsx` (spinner, lỗi, nút quay lại).

## 9. `/setup` — Setup wizard (chỉ khi chưa init)
- Trang: `web/src/features/setup/setup-wizard.tsx`
- Steps: `database-step` (loại DB, DSN), `usage-mode-step` (Self-use / Demo site), `admin-step` (username, password, confirm, logo), `complete-step`. `step-navigation` (Back/Next/Initialize), thanh tiến trình; lỗi inline theo field.

## 10. `/dashboard` (+`/$section`) — Console home
- Trang: `web/src/features/dashboard/index.tsx`; sections `overview | models | flow | users(admin)`.
- **overview**: Setup guide (collapsible, lưu localStorage), `SummaryCards` (Credit remaining, Last 24h usage, Runway, trạng thái Healthy/Low/Depleted, CTA add credits), `PerformanceHealthPanel` (admin), `ApiInfoPanel`, `AnnouncementsPanel` → dialog **Announcement Details**, `FAQPanel` (accordion), `UptimePanel`.
- **models**: `LogStatCards` (7 thẻ số), `PerformanceOverview` (admin), `ConsumptionDistributionChart`, `ModelCharts` (3 tab: Call Trend / Call Count Distribution / Call Count Ranking, VChart).
  - Dialogs: **Model Analytics Defaults** (`ModelsChartPreferences`: chart mặc định, range, granularity), **Filters** (`ModelsFilter`: quick range, start/end date-time, granularity, username khi admin).
- **flow**: Sankey (`FlowCharts`) + controls: metric (quota/tokens/requests), display limit, overflow mode, user multi-select, toggle từng stage; eye toggle ẩn dữ liệu nhạy cảm; dialog Filters (như trên).
- **users** (admin): 2 chart User Consumption Ranking / Trend + range/granularity/topN.

## 11. `/keys` — API Keys
- Trang: `web/src/features/keys/`; bảng `api-keys-table.tsx`
- Cột: select, Name, Status, API Key (mask + copy), Quota (thanh tiến trình), Group, Models, IP Restriction, Time, Expires, Actions.
- Toolbar: search, filter status/group/model, refresh, eye mask, bulk delete.
- Header actions: **Create API key**.
- **Drawer `api-keys-mutate-drawer.tsx`** (create/update): sections Basic Information (Name, Group, Auto group order, Status switch), Quota Settings (unlimited switch, quota input, quantity 1–N khi tạo), Expiration Time (Never + date picker), Advanced Settings (Model Limits multi-select, IP restriction textarea, allow-ips), footer Save.
- **`api-keys-delete-dialog.tsx`**: confirm xoá 1 key.
- **`api-keys-multi-delete-dialog.tsx`**: confirm xoá nhiều.
- **`dialogs/cc-switch-dialog.tsx`**: "Import to CC Switch" — chọn key, tạo deep link `ccswitch://`, nút Open CC Switch, cảnh báo nếu chưa cài.
- Row menu: Edit, Copy key, CC Switch, Enable/Disable, Delete.

## 12. `/wallet` — Wallet
- Trang: `web/src/features/wallet/index.tsx`
- `WalletStatsCard`: Current Balance, Remaining quota, Total consumed, Total Usage, API Requests.
- `RechargeFormCard`: Payment Method (tabs: online methods, Creem, Waffo), preset amount chips + Custom Amount, Minimum: $1, amount to pay, "Add Funds"; khối **Have a Code?** (input redemption code + Redeem + "Get one here"), link **Order History**.
- `SubscriptionPlansCard`: danh sách gói (title, price, quota, duration) + nút mua.
- `AffiliateRewardsCard`: Available Rewards, nút **Transfer Rewards**.
- `CreemProductsSection`: sản phẩm Creem one-time.
- Dialogs:
  - **`dialogs/billing-history-dialog.tsx`** — bảng Order History (Time, Amount, Payment, Status), phân trang page-size, empty "No billing records found", nút Complete Order cho order pending.
  - **`dialogs/payment-confirm-dialog.tsx`** — "Review your payment details": Payment Method, Topup Amount, You Pay, You save, Confirm Payment / Cancel.
  - **`dialogs/transfer-dialog.tsx`** — "Transfer Rewards": Available Rewards, Transfer Amount, Min, Transfer.
  - **`dialogs/creem-confirm-dialog.tsx`** — xác nhận mua sản phẩm Creem.

## 13. `/usage-logs` (+`/common|/task|/drawing`, `/audit`) — Logs
- Trang: `web/src/features/usage-logs/index.tsx`; sections `common | drawing | task`; `audit` riêng.
- **Common logs**: `LogStatCards` (số request, quota, token, cache), filter bar (`CommonLogsFilterBar`: type multi-select, model, token, group, channel, username, request ID, upstream request ID, time range picker), export CSV (`export-logs-dialog.tsx`: chọn range/format), refresh, cột Time|Channel|User|Token|Model|Stream|Cost|Timing|Details, mobile card view.
- **Task logs**: cột Submit Time|User|Plugin|Task ID|Status|Artifacts|Details; `TaskArtifacts` (tải file sinh ra).
- **Drawing logs**: cột MJ riêng (type, prompt, progress, images).
- **Audit logs**: bảng thao tác quản trị (actor, action, target, time, IP), filter.
- Dialogs:
  - **`details-dialog.tsx`** (44KB, màn chi tiết lớn): tabs/khối Billing Details (model ratio, completion ratio, cache, group ratio, billable token breakdown), Request, Response, Error, Timing, Metadata; copy request ID.
  - **`prompt-dialog.tsx`** — xem prompt đầy đủ.
  - **`fail-reason-dialog.tsx`** — lý do lỗi.
  - **`image-dialog.tsx`** — lightbox ảnh sinh.
  - **`audio-preview-dialog.tsx`** — player audio.
  - **`task-details-dialog.tsx`** — chi tiết task (status, plugin, artifacts, timeline).
  - **`user-info-dialog.tsx`** — thông tin user (admin).
  - **`export-logs-dialog.tsx`** — xuất log.

## 14. `/playground` — Playground
- Trang: `web/src/features/playground/index.tsx`
- Layout: cột chat (`PlaygroundChat`, `PlaygroundMessageContent` markdown/stream, `MessageActions`: copy/edit/retry, `MessageMetadata`: tokens/latency/model), composer (`PlaygroundInput`, `InputControls`: model select, stream switch, key select, `InputTools`: attach, clear), panel phải `ParameterPanel` (temperature, top_p, max_tokens, presence/frequency penalty, seed, stop — chỉ tham số bật mới gửi).
- Dialogs: **Clear chat history** confirm, **Raw response** viewer, **Parameters** (mobile), error state "Model Price Not Configured" + link Settings, empty state gợi ý prompt.

## 15. `/profile` — Profile
- Trang: `web/src/features/profile/index.tsx`
- `ProfileHeader` (avatar, username, role badge, group), `ProfileSettingsCard` (display name, email read-only, save), `LanguagePreferencesCard` (language select), `SidebarModulesCard` (chọn module hiển thị sidebar), `CheckinCalendarCard` (lịch điểm danh + streak), tab **Notifications** (`tabs/notification-tab.tsx`: quota warning, webhook URL/secret, Bark, Gotify, email).
- Dialogs trong notification tab: test webhook/bark/gotify (confirm + toast).

## 16. `/security` — Security & Access (mọi user)
- Trang: `web/src/features/security/index.tsx`, layout 2 cột card.
- Cards: `AccountActionCard` (đổi mật khẩu, xoá tài khoản), `AccountBindings` (email/Telegram/WeChat/OAuth), `LoginSessionsCard`, `AccessTokensCard`, `PasskeyCard`, `TwoFACard`, `PrivacyCard`.
- Dialogs:
  - `change-password-dialog.tsx` (mật khẩu hiện tại, mới, xác nhận; strength hint)
  - `delete-account-dialog.tsx` (gõ xác nhận + step-up verification)
  - `email-bind-dialog.tsx`, `telegram-bind-dialog.tsx`, `wechat-bind-dialog.tsx` (mã xác thực, QR)
  - `access-token-create-dialog.tsx` (name, scopes multi-select, expiry)
  - `access-token-dialog.tsx` (hiện token 1 lần + copy), `access-token-edit-dialog.tsx`
  - `two-fa-setup-dialog.tsx` (QR + secret + mã 6 số), `two-fa-disable-dialog.tsx`, `two-fa-backup-dialog.tsx` (danh sách mã dự phòng + copy/tải)
  - `login-session-dialogs.tsx` (chi tiết phiên: thiết bị, IP, thời gian, revoke), revoke-others confirm.

## 17. `/subscriptions` — Subscriptions (admin)
- Đã liệt kê trong inventory-admin (§10).

## 18. `/chat/$chatId`, `/chat2link`
- `chat/$chatId`: mở preset chat bên thứ ba (web/native), chọn API key đang hoạt động, cảnh báo thiếu key, loading spinner.
- `chat2link`: trang chuyển tiếp tạo deep link.

## 19. Errors
- `/401`, `/403`, `/404`, `/500`, `/503`, `/errors/$error`: `web/src/features/errors/*` — mã lỗi, mô tả, nút Back/Home/Retry, link docs.

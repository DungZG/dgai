# Prompt gửi agent thiết kế (pendev)

> Copy toàn bộ khối dưới đây vào agent thiết kế, kèm 2 file
> `docs/ui/inventory-user.md` và `docs/ui/inventory-admin.md`.

---

## Bối cảnh

Bạn thiết kế lại giao diện cho **dgai** — một nền tảng bán API AI (API gateway +
ví credit + bảng quản trị) chạy bằng React 19, TanStack Router/Query, Base UI,
Tailwind CSS 4, i18next. Người dùng là lập trình viên Việt Nam dùng Claude Code,
Codex, Cursor; quản trị viên là chủ nền tảng.

Sản phẩm hiện có 2 vùng:

| Vùng | Người dùng | Mục tiêu |
| --- | --- | --- |
| Public | Khách, dev | Hiểu sản phẩm trong 10 giây, thấy giá, lấy key, đọc docs |
| Console | Dev đã đăng nhập | Tạo key, xem usage, nạp tiền, chat thử model |
| Admin | Chủ nền tảng | Quản lý kênh upstream, giá, người dùng, doanh thu |

## Ràng buộc kỹ thuật (không được vi phạm)

- Stack cố định: React 19, TanStack Router (file-based routes), TanStack Query,
  TanStack Table, Base UI, Tailwind 4, i18next.
- Design token nằm ở `web/src/styles/theme.css` (`--background`, `--foreground`,
  `--card`, `--primary`, `--muted`, `--destructive`, `--success`, `--warning`,
  `--radius`, …) và `theme-presets.css`. Mọi màu phải dùng token, không hardcode.
- Có **dark mode** (`.dark`) và **10 theme preset** (`default`, `anthropic`,
  `simple-large`, `underground`, `rose-garden`, `lake-view`, `sunset-glow`,
  `forest-whisper`, `ocean-breeze`, `lavender-dream`). Thiết kế phải đẹp ở cả
  light và dark, và không được giả định một preset cụ thể.
- Có 7 ngôn ngữ (en, vi, zh, zh-TW, fr, ru, ja). Không thiết kế layout chỉ vừa
  tiếng Anh — chuỗi tiếng Đức/Việt dài hơn 30% là chuyện bình thường.
- Component kit có sẵn ở `web/src/components/ui/` (button, card, table, tabs,
  dialog, drawer, sheet, command, chart, …). Không thay bằng thư viện ngoài.
- Chart dùng VChart (`@visactor/react-vchart`) và Recharts — đã có wrapper.
- Mọi text phải đi qua `t('English source string')`.

## Những gì cần bạn làm

1. Đọc 2 file inventory (`inventory-user.md`, `inventory-admin.md`) — liệt kê
   **mọi màn hình, mọi dialog/drawer** kèm file nguồn.
2. Đề xuất **hệ thống thiết kế mới**: typography scale, spacing, radius, màu,
   elevation, motion, iconography, trạng thái (hover/focus/disabled/loading/empty).
3. Thiết kế lại theo thứ tự ưu tiên:
   - **P0**: Home, Docs, Pricing (Model Square), Sign-in/Sign-up, Wallet, API Keys.
   - **P1**: Dashboard (overview/models/flow), Usage Logs, Playground, Profile,
     Security.
   - **P2**: Channels, Models, Users, Redemption Codes, Subscriptions.
   - **P3**: System Settings (8 nhóm, ~30 section), System Info, Task Plugins.
4. Với mỗi màn hình: wireframe + spec (component, spacing, state, responsive
   breakpoint, dark mode) và mapping sang component có sẵn.
5. Ghi rõ chỗ nào **giữ nguyên hành vi** — không được đổi luồng chức năng, chỉ
   đổi trình bày. Dialog và field phải giữ đúng như inventory.

## Điểm nhấn thương hiệu

- Tên: **dgai**. Sản phẩm bán API giá rẻ; định vị "rẻ, nhanh, minh bạch".
- Khách chính dùng CLI (Claude Code, Codex) — giao diện nên nghiêng về
  developer-tool: mật độ thông tin cao, code block rõ, copy một chạm.
- Hai con số phải nổi bật ở mọi nơi liên quan tiền: **số dư ví** và **chi phí
  mỗi request**.
- Cache là điểm bán: hiển thị rõ "cached tokens" và mức tiết kiệm.

## Ràng buộc về nội dung phải giữ

- Bảng dữ liệu lớn (channels, models, logs) cần: cột ghim, sắp xếp, lọc, phân
  trang server-side, chế độ bulk-select, sticky header, view mobile dạng card.
- Form settings dài cần: dirty indicator, nút Save/Reset cố định, chặn rời trang
  khi chưa lưu, lỗi theo field.
- Dialog xác nhận nguy hiểm (xoá user, xoá channel, đổi quyền) phải giữ
  bước xác thực lại (step-up verification) — không được giảm còn 1 cú click.
- Số liệu tiền tệ luôn có đơn vị rõ (USD, hoặc đơn vị hiển thị admin cấu hình).
- Không dùng hình ảnh/illustration nặng; sản phẩm này là công cụ.

## Deliverable

- `styleguide.md` — token, typography, spacing, màu, motion.
- `screens/*.md` — mỗi màn hình một file: wireframe ASCII/SVG + spec + state +
  mapping component.
- `components.md` — component mới cần thêm (nếu có) và lý do không tái dùng
  component hiện tại.
- `changelog.md` — thay đổi so với giao diện hiện tại.

## Không được làm

- Không đổi route, không đổi API contract, không đổi tên field.
- Không bỏ dialog/drawer nào trong inventory.
- Không thêm dependency UI mới khi `web/src/components/ui/` đã có.
- Không hardcode màu/kích thước ngoài token.
- Không giả định nội dung tiếng Anh là độ dài chuẩn.

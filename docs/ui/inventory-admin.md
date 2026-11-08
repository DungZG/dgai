# Inventory — màn hình & popup phía quản trị (dgai web)

## Khung chung
- Route layout: `web/src/routes/_authenticated/route.tsx` — guard đăng nhập (redirect `/sign-in`); mỗi route admin tự guard role trong `beforeLoad`, redirect `/403`.
- Khung trang: `SectionPageLayout` (`web/src/components/layout`) — Title / Actions / Content. Bảng dùng `DataTablePage` (`web/src/components/data-table`): toolbar search debounce + filter dropdown + pagination server-side + mobile card view + bulk actions.
- Settings: `SettingsPage` (`web/src/features/system-settings/components/settings-page.tsx`) — load options toàn cục, loading text / `ErrorState` retry; section registry `createSectionRegistry`; form: Save Changes / Reset portal lên header, `FormDirtyIndicator` + `FormNavigationGuard` (chặn rời trang khi dirty); field helper `SettingsSwitchItem`, `SettingsFormGrid`.

---

## 1. `/dashboard/$section` — Dashboard
- Routes: `dashboard/index.tsx` (redirect), `$section.tsx`; sections `overview`, `models`, `flow`, `users` (adminOnly).
- **overview**: Setup guide (collapsible), `SummaryCards` (Credit remaining / Last 24h usage / Runway + CTA wallet; Healthy/Low/Depleted), `PerformanceHealthPanel` (admin), `ApiInfoPanel`, `AnnouncementsPanel` (+ `announcement-detail-dialog.tsx`), `FAQPanel`, `UptimePanel`.
- **models**: `LogStatCards` (7 stat), `PerformanceOverview` (admin), `ConsumptionDistributionChart`, `ModelCharts` (3 tab). Dialogs: `ModelsChartPreferences` ("Model Analytics Defaults"), `ModelsFilter` (quick range, date start/end, granularity, admin username).
- **flow**: Sankey `flow-charts.tsx`; controls metric/display limit/overflow/user multi-select/toggle stage; eye toggle; `ModelsFilter`.
- **users** (admin): `user-charts.tsx` — range, granularity, Top N; 2 chart.
- Loading: skeleton cho mọi chart (lazy + Suspense).

## 2. `/channels` — Kênh upstream
- Route: `channels/index.tsx` — role ≥ ADMIN; URL search: page, pageSize, filter, status[], type[], group[], model.
- Trang: `web/src/features/channels/index.tsx` — Title + badge "Max Retries" (link `/system-settings/request-policies/routing`).
- Header actions: Switch Batch Operations / Tag Mode, `ChannelSortControl`, "Create Channel", menu ⋯: Test All, Update All Balances, Detect/Apply All Upstream Updates, Repair Consistency, Delete All Disabled (+ConfirmDialog).
- Bảng `channels-columns.tsx`: select, ID (mask), Name (+ upstream +N/−N), Type (logo+label), Status, Models, Groups, Tag, Priority (inline spinner, drag reorder), Weight, Used/Remaining, Response, Last Tested, Actions (pin phải).
- Toolbar: search name/ID/key, input filter model, filter type/group, eye, refresh. Bulk: Enable, Disable, Set Tag, Delete.
- Row menu: Edit, Test Connection, Query Balance, Fetch Models, Upstream Updates, Manage Ollama Models, Copy Channel, Manage Keys, Delete.
- Dialogs/Drawers (state ở `channels-provider.tsx`):
  - `drawers/channel-mutate-drawer.tsx` — Create/Update: **Provider picker** (`channel-provider-picker.tsx`) → form 3 khối (`channel-configuration.tsx`): Basic (`channel-basic-section.tsx`: name, status, tag, remark), API access (`channel-api-access-section.tsx` + `channel-auth-section.tsx`: base_url, key, multi-key, field mã hoá theo loại Azure/Vertex/AWS/Bedrock/Cloudflare/Ollama), Models (`channel-models-section.tsx`: fetch models, groups, model mapping); block phụ: Routing Strategy (priority/weight/test model/auto ban), Request processing, Override Rules (status_code_mapping, param_override, header_override), Internal Notes, Upstream Model Detection.
  - `channel-test-dialog.tsx` — Test Connection (sheet lớn): endpoint type combobox, stream switch, bảng model/status/result/actions, batch test, stop, select successful, delete failed, copy models; `FailureDetailsSheet`.
  - `balance-query-dialog.tsx`, `fetch-models-dialog.tsx`, `ollama-models-dialog.tsx`, `multi-key-manage-dialog.tsx` (+`multi-key-statistics-card.tsx`), `copy-channel-dialog.tsx`, `inference-status-dialog.tsx`, `tag-batch-edit-dialog.tsx`, `edit-tag-dialog.tsx`, `upstream-update-dialog.tsx`, `codex-usage-dialog.tsx`, `configure-models-dialog.tsx`, `advanced-custom-editor-dialog.tsx`, `param-override-editor-dialog.tsx`, `missing-models-confirmation-dialog.tsx`, `status-code-risk-dialog.tsx`, `passthrough-warning-dialog.tsx`.

## 3. `/models` (+`/$section`) — Models
- Sections: `metadata` (Models) / `vendors` / `deployments`. Route guard role ≥ ADMIN.
- **metadata**: actions "Sync metadata", "Sync pricing", "Add Model", ⋯ (Missing Models, Prefill Groups). Bảng `models-columns.tsx`: select, Model (pin trái), Pricing, Model square visibility, Channels and groups (popover `model-connections.tsx`), Tags, Sync policy, Display policy, Vendor, Match Type, Description, Custom endpoints, Created, Updated, ID, Actions.
  - Bulk: delete → `ModelDeleteDialog`.
- **vendors**: "Add Vendor" → `vendor-mutate-dialog.tsx`; bảng Vendor/Description/Linked models/ID/Created/Updated/Actions; bulk Merge/Delete → `vendor-operation-dialog.tsx`.
- **deployments**: "Create deployment" → `dialogs/create-deployment-drawer.tsx`; guard `deployment-access-guard.tsx`; bảng ID/Name/Status/Provider/Time remaining/Hardware/Created; row: `view-details-dialog.tsx`, `view-logs-dialog.tsx`, `update-config-dialog.tsx`, `extend-deployment-dialog.tsx`, `rename-deployment-dialog.tsx`.
- Dialogs chung: `ModelMutateDrawer` (`drawers/model-mutate-drawer.tsx`), `SyncWizardDialog` (multi-step, bảng Applied changes), `MissingModelsDialog`, `PrefillGroupManagement` + `prefill-group-management-dialog.tsx` + `drawers/prefill-group-form-drawer.tsx`, `DescriptionDialog`, `PriceSyncDialog`.

## 4. `/users` — Người dùng
- Role ≥ ADMIN; search: page, pageSize, filter, status, role, group.
- Actions: "Add User".
- Bảng `users-columns.tsx`: select, ID, Username (+display name, remark), Status, Available Balance (mở `user-quota-dialog.tsx`), User Group, Role, Invite Info, Time (created/access), Actions.
- Row menu: Edit, Enable/Disable, Demote/Promote, Manage Bindings, Manage Subscriptions, Reset Passkey, Reset 2FA, Delete.
- Dialogs:
  - `users-mutate-drawer.tsx`: Username, Role, Display Name, Password, Group combobox, Remaining Quota (+ "Adjust Quota" → `user-quota-dialog.tsx`: Add/Subtract/Override + preview), Remark, **Admin permissions matrix** (checkbox nhóm). Đổi permission → step-up verification.
  - `users-delete-dialog.tsx` (ConfirmDialog + verification)
  - `dialogs/user-binding-dialog.tsx` — Manage account bindings: list provider, bound/unbound, Unlink → confirm + verification.
  - `UserSubscriptionsDialog` (dùng chung với Subscriptions).
  - ConfirmDialog Reset Passkey / Reset 2FA (verification).

## 5. `/redemption-codes` — Mã nạp
- Role ≥ ADMIN.
- Actions: "Delete Invalid", "Create Code".
- Bảng `redemptions-columns.tsx`: select, ID, Name, Status, Redemption Code (mask + copy), Quota, Created, Expires, Redeemed By, Actions. Bulk: Copy selected, Delete.
- Dialogs (`redemptions-dialogs.tsx`):
  - `redemptions-mutate-drawer.tsx`: Name, Quota, Expiration (Never/1 month/7 days/1 day + date), Quantity 1–100 (create only).
  - `redemptions-export-dialog.tsx`: danh sách code vừa tạo, copy, "Save as a file", radio TXT/CSV, Done.

## 6. `/task-plugins` — Task plugins (ROOT)
- Header: Switch "Enable task plugins" (tắt → confirm liệt kê override + task đang chạy), popover giải thích, "Upload plugin".
- Tabs: **Installed** (`plugins-table.tsx`: Plugin, Active version, Source, Channel type, Models, Enabled switch, Runtime status, Actions), **Marketplace** (`marketplace-panel.tsx`: chọn index source, card plugin → `marketplace-install-dialog.tsx`).
- Sheets/Dialogs: `plugin-detail-sheet.tsx` (tabs Overview, Billing parameters, Plugin source, Version history, Source diff, Plugin sandbox), `upload-dialog.tsx` (file plugin.js ≤8 MiB, URL import, textarea source, icon ≤512 KiB, Remark), `marketplace-sources-dialog.tsx`.

## 7. `/system-settings` — Settings (ROOT toàn bộ)
- Shell: `SystemSettings` = `<Outlet/>`; điều hướng section = path param; mỗi section một form Save/Reset + dirty guard.

### 7.1 `site/$section`
- **system-info**: SystemName, ServerAddress, TaskPublicAddress, Logo URL, docs_link, Footer (textarea), About (textarea), HomePageContent (textarea), legal.user_agreement, legal.privacy_policy.
- **notice**: Notice textarea.
- **header-navigation**: bật/tắt module nav header.
- **sidebar-modules**: bật/tắt module sidebar admin.

### 7.2 `auth/$section`
- **basic-auth**: PasswordLoginEnabled, RegisterEnabled, PasswordRegisterEnabled, EmailVerificationEnabled, EmailDomainRestrictionEnabled, EmailAliasRestrictionEnabled; EmailDomainWhitelist textarea.
- **oauth**: GitHub, Discord, OIDC, Telegram, LinuxDO, WeChat — mỗi provider switch + client id/secret (+ endpoint riêng); callback URL hiển thị + copy.
- **passkey**: enabled, rp_display_name, rp_id, legacy_rp_ids, origins, allow_insecure_origin, user_verification, attachment_preference.
- **bot-protection**: TurnstileCheckEnabled, SiteKey, SecretKey.
- **custom-oauth**: bảng provider + `provider-form-dialog.tsx` (enabled, name, slug, icon, client_id, client_secret, auth_style, well_known, endpoints, scopes, user_id_field, username_field).

### 7.3 `billing/$section`
- **quota**: QuotaForNewUser, trust_quota_usd, pre_consume_multiplier, QuotaForInviter/Invitee, enable_free_model_pre_consume, TopUpLink.
- **currency**: QuotaPerUnit, quota_display_type, USDExchangeRate, custom_currency_symbol, custom_currency_exchange_rate, DisplayInCurrencyEnabled, DisplayTokenStatEnabled.
- **model-pricing**: tabs `models | unset-models | tool-prices | upstream-sync`; bảng tỷ giá + `model-ratio-visual-editor.tsx` / `model-pricing-sheet.tsx`; tiered editor (`tiered-pricing-editor.tsx`); task pricing (`task-usage-pricing-editor.tsx`, `task-plugin-pricing-editor.tsx`, `task-pricing-matrix.tsx`); tool prices; upstream sync (`upstream-ratio-sync*.tsx`); `request-simulation.tsx`; `visual-billing-document-editor.tsx`.
- **group-pricing**: TopupGroupRatio, GroupRatio (rows: name/ratio/topup ratio/user selectable/description), UserUsableGroups, GroupGroupRatio, AutoGroups, MaxTokenAutoGroups, DefaultUseAutoGroup, GroupSpecialUsableGroup (`group-special-usable-editor.tsx`).
- **payment**: banner **Compliance** (dialog gõ câu xác nhận); PayAddress, EpayId, EpayKey, Price, MinTopUp, CustomCallbackAddress, PayMethods (`payment-methods-visual-editor.tsx` + `payment-method-dialog.tsx`), AmountOptions/AmountDiscount (`amount-options-visual-editor.tsx`, `amount-discount-dialog.tsx`), Stripe (ApiSecret, WebhookSecret, PriceId, UnitPrice, MinTopUp, PromotionCodesEnabled), Creem (ApiKey, WebhookSecret, TestMode, products editor), Waffo.
- **checkin**: enabled, minQuota, maxQuota.

### 7.4 `models/$section`
- **global**: pass_through_request_enabled, thinking_model_blacklist (JSON), chat_completions_to_responses_policy (JSON), ping_interval_enabled, ping_interval_seconds.
- **gemini**: safety_settings, version_settings, supported_imagine_models, thinking_adapter_*, function_call_thought_signature_enabled, remove_function_response_id_enabled.
- **claude**: model_headers_settings, default_max_tokens, thinking_adapter_*.
- **grok**: violation_deduction_enabled, violation_deduction_amount.
- **model-deployment**: io.net enabled + API key + test connection.
- (legacy redirect: `channel-affinity`, `routing-reliability` → request-policies/routing)

### 7.5 `operations/$section`
- **behavior**: DefaultCollapseSidebar, DemoSiteEnabled, SelfUseModeEnabled.
- **alerts**: perf_metrics_setting.enabled, flush_interval, bucket_time, retention_days.
- **email**: EmailProvider (Resend/SMTP), Resend fields, SMTP fields, QuotaRemindEnabled + threshold.
- **worker**: WorkerUrl, WorkerValidKey, WorkerAllowHttpImageRequestEnabled.
- **logs**: LogConsumeEnabled, user_export_enabled, max_range_days; `history-log-cleanup.tsx`; `server-log-files.tsx` (cleanup by count/size + AlertDialog).
- **performance**: disk_cache_enabled, threshold_mb, max_size_mb, disk_cache_path, monitor_enabled, cpu/memory/disk threshold.
- **update-checker**: phiên bản hiện tại + start time + check update.

### 7.6 `request-policies/$section`
- **filtering**: CheckSensitiveEnabled, CheckSensitiveOnPromptEnabled, SensitiveWords textarea.
- **routing**: RetryTimes, AutomaticRetryStatusCodes (tags), channel_affinity_setting.enabled, session_mode (RadioGroup), switch_on_success, keep_on_channel_disabled, rules (Channel Affinity UI: bảng rule, `rule-editor-dialog.tsx`, `cache-stats-dialog.tsx`).
- **health**: auto_test_channel_enabled, channel_test_mode, auto_test_channel_minutes, channel_test_concurrency, AutomaticEnable/DisableChannelEnabled, ChannelDisableThreshold, AutomaticDisableStatusCodes, AutomaticDisableKeywords; `decision-record.tsx`.

### 7.7 `security/$section`
- **rate-limit**: ModelRequestRateLimitEnabled, DurationMinutes, Count, SuccessCount, group editor (`rate-limit-visual-editor.tsx` + `rate-limit-dialog.tsx`).
- **ssrf**: enable_ssrf_protection, allow_private_ip, domain_filter_mode, ip_filter_mode, domain_list, ip_list, allowed_ports, apply_ip_filter_for_domain.
- **token-limits**: token_setting.max_user_tokens.

### 7.8 `content/$section`
- **dashboard**: DataExportEnabled, DataExportInterval, DataExportDefaultTime.
- **announcements**: enabled + item editor (content, publishDate, type, extra).
- **api-info**: enabled + item (url, route, description, color).
- **faq**: enabled + item (question, answer).
- **uptime-kuma**: enabled + item (categoryName, url, slug).
- **chat**: Chats JSON textarea hoặc visual (name + url).
- **drawing**: DrawingEnabled, MjNotifyEnabled, MjAccountFilterEnabled, MjForwardUrlEnabled, MjModeClearEnabled, MjActionCheckSuccessEnabled.

## 8. `/system-info` — System Info (ROOT)
- `system-instances-panel.tsx`: bảng Instances/Status/Role/CPU/Memory/Storage/Version/Runtime/Started/Last Seen/Actions; popover Configure NODE_NAME; xoá stale instance; storage tooltip.
- `system-tasks-panel.tsx`: Active Tasks + Refresh; `system-tasks-table.tsx` (Type/Status/Progress/Executor/Updated/Detail); `system-task-history.tsx` (filter Type/Status, Clean task history).

## 9. `/security` — Security & Access (mọi user, không phải admin)
- Xem inventory-user §16.

## 10. `/subscriptions` — Gói đăng ký (ADMIN)
- Alert Stripe/Creem; banner khoá nếu chưa confirm compliance. Actions: "Create Plan".
- Bảng `subscriptions-columns.tsx`: ID, Plan, Price, Validity, Quota Reset, Priority, Status, Payment Channel, Plan Quota, Upgrade Group, Actions.
- Dialogs (`subscriptions-dialogs.tsx`):
  - `subscriptions-mutate-drawer.tsx`: title, subtitle, price_amount, total_amount, upgrade_group/downgrade_group, max_purchase_per_user, sort_order, enabled, allow_balance_pay, allow_wallet_overflow, duration_unit + value/custom, quota_reset_period + custom, stripe/creem/waffo product ids.
  - `toggle-status-dialog.tsx`, `reset-subscriptions-dialog.tsx`, `subscription-purchase-dialog.tsx`, `user-subscriptions-dialog.tsx`.
- Row actions: Edit, Reset subscription quota, Enable/Disable.

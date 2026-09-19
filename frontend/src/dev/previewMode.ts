import type { PublicSettings, User } from '@/types'

/**
 * Local-only UI preview switch.
 *
 * The flag is intentionally gated by Vite's dev marker so a preview session
 * can never be bundled into a production build. Start Vite with
 * `VITE_PREVIEW_MODE=true` when the backend is not available.
 */
export const isPreviewMode =
  import.meta.env.DEV && String(import.meta.env.VITE_PREVIEW_MODE || '').toLowerCase() === 'true'

export const previewUser: User = {
  id: 2123,
  username: 'xiaoming-demo',
  email: 'demo@xiaoming-api.local',
  avatar_url: null,
  role: 'user',
  balance: 128.64,
  frozen_balance: 0,
  concurrency: 8,
  rpm_limit: 120,
  status: 'active',
  allowed_groups: null,
  balance_notify_enabled: true,
  balance_notify_threshold: 10,
  balance_notify_extra_emails: [],
  email_bound: true,
  created_at: '2026-08-01T08:00:00Z',
  updated_at: '2026-09-19T08:00:00Z',
}

export const previewSettings: PublicSettings = {
  registration_enabled: true,
  email_verify_enabled: false,
  force_email_on_third_party_signup: false,
  registration_email_suffix_whitelist: [],
  promo_code_enabled: true,
  password_reset_enabled: true,
  invitation_code_enabled: false,
  turnstile_enabled: false,
  turnstile_site_key: '',
  passkey_enabled: false,
  site_name: '小明 API',
  site_logo: '/xiaoming-logo.svg',
  site_subtitle: '统一模型接入与用量工作台',
  api_base_url: 'http://localhost:3000/v1',
  contact_info: 'QQ群：小明 API 体验群',
  doc_url: '/docs',
  home_content: '',
  // The local preview is meant to exercise the long landing page.
  // Keep the production setting untouched; this fixture only controls the
  // development-only preview session.
  compact_home_enabled: false,
  hide_ccs_import_button: false,
  payment_enabled: true,
  risk_control_enabled: true,
  table_default_page_size: 20,
  table_page_size_options: [20, 50, 100],
  custom_menu_items: [],
  custom_endpoints: [],
  linuxdo_oauth_enabled: false,
  dingtalk_oauth_enabled: false,
  wechat_oauth_enabled: false,
  oidc_oauth_enabled: false,
  oidc_oauth_provider_name: 'OIDC',
  github_oauth_enabled: false,
  google_oauth_enabled: false,
  backend_mode_enabled: false,
  version: 'preview',
  server_timezone: 'Asia/Shanghai',
  server_utc_offset: '+08:00',
  balance_low_notify_enabled: true,
  account_quota_notify_enabled: true,
  balance_low_notify_threshold: 10,
  channel_monitor_enabled: true,
  channel_monitor_mode: 'v2',
  channel_monitor_default_interval_seconds: 60,
  channel_monitor_hide_throughput: false,
  channel_monitor_show_quota: true,
  channel_monitor_hide_user_ranking: false,
  available_channels_enabled: true,
  subscription_enabled: true,
  payment_balance_disabled: false,
  model_plaza_enabled: true,
  model_plaza_require_auth: false,
  plugin_management_enabled: false,
  service_quota_enabled: true,
  affiliate_enabled: true,
  allow_user_view_error_requests: true,
}

/** Seed the same storage keys used by the real auth store for a local preview. */
export function installPreviewSession(): void {
  if (!isPreviewMode || typeof window === 'undefined') return

  localStorage.setItem('auth_token', 'preview-token')
  localStorage.removeItem('refresh_token')
  localStorage.removeItem('token_expires_at')
  localStorage.setItem('auth_user', JSON.stringify(previewUser))
  sessionStorage.removeItem('auth_expired')
  window.__APP_CONFIG__ = { ...previewSettings }
}

export const previewDashboardStats = {
  total_api_keys: 3,
  active_api_keys: 2,
  total_requests: 1842,
  total_input_tokens: 12_184_220,
  total_output_tokens: 2_918_440,
  total_cache_creation_tokens: 334_120,
  total_cache_read_tokens: 8_931_600,
  total_tokens: 24_368_380,
  total_cost: 46.8214,
  total_actual_cost: 38.6142,
  today_requests: 399,
  today_input_tokens: 2_209_010,
  today_output_tokens: 57_968,
  today_cache_creation_tokens: 29_060,
  today_cache_read_tokens: 9_861_500,
  today_tokens: 12_157_538,
  today_cost: 17.7132,
  today_actual_cost: 14.6029,
  average_duration_ms: 842,
  rpm: 6.7,
  tpm: 203_280,
}

const previewDates = ['2026-09-14', '2026-09-15', '2026-09-16', '2026-09-17', '2026-09-18', '2026-09-19']

export const previewTrend = previewDates.map((date, index) => ({
  date,
  requests: [188, 244, 310, 276, 425, 399][index],
  input_tokens: [810_200, 1_102_400, 1_534_000, 1_206_800, 2_209_010, 2_209_010][index],
  output_tokens: [42_100, 58_200, 76_400, 64_500, 116_200, 57_968][index],
  cache_creation_tokens: [8_200, 12_400, 19_700, 17_800, 29_060, 29_060][index],
  cache_read_tokens: [1_920_000, 2_110_000, 2_980_000, 2_640_000, 9_861_500, 9_861_500][index],
  total_tokens: [2_780_500, 3_283_000, 4_610_100, 3_929_100, 12_215_770, 12_157_538][index],
  cost: [4.81, 5.72, 7.93, 6.92, 17.71, 17.71][index],
  actual_cost: [3.95, 4.71, 6.53, 5.71, 14.60, 14.60][index],
}))

export const previewModels = [
  { model: 'gpt-5.6-sol', requests: 782, input_tokens: 5_120_000, output_tokens: 1_240_000, cache_creation_tokens: 120_000, cache_read_tokens: 4_100_000, total_tokens: 10_580_000, cost: 21.42, actual_cost: 17.68 },
  { model: 'claude-fable-5', requests: 624, input_tokens: 4_180_000, output_tokens: 1_020_000, cache_creation_tokens: 95_000, cache_read_tokens: 3_190_000, total_tokens: 8_485_000, cost: 16.19, actual_cost: 13.37 },
  { model: 'gemini-3.1-pro', requests: 436, input_tokens: 2_884_220, output_tokens: 658_440, cache_creation_tokens: 119_120, cache_read_tokens: 1_641_600, total_tokens: 5_303_380, cost: 9.21, actual_cost: 7.56 },
]

export const previewGroups = [
  { id: 1, name: 'GPT 稳定号池', description: '低延迟 OpenAI 通道', platform: 'openai', rate_multiplier: 1, is_exclusive: false, subscription_type: 'standard', peak_rate_enabled: false, peak_rate_multiplier: 1, peak_start: null, peak_end: null },
  { id: 2, name: 'Claude 长上下文', description: '长上下文与缓存优化', platform: 'anthropic', rate_multiplier: 1.08, is_exclusive: false, subscription_type: 'standard', peak_rate_enabled: false, peak_rate_multiplier: 1, peak_start: null, peak_end: null },
  { id: 3, name: 'Gemini 低价池', description: '高吞吐备用通道', platform: 'gemini', rate_multiplier: 0.82, is_exclusive: false, subscription_type: 'standard', allow_image_generation: true, allow_batch_image_generation: true, image_rate_multiplier: 1, batch_image_discount_multiplier: 0.92, batch_image_hold_multiplier: 1, peak_rate_enabled: false, peak_rate_multiplier: 1, peak_start: null, peak_end: null },
]

export const previewApiKeys = [
  { id: 101, user_id: previewUser.id, key: 'sk-preview-7c8d1a4f2c6e9b', name: '主工作台', group_id: 1, routing_mode: 'auto', auto_route_group_ids: [2, 3], status: 'active', ip_whitelist: [], ip_blacklist: [], last_used_at: '2026-09-19T12:14:00Z', last_used_ip: '127.0.0.1', quota: 0, quota_used: 38.6142, expires_at: null, created_at: '2026-08-03T09:00:00Z', updated_at: '2026-09-19T12:14:00Z', current_concurrency: 2, group: previewGroups[0], rate_limit_5h: 0, rate_limit_1d: 0, rate_limit_7d: 0, usage_5h: 8.2, usage_1d: 28.4, usage_7d: 72.6, window_5h_start: '2026-09-19T08:00:00Z', window_1d_start: '2026-09-19T00:00:00Z', window_7d_start: '2026-09-13T00:00:00Z', reset_5h_at: null, reset_1d_at: null, reset_7d_at: null },
  { id: 102, user_id: previewUser.id, key: 'sk-preview-3ab8f111d09e4a', name: '移动端测试', group_id: 2, routing_mode: 'manual', auto_route_group_ids: [], status: 'active', ip_whitelist: [], ip_blacklist: [], last_used_at: '2026-09-18T18:42:00Z', last_used_ip: '127.0.0.1', quota: 100, quota_used: 12.84, expires_at: '2026-12-31T23:59:59Z', created_at: '2026-08-12T10:20:00Z', updated_at: '2026-09-18T18:42:00Z', current_concurrency: 0, group: previewGroups[1], rate_limit_5h: 0, rate_limit_1d: 0, rate_limit_7d: 0, usage_5h: 1.8, usage_1d: 4.2, usage_7d: 18.9, window_5h_start: '2026-09-19T08:00:00Z', window_1d_start: '2026-09-19T00:00:00Z', window_7d_start: '2026-09-13T00:00:00Z', reset_5h_at: null, reset_1d_at: null, reset_7d_at: null },
  { id: 103, user_id: previewUser.id, key: 'sk-preview-91f2d4a6c8e0b3', name: '生图工作台', group_id: 3, routing_mode: 'fixed', auto_route_group_ids: [], status: 'active', ip_whitelist: [], ip_blacklist: [], last_used_at: '2026-09-19T10:06:00Z', last_used_ip: '127.0.0.1', quota: 50, quota_used: 6.42, expires_at: null, created_at: '2026-08-21T14:30:00Z', updated_at: '2026-09-19T10:06:00Z', current_concurrency: 1, group: previewGroups[2], rate_limit_5h: 0, rate_limit_1d: 0, rate_limit_7d: 0, usage_5h: 0.9, usage_1d: 2.1, usage_7d: 6.42, window_5h_start: '2026-09-19T08:00:00Z', window_1d_start: '2026-09-19T00:00:00Z', window_7d_start: '2026-09-13T00:00:00Z', reset_5h_at: null, reset_1d_at: null, reset_7d_at: null },
]

export const previewUsageLogs = previewDates.slice().reverse().map((date, index) => ({
  id: 500 + index,
  user_id: previewUser.id,
  api_key_id: 101,
  account_id: null,
  request_id: `preview-${date.replace(/-/g, '')}-${index}`,
  model: previewModels[index % previewModels.length].model,
  service_tier: null,
  reasoning_effort: index % 2 === 0 ? 'medium' : null,
  inbound_endpoint: '/v1/responses',
  upstream_endpoint: '/v1/responses',
  group_id: previewGroups[index % previewGroups.length].id,
  subscription_id: null,
  input_tokens: 320_000 + index * 41_000,
  output_tokens: 48_000 + index * 8_000,
  cache_creation_tokens: 7_500,
  cache_read_tokens: 180_000 + index * 20_000,
  cache_creation_5m_tokens: 7_500,
  cache_creation_1h_tokens: 0,
  input_cost: 0.8,
  output_cost: 0.32,
  cache_creation_cost: 0.05,
  cache_read_cost: 0.12,
  total_cost: 1.29 + index * 0.16,
  actual_cost: 1.05 + index * 0.13,
  rate_multiplier: 1,
  long_context_billing_applied: false,
  billing_type: 0,
  request_type: 'stream',
  stream: true,
  openai_ws_mode: false,
  native_compaction_v2: false,
  duration_ms: 520 + index * 48,
  first_token_ms: 280 + index * 24,
  image_count: 0,
  image_size: null,
  image_input_size: null,
  image_output_size: null,
  image_size_source: null,
  image_size_breakdown: null,
  image_input_tokens: 0,
  image_input_cost: 0,
  image_output_tokens: 0,
  image_output_cost: 0,
  user_agent: 'Sub2API Preview',
  ip_address: '127.0.0.1',
  cache_ttl_overridden: false,
  billing_mode: 'balance',
  created_at: `${date}T12:14:00Z`,
}))

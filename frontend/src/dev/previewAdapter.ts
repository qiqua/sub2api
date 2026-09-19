import axios, {
  AxiosHeaders,
  type AxiosAdapter,
  type AxiosResponse,
  type InternalAxiosRequestConfig,
} from 'axios'
import {
  previewApiKeys,
  previewDashboardStats,
  previewGroups,
  previewModels,
  previewSettings,
  previewTrend,
  previewUsageLogs,
  previewUser,
} from './previewMode'

type PreviewRecord = Record<string, any>

// Keep the fixture state local to this browser tab. This gives the preview enough
// interactivity for key creation/toggling without pretending to persist anything.
const keys = previewApiKeys as PreviewRecord[]
const usageLogs = previewUsageLogs as PreviewRecord[]

function requestPath(config: InternalAxiosRequestConfig): string {
  const raw = String(config.url || '')
  try {
    return new URL(raw, window.location.origin).pathname.replace(/^\/api\/v1/, '') || '/'
  } catch {
    return raw.split('?')[0].replace(/^\/api\/v1/, '') || '/'
  }
}

function requestParams(config: InternalAxiosRequestConfig): PreviewRecord {
  if (config.params && typeof config.params === 'object') {
    return config.params as PreviewRecord
  }
  return {}
}

function parseBody(config: InternalAxiosRequestConfig): PreviewRecord {
  if (!config.data) return {}
  if (typeof config.data === 'object') return config.data as PreviewRecord
  try {
    return JSON.parse(String(config.data)) as PreviewRecord
  } catch {
    return {}
  }
}

function page<T>(items: T[], params: PreviewRecord): PreviewRecord {
  const pageSize = Math.max(1, Number(params.page_size) || 20)
  const currentPage = Math.max(1, Number(params.page) || 1)
  const total = items.length
  const start = (currentPage - 1) * pageSize
  return {
    items: items.slice(start, start + pageSize),
    total,
    page: currentPage,
    page_size: pageSize,
    pages: Math.max(1, Math.ceil(total / pageSize)),
  }
}

function clone<T>(value: T): T {
  return JSON.parse(JSON.stringify(value)) as T
}

function today(): string {
  return new Date().toISOString().slice(0, 10)
}

function trendPayload(params: PreviewRecord): PreviewRecord {
  const startDate = String(params.start_date || previewTrend[0]?.date || today())
  const endDate = String(params.end_date || previewTrend[previewTrend.length - 1]?.date || today())
  return {
    trend: clone(previewTrend),
    start_date: startDate,
    end_date: endDate,
    granularity: String(params.granularity || 'day'),
  }
}

function statsPayload(): PreviewRecord {
  return {
    period: 'preview',
    total_requests: previewDashboardStats.total_requests,
    total_input_tokens: previewDashboardStats.total_input_tokens,
    total_output_tokens: previewDashboardStats.total_output_tokens,
    total_cache_tokens:
      previewDashboardStats.total_cache_creation_tokens + previewDashboardStats.total_cache_read_tokens,
    total_cache_read_tokens: previewDashboardStats.total_cache_read_tokens,
    total_cache_creation_tokens: previewDashboardStats.total_cache_creation_tokens,
    total_tokens: previewDashboardStats.total_tokens,
    total_cost: previewDashboardStats.total_cost,
    total_actual_cost: previewDashboardStats.total_actual_cost,
    average_duration_ms: previewDashboardStats.average_duration_ms,
    models: Object.fromEntries(previewModels.map((model) => [model.model, model.total_tokens])),
    endpoints: [
      {
        endpoint: '/v1/responses',
        requests: previewDashboardStats.total_requests,
        total_tokens: previewDashboardStats.total_tokens,
        cost: previewDashboardStats.total_cost,
        actual_cost: previewDashboardStats.total_actual_cost,
      },
    ],
  }
}

function keyList(params: PreviewRecord): PreviewRecord {
  const search = String(params.search || '').trim().toLowerCase()
  const status = String(params.status || '').trim()
  const groupId = params.group_id == null || params.group_id === '' ? null : Number(params.group_id)
  const sortBy = String(params.sort_by || 'created_at')
  const sortOrder = String(params.sort_order || 'desc') === 'asc' ? 1 : -1
  const filtered = keys
    .filter((key) => !search || `${key.name} ${key.key}`.toLowerCase().includes(search))
    .filter((key) => !status || key.status === status)
    .filter((key) => groupId === null || key.group_id === groupId)
    .slice()
    .sort((left, right) => {
      const a = left[sortBy]
      const b = right[sortBy]
      return String(a ?? '').localeCompare(String(b ?? '')) * sortOrder
    })
  return page(filtered.map((key) => normalizeKey(clone(key))), params)
}

function normalizeKey(key: PreviewRecord): PreviewRecord {
  // Older fixtures used "manual" before the API named this mode "fixed".
  if (key.routing_mode === 'manual') key.routing_mode = 'fixed'
  return key
}

function findKey(path: string): PreviewRecord | undefined {
  const match = path.match(/^\/keys\/(\d+)$/)
  return match ? keys.find((key) => key.id === Number(match[1])) : undefined
}

function handleKeys(method: string, path: string, config: InternalAxiosRequestConfig): unknown {
  const body = parseBody(config)
  if (method === 'get' && path === '/keys') return keyList(requestParams(config))

  const existing = findKey(path)
  if (existing && method === 'put') {
    if (body.reset_quota) existing.quota_used = 0
    if (body.reset_rate_limit_usage) {
      existing.usage_5h = 0
      existing.usage_1d = 0
      existing.usage_7d = 0
    }
    Object.assign(existing, body)
    return clone(normalizeKey(existing))
  }
  if (existing && method === 'delete') {
    const index = keys.indexOf(existing)
    if (index >= 0) keys.splice(index, 1)
    return { message: 'Preview key deleted' }
  }

  if (method === 'post' && path === '/keys') {
    const id = Math.max(0, ...keys.map((key) => Number(key.id) || 0)) + 1
    const groupId = body.group_id == null ? 1 : Number(body.group_id)
    const group = previewGroups.find((item) => item.id === groupId) || previewGroups[0]
    const created: PreviewRecord = {
      id,
      user_id: previewUser.id,
      key: `sk-preview-${Math.random().toString(36).slice(2, 16)}`,
      name: String(body.name || `演示密钥 ${id}`),
      group_id: group?.id ?? null,
      routing_mode: body.routing_mode || 'fixed',
      auto_route_group_ids: body.auto_route_group_ids || [],
      status: 'active',
      ip_whitelist: body.ip_whitelist || [],
      ip_blacklist: body.ip_blacklist || [],
      last_used_at: null,
      last_used_ip: null,
      quota: Number(body.quota || 0),
      quota_used: 0,
      expires_at: null,
      created_at: new Date().toISOString(),
      updated_at: new Date().toISOString(),
      current_concurrency: 0,
      group,
      rate_limit_5h: Number(body.rate_limit_5h || 0),
      rate_limit_1d: Number(body.rate_limit_1d || 0),
      rate_limit_7d: Number(body.rate_limit_7d || 0),
      usage_5h: 0,
      usage_1d: 0,
      usage_7d: 0,
      window_5h_start: null,
      window_1d_start: null,
      window_7d_start: null,
      reset_5h_at: null,
      reset_1d_at: null,
      reset_7d_at: null,
    }
    keys.unshift(created)
    return clone(created)
  }

  return {}
}

function handleUsage(method: string, path: string, config: InternalAxiosRequestConfig): unknown {
  const params = requestParams(config)
  if (method === 'post' && path === '/usage/dashboard/api-keys-usage') {
    const body = parseBody(config)
    const ids = Array.isArray(body.api_key_ids) ? body.api_key_ids.map(Number) : []
    return {
      stats: Object.fromEntries(
        ids.map((id) => {
          const key = keys.find((item) => item.id === id)
          return [
            String(id),
            {
              api_key_id: id,
              today_actual_cost: key?.quota_used ? Math.min(Number(key.quota_used), 14.6) : 0,
              total_actual_cost: key?.quota_used || 0,
            },
          ]
        }),
      ),
    }
  }
  if (method !== 'get') return {}
  if (path === '/usage/dashboard/stats') return clone(previewDashboardStats)
  if (path === '/usage/dashboard/trend') return trendPayload(params)
  if (path === '/usage/dashboard/models') {
    return {
      models: clone(previewModels),
      start_date: String(params.start_date || previewTrend[0]?.date || today()),
      end_date: String(params.end_date || previewTrend[previewTrend.length - 1]?.date || today()),
    }
  }
  if (path === '/usage/dashboard/snapshot-v2') {
    return {
      generated_at: new Date().toISOString(),
      ...trendPayload(params),
      models: clone(previewModels),
      groups: clone(
        previewGroups.map((group, index) => ({
          group_id: group.id,
          group_name: group.name,
          requests: [782, 624, 436][index] || 0,
          total_tokens: [10_580_000, 8_485_000, 5_303_380][index] || 0,
          cost: [21.42, 16.19, 9.21][index] || 0,
          actual_cost: [17.68, 13.37, 7.56][index] || 0,
        })),
      ),
    }
  }
  if (path === '/usage/stats') return statsPayload()
  if (path === '/usage/errors') return page([], params)
  if (path === '/usage') return page(clone(usageLogs), params)
  return {}
}

function response(config: InternalAxiosRequestConfig, data: unknown): AxiosResponse {
  return {
    data,
    status: 200,
    statusText: 'OK',
    headers: new AxiosHeaders({ 'x-sub2api-preview': '1' }),
    config,
    request: undefined,
  }
}

/** Axios adapter used only when the Vite preview switch is enabled. */
export const previewAdapter: AxiosAdapter = async (config) => {
  if (config.signal?.aborted) {
    throw new axios.CanceledError('Preview request cancelled')
  }

  const method = String(config.method || 'get').toLowerCase()
  const path = requestPath(config)
  let data: unknown

  if (path === '/auth/me' || path === '/user/profile') data = clone(previewUser)
  else if (path === '/auth/logout' || path === '/auth/revoke-all-sessions') data = { message: 'ok' }
  else if (path === '/settings/public') data = clone(previewSettings)
  else if (path === '/keys' || path.startsWith('/keys/')) data = handleKeys(method, path, config)
  else if (path.startsWith('/usage')) data = handleUsage(method, path, config)
  else if (path === '/groups/available') data = clone(previewGroups)
  else if (path === '/groups/rates') data = { 1: 1, 2: 1.08, 3: 0.82 }
  else if (path === '/channels/available') data = []
  else if (path === '/user/platform-quotas') data = { platform_quotas: [] }
  else if (path === '/announcements') data = []
  else if (path.startsWith('/announcements/')) data = { message: 'ok' }
  else if (path.startsWith('/subscriptions')) data = []
  else if (path.startsWith('/batch-image')) data = { enabled: true }
  else if (path.startsWith('/admin/compliance')) data = { acknowledged: true, required: false }
  else data = {}

  return response(config, data)
}

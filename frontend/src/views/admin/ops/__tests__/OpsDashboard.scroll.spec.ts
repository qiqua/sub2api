import { flushPromises, mount } from '@vue/test-utils'
import { defineComponent } from 'vue'
import { beforeEach, describe, expect, it, vi } from 'vitest'
import OpsDashboard from '../OpsDashboard.vue'

const { query, fetchSettings } = vi.hoisted(() => ({
  query: {} as Record<string, string>,
  fetchSettings: vi.fn().mockResolvedValue(undefined),
}))

vi.mock('vue-router', () => ({
  useRoute: () => ({ query }),
  useRouter: () => ({ replace: vi.fn() }),
}))

vi.mock('vue-i18n', async (importOriginal) => ({
  ...await importOriginal<typeof import('vue-i18n')>(),
  useI18n: () => ({ t: (key: string) => key }),
}))

vi.mock('@/stores', () => ({
  useAppStore: () => ({ showError: vi.fn() }),
  useAdminSettingsStore: () => ({
    opsMonitoringEnabled: true,
    opsQueryModeDefault: 'auto',
    fetch: fetchSettings,
  }),
}))

vi.mock('@/api/admin/ops', () => ({
  default: {},
  opsAPI: {
    getAdvancedSettings: vi.fn().mockResolvedValue({
      display_alert_events: true,
      display_openai_token_stats: false,
      auto_refresh_enabled: false,
      auto_refresh_interval_seconds: 30,
    }),
    getMetricThresholds: vi.fn().mockResolvedValue(null),
    getDashboardSnapshotV2: vi.fn().mockResolvedValue({ overview: {}, throughput_trend: { points: [] }, error_trend: { points: [] } }),
    getThroughputTrend: vi.fn().mockResolvedValue({ points: [] }),
    getLatencyHistogram: vi.fn().mockResolvedValue({ buckets: [] }),
    getErrorDistribution: vi.fn().mockResolvedValue({}),
  },
}))

const stubs = {
  AppLayout: defineComponent({ template: '<main><slot /></main>' }),
  BaseDialog: true,
  OpsDashboardHeader: true,
  OpsDashboardSkeleton: true,
  OpsConcurrencyCard: true,
  OpsSwitchRateTrendChart: true,
  OpsThroughputTrendChart: true,
  OpsLatencyChart: true,
  OpsErrorDistributionChart: true,
  OpsErrorTrendChart: true,
  OpsOpenAITokenStatsCard: true,
  OpsAlertEventsCard: true,
  OpsSystemLogTable: true,
  OpsSettingsDialog: true,
  OpsAlertRulesCard: true,
  OpsErrorDetailsModal: true,
  OpsErrorDetailModal: true,
  OpsRequestDetailsModal: true,
}

describe('OpsDashboard scrolling', () => {
  beforeEach(() => {
    delete query.fullscreen
    fetchSettings.mockResolvedValue(undefined)
  })

  it.each([false, true])('keeps the overview and bottom logs in one scroll area (fullscreen=%s)', async (fullscreen) => {
    if (fullscreen) query.fullscreen = '1'
    const wrapper = mount(OpsDashboard, { global: { stubs } })
    await flushPromises()

    const page = wrapper.get('.app-page-scroll')
    expect(page.element.contains(wrapper.get('ops-dashboard-header-stub').element)).toBe(true)
    expect(page.element.contains(wrapper.get('ops-system-log-table-stub').element)).toBe(true)
    expect(page.findAll('.overflow-y-auto')).toHaveLength(0)
    wrapper.unmount()
  })

  it.each([false, true])('keeps a tall loading skeleton in the same scroll area (fullscreen=%s)', (fullscreen) => {
    if (fullscreen) query.fullscreen = '1'
    fetchSettings.mockReturnValue(new Promise(() => {}))
    const wrapper = mount(OpsDashboard, { global: { stubs } })

    const page = wrapper.get('.app-page-scroll')
    expect(page.element.contains(wrapper.get('ops-dashboard-skeleton-stub').element)).toBe(true)
    wrapper.unmount()
  })
})

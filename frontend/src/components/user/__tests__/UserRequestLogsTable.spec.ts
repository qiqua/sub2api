import { mount } from '@vue/test-utils'
import { describe, expect, it, vi } from 'vitest'
import type { UsageLog } from '@/types'

import UserRequestLogsTable from '../UserRequestLogsTable.vue'

vi.mock('vue-i18n', async () => {
  const actual = await vi.importActual<typeof import('vue-i18n')>('vue-i18n')
  return {
    ...actual,
    useI18n: () => ({ t: (key: string) => key }),
  }
})

describe('UserRequestLogsTable output rate', () => {
  const row = {
    id: 1,
    created_at: '2026-10-15T00:00:00Z',
    output_tokens: 1000,
    duration_ms: 20_000,
    first_token_ms: 5000,
    image_count: 0,
    image_output_tokens: 0,
    billing_mode: 'token',
    request_type: 'stream',
  } as UsageLog

  it.each([
    { label: 'TTFT is included in total duration', fields: {}, expected: '50.0 tok/s' },
    { label: 'missing TTFT', fields: { first_token_ms: null }, expected: '50.0 tok/s' },
    { label: 'TTFT equals total duration', fields: { first_token_ms: 20_000 }, expected: '50.0 tok/s' },
    { label: 'image count', fields: { image_count: 1 }, expected: '\u2014' },
    { label: 'image output tokens', fields: { image_output_tokens: 100 }, expected: '\u2014' },
    { label: 'image billing', fields: { billing_mode: 'image' }, expected: '\u2014' },
    { label: 'zero output', fields: { output_tokens: 0 }, expected: '\u2014' },
  ])('renders the shared output-rate policy for $label', ({ fields, expected }) => {
    const wrapper = mount(UserRequestLogsTable, {
      props: { data: [{ ...row, ...fields }] },
      global: { stubs: { Icon: true } },
    })

    expect(wrapper.get('.metric-cell').text()).toBe(expected)
    wrapper.unmount()
  })
})

describe('UserRequestLogsTable page-size slider', () => {
  it('supports every page size from 1 through 100', async () => {
    const wrapper = mount(UserRequestLogsTable, {
      props: {
        data: [],
        total: 123,
        page: 1,
        pageSize: 55,
      },
      global: {
        stubs: {
          Icon: true,
        },
      },
    })

    const slider = wrapper.find<HTMLInputElement>('.page-size-slider')
    expect(slider.attributes()).toMatchObject({ min: '1', max: '100', step: '1' })
    expect(slider.element.value).toBe('55')

    await slider.setValue('37')

    expect(wrapper.emitted('update:pageSize')).toEqual([[37]])
    wrapper.unmount()
  })
})

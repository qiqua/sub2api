import { mount } from '@vue/test-utils'
import { describe, expect, it, vi } from 'vitest'

import UserRequestLogsTable from '../UserRequestLogsTable.vue'

vi.mock('vue-i18n', async () => {
  const actual = await vi.importActual<typeof import('vue-i18n')>('vue-i18n')
  return {
    ...actual,
    useI18n: () => ({ t: (key: string) => key }),
  }
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

import { describe, expect, it } from 'vitest'
import { mount } from '@vue/test-utils'
import App from './App.vue'

describe('App', () => {
  it('挂载后能渲染出应用外壳', () => {
    const wrapper = mount(App)
    expect(wrapper.text()).toContain('神奇弹幕')
  })
})

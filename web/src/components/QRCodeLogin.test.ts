import { describe, expect, it, vi } from 'vitest'
import { mount, flushPromises } from '@vue/test-utils'
import QRCodeLogin from './QRCodeLogin.vue'

function jsonOnce(body: unknown, status = 200) {
  return new Response(JSON.stringify(body), {
    status,
    headers: { 'Content-Type': 'application/json' },
  })
}

describe('QRCodeLogin', () => {
  it('轮询到 success 时通知父组件并停止轮询', async () => {
    const f = vi
      .fn()
      .mockResolvedValueOnce(jsonOnce({ key: 'K', url: 'https://example.invalid/x' }))
      .mockResolvedValueOnce(jsonOnce({ status: 'scanned' }))
      .mockResolvedValueOnce(jsonOnce({ status: 'success', account: '小号' }))
    vi.stubGlobal('fetch', f)

    const w = mount(QRCodeLogin, { props: { accountName: '小号' } })
    await flushPromises() // 挂载即自动发起扫码（第 1 次 fetch），拿到 key/url 后开始轮询
    expect(w.vm.polling).toBe(true)

    // 用暴露出来的 pollOnce 手动驱动轮询，不等真实的 setInterval
    await w.vm.pollOnce() // 第 2 次 fetch：scanned，仍在轮询
    expect(w.vm.polling).toBe(true)
    expect(w.emitted('success')).toBeUndefined()

    await w.vm.pollOnce() // 第 3 次 fetch：success
    expect(w.emitted('success')).toEqual([['小号']])
    expect(w.vm.polling).toBe(false)

    // 停止之后即便再手动调一次，也不该再打后端——这是「停止轮询」
    // 唯一能确定性验证的方式：调用次数不再增长
    await w.vm.pollOnce()
    expect(f).toHaveBeenCalledTimes(3)
  })

  it('轮询拿到 404 时停止并提示重新发起——扫码会话只活 3 分钟', async () => {
    // 后端的扫码会话 TTL 是 3 分钟，过期后 qrSessions.take() 找不到这个 key，
    // handleQRCodePoll 回 404「扫码会话不存在或已过期，请重新发起」。
    // 不停下来的话会一直打后端，而用户看到的是二维码一直转圈。
    const f = vi
      .fn()
      .mockResolvedValueOnce(jsonOnce({ key: 'K', url: 'https://example.invalid/x' }))
      .mockResolvedValueOnce(jsonOnce({ status: 'waiting' }))
      .mockResolvedValueOnce(jsonOnce({ error: '扫码会话不存在或已过期，请重新发起' }, 404))
    vi.stubGlobal('fetch', f)

    const w = mount(QRCodeLogin, { props: { accountName: '小号' } })
    await flushPromises()
    expect(w.vm.polling).toBe(true)

    await w.vm.pollOnce() // waiting，仍在轮询
    expect(w.vm.polling).toBe(true)

    await w.vm.pollOnce() // 404
    expect(w.vm.polling).toBe(false)
    expect(w.emitted('success')).toBeUndefined()
    expect(w.text()).toContain('过期')

    // 再调一次不该继续请求——过期之后必须真正停下，而不是下次轮询才停
    await w.vm.pollOnce()
    expect(f).toHaveBeenCalledTimes(3)
  })
})

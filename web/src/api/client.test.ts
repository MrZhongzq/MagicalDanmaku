import { beforeEach, describe, expect, it, vi } from 'vitest'
import { ApiError, request, setUnauthorizedHandler } from './client'

function mockFetch(status: number, body: unknown, contentType = 'application/json') {
  return vi.fn().mockResolvedValue(
    new Response(typeof body === 'string' ? body : JSON.stringify(body), {
      status,
      headers: { 'Content-Type': contentType },
    }),
  )
}

describe('request', () => {
  beforeEach(() => {
    vi.unstubAllGlobals()
    // setUnauthorizedHandler 是模块级单例：不重置的话，前一条测试注册的
    // mock 会在下一条测试里被意外调用，断言互相干扰。
    setUnauthorizedHandler(() => {})
  })

  it('成功时返回解析后的 JSON', async () => {
    vi.stubGlobal('fetch', mockFetch(200, { name: '小号' }))
    await expect(request<{ name: string }>('GET', '/api/accounts')).resolves.toEqual({
      name: '小号',
    })
  })

  it('带 credentials 发请求——会话是 HttpOnly Cookie，不带就永远是未登录', async () => {
    const f = mockFetch(200, {})
    vi.stubGlobal('fetch', f)
    await request('GET', '/api/auth/me')
    expect(f.mock.calls[0][1]).toMatchObject({ credentials: 'same-origin' })
  })

  it('把后端的 {"error": "..."} 原样抛出来', async () => {
    vi.stubGlobal('fetch', mockFetch(422, { error: '规则 关键词回复 的正则非法' }))
    await expect(request('POST', '/api/x')).rejects.toThrow('规则 关键词回复 的正则非法')
  })

  it('ApiError 带上状态码，调用方才能区分 403 与 404', async () => {
    vi.stubGlobal('fetch', mockFetch(403, { error: '你在 小号@123 上没有 rule:write 权限' }))
    await expect(request('POST', '/api/x')).rejects.toMatchObject({ status: 403 })
  })

  // 后端的 SPA 回退会把未知路径返回成 HTML。若不特判，JSON.parse 抛出的
  // 是「Unexpected token <」，完全指不到真正的原因。
  it('响应不是 JSON 时给出能看懂的错误，而不是 JSON 解析异常', async () => {
    vi.stubGlobal('fetch', mockFetch(200, '<!doctype html><html></html>', 'text/html'))
    await expect(request('GET', '/api/typo')).rejects.toThrow(/不是 JSON/)
  })

  it('204 无响应体时不去解析', async () => {
    vi.stubGlobal('fetch', vi.fn().mockResolvedValue(new Response(null, { status: 204 })))
    await expect(request('DELETE', '/api/x')).resolves.toBeUndefined()
  })

  it('网络层失败时也抛 ApiError，status 为 0', async () => {
    vi.stubGlobal('fetch', vi.fn().mockRejectedValue(new TypeError('Failed to fetch')))
    await expect(request('GET', '/api/x')).rejects.toBeInstanceOf(ApiError)
  })

  // 登录失败的 401 必须保留后端的原话。
  //
  // 一律当成会话过期的话，输错密码的人会看到「登录已过期，请重新登录」
  // ——他还没登录过，这句话完全不知所云，而且把真正的原因盖掉了。
  it('登录接口的 401 保留后端的错误文案', async () => {
    vi.stubGlobal('fetch', mockFetch(401, { error: '用户名或密码错误' }))
    await expect(request('POST', '/api/auth/login', {})).rejects.toThrow('用户名或密码错误')
  })

  // 人就在登录页上，不该再被「送回登录页」
  it('登录接口的 401 不触发跳回登录页的回调', async () => {
    const onUnauth = vi.fn()
    setUnauthorizedHandler(onUnauth)
    vi.stubGlobal('fetch', mockFetch(401, { error: '用户名或密码错误' }))
    await expect(request('POST', '/api/auth/login', {})).rejects.toThrow()
    expect(onUnauth).not.toHaveBeenCalled()
  })

  it('其余接口的 401 仍然触发跳回登录页', async () => {
    const onUnauth = vi.fn()
    setUnauthorizedHandler(onUnauth)
    vi.stubGlobal('fetch', mockFetch(401, { error: '未登录' }))
    await expect(request('GET', '/api/bindings')).rejects.toThrow()
    expect(onUnauth).toHaveBeenCalledOnce()
  })
})

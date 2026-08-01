/** ApiError 承载后端返回的中文说明与 HTTP 状态码。 */
export class ApiError extends Error {
  /** HTTP 状态码。网络层失败（根本没连上）时为 0。 */
  readonly status: number

  constructor(status: number, message: string) {
    super(message)
    this.name = 'ApiError'
    this.status = status
  }
}

/** 未登录时的回调，由路由守卫注册。 */
let onUnauthorized: (() => void) | null = null

export function setUnauthorizedHandler(fn: () => void) {
  onUnauthorized = fn
}

export type Method = 'GET' | 'POST' | 'PUT' | 'PATCH' | 'DELETE'

/**
 * request 是前端访问后端的唯一出口。
 *
 * 所有错误都归一成 ApiError：调用方只需要 catch 一种东西，
 * 并且总能拿到一句可以直接显示给用户的中文说明。
 */
export async function request<T = void>(method: Method, path: string, body?: unknown): Promise<T> {
  let resp: Response
  try {
    resp = await fetch(path, {
      method,
      // 会话是 HttpOnly Cookie。不带 credentials 的话每个请求都是未登录，
      // 而表现是「登录成功后立刻又被踢回登录页」，很难查。
      credentials: 'same-origin',
      headers: body === undefined ? {} : { 'Content-Type': 'application/json' },
      body: body === undefined ? undefined : JSON.stringify(body),
    })
  } catch (e) {
    // 网络层失败：后端没起、断网、被防火墙挡住
    throw new ApiError(0, `连不上服务端：${e instanceof Error ? e.message : String(e)}`)
  }

  if (resp.status === 401) {
    // 401 有两种含义，不能一概而论：
    //
    //   - 登录接口的 401 是「用户名或密码错误」。一律当成会话过期的话，
    //     输错密码的人会看到「登录已过期，请重新登录」——他还没登录过，
    //     这句话完全不知所云，而且把真正的原因盖掉了。
    //   - 其余接口的 401 才是会话真的过期了，要把人送回登录页。
    //
    // 所以只有副作用（跳回登录页）需要区分，错误消息一律用后端给的那句，
    // 走下面 !resp.ok 的通用分支去读 {"error": "..."}。
    if (path !== '/api/auth/login') {
      onUnauthorized?.()
    }
  }

  if (resp.status === 204 || resp.headers.get('Content-Length') === '0') {
    return undefined as T
  }

  const contentType = resp.headers.get('Content-Type') ?? ''
  if (!contentType.includes('application/json')) {
    // 后端的 SPA 回退会把未知路径返回成 HTML。不特判的话，
    // JSON.parse 抛的是「Unexpected token <」，完全指不到真正的原因。
    throw new ApiError(
      resp.status,
      `响应不是 JSON（Content-Type: ${contentType || '空'}），` +
        `多半是请求路径写错了：${method} ${path}`,
    )
  }

  let data: unknown
  try {
    data = await resp.json()
  } catch {
    throw new ApiError(resp.status, `响应不是合法 JSON：${method} ${path}`)
  }

  if (!resp.ok) {
    const msg =
      typeof data === 'object' && data !== null && 'error' in data
        ? String((data as { error: unknown }).error)
        : `请求失败（HTTP ${resp.status}）`
    throw new ApiError(resp.status, msg)
  }

  return data as T
}

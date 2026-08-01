import { computed, ref } from 'vue'
import { defineStore } from 'pinia'
import { ApiError, request } from '@/api'
import type { Binding, MeResponse, Membership, Permission, User } from '@/api'

export const useAuthStore = defineStore('auth', () => {
  const user = ref<User | null>(null)
  /** 当前用户的全部授权。fetchMe 一并带回来，省得每页再查一次。 */
  const memberships = ref<Membership[]>([])
  /** 首次 fetchMe 完成前为 true，用来避免闪一下登录页再跳回来。 */
  const loading = ref(true)

  const isLoggedIn = computed(() => user.value !== null)

  async function login(username: string, password: string) {
    user.value = await request<User>('POST', '/api/auth/login', { username, password })
  }

  async function logout() {
    try {
      await request('POST', '/api/auth/logout')
    } finally {
      // 后端失败也要清本地状态：留着一个已经无效的用户对象，
      // 界面会显示成已登录而每个请求都 401，比直接登出更难理解
      user.value = null
      memberships.value = []
    }
  }

  /**
   * fetchMe 是刷新页面后恢复会话的唯一手段。
   *
   * 会话 Cookie 是 HttpOnly，前端读不到，只能问后端「我是谁」。
   * 401 是正常情况（没登录过），不该抛出去让调用方处理。
   */
  async function fetchMe() {
    try {
      // **注意 /api/auth/me 返回的是 {user, memberships} 而不是裸的 User。**
      // 当成 User 用的话 username 与 isAdmin 都是 undefined——界面看起来
      // 登录成功但用户名空着、管理员的管理入口消失，而且不报任何错。
      const me = await request<MeResponse>('GET', '/api/auth/me')
      user.value = me.user
      memberships.value = me.memberships
    } catch (e) {
      if (e instanceof ApiError && e.status === 401) {
        user.value = null
        memberships.value = []
        return
      }
      throw e
    } finally {
      loading.value = false
    }
  }

  /**
   * hasPerm 判断当前用户在某个绑定上有没有某个权限点。
   *
   * 数据来源是绑定列表里后端算好的 permissions 字段——**前端不自己推导**。
   * 后端那边是「管理员 → 账号所有者 → 授权行」三条通路的并集，
   * 在前端重推一遍必然漂，而漂掉的表现是「按钮灰着但请求其实能成」
   * 或者反过来，都比直接报错难查。
   *
   * 管理员单独放行是因为后端对管理员返回的就是全部权限点，
   * 这里只是让 permissions 还没加载出来时也能正确渲染。
   */
  function hasPerm(binding: Pick<Binding, 'permissions'>, p: Permission): boolean {
    if (user.value?.isAdmin) return true
    return binding.permissions.includes(p)
  }

  return { user, memberships, loading, isLoggedIn, login, logout, fetchMe, hasPerm }
})

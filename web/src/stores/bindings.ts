import { computed, ref } from 'vue'
import { defineStore } from 'pinia'
import { ApiError, request } from '@/api'
import type { Binding } from '@/api'

/** 上次选中的绑定记在 localStorage，开播时会在几个页面之间来回切。 */
const STORAGE_KEY = 'magicd.currentBinding'

export const useBindingsStore = defineStore('bindings', () => {
  const list = ref<Binding[]>([])
  const currentId = ref<number | null>(null)
  const loading = ref(false)
  /** 加载失败的原因。为 null 表示上次加载成功。 */
  const loadError = ref<string | null>(null)

  const current = computed(() => list.value.find((b) => b.id === currentId.value) ?? null)

  function select(id: number) {
    currentId.value = id
    localStorage.setItem(STORAGE_KEY, String(id))
  }

  async function refresh() {
    loading.value = true
    loadError.value = null
    try {
      list.value = await request<Binding[]>('GET', '/api/bindings')

      const remembered = Number(localStorage.getItem(STORAGE_KEY))
      const stillThere = list.value.some((b) => b.id === remembered)
      if (stillThere) {
        currentId.value = remembered
      } else {
        // 记住的那个可能已经被删了，或者授权被撤了变成不可见。
        // 回退到第一个而不是留一个空选中——空选中会让每个页面
        // 都显示「请先选择直播间」，而用户明明看到列表里有东西
        currentId.value = list.value[0]?.id ?? null
        if (currentId.value !== null) {
          localStorage.setItem(STORAGE_KEY, String(currentId.value))
        }
      }
    } catch (e) {
      // 不能静默。加载失败时选择器会显示「没有可用的直播间」，
      // 与「这个账号确实没绑过直播间」在界面上完全无法区分——
      // 用户会以为是自己的数据有问题，而不是服务端出错了。
      loadError.value = e instanceof ApiError ? e.message : '加载直播间列表失败'
      list.value = []
    } finally {
      loading.value = false
    }
  }

  return { list, current, currentId, loading, loadError, select, refresh }
})

import { computed, ref } from 'vue'
import { defineStore } from 'pinia'
import { request } from '@/api'
import type { Binding } from '@/api'

/** 上次选中的绑定记在 localStorage，开播时会在几个页面之间来回切。 */
const STORAGE_KEY = 'magicd.currentBinding'

export const useBindingsStore = defineStore('bindings', () => {
  const list = ref<Binding[]>([])
  const currentId = ref<number | null>(null)
  const loading = ref(false)

  const current = computed(() => list.value.find((b) => b.id === currentId.value) ?? null)

  function select(id: number) {
    currentId.value = id
    localStorage.setItem(STORAGE_KEY, String(id))
  }

  async function refresh() {
    loading.value = true
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
    } finally {
      loading.value = false
    }
  }

  return { list, current, currentId, loading, select, refresh }
})

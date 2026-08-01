import { onUnmounted, ref, shallowRef } from 'vue'

/** 推给浏览器的事件形状，与后端 activity_handler.go 的 streamEventView 对应。 */
export interface StreamEvent {
  id: string
  type: string
  roomId: string
  timestamp: string
  payload: unknown
}

/**
 * 后端会推的命名事件类型，与 `server/internal/event/type.go` 的
 * `Type` 常量逐一对应（共 19 种）。
 *
 * **这份清单必须跟 type.go 手动同步**，不是照抄某个旧文档：SSE 的
 * onmessage 只接默认（未命名）事件，而后端每条都带 `event: <type>`
 * （见 activity_handler.go 的 `write("event: %s\ndata: %s\n\n", ev.Type, ...)`），
 * 所以必须逐个 addEventListener——只写 onmessage 的话一条都收不到，
 * 且不报任何错；漏掉某个类型的表现同样是「那类事件永远不显示」，
 * 而不是报错。
 */
const EVENT_TYPES = [
  'danmaku', // 弹幕
  'super_chat', // 醒目留言
  'super_chat_delete', // 醒目留言被删除
  'gift', // 礼物
  'gift_combo', // 礼物连击
  'guard_buy', // 上舰
  'user_enter', // 用户进场
  'user_follow', // 用户关注
  'user_share', // 用户分享
  'user_like', // 用户点赞
  'live_start', // 开播
  'live_stop', // 下播
  'room_change', // 房间信息变更
  'user_blocked', // 用户被禁言
  'online_rank_update', // 高能榜变化
  'room_stats_update', // 房间统计数据变化
  'battle', // PK 大乱斗
  'manual', // 操作者从 WebUI 手动触发
  'unknown', // 未识别的 CMD
]

export interface EventStreamOptions {
  /** 内存里最多留多少条。 */
  max?: number
}

export function useEventStream(bindingId: number, opts: EventStreamOptions = {}) {
  const max = opts.max ?? 500
  // shallowRef：事件对象本身不会被改，不需要深响应，几万条的深代理很贵
  const events = shallowRef<StreamEvent[]>([])
  const connected = ref(false)
  const error = ref<string | null>(null)

  const es = new EventSource(`/api/bindings/${bindingId}/stream`)
  // 测试用的 FakeEventSource.close() 只标记 closed，并不会真的让后续
  // emit 失效（真实浏览器的 EventSource 关闭后也不保证百分百不再触发
  // 已排队的回调）。所以「关闭后不再接收」这条要素必须由本地状态兜底，
  // 不能指望底层连接一关回调就不会再跑。
  let closed = false

  es.onopen = () => {
    connected.value = true
    error.value = null
  }
  es.onerror = () => {
    // EventSource 自带断线重连，这里只反映状态不主动重连——
    // 自己再实现一套重连会和浏览器内建的那套打架
    connected.value = false
    error.value = '实时连接断开，正在自动重连'
  }

  for (const type of EVENT_TYPES) {
    es.addEventListener(type, (e) => {
      if (closed) return
      let ev: StreamEvent
      try {
        ev = JSON.parse((e as MessageEvent).data) as StreamEvent
      } catch {
        return // 坏数据丢掉，不该让整个页面炸
      }
      // 新的在前，超出上限从尾部丢：日志页通常最新在上，直接看最新动态
      // 不用滚到底部。一场直播几万条，不设上限的话内存一直涨，而用户
      // 只是把日志页开着没关
      const next = [ev, ...events.value]
      events.value = next.length > max ? next.slice(0, max) : next
    })
  }

  function close() {
    closed = true
    es.close()
    connected.value = false
  }

  onUnmounted(close)

  return { events, connected, error, close }
}

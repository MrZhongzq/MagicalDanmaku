-- 直播间开播状态心跳（P5-2 任务 1b/2）。
--
-- live_status 是三态，语义与迁移 003 的账号登录态完全对称：
-- living（开播中）/ offline（未开播，含轮播）/ unknown（尚未检测过，
-- 或最近一次探测本身失败）。网络不通/被风控不等于真的没开播，两者
-- 不能共用同一个值表示，否则界面会把"探测失败"误报成"确认没开播"——
-- 这正是 P5-2 需求反复强调的一条红线。默认值与"从未检测过"共用
-- unknown，与账号登录态是同一个设计。
--
-- anchor_uid/anchor_name 是主播身份（UID + 昵称），探测成功时才写入
-- 并覆盖；探测失败时应用层要保留上一次已知值，不能被空值抹掉，所以
-- 这里的默认值是空字符串而不是 NULL——CASE WHEN 判断空串比判断 NULL
-- 简单，写法与迁移 003 里 uid 字段的 upsert 逻辑一致。
ALTER TABLE bindings
    ADD COLUMN live_status TEXT NOT NULL DEFAULT 'unknown'
        CHECK (live_status IN ('living', 'offline', 'unknown')),
    ADD COLUMN live_checked_at TIMESTAMPTZ,
    ADD COLUMN anchor_uid TEXT NOT NULL DEFAULT '',
    ADD COLUMN anchor_name TEXT NOT NULL DEFAULT '';

-- 账号登录态检测（P4-3 任务 8）。
--
-- login_state 是三态：valid（登录有效）/ invalid（登录已失效）/
-- unknown（尚未检测过，或最近一次检测本身失败——网络不通不等于账号
-- 掉线，两者不能用同一个值表示，否则界面会把「探测失败」误报成
-- 「账号已失效」）。默认值与「从未检测过」共用 unknown，语义上是
-- 一致的：在拿到第一次确定结果之前，登录态本来就是未知的。
--
-- login_checked_at 记录最近一次尝试检测的时间（无论检测成功与否），
-- 供界面在需要时判断「多久没探测过了」。
ALTER TABLE accounts
    ADD COLUMN login_state TEXT NOT NULL DEFAULT 'unknown'
        CHECK (login_state IN ('valid', 'invalid', 'unknown')),
    ADD COLUMN login_checked_at TIMESTAMPTZ;

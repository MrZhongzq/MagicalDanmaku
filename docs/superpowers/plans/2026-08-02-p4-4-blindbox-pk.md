# P4-4 盲盒与 PK 实施计划

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development.

**Goal:** 把悬空清单最后四条（第 7、10、11、15 条）做掉，并按用户澄清修正聚合窗口语义。

**样本来源：** `server/blindbox.jsonl`（551 行，用户 2026-08-02 在真实直播间抓的，含一整场 PK 与 17 个盲盒）。**这是本批次唯一的事实依据，所有字段判断以它为准。**

## Global Constraints

- module `github.com/MrZhongzq/MagicalDanmaku/server`，代码在 `server/`
- **纯 Go 依赖，不引入任何新依赖**，`CGO_ENABLED=0` 六平台交叉编译必须通过
- 只用标准库 `http.ServeMux`
- 注释、错误信息、提交信息一律中文
- **一切改变状态的接口不得用 GET**
- **授权判定只有 `guard.go` 一处实现**
- `gofmt -l .` 无输出、`go vet ./...` 干净
- TDD：先写失败测试 → 确认失败 → 实现 → 确认通过 → 提交
- **黄金样本进 `testdata/`**：从 `blindbox.jsonl` 里挑出的报文要落成测试固件，别让测试依赖那个大文件

## 用户澄清的四条事实（全部来自真实样本，不是推断）

### 一、金额单位是 1/100 电池

`total_coin`、`price`、`blind_gift.original_gift_price`、`gift_tip_price` 全部是**电池 × 100**。

- 幸运盲盒 `original_gift_price: 5000` = **50 电池**
- 心动盲盒 `15000` = **150 电池**
- 小熊虫盲盒 `9000` = **90 电池**

**库里存原始整数，只在展示层除以 100。** 中间用浮点算钱会累积误差。

### 二、盲盒字段语义（17 条样本全部验证通过）

```
blind_gift == null              → 普通礼物
blind_gift != null              → 盲盒

price                           = 爆出礼物的单价
total_coin                      = 用户实际花掉的（盲盒售价 × num）← 不是产出
blind_gift.original_gift_price  = 盲盒售价（单个）
blind_gift.gift_tip_price       = 爆出礼物的价值（恒等于 price）
blind_gift.original_gift_name   = 盲盒名称
blind_gift.original_gift_id     = 盲盒 ID
blind_gift.blind_gift_config_id = 配置 ID
```

断言在 17 条样本上成立且无例外：`total_coin == original_gift_price * num`、`price == gift_tip_price`。

**盈亏 = price×num − total_coin。**

### 三、聚合窗口是「乙语义」，不是现在实现的静默计时

用户原话：

> 从第一个盲盒礼物起开始计算，假设 x=10，那从第一个礼物往后数 10 秒的礼物算一轮，第 11 秒就是下一轮
>
> **乙**：上一轮结束后，下一个到来的事件重新起锚

而 `aggregate.go` 现在是**静默计时**（每来一个事件把定时器往后推，安静满 `Window` 才结算）。

**两者在真实数据上差很多**：17 条盲盒按乙语义分 5+1+1+1 轮，按静默计时会并成 4 轮（其中一轮 9 个）。

**这一条影响所有合并窗口**（进房欢迎、礼物答谢全走同一个聚合器），不只是盲盒。

### 四、盲盒的聚合键是「送礼人 × 盲盒类型」

用户原话：

> 正确逻辑是获取谁干的，获取啥礼物，开始计时
>
> 心动和幸运交叉送也要分开统计盈亏

样本里证据确凿：`ts 313..321` 那段，同一个人交叉送幸运与心动。不分开的话会算成「投入 60000 产出 54400 亏 5600」，而实际是**幸运赚 5400、心动亏 11000**——两个完全不同的结论。

### 五、PK 的 `init_info`/`match_info` 是「发起方/被匹配方」，不是「自己/对面」

样本里 `init_info.room_id = 1838150399`（对面），`match_info.room_id = 1781257934`（我方）——因为那场是对面发起的。

**唯一正确的判法**：拿本绑定的房间号去比对 `PK_INFO.data.members[]`，`room_id != 自己` 的才是对面。

按 `init_info` 当自己写的话，**用户主动发起 PK 时会把自己当对面播报，而且不报错**。

`members` 是数组，`muti_pk_type: 3`、`template_id: multi_conn_grid` —— B 站支持多人 PK，**不能假设只有两方**。

---

## Task 1: 盲盒字段进协议层

**Files:**
- Modify: `server/internal/event/payload.go`（`Gift` 加盲盒字段）
- Modify: `server/internal/connector/bilibili/cmdmap/`（`SEND_GIFT` 的映射）
- Create: `server/testdata/` 下的黄金样本
- Modify: 对应测试

**Interfaces:**
- Produces: `event.BlindBox` 结构体、`event.Gift.BlindBox *BlindBox`

### 设计

```go
// BlindBox 是盲盒礼物的附加信息。为 nil 表示这不是盲盒。
//
// 金额单位都是 1/100 电池——B 站原始报文就是这个单位，存原始整数、
// 只在展示层除以 100。中间用浮点算钱会累积误差。
type BlindBox struct {
	Name     string // 盲盒名称，如「幸运盲盒」
	GiftID   int64  // 盲盒自身的礼物 ID
	Price    int64  // 盲盒售价（单个），1/100 电池
	TipPrice int64  // 爆出礼物的价值，1/100 电池；恒等于 Gift.Price
}
```

`Gift` 加 `BlindBox *BlindBox`。

**不要给 `Gift` 加「盈亏」字段**——那是聚合期算出来的，不属于单条事件。

### 测试要钉住的

1. `blind_gift: null` → `BlindBox` 为 nil
2. 盲盒报文 → 四个字段都对
3. **`total_coin == Price * num`** 与 **`Gift.Price == TipPrice`** 在黄金样本上成立
4. 三种盲盒（幸运/心动/小熊虫）各一条黄金样本

**黄金样本从 `server/blindbox.jsonl` 里挑，落到 `testdata/` 下。** 挑的时候把 `uid`/`uname` 换成假值——那是真实用户信息。

---

## Task 2: 聚合窗口改成乙语义

**Files:**
- Modify: `server/internal/rules/aggregate.go` 与测试

**这是本批次风险最高的一个**——所有合并窗口都走它。

### 现在是什么

`Aggregator.Add`：每来一个事件重设 `idle` 定时器（`time.AfterFunc(a.spec.Window, ...)`），安静满 `Window` 才 `onTimeout`。`MaxWait` 从首个事件起只设一次，兜底防止永不结算。

### 要改成什么

**窗口从首个事件起算，固定 `Window` 时长后结算。窗口关闭后，下一个到达的事件重新起锚。**

也就是把 `idle` 那个「每次推后」的行为去掉，改成首个事件时设一次定时器、到点就结算。

### 要想清楚的三件事

1. **`MaxWait` 还有意义吗？** 乙语义下窗口本来就有固定上界（`Window`），`MaxWait` 的原始用途（防止持续送礼永不结算）已经不存在。**判断：保留还是废弃？** 保留的话它与 `Window` 的关系是什么。**在报告里说明**，不要含糊
2. **已有配置的兼容**：现存的规则里 `Window` 的语义变了，同一个数值下行为会变。**这是有意的**（用户要的就是这个），但要在提交信息与文档里写明
3. **`sortBucketsBySeq` 与 `mergeBuckets` 不用改**——它们处理的是窗口内的分组与合并，与窗口何时关闭无关。**确认一下再动手**

### 测试要钉住的

1. 首个事件起 `Window` 秒后结算，中途来的事件**不延长**窗口
2. 窗口关闭后，下一个事件重新起锚（不是固定网格）
3. 窗口内无事件时不产生空轮
4. **用真实时间戳跑一遍**：把样本里那 17 条的时间戳喂进去，断言分组结果与 `5+1+1+1` 一致

第 4 条是这个任务最有价值的测试——它用真实数据钉住语义。

**测试不要用真实 `time.Sleep`。** 现有测试是怎么控制时间的？去看一眼，照它来；如果它用的是真实定时器，考虑注入一个时钟。

---

## Task 3: 盲盒单独聚合与盈亏

**Files:**
- Modify: `server/internal/rules/spec/spec.go`（`Aggregate.By` 加盲盒分组）
- Modify: `server/internal/rules/aggregate.go`、`vars.go`
- Modify: 对应测试

### 设计

**聚合键 = (送礼人 uid, 盲盒名称)。**

现有的 `AggregateBy` 有哪些取值？去 `rules` 包看。加一个 `AggregateByBlindBox`（或类似），它的 `groupKey` 是 `uid + "\x00" + blindBoxName`。

**模板变量**：合并结算时填

```
blindBox.name       盲盒名称
blindBox.count      个数
blindBox.cost       投入（1/100 电池）
blindBox.gain       产出（1/100 电池）
blindBox.profit     盈亏 = gain - cost，可为负
blindBox.costYuan   投入（元，展示用）
blindBox.gainYuan   产出（元）
blindBox.profitYuan 盈亏（元）
```

**「元」那三个怎么算？** 1 电池 = 0.1 元，原始值是 1/100 电池，所以 `元 = 原始值 / 1000`。**去核实这个换算**——如果拿不准就只提供电池，别猜出一个错的金额。

**这些变量要进 `VariableCatalog`**（P4-3 Task 7 建的清单），否则条件构建器里选不到。**`commonVariables` 那个人工维护的盲区在 P4-3 已经记过一次，这次别再漏。**

### 硬性要求：盲盒不进常规礼物统计

用户原话：「盲盒类单独计算」。所以：

- 常规礼物答谢的合并窗口**不能收盲盒事件**
- 统计页的「礼物件数/礼物种类」**不含盲盒**
- 盲盒有自己的一套

**去看现有的礼物规则怎么匹配的**——是靠 `on: [gift]` 吗？那盲盒也是 `gift` 事件，怎么分开？**这是这个任务的核心设计问题**，可能需要：
- 新增一个事件类型 `gift_blindbox`，在协议层就分流，或
- 保持 `gift` 但让条件能筛（`when: {field: "gift.isBlindBox", op: "eq", value: true}`）

**两种各有代价，你判断并在报告里说明。** 我倾向后者（不增加事件类型，靠条件筛），但前者对「默认不混」更友好。

---

## Task 4: PK 事件进协议层

**Files:**
- Modify: `server/internal/event/payload.go`、`type.go`
- Modify: `server/internal/connector/bilibili/cmdmap/`
- Create: `testdata/` 黄金样本

### 设计

现在 `Battle` 只有 `SubCommand string`。要能解析出：

```go
// PkMember 是一场 PK 里的一方。
type PkMember struct {
	RoomID   string
	UID      string
	Username string
	Face     string
	Votes    int64
	IsWinner bool
}

// Battle 是 PK 事件。
type Battle struct {
	SubCommand string     // 原始 CMD 名，保留
	PkID       string     // 这场 PK 的 ID
	Members    []PkMember // **可能多于两方**，B 站支持多人 PK
	StartTime  int64
	EndTime    int64
}
```

**数据源是 `PK_INFO`**（不是 `PK_BATTLE_START_NEW`）——`PK_INFO.data.members[]` 里 `room_id`/`uid`/`uname`/`face`/`votes` 全都有，`pk_basic` 里有 `pk_id`/`start_time`/`end_time`。

**不要用 `init_info`/`match_info` 判自己/对面**——那是发起方/被匹配方。判自己只能拿本绑定的房间号比对。

### 模板变量

```
pk.pkId
pk.opponents      对面各方（room_id != 自己的那些）
pk.opponent.uname 只有一个对面时的便捷取法
```

**「对面」的判定要在哪一层做？** 协议层不知道「自己」是谁（`Event` 里有 `RoomID`，但那是收到事件的房间）。**判断：在 cmdmap 里用 `Event.RoomID` 过滤，还是把全部 members 交给规则层由 `vars.go` 过滤？** 在报告里说明。

### 用户要的另外三项不在弹幕流里

「对面直播间人数、大航海总数、大航海在线数」**样本里没有**。要拿 `room_id` 另调接口。

**本任务不做这三项**，只做弹幕流里有的。另开 Task 5。

---

## Task 5: 对面房间的人数与大航海（要另调接口）

**先查证接口再动手。** 原 C++ 项目里有没有？没有就查 B 站现有接口。

**查不到可靠定义就不做**，退回悬空清单并说明——这一期已经因为「不猜」避免过一次错误实现（拉黑那条）。

---

## Task 6: PK 串门欢迎

样本里 Q亦巧儿（对面主播，uid 3707039169644702）在 PK 期间给本房间送了礼物（行 446）。

**但报文里没有任何「这个人来自对面直播间」的标识**——`uid` 就是个普通用户 uid。

所以只能：**记下 PK 期间对面的 room_id/uid，比对进房事件的 uid**。

**这条要在报告里明确说清它的局限**：只能认出对面**主播本人**，认不出对面的**观众**。用户要的「PK 串门欢迎」如果指的是后者，这个做法覆盖不了，要另想办法（或者退回悬空清单）。

---

## Task 7: 接进统计与前端

- 统计页加盲盒盈亏（用真实数据，不再是占位符）
- 弹幕姬页的盲盒开关接上真实规则
- PK 播报接上真实字段
- 撤掉对应的悬空标记
- 更新悬空清单

**「元」的换算只在展示层做。**

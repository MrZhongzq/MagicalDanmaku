package httpapi_test

import (
	"context"
	"encoding/json"
	"net/http"
	"strings"
	"sync"
	"testing"

	"github.com/MrZhongzq/MagicalDanmaku/server/internal/perm"
	"github.com/MrZhongzq/MagicalDanmaku/server/internal/store"
)

// fakeLifecycle 记录被调用的绑定 ID，验证 handler 在数据库状态改对之后
// 是否正确地把「让改动在运行期生效」这件事交给了注入的实现——
// 不必真的建连接、真的装配规则引擎（那是 cmd/magicd 的 runtimeManager
// 自己的测试范围），这里只关心 handler 有没有在正确的时机调用它。
type fakeLifecycle struct {
	mu       sync.Mutex
	started  []int64
	stopped  []int64
	startErr error
}

func (f *fakeLifecycle) StartBinding(_ context.Context, id int64) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.started = append(f.started, id)
	return f.startErr
}

func (f *fakeLifecycle) StopBinding(_ context.Context, id int64) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.stopped = append(f.stopped, id)
}

func (f *fakeLifecycle) snapshot() (started, stopped []int64) {
	f.mu.Lock()
	defer f.mu.Unlock()
	return append([]int64{}, f.started...), append([]int64{}, f.stopped...)
}

type bindingView struct {
	ID            int64    `json:"id"`
	AccountName   string   `json:"accountName"`
	RoomID        string   `json:"roomId"`
	Enabled       bool     `json:"enabled"`
	RuleCount     int      `json:"ruleCount"`
	Permissions   []string `json:"permissions"`
	LiveStatus    string   `json:"liveStatus"`
	LiveCheckedAt *string  `json:"liveCheckedAt"`
	AnchorUID     string   `json:"anchorUid"`
	AnchorName    string   `json:"anchorName"`
}

func TestListBindingsIncludesCallerPermissions(t *testing.T) {
	srv, st := newTestServer(t)
	loginAs(t, srv, st, "张三", false)
	mustBindingFor(t, st, "张三", "小号", "123")

	li := loginAs(t, srv, st, "李四", false)
	if err := st.Grant(context.Background(), "李四", "小号", "123",
		[]perm.Permission{perm.RuleRead, perm.EventRead}); err != nil {
		t.Fatalf("授权报错: %v", err)
	}

	resp := jsonRequest(t, li, "GET", srv.URL+"/api/bindings", "")
	defer resp.Body.Close()

	var got []bindingView
	if err := json.NewDecoder(resp.Body).Decode(&got); err != nil {
		t.Fatalf("解析报错: %v", err)
	}
	if len(got) != 1 {
		t.Fatalf("绑定数 = %d, 期望 1", len(got))
	}
	have := map[string]bool{}
	for _, p := range got[0].Permissions {
		have[p] = true
	}
	if !have["rule:read"] || !have["event:read"] {
		t.Errorf("权限点 = %v, 期望含 rule:read 与 event:read", got[0].Permissions)
	}
	if have["rule:write"] {
		t.Errorf("不该有 rule:write: %v", got[0].Permissions)
	}
}

func TestListBindingsAdminGetsAllPermissions(t *testing.T) {
	srv, st := newTestServer(t)
	loginAs(t, srv, st, "张三", false)
	mustBindingFor(t, st, "张三", "小号", "123")

	admin := loginAs(t, srv, st, "管理员", true)
	resp := jsonRequest(t, admin, "GET", srv.URL+"/api/bindings", "")
	defer resp.Body.Close()

	var got []bindingView
	if err := json.NewDecoder(resp.Body).Decode(&got); err != nil {
		t.Fatalf("解析报错: %v", err)
	}
	if len(got) != 1 {
		t.Fatalf("绑定数 = %d", len(got))
	}
	if len(got[0].Permissions) != len(perm.All()) {
		t.Errorf("管理员应拿到全部 %d 个权限点，实际 %d 个",
			len(perm.All()), len(got[0].Permissions))
	}
}

// 所有者在自己的绑定上应看到除 member:manage 外的全部权限点。
//
// 若这里返回 []，前端会把按钮全灰掉，而 PATCH 其实能成——
// 「列表说没权限、请求却成了」比直接报错更难查。
//
// member:manage 必须显式断言不在里面：只对个数比对，会被将来
// 加权限点时的巧合蒙混过去。把第三方拉进授权体系是管理员级别的
// 决定，不是账号所有权的附带品。
func TestListBindingsGivesOwnerAllPermissionsExceptMemberManage(t *testing.T) {
	srv, st := newTestServer(t)
	c := loginAs(t, srv, st, "张三", false)
	mustBindingFor(t, st, "张三", "小号", "123")

	resp := jsonRequest(t, c, "GET", srv.URL+"/api/bindings", "")
	defer resp.Body.Close()

	var got []bindingView
	if err := json.NewDecoder(resp.Body).Decode(&got); err != nil {
		t.Fatalf("解析报错: %v", err)
	}
	if len(got) != 1 {
		t.Fatalf("绑定数 = %d, 期望 1", len(got))
	}
	if len(got[0].Permissions) != len(perm.All())-1 {
		t.Errorf("所有者的权限点 = %v, 期望 %d 个（全部减去 member:manage）",
			got[0].Permissions, len(perm.All())-1)
	}
	for _, p := range got[0].Permissions {
		if p == string(perm.MemberManage) {
			t.Errorf("所有者不该凭所有权获得 member:manage: %v", got[0].Permissions)
		}
	}
}

// 所有者若被显式授予了 member:manage，列表必须体现出来——这是
// Task 8b 裁决之后的标准配置：所有者与授权行是并集，不是二选一。
//
// permissionSet.of 若在命中 owned 分支后提前 return，就会漏掉
// byBinding 里的 memberships，而 store.Can 的 SQL 是三条 OR，
// memberships 分支仍会命中。表现是「列表说没权限、PUT 请求却成了」。
func TestListBindingsUnionsOwnerAndExplicitGrant(t *testing.T) {
	srv, st := newTestServer(t)
	c := loginAs(t, srv, st, "张三", false)
	bid := mustBindingFor(t, st, "张三", "小号", "123")

	// 张三是所有者，同时被显式授予了 member:manage
	if err := st.Grant(context.Background(), "张三", "小号", "123",
		[]perm.Permission{perm.MemberManage}); err != nil {
		t.Fatalf("授权报错: %v", err)
	}

	resp := jsonRequest(t, c, "GET", srv.URL+"/api/bindings", "")
	defer resp.Body.Close()

	var got []bindingView
	if err := json.NewDecoder(resp.Body).Decode(&got); err != nil {
		t.Fatalf("解析报错: %v", err)
	}
	if len(got) != 1 {
		t.Fatalf("绑定数 = %d, 期望 1", len(got))
	}
	have := map[string]bool{}
	for _, p := range got[0].Permissions {
		have[p] = true
	}
	if !have[string(perm.MemberManage)] {
		t.Errorf("所有者被显式授予 member:manage 后，列表应含它，实际 %v", got[0].Permissions)
	}

	zhang, err := st.GetUserByName(context.Background(), "张三")
	if err != nil {
		t.Fatalf("查用户报错: %v", err)
	}
	can, err := st.Can(context.Background(), zhang.ID, bid, perm.MemberManage)
	if err != nil {
		t.Fatalf("Can 报错: %v", err)
	}
	if !can {
		t.Fatal("store.Can 应认为张三有 member:manage（前提假设错误，测试本身有问题）")
	}
	if have[string(perm.MemberManage)] != can {
		t.Errorf("列表判定 = %v，store.Can 判定 = %v，二者必须一致", have[string(perm.MemberManage)], can)
	}
}

func TestCreateBinding(t *testing.T) {
	srv, st := newTestServer(t)
	c := loginAs(t, srv, st, "张三", false)
	mustBindingFor(t, st, "张三", "小号", "111")

	resp := jsonRequest(t, c, "POST", srv.URL+"/api/bindings",
		`{"accountName":"小号","roomId":"222"}`)
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusCreated {
		t.Fatalf("状态码 = %d, 期望 201", resp.StatusCode)
	}

	if _, err := st.GetBinding(context.Background(), "小号", "222"); err != nil {
		t.Errorf("绑定应已建好: %v", err)
	}
}

func TestCreateBindingRequiresAccountOwnership(t *testing.T) {
	srv, st := newTestServer(t)
	loginAs(t, srv, st, "张三", false)
	mustBindingFor(t, st, "张三", "小号", "111")

	li := loginAs(t, srv, st, "李四", false)
	resp := jsonRequest(t, li, "POST", srv.URL+"/api/bindings",
		`{"accountName":"小号","roomId":"222"}`)
	defer resp.Body.Close()
	if resp.StatusCode == http.StatusCreated {
		t.Error("非所有者不该能给别人的账号加直播间")
	}
}

func TestCreateBindingRejectsEmptyRoom(t *testing.T) {
	srv, st := newTestServer(t)
	c := loginAs(t, srv, st, "张三", false)
	mustBindingFor(t, st, "张三", "小号", "111")

	resp := jsonRequest(t, c, "POST", srv.URL+"/api/bindings",
		`{"accountName":"小号","roomId":"  "}`)
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusUnprocessableEntity {
		t.Errorf("空房间号状态码 = %d, 期望 422", resp.StatusCode)
	}
}

// 重复创建是幂等的，不该报错——重复点一下按钮不该看到红色报错
func TestCreateBindingIsIdempotent(t *testing.T) {
	srv, st := newTestServer(t)
	c := loginAs(t, srv, st, "张三", false)
	mustBindingFor(t, st, "张三", "小号", "111")

	body := `{"accountName":"小号","roomId":"222"}`
	r1 := jsonRequest(t, c, "POST", srv.URL+"/api/bindings", body)
	r1.Body.Close()
	r2 := jsonRequest(t, c, "POST", srv.URL+"/api/bindings", body)
	defer r2.Body.Close()
	if r2.StatusCode != http.StatusCreated && r2.StatusCode != http.StatusOK {
		t.Errorf("重复创建状态码 = %d, 不该报错", r2.StatusCode)
	}

	bs, err := st.ListBindings(context.Background())
	if err != nil {
		t.Fatalf("列绑定报错: %v", err)
	}
	if len(bs) != 2 { // 111 与 222
		t.Errorf("绑定数 = %d, 期望 2（不该重复）", len(bs))
	}
}

func TestToggleBindingRequiresRuleWrite(t *testing.T) {
	srv, st := newTestServer(t)
	loginAs(t, srv, st, "张三", false)
	bid := mustBindingFor(t, st, "张三", "小号", "123")

	li := loginAs(t, srv, st, "李四", false)
	if err := st.Grant(context.Background(), "李四", "小号", "123",
		[]perm.Permission{perm.RuleRead}); err != nil {
		t.Fatalf("授权报错: %v", err)
	}

	resp := jsonRequest(t, li, "PATCH", srv.URL+"/api/bindings/"+itoa(bid), `{"enabled":false}`)
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusForbidden {
		t.Errorf("只有 rule:read 时状态码 = %d, 期望 403", resp.StatusCode)
	}
}

func TestToggleBinding(t *testing.T) {
	srv, st := newTestServer(t)
	c := loginAs(t, srv, st, "张三", false)
	bid := mustBindingFor(t, st, "张三", "小号", "123")

	resp := jsonRequest(t, c, "PATCH", srv.URL+"/api/bindings/"+itoa(bid), `{"enabled":false}`)
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("状态码 = %d", resp.StatusCode)
	}

	b, err := st.GetBinding(context.Background(), "小号", "123")
	if err != nil {
		t.Fatalf("查绑定报错: %v", err)
	}
	if b.Enabled {
		t.Error("应已停用")
	}
}

func TestDeleteBindingByOwner(t *testing.T) {
	srv, st := newTestServer(t)
	c := loginAs(t, srv, st, "张三", false)
	bid := mustBindingFor(t, st, "张三", "小号", "123")

	resp := jsonRequest(t, c, "DELETE", srv.URL+"/api/bindings/"+itoa(bid), "")
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("状态码 = %d", resp.StatusCode)
	}
	if _, err := st.GetBinding(context.Background(), "小号", "123"); err == nil {
		t.Error("删除后应查不到")
	}
}

// 有 rule:write 不等于能删绑定——删绑定会带走全部规则与授权，
// 是账号所有权级别的操作
func TestDeleteBindingRequiresOwnership(t *testing.T) {
	srv, st := newTestServer(t)
	loginAs(t, srv, st, "张三", false)
	bid := mustBindingFor(t, st, "张三", "小号", "123")

	li := loginAs(t, srv, st, "李四", false)
	if err := st.Grant(context.Background(), "李四", "小号", "123",
		[]perm.Permission{perm.RuleWrite}); err != nil {
		t.Fatalf("授权报错: %v", err)
	}

	resp := jsonRequest(t, li, "DELETE", srv.URL+"/api/bindings/"+itoa(bid), "")
	defer resp.Body.Close()
	// 403 而不是 404：李四对这个绑定有可见性（他有 rule:write），
	// 告诉他「存在但你不是所有者」不算泄漏
	if resp.StatusCode != http.StatusForbidden {
		t.Errorf("有 rule:write 时状态码 = %d, 期望 403", resp.StatusCode)
	}
	if _, err := st.GetBinding(context.Background(), "小号", "123"); err != nil {
		t.Error("绑定不该被删掉")
	}
}

// 与这个绑定毫无关系的人删它，必须收到 404 而不是 403。
//
// DELETE 走的是 requireAuth 不是 requirePerm，没有守卫替它做可见性
// 判断。若无条件回 403，拿绑定 ID 从 1 递增试一遍就能枚举出部署里
// 有哪些绑定——「不存在」与「存在但不归你」必须不可区分。
func TestDeleteBindingByStrangerLooksLikeNotFound(t *testing.T) {
	srv, st := newTestServer(t)
	loginAs(t, srv, st, "张三", false)
	bid := mustBindingFor(t, st, "张三", "小号", "123")

	wu := loginAs(t, srv, st, "王五", false) // 无任何授权、不拥有任何账号

	resp := jsonRequest(t, wu, "DELETE", srv.URL+"/api/bindings/"+itoa(bid), "")
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusNotFound {
		t.Errorf("陌生人删绑定状态码 = %d, 期望 404（403 会泄漏该绑定存在）", resp.StatusCode)
	}

	var body map[string]string
	if err := json.NewDecoder(resp.Body).Decode(&body); err != nil {
		t.Fatalf("解析报错: %v", err)
	}
	// 文案也不能泄漏：不得出现账号名、房间号，或「所有者」这类
	// 暗示「它存在，只是不归你」的措辞
	for _, leak := range []string{"小号", "123", "所有者"} {
		if strings.Contains(body["error"], leak) {
			t.Errorf("错误文案泄漏了 %q: %q", leak, body["error"])
		}
	}

	if _, err := st.GetBinding(context.Background(), "小号", "123"); err != nil {
		t.Error("绑定不该被删掉")
	}
}

// ---- P5-1：绑定的增删启停要让运行时在进程内跟着变，不需要重启 ----
//
// 这些测试只钉「handler 有没有在数据库状态改对之后调用注入的
// BindingLifecycle」，不关心 StartBinding/StopBinding 内部怎么建连接——
// 那是 cmd/magicd 的 runtimeManager 自己的测试范围（见 run_test.go /
// runtime_manager_test.go）。这里假的实现只负责记下被调用的绑定 ID。

// fakeRoomStatusProbe 记录被探测的绑定 ID，可选地模拟真实探测成功后
// 写库的副作用（simulateWrite 非空时），供验证响应体是否反映了立即
// 检测的最新结果。
type fakeRoomStatusProbe struct {
	mu      sync.Mutex
	probed  []int64
	st      *store.Store
	liveTo  string // 非空时，ProbeNow 会把探测到的绑定写成这个状态
	anchor  string
	anchorU string
}

func (f *fakeRoomStatusProbe) ProbeNow(ctx context.Context, bindingID int64) {
	f.mu.Lock()
	f.probed = append(f.probed, bindingID)
	f.mu.Unlock()
	if f.liveTo != "" {
		_ = f.st.UpdateBindingRoomStatus(ctx, bindingID, f.liveTo, f.anchorU, f.anchor)
	}
}

func (f *fakeRoomStatusProbe) snapshot() []int64 {
	f.mu.Lock()
	defer f.mu.Unlock()
	return append([]int64{}, f.probed...)
}

// TestCreateBindingProbesRoomStatusImmediately 钉住 P5-2 任务 2 的第二条：
// 加直播间后必须立刻探测一次，且响应体要反映探测到的最新结果（主播
// UID + 昵称 + 开播状态），而不是加完之后还得等 60 秒心跳才看得到。
func TestCreateBindingProbesRoomStatusImmediately(t *testing.T) {
	srv, st, api := newTestServerWithAPI(t)
	probe := &fakeRoomStatusProbe{st: st, liveTo: store.RoomLiveLiving, anchorU: "20285041", anchor: "舞月雅白"}
	api.SetRoomStatusProbe(probe)
	c := loginAs(t, srv, st, "张三", false)
	mustBindingFor(t, st, "张三", "小号", "111")

	resp := jsonRequest(t, c, "POST", srv.URL+"/api/bindings",
		`{"accountName":"小号","roomId":"222"}`)
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusCreated {
		t.Fatalf("状态码 = %d, 期望 201", resp.StatusCode)
	}

	b, err := st.GetBinding(context.Background(), "小号", "222")
	if err != nil {
		t.Fatalf("查绑定报错: %v", err)
	}
	if got := probe.snapshot(); len(got) != 1 || got[0] != b.ID {
		t.Errorf("ProbeNow 调用记录 = %v, 期望恰好 [%d]", got, b.ID)
	}

	var got bindingView
	if err := json.NewDecoder(resp.Body).Decode(&got); err != nil {
		t.Fatalf("解析响应报错: %v", err)
	}
	if got.LiveStatus != store.RoomLiveLiving {
		t.Errorf("响应里的 LiveStatus = %q, 期望 %q（应反映立即检测的结果）", got.LiveStatus, store.RoomLiveLiving)
	}
	if got.AnchorUID != "20285041" || got.AnchorName != "舞月雅白" {
		t.Errorf("响应里的主播身份 = uid=%q name=%q，期望反映立即检测的结果", got.AnchorUID, got.AnchorName)
	}
}

// TestCreateBindingIdempotentOnDisabledBindingDoesNotProbeRoomStatus 与
// 幂等分支不重新拉起运行时是同一个道理：重复创建落在一个已停用的绑定
// 上时，不该悄悄替它做一次开播状态探测。
func TestCreateBindingIdempotentOnDisabledBindingDoesNotProbeRoomStatus(t *testing.T) {
	srv, st, api := newTestServerWithAPI(t)
	probe := &fakeRoomStatusProbe{st: st}
	api.SetRoomStatusProbe(probe)
	c := loginAs(t, srv, st, "张三", false)
	mustBindingFor(t, st, "张三", "小号", "111")

	if err := st.SetBindingEnabled(context.Background(), "小号", "111", false); err != nil {
		t.Fatalf("停用报错: %v", err)
	}

	resp := jsonRequest(t, c, "POST", srv.URL+"/api/bindings",
		`{"accountName":"小号","roomId":"111"}`)
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusCreated && resp.StatusCode != http.StatusOK {
		t.Fatalf("状态码 = %d", resp.StatusCode)
	}

	if got := probe.snapshot(); len(got) != 0 {
		t.Errorf("已停用的绑定不该被重新探测，ProbeNow 调用记录 = %v", got)
	}
}

// 新增绑定后必须立刻尝试起运行时，否则用户添加账号+直播间之后什么都
// 不会发生——这正是 P5-1 要修的真机故障：webui 加完绑定，日志一条不动，
// 只有手工重启进程才连得上。
func TestCreateBindingStartsRuntime(t *testing.T) {
	srv, st, api := newTestServerWithAPI(t)
	lc := &fakeLifecycle{}
	api.SetBindingLifecycle(lc)
	c := loginAs(t, srv, st, "张三", false)
	mustBindingFor(t, st, "张三", "小号", "111")

	resp := jsonRequest(t, c, "POST", srv.URL+"/api/bindings",
		`{"accountName":"小号","roomId":"222"}`)
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusCreated {
		t.Fatalf("状态码 = %d, 期望 201", resp.StatusCode)
	}

	b, err := st.GetBinding(context.Background(), "小号", "222")
	if err != nil {
		t.Fatalf("查绑定报错: %v", err)
	}
	started, _ := lc.snapshot()
	if len(started) != 1 || started[0] != b.ID {
		t.Errorf("StartBinding 调用记录 = %v, 期望恰好 [%d]", started, b.ID)
	}
}

// 重复创建（幂等分支）落在一个已被用户手动停用的绑定上时，不该把它
// 悄悄重新拉起——UpsertBinding 的注释就写了「不改动 enabled」，
// StartBinding 同理不该在这条路径上被调用。
func TestCreateBindingIdempotentOnDisabledBindingDoesNotStartRuntime(t *testing.T) {
	srv, st, api := newTestServerWithAPI(t)
	lc := &fakeLifecycle{}
	api.SetBindingLifecycle(lc)
	c := loginAs(t, srv, st, "张三", false)
	mustBindingFor(t, st, "张三", "小号", "111")

	if err := st.SetBindingEnabled(context.Background(), "小号", "111", false); err != nil {
		t.Fatalf("停用报错: %v", err)
	}

	resp := jsonRequest(t, c, "POST", srv.URL+"/api/bindings",
		`{"accountName":"小号","roomId":"111"}`)
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusCreated && resp.StatusCode != http.StatusOK {
		t.Fatalf("状态码 = %d", resp.StatusCode)
	}

	started, _ := lc.snapshot()
	if len(started) != 0 {
		t.Errorf("已停用的绑定不该被重新拉起，StartBinding 调用记录 = %v", started)
	}
}

// 启用一个绑定（PATCH enabled:true）要让它在运行期真的连上。
func TestPatchBindingEnableStartsRuntime(t *testing.T) {
	srv, st, api := newTestServerWithAPI(t)
	lc := &fakeLifecycle{}
	api.SetBindingLifecycle(lc)
	c := loginAs(t, srv, st, "张三", false)
	bid := mustBindingFor(t, st, "张三", "小号", "123")
	if err := st.SetBindingEnabled(context.Background(), "小号", "123", false); err != nil {
		t.Fatalf("停用报错: %v", err)
	}

	resp := jsonRequest(t, c, "PATCH", srv.URL+"/api/bindings/"+itoa(bid), `{"enabled":true}`)
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("状态码 = %d", resp.StatusCode)
	}

	started, stopped := lc.snapshot()
	if len(started) != 1 || started[0] != bid {
		t.Errorf("StartBinding 调用记录 = %v, 期望恰好 [%d]", started, bid)
	}
	if len(stopped) != 0 {
		t.Errorf("启用时不该调用 StopBinding，实际 = %v", stopped)
	}
}

// 停用一个绑定（PATCH enabled:false）要让它在运行期真的断开——这正是
// 「拆除」这一半，P4-4 踩过的坑同样适用：不摘干净就是悬挂的连接/
// goroutine/定时任务。
func TestPatchBindingDisableStopsRuntime(t *testing.T) {
	srv, st, api := newTestServerWithAPI(t)
	lc := &fakeLifecycle{}
	api.SetBindingLifecycle(lc)
	c := loginAs(t, srv, st, "张三", false)
	bid := mustBindingFor(t, st, "张三", "小号", "123")

	resp := jsonRequest(t, c, "PATCH", srv.URL+"/api/bindings/"+itoa(bid), `{"enabled":false}`)
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("状态码 = %d", resp.StatusCode)
	}

	started, stopped := lc.snapshot()
	if len(stopped) != 1 || stopped[0] != bid {
		t.Errorf("StopBinding 调用记录 = %v, 期望恰好 [%d]", stopped, bid)
	}
	if len(started) != 0 {
		t.Errorf("停用时不该调用 StartBinding，实际 = %v", started)
	}
}

// 删库成功之后必须紧接着拆运行时——不拆的话，运行时会继续悬挂着一个
// 数据库里已经查不到的绑定：定时任务、连接、goroutine 全部悬空，且
// 再也没有任何 API 路径能摸到它、清理它。
func TestDeleteBindingStopsRuntime(t *testing.T) {
	srv, st, api := newTestServerWithAPI(t)
	lc := &fakeLifecycle{}
	api.SetBindingLifecycle(lc)
	c := loginAs(t, srv, st, "张三", false)
	bid := mustBindingFor(t, st, "张三", "小号", "123")

	resp := jsonRequest(t, c, "DELETE", srv.URL+"/api/bindings/"+itoa(bid), "")
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("状态码 = %d", resp.StatusCode)
	}

	_, stopped := lc.snapshot()
	if len(stopped) != 1 || stopped[0] != bid {
		t.Errorf("StopBinding 调用记录 = %v, 期望恰好 [%d]", stopped, bid)
	}
}

// 没有权限的删除请求不该碰运行时——校验失败必须在调用 StopBinding
// 之前短路，否则枚举攻击者的每一次尝试都会白白拆一次别人的运行时。
func TestDeleteBindingForbiddenDoesNotStopRuntime(t *testing.T) {
	srv, st, api := newTestServerWithAPI(t)
	lc := &fakeLifecycle{}
	api.SetBindingLifecycle(lc)
	loginAs(t, srv, st, "张三", false)
	bid := mustBindingFor(t, st, "张三", "小号", "123")

	li := loginAs(t, srv, st, "李四", false)
	if err := st.Grant(context.Background(), "李四", "小号", "123",
		[]perm.Permission{perm.RuleWrite}); err != nil {
		t.Fatalf("授权报错: %v", err)
	}

	resp := jsonRequest(t, li, "DELETE", srv.URL+"/api/bindings/"+itoa(bid), "")
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusForbidden {
		t.Fatalf("状态码 = %d, 期望 403", resp.StatusCode)
	}

	_, stopped := lc.snapshot()
	if len(stopped) != 0 {
		t.Errorf("无权限的删除尝试不该拆运行时，StopBinding 调用记录 = %v", stopped)
	}
}

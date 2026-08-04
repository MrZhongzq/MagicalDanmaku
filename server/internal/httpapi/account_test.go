package httpapi_test

import (
	"context"
	"encoding/json"
	"net/http"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/MrZhongzq/MagicalDanmaku/server/internal/connector/bilibili/auth"
	"github.com/MrZhongzq/MagicalDanmaku/server/internal/perm"
	"github.com/MrZhongzq/MagicalDanmaku/server/internal/store"
)

// fakeQR 是可控的扫码实现，避免测试打真实 B 站接口。
type fakeQR struct {
	key    string
	url    string
	result auth.PollResult
	err    error
}

func (f *fakeQR) Generate(context.Context) (*auth.QRCode, error) {
	if f.err != nil {
		return nil, f.err
	}
	return &auth.QRCode{Key: f.key, URL: f.url}, nil
}

func (f *fakeQR) Poll(_ context.Context, key string) (*auth.PollResult, error) {
	if f.err != nil {
		return nil, f.err
	}
	if key != f.key {
		return &auth.PollResult{Status: auth.PollExpired}, nil
	}
	r := f.result
	return &r, nil
}

// 账号列表绝不能带 Cookie
func TestListAccountsNeverLeaksCookie(t *testing.T) {
	srv, st := newTestServer(t)
	c := loginAs(t, srv, st, "张三", false)
	mustBindingFor(t, st, "张三", "小号", "123")

	resp := jsonRequest(t, c, "GET", srv.URL+"/api/accounts", "")
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("状态码 = %d", resp.StatusCode)
	}

	var raw json.RawMessage
	if err := json.NewDecoder(resp.Body).Decode(&raw); err != nil {
		t.Fatalf("解析报错: %v", err)
	}
	// 直接在原始 JSON 里找，比逐字段断言更难漏
	if bytesContains(raw, "SESSDATA") || bytesContains(raw, "cookie") {
		t.Errorf("账号列表泄漏了 Cookie: %s", raw)
	}
}

func bytesContains(b []byte, sub string) bool {
	return len(b) >= len(sub) && stringIndex(string(b), sub) >= 0
}

func stringIndex(s, sub string) int {
	for i := 0; i+len(sub) <= len(s); i++ {
		if s[i:i+len(sub)] == sub {
			return i
		}
	}
	return -1
}

// 账号卡片要能显示登录态，否则「待后端支持」的提示永远撤不掉。
func TestListAccountsIncludesLoginState(t *testing.T) {
	srv, st := newTestServer(t)
	c := loginAs(t, srv, st, "张三", false)
	mustBindingFor(t, st, "张三", "小号", "123")

	if err := st.UpdateAccountLoginState(context.Background(), "小号", store.LoginStateInvalid); err != nil {
		t.Fatalf("写入登录态报错: %v", err)
	}

	resp := jsonRequest(t, c, "GET", srv.URL+"/api/accounts", "")
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("状态码 = %d", resp.StatusCode)
	}

	var got []struct {
		Name           string  `json:"name"`
		LoginState     string  `json:"loginState"`
		LoginCheckedAt *string `json:"loginCheckedAt"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&got); err != nil {
		t.Fatalf("解析报错: %v", err)
	}
	if len(got) != 1 {
		t.Fatalf("账号数 = %d, 期望 1", len(got))
	}
	if got[0].LoginState != "invalid" {
		t.Errorf("loginState = %q, 期望 invalid", got[0].LoginState)
	}
	if got[0].LoginCheckedAt == nil || *got[0].LoginCheckedAt == "" {
		t.Errorf("loginCheckedAt 应有值，实际 %v", got[0].LoginCheckedAt)
	}
}

// 尚未检测过的账号应报告「未知」，而不是缺省成「有效」——那会让用户
// 误以为一个从未探测过的账号是安全的。
func TestListAccountsReportsUnknownLoginStateBeforeFirstCheck(t *testing.T) {
	srv, st := newTestServer(t)
	c := loginAs(t, srv, st, "张三", false)
	mustBindingFor(t, st, "张三", "小号", "123")

	resp := jsonRequest(t, c, "GET", srv.URL+"/api/accounts", "")
	defer resp.Body.Close()

	var got []struct {
		LoginState     string  `json:"loginState"`
		LoginCheckedAt *string `json:"loginCheckedAt"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&got); err != nil {
		t.Fatalf("解析报错: %v", err)
	}
	if len(got) != 1 || got[0].LoginState != "unknown" {
		t.Errorf("未检测过的账号 loginState = %+v, 期望 unknown", got)
	}
	if got[0].LoginCheckedAt != nil {
		t.Errorf("未检测过时 loginCheckedAt 应为 null，实际 %v", *got[0].LoginCheckedAt)
	}
}

func TestListAccountsFiltersByVisibility(t *testing.T) {
	srv, st := newTestServer(t)
	loginAs(t, srv, st, "张三", false)
	loginAs(t, srv, st, "王五", false)
	mustBindingFor(t, st, "张三", "张三的号", "111")
	mustBindingFor(t, st, "王五", "王五的号", "222")

	li := loginAs(t, srv, st, "李四", false)
	if err := st.Grant(context.Background(), "李四", "张三的号", "111",
		[]perm.Permission{perm.EventRead}); err != nil {
		t.Fatalf("授权报错: %v", err)
	}

	resp := jsonRequest(t, li, "GET", srv.URL+"/api/accounts", "")
	defer resp.Body.Close()

	var got []struct {
		Name string `json:"name"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&got); err != nil {
		t.Fatalf("解析报错: %v", err)
	}
	if len(got) != 1 || got[0].Name != "张三的号" {
		t.Errorf("李四应只看到「张三的号」，实际 %+v", got)
	}
}

func TestQRCodeStartReturnsURL(t *testing.T) {
	srv, st, api := newTestServerWithAPI(t)
	api.SetQRLogin(&fakeQR{key: "KEY123", url: "https://qr.example/abc"})
	c := loginAs(t, srv, st, "张三", false)

	resp := jsonRequest(t, c, "POST", srv.URL+"/api/accounts/qrcode", `{"name":"小号"}`)
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("状态码 = %d", resp.StatusCode)
	}

	var body struct {
		Key string `json:"key"`
		URL string `json:"url"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&body); err != nil {
		t.Fatalf("解析报错: %v", err)
	}
	if body.Key != "KEY123" || body.URL != "https://qr.example/abc" {
		t.Errorf("响应 = %+v", body)
	}
}

func TestQRCodeStartRequiresName(t *testing.T) {
	srv, st, api := newTestServerWithAPI(t)
	api.SetQRLogin(&fakeQR{key: "K", url: "u"})
	c := loginAs(t, srv, st, "张三", false)

	resp := jsonRequest(t, c, "POST", srv.URL+"/api/accounts/qrcode", `{"name":"  "}`)
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusUnprocessableEntity {
		t.Errorf("空账号名状态码 = %d, 期望 422", resp.StatusCode)
	}
}

// 对别人已占用的账号名发起扫码，必须返回 404 而不是 403。
//
// 403 加上「不属于你」的文案等于确认了这个名字存在，任何登录用户
// 拿任意名字试一次就能探测出部署里有哪些账号。
func TestQRCodeStartOnOthersAccountLooksLikeNotFound(t *testing.T) {
	srv, st, api := newTestServerWithAPI(t)
	api.SetQRLogin(&fakeQR{key: "K", url: "u"})

	loginAs(t, srv, st, "王五", false)
	mustBindingFor(t, st, "王五", "王五的号", "111")

	li := loginAs(t, srv, st, "李四", false)
	resp := jsonRequest(t, li, "POST", srv.URL+"/api/accounts/qrcode", `{"name":"王五的号"}`)
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusNotFound {
		t.Errorf("状态码 = %d, 期望 404（403 会泄漏该账号名已被占用）", resp.StatusCode)
	}

	var body map[string]string
	_ = json.NewDecoder(resp.Body).Decode(&body)
	if strings.Contains(body["error"], "不属于") {
		t.Errorf("错误文案泄漏了归属信息: %q", body["error"])
	}

	// 拦截逻辑本身必须仍然生效：别人的账号没被动过
	acc, err := st.GetAccountByName(context.Background(), "王五的号")
	if err != nil {
		t.Fatalf("账号应仍存在: %v", err)
	}
	if acc.OwnerID == 0 {
		t.Error("账号归属不该被改动")
	}
}

func TestQRCodePollWaiting(t *testing.T) {
	srv, st, api := newTestServerWithAPI(t)
	api.SetQRLogin(&fakeQR{key: "K", url: "u", result: auth.PollResult{Status: auth.PollWaiting}})
	c := loginAs(t, srv, st, "张三", false)

	start := jsonRequest(t, c, "POST", srv.URL+"/api/accounts/qrcode", `{"name":"小号"}`)
	start.Body.Close()

	resp := jsonRequest(t, c, "POST", srv.URL+"/api/accounts/qrcode/K", "")
	defer resp.Body.Close()

	var body struct {
		Status string `json:"status"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&body); err != nil {
		t.Fatalf("解析报错: %v", err)
	}
	if body.Status != "waiting" {
		t.Errorf("status = %q, 期望 waiting", body.Status)
	}
}

// 扫码成功时账号在服务端建好，Cookie 绝不回传
func TestQRCodePollSuccessCreatesAccount(t *testing.T) {
	srv, st, api := newTestServerWithAPI(t)
	cookie := "SESSDATA=abc; bili_jct=def; DedeUserID=10086"
	api.SetQRLogin(&fakeQR{key: "K", url: "u",
		result: auth.PollResult{Status: auth.PollSuccess, Cookie: cookie}})
	c := loginAs(t, srv, st, "张三", false)

	start := jsonRequest(t, c, "POST", srv.URL+"/api/accounts/qrcode", `{"name":"小号"}`)
	start.Body.Close()

	resp := jsonRequest(t, c, "POST", srv.URL+"/api/accounts/qrcode/K", "")
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("状态码 = %d", resp.StatusCode)
	}

	var raw json.RawMessage
	if err := json.NewDecoder(resp.Body).Decode(&raw); err != nil {
		t.Fatalf("解析报错: %v", err)
	}
	if bytesContains(raw, "SESSDATA") {
		t.Errorf("扫码响应泄漏了 Cookie: %s", raw)
	}

	acc, err := st.GetAccountByName(context.Background(), "小号")
	if err != nil {
		t.Fatalf("账号应已建好: %v", err)
	}
	if acc.Cookie != cookie {
		t.Errorf("库里的 Cookie = %q", acc.Cookie)
	}
	if acc.UID != "10086" {
		t.Errorf("UID = %q, 期望 10086（应从 Cookie 解析）", acc.UID)
	}
}

// 已存在的账号走换 Cookie 而不是重复建号
func TestQRCodePollSuccessUpdatesExisting(t *testing.T) {
	srv, st, api := newTestServerWithAPI(t)
	c := loginAs(t, srv, st, "张三", false)
	mustBindingFor(t, st, "张三", "小号", "123")

	newCookie := "SESSDATA=new; bili_jct=x; DedeUserID=999"
	api.SetQRLogin(&fakeQR{key: "K", url: "u",
		result: auth.PollResult{Status: auth.PollSuccess, Cookie: newCookie}})

	start := jsonRequest(t, c, "POST", srv.URL+"/api/accounts/qrcode", `{"name":"小号"}`)
	start.Body.Close()
	resp := jsonRequest(t, c, "POST", srv.URL+"/api/accounts/qrcode/K", "")
	resp.Body.Close()

	acc, err := st.GetAccountByName(context.Background(), "小号")
	if err != nil {
		t.Fatalf("查账号报错: %v", err)
	}
	if acc.Cookie != newCookie {
		t.Errorf("Cookie 应被更新，实际 %q", acc.Cookie)
	}

	all, err := st.ListAccounts(context.Background())
	if err != nil {
		t.Fatalf("列账号报错: %v", err)
	}
	if len(all) != 1 {
		t.Errorf("不该重复建号，实际 %d 个账号", len(all))
	}
}

// 别人发起的扫码，我拿着 key 也换不出账号
func TestQRCodePollRejectsOtherUsersKey(t *testing.T) {
	srv, st, api := newTestServerWithAPI(t)
	api.SetQRLogin(&fakeQR{key: "K", url: "u",
		result: auth.PollResult{Status: auth.PollSuccess, Cookie: "SESSDATA=x; DedeUserID=1"}})

	zhang := loginAs(t, srv, st, "张三", false)
	start := jsonRequest(t, zhang, "POST", srv.URL+"/api/accounts/qrcode", `{"name":"小号"}`)
	start.Body.Close()

	li := loginAs(t, srv, st, "李四", false)
	resp := jsonRequest(t, li, "POST", srv.URL+"/api/accounts/qrcode/K", "")
	defer resp.Body.Close()
	// handleQRCodePoll 里 pending.UserID != u.ID 时无条件回 404
	// （与「扫码会话不存在或已过期」同一句文案），不存在 403 分支。
	if resp.StatusCode != http.StatusNotFound {
		t.Errorf("别人的扫码会话状态码 = %d, 期望 404", resp.StatusCode)
	}
	if _, err := st.GetAccountByName(context.Background(), "小号"); err == nil {
		t.Error("不该替别人把账号建出来")
	}
}

// 扫码发起后、落库前，账号被别人抢先建出来——不能把 Cookie 写进去。
//
// handleQRCodeStart 看到的是至多 3 分钟前的状态。张三发起扫码时「小号」
// 还不存在，Start 检查放行；但 3 分钟内王五抢先建出了同名账号（现实中
// 是撞了一个可猜的名字）。若 saveScannedAccount 落库前不重查归属，
// 张三完成扫码就会把自己的 B 站 Cookie 换进王五的账号行——owner_id
// 不变，王五的机器人从此以张三的身份发言。
func TestQRCodePollRefusesAccountCreatedByOthersMeanwhile(t *testing.T) {
	srv, st, api := newTestServerWithAPI(t)
	attackerCookie := "SESSDATA=attacker; bili_jct=x; DedeUserID=666"
	api.SetQRLogin(&fakeQR{key: "K", url: "u",
		result: auth.PollResult{Status: auth.PollSuccess, Cookie: attackerCookie}})

	// 张三对尚不存在的账号名「小号」发起扫码——Start 检查放行
	zhang := loginAs(t, srv, st, "张三", false)
	start := jsonRequest(t, zhang, "POST", srv.URL+"/api/accounts/qrcode", `{"name":"小号"}`)
	start.Body.Close()

	// 王五在这期间抢先把「小号」建了出来
	loginAs(t, srv, st, "王五", false)
	wangwu, err := st.GetUserByName(context.Background(), "王五")
	if err != nil {
		t.Fatalf("查用户报错: %v", err)
	}
	victimCookie := "SESSDATA=victim; bili_jct=y; DedeUserID=999"
	if _, err := st.CreateAccount(context.Background(), store.AccountInput{
		Name: "小号", UID: "999", Cookie: victimCookie, OwnerID: wangwu.ID,
		RateLimit: time.Second, MaxLength: 40,
	}); err != nil {
		t.Fatalf("建账号报错: %v", err)
	}

	// 张三完成扫码——必须失败，且响应体不能泄漏「这个账号归王五」
	resp := jsonRequest(t, zhang, "POST", srv.URL+"/api/accounts/qrcode/K", "")
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusNotFound {
		t.Errorf("状态码 = %d, 期望 404（与 handleQRCodeStart 的 404 文案一致）", resp.StatusCode)
	}
	var body map[string]string
	_ = json.NewDecoder(resp.Body).Decode(&body)
	if strings.Contains(body["error"], "不属于") || strings.Contains(body["error"], "王五") {
		t.Errorf("错误文案泄漏了归属信息: %q", body["error"])
	}

	acc, err := st.GetAccountByName(context.Background(), "小号")
	if err != nil {
		t.Fatalf("账号应仍存在: %v", err)
	}
	if acc.OwnerID != wangwu.ID {
		t.Errorf("账号归属被改动: OwnerID = %d, 期望 %d（王五）", acc.OwnerID, wangwu.ID)
	}
	// 核心断言：Cookie 一个字节都不该变——这是唯一能钉住
	// 「Cookie 没被换掉」的断言，只查 OwnerID 不够。
	if acc.Cookie != victimCookie {
		t.Errorf("Cookie 被换成了攻击者的: %q, 期望仍是王五的原值 %q", acc.Cookie, victimCookie)
	}
}

// fakeLoginProbe 记录被探测的账号名，验证 handler 在扫码成功之后是否
// 正确调用了注入的 LoginProbe——不必真的打 B 站接口（那是 cmd/magicd
// 里 checkAccountLogin 自己的测试范围），这里只关心 handler 有没有在
// 正确的时机（且只在扫码成功时）调用它。
type fakeLoginProbe struct {
	mu     sync.Mutex
	probed []string
}

func (f *fakeLoginProbe) ProbeNow(_ context.Context, accountName string) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.probed = append(f.probed, accountName)
}

func (f *fakeLoginProbe) snapshot() []string {
	f.mu.Lock()
	defer f.mu.Unlock()
	return append([]string{}, f.probed...)
}

// TestQRCodePollSuccessProbesLoginImmediately 钉住 P5-2 任务 2 的第一条：
// 扫码成功后必须立即探测一次登录态，不能让用户等后台 10 分钟一轮的
// 检测循环才发现扫码没有真的成功。
func TestQRCodePollSuccessProbesLoginImmediately(t *testing.T) {
	srv, st, api := newTestServerWithAPI(t)
	cookie := "SESSDATA=abc; bili_jct=def; DedeUserID=10086"
	api.SetQRLogin(&fakeQR{key: "K", url: "u",
		result: auth.PollResult{Status: auth.PollSuccess, Cookie: cookie}})
	probe := &fakeLoginProbe{}
	api.SetLoginProbe(probe)
	c := loginAs(t, srv, st, "张三", false)

	start := jsonRequest(t, c, "POST", srv.URL+"/api/accounts/qrcode", `{"name":"小号"}`)
	start.Body.Close()
	resp := jsonRequest(t, c, "POST", srv.URL+"/api/accounts/qrcode/K", "")
	resp.Body.Close()

	if got := probe.snapshot(); len(got) != 1 || got[0] != "小号" {
		t.Errorf("ProbeNow 调用记录 = %v, 期望恰好 [\"小号\"]", got)
	}
}

// TestQRCodePollWaitingDoesNotProbeLogin 验证还没扫完（状态不是
// success）时不该探测——账号这时候可能压根还没建好/换好 Cookie。
func TestQRCodePollWaitingDoesNotProbeLogin(t *testing.T) {
	srv, st, api := newTestServerWithAPI(t)
	api.SetQRLogin(&fakeQR{key: "K", url: "u", result: auth.PollResult{Status: auth.PollWaiting}})
	probe := &fakeLoginProbe{}
	api.SetLoginProbe(probe)
	c := loginAs(t, srv, st, "张三", false)

	start := jsonRequest(t, c, "POST", srv.URL+"/api/accounts/qrcode", `{"name":"小号"}`)
	start.Body.Close()
	resp := jsonRequest(t, c, "POST", srv.URL+"/api/accounts/qrcode/K", "")
	resp.Body.Close()

	if got := probe.snapshot(); len(got) != 0 {
		t.Errorf("等待扫码阶段不该触发探测，实际调用记录 = %v", got)
	}
}

func TestQRCodePollUnknownKey(t *testing.T) {
	srv, st, api := newTestServerWithAPI(t)
	api.SetQRLogin(&fakeQR{key: "K", url: "u"})
	c := loginAs(t, srv, st, "张三", false)

	resp := jsonRequest(t, c, "POST", srv.URL+"/api/accounts/qrcode/没这个KEY", "")
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusNotFound {
		t.Errorf("状态码 = %d, 期望 404", resp.StatusCode)
	}
}

// 改账号参数要求账号所有权，不存在能授出去的权限点。
//
// 曾经有过一个 account:manage 权限点，已删除：它是绑定级的，而账号
// 设置是账号级的，在绑定 A 上授予就能改到同账号下绑定 B 的行为。
// 所以这条路径只认 isAccountOwner（所有者或管理员）。
func TestPatchAccountRequiresOwnership(t *testing.T) {
	srv, st := newTestServer(t)
	loginAs(t, srv, st, "张三", false)
	mustBindingFor(t, st, "张三", "小号", "123")

	li := loginAs(t, srv, st, "李四", false)
	// 先给李四一条该绑定上的授权，证明「有绑定级权限也改不了账号」——
	// 不给的话这条测试和「李四什么都没有」区分不开
	if err := st.Grant(context.Background(), "李四", "小号", "123",
		[]perm.Permission{perm.RuleWrite, perm.MemberManage}); err != nil {
		t.Fatalf("授权报错: %v", err)
	}

	resp := jsonRequest(t, li, "PATCH", srv.URL+"/api/accounts/小号", `{"rateLimitMs":2000}`)
	defer resp.Body.Close()
	// handlePatchAccount 里 !isAccountOwner(u, acc) 时无条件回 404
	// （「不是所有者就当作不存在」），不存在 403 分支。
	if resp.StatusCode != http.StatusNotFound {
		t.Errorf("非所有者状态码 = %d, 期望 404", resp.StatusCode)
	}

	// 参数一个字节都不该变
	acc, err := st.GetAccountByName(context.Background(), "小号")
	if err != nil {
		t.Fatalf("查账号报错: %v", err)
	}
	if acc.RateLimit == 2000*time.Millisecond {
		t.Error("非所有者不该改得动限流参数")
	}
}

func TestPatchAccountByOwner(t *testing.T) {
	srv, st := newTestServer(t)
	c := loginAs(t, srv, st, "张三", false)
	mustBindingFor(t, st, "张三", "小号", "123")

	resp := jsonRequest(t, c, "PATCH", srv.URL+"/api/accounts/小号",
		`{"rateLimitMs":2500,"maxLength":30}`)
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("状态码 = %d", resp.StatusCode)
	}

	acc, err := st.GetAccountByName(context.Background(), "小号")
	if err != nil {
		t.Fatalf("查账号报错: %v", err)
	}
	if acc.RateLimit != 2500*time.Millisecond || acc.MaxLength != 30 {
		t.Errorf("账号参数 = %v / %d", acc.RateLimit, acc.MaxLength)
	}
}

// fakeAccountRuntimeUpdater 记录被通知过的账号名，验证 handler 保存成功
// 之后有没有把"运行时该跟着变"这件事交给注入的实现——真正的热传播逻辑
// （改限流器、改字数上限）是 cmd/magicd 的 runtimeManager 自己的测试
// 范围（runtime_manager_test.go），这里只关心 handler 有没有在正确的
// 时机、用正确的账号名调用它。
type fakeAccountRuntimeUpdater struct {
	mu       sync.Mutex
	notified []string
}

func (f *fakeAccountRuntimeUpdater) UpdateAccountRuntime(_ context.Context, accountName string) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.notified = append(f.notified, accountName)
}

func (f *fakeAccountRuntimeUpdater) snapshot() []string {
	f.mu.Lock()
	defer f.mu.Unlock()
	return append([]string{}, f.notified...)
}

// TestPatchAccountNotifiesRuntimeUpdater 是 P6 任务 3 的修复：账号的
// 发送间隔/字数上限改了不会热传播到运行中的绑定（SetMaxLength 只在
// 装配时调一次，要重启才生效）。handlePatchAccount 保存成功后必须调用
// 注入的 AccountRuntimeUpdater，把"这个账号的运行参数变了"这件事通知
// 出去——具体怎么把新值落到运行中的限流器/Actions 上是 runtimeManager
// 的职责，httpapi 这一层只负责在正确的时机触发。
func TestPatchAccountNotifiesRuntimeUpdater(t *testing.T) {
	srv, st, api := newTestServerWithAPI(t)
	updater := &fakeAccountRuntimeUpdater{}
	api.SetAccountRuntimeUpdater(updater)
	c := loginAs(t, srv, st, "张三", false)
	mustBindingFor(t, st, "张三", "小号", "123")

	resp := jsonRequest(t, c, "PATCH", srv.URL+"/api/accounts/小号",
		`{"rateLimitMs":2500,"maxLength":30}`)
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("状态码 = %d", resp.StatusCode)
	}

	got := updater.snapshot()
	if len(got) != 1 || got[0] != "小号" {
		t.Errorf("UpdateAccountRuntime 调用记录 = %v, 期望恰好 [\"小号\"]", got)
	}
}

// TestPatchAccountFailureDoesNotNotifyRuntimeUpdater 是上一条的反面：
// 校验失败（没有真的改动数据库）不该触发运行时同步——不然会白白让
// runtimeManager 去重新读一份跟改之前完全一样的配置。
func TestPatchAccountFailureDoesNotNotifyRuntimeUpdater(t *testing.T) {
	srv, st, api := newTestServerWithAPI(t)
	updater := &fakeAccountRuntimeUpdater{}
	api.SetAccountRuntimeUpdater(updater)
	c := loginAs(t, srv, st, "张三", false)
	mustBindingFor(t, st, "张三", "小号", "123")

	resp := jsonRequest(t, c, "PATCH", srv.URL+"/api/accounts/小号", `{"maxLength":999}`)
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusUnprocessableEntity {
		t.Fatalf("状态码 = %d, 期望 422", resp.StatusCode)
	}

	if got := updater.snapshot(); len(got) != 0 {
		t.Errorf("校验失败时不该通知运行时同步，实际调用记录 = %v", got)
	}
}

func TestDeleteAccountByOwner(t *testing.T) {
	srv, st := newTestServer(t)
	c := loginAs(t, srv, st, "张三", false)
	mustBindingFor(t, st, "张三", "小号", "123")

	resp := jsonRequest(t, c, "DELETE", srv.URL+"/api/accounts/小号", "")
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("状态码 = %d", resp.StatusCode)
	}
	if _, err := st.GetAccountByName(context.Background(), "小号"); err == nil {
		t.Error("删除后应查不到")
	}
}

// 删账号会级联删掉它名下全部绑定（bindings.account_id 是 ON DELETE
// CASCADE），但那只是数据库层面的事——运行时（P5-1 之后可以动态跑着）
// 不会跟着自动消失。不主动摘的话，这些绑定的连接/goroutine/定时任务会
// 变成永远没有任何 API 路径能摸到、也没人会去清理的悬挂资源，直到进程
// 重启。删账号前把它名下每个绑定都过一遍 StopBinding，堵住这个口子。
func TestDeleteAccountStopsRuntimeForAllItsBindings(t *testing.T) {
	srv, st, api := newTestServerWithAPI(t)
	lc := &fakeLifecycle{}
	api.SetBindingLifecycle(lc)
	c := loginAs(t, srv, st, "张三", false)
	bid1 := mustBindingFor(t, st, "张三", "小号", "111")

	acc, err := st.GetAccountByName(context.Background(), "小号")
	if err != nil {
		t.Fatalf("查账号报错: %v", err)
	}
	b2, err := st.UpsertBinding(context.Background(), acc.ID, "222")
	if err != nil {
		t.Fatalf("建第二个绑定报错: %v", err)
	}

	resp := jsonRequest(t, c, "DELETE", srv.URL+"/api/accounts/小号", "")
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("状态码 = %d", resp.StatusCode)
	}

	_, stopped := lc.snapshot()
	got := map[int64]bool{}
	for _, id := range stopped {
		got[id] = true
	}
	if !got[bid1] || !got[b2.ID] {
		t.Errorf("StopBinding 调用记录 = %v, 期望包含该账号名下两个绑定 [%d %d]", stopped, bid1, b2.ID)
	}
}

func TestDeleteAccountByStranger(t *testing.T) {
	srv, st := newTestServer(t)
	loginAs(t, srv, st, "张三", false)
	mustBindingFor(t, st, "张三", "小号", "123")

	li := loginAs(t, srv, st, "李四", false)
	resp := jsonRequest(t, li, "DELETE", srv.URL+"/api/accounts/小号", "")
	defer resp.Body.Close()
	if resp.StatusCode == http.StatusOK {
		t.Error("非所有者不该能删账号")
	}
	if _, err := st.GetAccountByName(context.Background(), "小号"); err != nil {
		t.Error("账号不该被删掉")
	}
}

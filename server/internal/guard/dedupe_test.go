package guard

import (
	"testing"
	"time"
)

var t0 = time.Date(2026, 8, 10, 7, 18, 23, 0, time.UTC)

const (
	room = "1934306767"
	uid  = "10086"
)

func TestSecondBroadcastOfSameBuyIsDropped(t *testing.T) {
	d := NewDedupe(10 * time.Second)

	// B 站对同一次上舰会先后下发 GUARD_BUY 与 USER_TOAST_MSG。真机数据里
	// 两条相隔 0.3~0.6 秒，内容除了来源以外完全一样。
	if !d.Accept(room, uid, 3, t0) {
		t.Fatal("第一条应当放行")
	}
	if d.Accept(room, uid, 3, t0.Add(400*time.Millisecond)) {
		t.Error("同一次上舰的第二次广播应当被丢掉，否则会答谢两次")
	}
}

func TestBuyAfterWindowIsAcceptedAgain(t *testing.T) {
	d := NewDedupe(10 * time.Second)

	if !d.Accept(room, uid, 3, t0) {
		t.Fatal("第一条应当放行")
	}
	// 窗口之外是另一次真实购买，不能吞。
	if !d.Accept(room, uid, 3, t0.Add(11*time.Second)) {
		t.Error("窗口外的再次上舰是另一件事，应当放行")
	}
}

func TestDifferentUsersDoNotBlockEachOther(t *testing.T) {
	d := NewDedupe(10 * time.Second)

	if !d.Accept(room, "甲", 3, t0) {
		t.Fatal("甲应当放行")
	}
	if !d.Accept(room, "乙", 3, t0.Add(100*time.Millisecond)) {
		t.Error("两个人几乎同时上舰是两件事，不能互相顶掉")
	}
}

func TestUpgradeToHigherGuardLevelIsNotDeduped(t *testing.T) {
	d := NewDedupe(10 * time.Second)

	if !d.Accept(room, uid, 3, t0) {
		t.Fatal("舰长应当放行")
	}
	// 同一个人紧接着买提督是另一笔，等级不同就不是同一次广播。
	if !d.Accept(room, uid, 2, t0.Add(time.Second)) {
		t.Error("不同大航海等级是不同的购买，不该被去重吞掉")
	}
}

func TestSameUserInDifferentRoomsIsNotDeduped(t *testing.T) {
	d := NewDedupe(10 * time.Second)

	if !d.Accept("甲房", uid, 3, t0) {
		t.Fatal("甲房应当放行")
	}
	if !d.Accept("乙房", uid, 3, t0.Add(time.Second)) {
		t.Error("同一个人在两个直播间各上一次舰是两件事")
	}
}

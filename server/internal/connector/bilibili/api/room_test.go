package api

import (
	"context"
	"net/http"
	"testing"
)

func TestRoomInfo(t *testing.T) {
	c, srv := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		if got := r.URL.Query().Get("room_id"); got != "21452505" {
			t.Errorf("room_id = %q", got)
		}
		w.Write([]byte(`{"code":0,"data":{
			"room_id":21452505,"short_id":123,"uid":20285041,
			"title":"今天也在唱歌","live_status":1,
			"area_id":21,"area_name":"视频唱见",
			"parent_area_id":1,"parent_area_name":"娱乐",
			"attention":12345,"online":678,
			"live_time":"2026-07-29 19:00:00"
		}}`))
	})
	c.SetBaseURL("roomInfo", srv.URL)

	info, err := c.RoomInfo(context.Background(), "21452505")
	if err != nil {
		t.Fatalf("RoomInfo 失败: %v", err)
	}
	if info.RoomID != "21452505" {
		t.Errorf("RoomID = %q", info.RoomID)
	}
	if info.UID != "20285041" {
		t.Errorf("UID = %q", info.UID)
	}
	if info.Title != "今天也在唱歌" {
		t.Errorf("Title = %q", info.Title)
	}
	if info.LiveStatus != 1 {
		t.Errorf("LiveStatus = %d", info.LiveStatus)
	}
	if !info.IsLiving() {
		t.Error("live_status=1 时 IsLiving 应为 true")
	}
	if info.AreaName != "视频唱见" || info.ParentAreaName != "娱乐" {
		t.Errorf("分区 = %q/%q", info.ParentAreaName, info.AreaName)
	}
	if info.Attention != 12345 {
		t.Errorf("Attention = %d", info.Attention)
	}
}

func TestDanmuInfo(t *testing.T) {
	var sawSignature bool
	c, srv := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		sawSignature = r.URL.Query().Get("w_rid") != ""
		w.Write([]byte(`{"code":0,"data":{
			"token":"tok-abc",
			"host_list":[
				{"host":"broadcastlv.chat.bilibili.com","port":2243,"wss_port":443,"ws_port":2244},
				{"host":"hw-bj-live-comet-01.chat.bilibili.com","port":2243,"wss_port":443,"ws_port":2244}
			]
		}}`))
	})
	c.SetBaseURL("danmuInfo", srv.URL)
	c.Signer().SetMixinKey("abcdefghijklmnopqrstuvwxyz012345")

	di, err := c.DanmuInfo(context.Background(), "21452505")
	if err != nil {
		t.Fatalf("DanmuInfo 失败: %v", err)
	}
	if !sawSignature {
		t.Error("getDanmuInfo 必须使用 wbi 签名")
	}
	if di.Token != "tok-abc" {
		t.Errorf("Token = %q", di.Token)
	}
	if len(di.Hosts) != 2 {
		t.Fatalf("Hosts 数量 = %d, 期望 2", len(di.Hosts))
	}
	want := "wss://broadcastlv.chat.bilibili.com:443/sub"
	if got := di.Hosts[0].WSSURL(); got != want {
		t.Errorf("WSSURL = %q, 期望 %q", got, want)
	}
}

func TestDanmuInfoRiskControl(t *testing.T) {
	c, srv := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte(`{"code":-352,"message":"风控校验失败"}`))
	})
	c.SetBaseURL("danmuInfo", srv.URL)
	c.Signer().SetMixinKey("abcdefghijklmnopqrstuvwxyz012345")

	_, err := c.DanmuInfo(context.Background(), "21452505")
	if err == nil {
		t.Fatal("应当返回错误")
	}
	if !IsRiskControl(err) {
		t.Errorf("应判定为风控，实际 %v", err)
	}
}

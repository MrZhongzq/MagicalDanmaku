package main

import (
	"strings"
	"testing"
)

func TestLoginSaveRequiresOwnerWhenAccountIsNew(t *testing.T) {
	// 参数校验必须在扫码之前完成：让人扫完码才发现参数错了最气人
	err := runLogin([]string{"--save", "小号", "-o", "x.txt"})
	if err == nil {
		t.Fatal("--save 与 -o 同时给出应报错")
	}
	if !strings.Contains(err.Error(), "-o") {
		t.Errorf("错误信息应说明冲突，实际: %v", err)
	}
}

func TestLoginOwnerWithoutSaveIsRejected(t *testing.T) {
	err := runLogin([]string{"--owner", "admin"})
	if err == nil {
		t.Fatal("只给 --owner 不给 --save 应报错")
	}
	if !strings.Contains(err.Error(), "--save") {
		t.Errorf("错误信息应提到 --save，实际: %v", err)
	}
}

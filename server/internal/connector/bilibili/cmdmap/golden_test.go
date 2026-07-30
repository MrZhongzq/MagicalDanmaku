package cmdmap

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/MrZhongzq/MagicalDanmaku/server/internal/event"
)

const testdataDir = "../../../../testdata/cmds"

// TestGoldenSamplesAllMap 遍历全部黄金样本，确保：
//  1. 每个样本都能被解析且不返回错误
//  2. 已注册的 CMD 不得落入 Unknown 分支
//  3. Raw 必须原样保留
//
// 后续任务只需往 testdata/cmds/ 添加样本文件，本测试自动覆盖。
func TestGoldenSamplesAllMap(t *testing.T) {
	entries, err := os.ReadDir(testdataDir)
	if err != nil {
		t.Fatalf("读取样本目录失败: %v", err)
	}

	var count int
	for _, e := range entries {
		if e.IsDir() || !strings.HasSuffix(e.Name(), ".json") {
			continue
		}
		count++
		name := e.Name()
		t.Run(name, func(t *testing.T) {
			raw, err := os.ReadFile(filepath.Join(testdataDir, name))
			if err != nil {
				t.Fatalf("读取失败: %v", err)
			}
			if !json.Valid(raw) {
				t.Fatalf("样本不是合法 JSON")
			}

			evs, err := Map(testCtx(), raw)
			if err != nil {
				t.Fatalf("Map 返回错误: %v", err)
			}
			if len(evs) == 0 {
				t.Fatal("Map 返回空事件列表，样本应至少产出一个事件")
			}

			// 样本文件名以 CMD 名开头，据此判断是否应被识别。
			cmdName := CommandOf(raw)
			if !strings.HasPrefix(name, cmdName) {
				t.Fatalf("样本文件名 %q 应以 CMD 名 %q 开头", name, cmdName)
			}

			for i, ev := range evs {
				if ev.Raw == nil {
					t.Errorf("第 %d 个事件的 Raw 为 nil", i)
				}
				if ev.ID == "" {
					t.Errorf("第 %d 个事件的 ID 为空", i)
				}
				if ev.RoomID == "" {
					t.Errorf("第 %d 个事件的 RoomID 为空", i)
				}
				if ev.Timestamp.IsZero() {
					t.Errorf("第 %d 个事件的 Timestamp 为零值", i)
				}
				if ev.Type == event.TypeUnknown {
					t.Errorf("第 %d 个事件落入 Unknown，说明 %s 的映射未注册", i, cmdName)
				}
			}
		})
	}

	if count == 0 {
		t.Fatal("样本目录为空")
	}
	t.Logf("已校验 %d 个黄金样本", count)
}

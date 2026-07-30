// Package buildinfo 承载编译期注入的版本信息。
package buildinfo

import (
	"fmt"
	"runtime"
	"runtime/debug"
)

// 以下变量由链接器在编译时通过 -ldflags -X 注入。
// 直接 go build 时保持默认值，此时会尝试从 VCS 信息回退推断。
var (
	// Version 是发布版本号，如 v7.0.0；开发构建为 dev。
	Version = "dev"
	// Commit 是构建所用的 git 提交短哈希。
	Commit = ""
	// Date 是构建时间，RFC3339 格式。
	Date = ""
)

// Info 是一次构建的完整标识。
type Info struct {
	Version   string
	Commit    string
	Date      string
	GoVersion string
	Platform  string
}

// Get 返回当前构建信息。
//
// Commit 未被注入时，尝试从 Go 1.18+ 嵌入的 VCS 信息回退读取，
// 这样直接 go build 出来的二进制也能报出准确的提交号。
func Get() Info {
	i := Info{
		Version:   Version,
		Commit:    Commit,
		Date:      Date,
		GoVersion: runtime.Version(),
		Platform:  runtime.GOOS + "/" + runtime.GOARCH,
	}
	if i.Commit == "" || i.Date == "" {
		fillFromVCS(&i)
	}
	return i
}

// fillFromVCS 从二进制内嵌的版本控制信息补齐缺失字段。
func fillFromVCS(i *Info) {
	bi, ok := debug.ReadBuildInfo()
	if !ok {
		return
	}
	var dirty bool
	for _, s := range bi.Settings {
		switch s.Key {
		case "vcs.revision":
			if i.Commit == "" && len(s.Value) >= 7 {
				i.Commit = s.Value[:7]
			}
		case "vcs.time":
			if i.Date == "" {
				i.Date = s.Value
			}
		case "vcs.modified":
			dirty = s.Value == "true"
		}
	}
	if dirty && i.Commit != "" {
		i.Commit += "-dirty"
	}
}

// String 返回单行版本描述。
func (i Info) String() string {
	s := "magicd " + i.Version
	if i.Commit != "" {
		s += " (" + i.Commit + ")"
	}
	return s
}

// Detail 返回多行的详细版本信息。
func (i Info) Detail() string {
	return fmt.Sprintf("magicd %s\n提交:   %s\n构建于: %s\nGo:     %s\n平台:   %s",
		orNA(i.Version), orNA(i.Commit), orNA(i.Date), i.GoVersion, i.Platform)
}

// orNA 把空串替换为占位符，避免输出出现空字段。
func orNA(s string) string {
	if s == "" {
		return "未知"
	}
	return s
}

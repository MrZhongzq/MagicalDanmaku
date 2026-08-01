// Package webui 只做一件事：把前端构建产物嵌进二进制。
//
// 单独成包是因为 go:embed 不能引用父目录，而 embed 在目录不存在时会
// 编译失败——本包的 dist/ 目录里始终有内容（哪怕只是占位 index.html），
// 保证 go build 任何时候都能过。
//
// 前端源码在仓库根目录的 web/（P4-2），构建时把 Vite 的 build.outDir
// 直接指到本包的 dist/ 目录，不需要额外的同步步骤——多一个同步步骤就
// 多一处会忘记执行的地方，而忘记的表现是「改了前端却没生效」，很难查。
package webui

import (
	"embed"
	"io/fs"
)

//go:embed all:dist
var files embed.FS

// FS 返回前端产物的文件系统，根目录即产物根。
func FS() (fs.FS, error) {
	return fs.Sub(files, "dist")
}

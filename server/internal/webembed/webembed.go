// Package webembed 内嵌前端构建产物。
// 构建脚本（build.sh / build.bat）会先把 web/dist 拷贝到本目录的 webdist/ 再编译；
// 仓库中仅保留占位文件保证源码可直接编译。
package webembed

import "embed"

//go:embed all:webdist
var FS embed.FS

// Package buildinfo 保存编译期写入的版本号。
package buildinfo

// Version 由 -ldflags -X 覆盖，默认表示本地开发构建。
var Version = "dev"

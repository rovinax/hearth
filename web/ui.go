// Package web 嵌入构建后的 React 前端。
package web

import "embed"

//go:embed all:dist
var Dist embed.FS

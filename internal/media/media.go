// Package media 发现本机的缩略图和转码程序。
// 处理本身留到后续阶段，主进程不解码图片或视频。
package media

import "os/exec"

type Binaries struct {
	FFmpeg  string
	FFProbe string
	Vips    string
}

func Discover() Binaries {
	return Binaries{
		FFmpeg:  look("ffmpeg"),
		FFProbe: look("ffprobe"),
		Vips:    look("vips"),
	}
}

func look(name string) string {
	path, err := exec.LookPath(name)
	if err != nil {
		return ""
	}
	return path
}

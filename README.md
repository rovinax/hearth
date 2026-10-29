# Hearth

家用照片、视频和文件的归档服务。页面是 React，构建后由 Go 进程一起提供。

## 现在有什么

- 配置：`hearth.toml`，可用环境变量覆盖
- SQLite（WAL）和 goose 迁移，启动时自动执行
- 前端在 `web/`，用 Vite 构建到 `web/dist`，再由服务嵌入
- `GET /healthz`、`GET /readyz`、`GET /api/meta`
- 管理员和用户登录。用户只看到自己的文件，以及别人标成「登录用户可见」的文件。管理员可以看到并删除全部文件
- 上传图片、视频和其他文件。相同内容只存一份，放在 `data/originals/`
- 图片和视频可以在页面里直接看。装了 `vips` 或 `ffmpeg` 时会生成缩略图。JPEG 会读取拍摄时间
- 文件拥有者可以生成、撤销分享链接。链接不需要登录，过期或撤销后失效
- 进程内任务循环，具体任务以后再注册
- 数据目录：`data/hearth.db`、`data/originals/`、`data/cache/`
- 启动时检查本机有没有 `ffmpeg`、`ffprobe`、`vips`

## 运行

需要 Go 1.27 和 Node.js 24。运行时只有 Go 进程，Node 只在构建前端时使用。

```bash
cp config.example.toml hearth.toml
export HEARTH_ADMIN_USER=admin
export HEARTH_ADMIN_PASSWORD='change-me-now'
make web
go test ./...
make run
```

第一次启动时，如果库里还没有可用的管理员，会用上面两个环境变量创建。已经有管理员之后，再启动不会改密码。打开 `/login` 进入文件库；管理员可以从导航进入用户和全部文件。开发界面时可以在 `web/` 里执行 `npm run dev`，它会把 `/api` 代理到 `127.0.0.1:8080`。

服务监听 `:8080`。`hearth.toml` 已在 `.gitignore` 里。

常用环境变量：`HEARTH_HTTP_ADDR`、`HEARTH_DATA_DIR`、`HEARTH_LOG_LEVEL`、`HEARTH_LOG_FORMAT`、`HEARTH_CONFIG`、`HEARTH_ADMIN_USER`、`HEARTH_ADMIN_PASSWORD`。

## Docker

```bash
docker compose up --build
```

镜像里带有 `ffmpeg` 和 `vips`，数据在卷 `hearth-data`。

## 目录

```text
cmd/hearth          进程入口
web                 React 界面
internal/config     配置和日志
internal/server     HTTP 和 JSON API
internal/store      SQLite、迁移、任务表
internal/jobs       任务循环
internal/media      外部工具探测
deploy              systemd 示例
```

任务表的 SQL 目前手写在 `internal/store/jobs.go`。`sqlc.yaml` 已放好，查询变多之后执行 `make sqlc`。

## 接下来

1. 按拍摄日期排成时间线，而不只是列表
2. 不能直接播放的视频转成浏览器能播的格式，结果放在 `cache/`
3. 人脸、地点和相似照片

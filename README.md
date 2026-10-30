# Hearth · 炉火

**Your family's photos, videos, and files — warm, private, self-hosted.**  
**家用照片、视频与文件归档：私密、自托管、开箱即用。**

Hearth is a small Go + React library for a household: upload once, browse on the web, share with family groups, and keep trash / favorites / collections tidy. Optional MCP tools let AI clients (Cursor 等) operate the same API.

---

## 中文

### 亮点

- **多类型合集**：一个合集可同时容纳图片 / 视频 / 音频 / 其他文件
- **权限清晰**：私有、登录用户可见（shared）、合集成员、用户组；管理员可管全局
- **回收站与收藏**：误删可恢复；常用文件一键收藏
- **主题**：含 sakura 等外观
- **全文检索**：SQLite FTS5，中英文文件名与合集名，多词 AND，前缀更靠前
- **分享链接**：无需登录的预览链接，可设过期
- **MCP**：`hearth-mcp` stdio 服务，对接 Cursor 等
- **Docker / 单二进制**：前端嵌入 `web/dist`，一条命令跑起来

> Android 客户端若在旁路仓库维护，可另行对接同一 HTTP API（本仓库以 Web + 服务端为主）。

### 快速开始

需要 **Go 1.27+** 与 **Node.js 24**（仅构建前端时需要 Node）。

```bash
cp config.example.toml hearth.toml
export HEARTH_ADMIN_USER=admin
export HEARTH_ADMIN_PASSWORD='change-me-now'
make web          # 构建 React → web/dist（嵌入二进制）
go test ./...
make run          # 监听 :8080
```

首次启动若还没有可用管理员，会用上面的环境变量创建。之后再启动不会改密码。打开 `/login` 进入文件库。

开发前端：

```bash
# 终端 1
make run
# 终端 2
cd web && npm run dev   # /api 代理到 127.0.0.1:8080
```

### Docker

```bash
docker compose up --build
```

镜像含 `ffmpeg` / `vips`，数据卷 `hearth-data`。

### 常用环境变量

| 变量 | 含义 |
|---|---|
| `HEARTH_HTTP_ADDR` | 监听地址（默认 `:8080`） |
| `HEARTH_DATA_DIR` | 数据目录 |
| `HEARTH_LOG_LEVEL` / `HEARTH_LOG_FORMAT` | 日志 |
| `HEARTH_CONFIG` | 配置文件路径 |
| `HEARTH_ADMIN_USER` / `HEARTH_ADMIN_PASSWORD` | 引导管理员 |

`hearth.toml` 已在 `.gitignore` 中，请勿提交真实密码。

### MCP

```bash
make mcp   # -> bin/hearth-mcp
```

| 变量 | 必需 | 默认 | 说明 |
|---|---|---|---|
| `HEARTH_BASE_URL` | 否 | `http://127.0.0.1:8080` | HTTP 根地址 |
| `HEARTH_USERNAME` | 是 | — | 登录用户 |
| `HEARTH_PASSWORD` | 是 | — | 登录密码 |

示例描述符见 `mcp/hearth.mcp.json`。不要把真实密码写进仓库。

### 目录结构

```text
cmd/hearth       主进程
cmd/hearth-mcp   stdio MCP
mcp/             Cursor mcpServers 示例
web/             React（构建产物嵌入）
internal/config  配置与日志
internal/server  HTTP / JSON API
internal/store   SQLite、迁移、检索
internal/jobs    任务循环
internal/media   ffmpeg / vips 探测
deploy/          systemd 示例
```

### 检索说明

`GET /api/search?q=` 使用 FTS5（`unicode61`）索引文件名与所属合集名、以及合集名本身。多个词之间为 AND；默认排序优先精确名 → 前缀 → 包含，再按 bm25。权限规则与列表接口一致。

---

## English

### Highlights

- **Multi-kind collections** — images, video, audio, and other files in one set
- **Clear permissions** — private, shared-to-signed-in, collection members, groups; admins see everything
- **Trash & favorites** — restore mistakes; pin what you use often
- **Themes** — including sakura
- **Search** — SQLite FTS5 for Chinese and English names, multi-token AND, better ranking
- **Share links** — login-free preview links with expiry
- **MCP** — `hearth-mcp` for Cursor and other stdio clients
- **Docker / single binary** — UI embedded from `web/dist`

> An Android client, if maintained as a sibling project, can use the same HTTP API. This repo focuses on the server and web UI.

### Quick start

Requires **Go 1.27+** and **Node.js 24** (Node only to build the UI).

```bash
cp config.example.toml hearth.toml
export HEARTH_ADMIN_USER=admin
export HEARTH_ADMIN_PASSWORD='change-me-now'
make web
go test ./...
make run    # listens on :8080
```

On first boot with no enabled admin, the env vars above create one. Open `/login` for the library.

### Docker

```bash
docker compose up --build
```

### MCP

```bash
make mcp   # -> bin/hearth-mcp
```

Set `HEARTH_BASE_URL`, `HEARTH_USERNAME`, and `HEARTH_PASSWORD`. See `mcp/hearth.mcp.json`. Never commit real passwords.

### Search

`GET /api/search?q=` uses FTS5 (`unicode61`) over file names (plus associated collection name) and collection names. Tokens are ANDed; default ranking prefers exact → prefix → contains, then bm25. Permission checks match the rest of the library API.

### License

MIT — see [LICENSE](LICENSE).

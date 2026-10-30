# Contributing · 参与贡献

感谢你愿意给 Hearth 添柴加火。 / Thanks for helping Hearth grow.

## 中文

1. Fork 并创建分支，保持改动聚焦。
2. 用 `go test ./...` 与必要的前端构建验证；涉及检索时请覆盖中英文文件名。
3. 不要提交 `hearth.toml`、真实密码、本机绝对路径、上传日志、`data/`、`*.apk` 等私密或生成物（`web/dist` 若为嵌入所需可按仓库惯例提交）。
4. 提交说明写清「做了什么 / 为什么」；PR 里附上验证方式。

## English

1. Fork and branch; keep changes focused.
2. Run `go test ./...` (and rebuild the web UI when needed). Search changes should cover Chinese and English names.
3. Do not commit secrets, `hearth.toml`, personal absolute paths, upload logs, or `data/`. Follow the repo’s existing practice for embedded `web/dist`.
4. Write clear commit messages and describe how you verified the change in the PR.

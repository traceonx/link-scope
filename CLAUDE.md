# 项目约定

LinkScope · 链接透视：运行在 NAS（飞牛 fnOS）上的链接检测工具，单镜像部署。

- 每次提交都要升级版本号：同时修改根目录 `VERSION` 与 `web/package.json` 的 version，并在 `CHANGELOG.md` 顶部新增该版本条目（格式 `## x.y.z · YYYY-MM-DD`，CI 会检查）。修复升 patch，新功能 / 部署变化升 minor。**正式版发布前版本号保持 0.x，不得升到 1.0**（由用户决定何时发布 1.0）。
- 提交前运行 `gofmt -l .`（应无输出）、`go vet ./...`、`go test -race ./...`，前端改动后运行 `cd web && npm run build`（含 vue-tsc 类型检查）。
- 镜像通过 GitHub Actions（`.github/workflows/docker.yml`）构建并推送到 Docker Hub `sunqz/link-scope`；部署文件在 `deploy/`（Dockerfile、entrypoint.sh、docker-compose.yml），数据在 `/app/data`。
- 一键更新 / 自动更新在 `internal/update`（参考 shop-cashback 的 update.ts）：docker.sock + watchtower `--run-once --no-pull`，状态写 `DATA_DIR/update-state.json`。
- 出站代理只用环境变量 `PROXY_HOST`，目前只有 Docker Hub 版本查询走代理；不做界面配置。不要提交真实密钥或 `.env`。

## 代码结构

- `internal/scanner`：扫描核心，不依赖 Web。性能敏感：遍历用并发 `os.ReadDir` + `syscall.Lstat`，指纹计算复用 worker 缓冲区，不要在热路径里分配大块内存。改动后可跑 `LS_BENCH_N=200000 go test -run TestLargeScan -v ./internal/scanner/` 对比耗时。
- `internal/jobs`：后台扫描任务与结果持久化（`last_scan.json`）；`folders.go` 按检测目录的一级子项汇总文件夹视图（结果变化时清空缓存）。
- `internal/server`：HTTP 接口，Go 标准库 `net/http` 路由；`/api/health` 不需要登录。
- `internal/fileops`：删除与回收站。回收站在所属根目录下的 `.linkscope-trash`（扫描始终跳过）；只能操作 `LS_ROOTS` 内、非根目录的路径；路径本身是软链接时删除链接而不是目标。释放空间按 (dev, inode) 统计：选中路径数 ≥ nlink 才算释放。
- `internal/torrents`：下载器（qBittorrent WebUI API / Transmission RPC，多实例）与站点识别；配置在 `DATA_DIR/downloaders.json`（含密码，0600，接口不返回密码）。`jobs/annotate.go` 把种子信息标注到扫描结果：标签 torrent / seeding / unregistered / orphan。
- `web/`：Vue 3 + Vite + TypeScript + Naive UI；构建产物 `web/dist` 通过 `go:embed` 打包进程序。

## 前端规范（web/）

- 目录：`components/` 组件，`views/` 页面，`api.ts` 接口，`types.ts` 类型，`format.ts` / `badges.ts` 工具，`style.css` 全局变量。
- 颜色只用 `style.css` 里的 CSS 变量（已含深色模式），不写死颜色；Naive UI 主题在 `App.vue` 的 themeOverrides。
- 弹窗用 `NModal preset="card"`，确认操作用 `useDialog()`，提示用 `useMessage()`，不用原生 alert / confirm。
- 状态颜色：已链接 蓝、未链接 橙、重复副本 紫、STRM 青、失效 / 已删种 红、做种中 绿、孤儿文件 橙；颜色之外还需有文字标签。
- NAS 局域网多为 http，复制到剪贴板要用 `format.ts` 的 `copyText`（含降级方案）。

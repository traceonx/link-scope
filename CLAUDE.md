# 项目约定

LinkScope · 链接透视：运行在 NAS（飞牛 fnOS）上的下载目录清理工具，单镜像部署。

产品定位：用户用 qBittorrent 下载到下载目录、MoviePilot 用硬链接整理到媒体库；本工具找出下载目录中**没被媒体库用上（未硬链接）的游离文件**并安全清理。硬链接是判断依据，种子 / 站点信息是辅助。界面用语：待清理（未链接）、部分游离（文件夹内部分未链接）、已整理（已链接）；不要在主流程反复强调硬链接删除风险，inode 等技术信息放在「高级信息」里。

- 每次提交都要升级版本号：同时修改根目录 `VERSION` 与 `web/package.json` 的 version，并在 `CHANGELOG.md` 顶部新增该版本条目（格式 `## x.y.z · YYYY-MM-DD`，CI 会检查）。修复升 patch，新功能 / 部署变化升 minor。**正式版发布前版本号保持 0.x，不得升到 1.0**（由用户决定何时发布 1.0）。
- 提交前运行 `gofmt -l .`（应无输出）、`go vet ./...`、`go test -race ./...`，前端改动后运行 `cd web && npm run build`（含 vue-tsc 类型检查）。
- 镜像通过 GitHub Actions（`.github/workflows/docker.yml`）构建并推送到 Docker Hub `sunqz/link-scope`；部署文件在 `deploy/`（Dockerfile、entrypoint.sh、docker-compose.yml），数据在 `/app/data`。
- 一键更新 / 自动更新在 `internal/update`（参考 shop-cashback 的 update.ts）：docker.sock + watchtower `--run-once --no-pull`，状态写 `DATA_DIR/update-state.json`。
- 出站代理只用环境变量 `PROXY_HOST`，目前只有 Docker Hub 版本查询走代理；不做界面配置。不要提交真实密钥或 `.env`。

## 代码结构

- `internal/scanner`：扫描核心，不依赖 Web。性能敏感：遍历用并发 `os.ReadDir` + `syscall.Lstat`，指纹计算复用 worker 缓冲区，不要在热路径里分配大块内存。改动后可跑 `LS_BENCH_N=200000 go test -run TestLargeScan -v ./internal/scanner/` 对比耗时。
- `internal/jobs`：后台扫描任务与结果持久化（`last_scan.json`）；`folders.go` 按检测目录的一级子项汇总文件夹视图（结果变化时清空缓存）。
- `internal/logs`：日志收集。`logs.Install()` 让标准库 `log` 同时写到标准错误和内存环形缓冲（最近 2000 条），`GET /api/logs?after=<id>` 增量读取；警告 / 错误用 `logs.Warnf` / `logs.Errorf`（带 `[WARN]` / `[ERROR]` 前缀），普通 `log.Printf` 为信息。日志消息以 `[扫描]`、`[下载器]`、`[删除]` 等模块标签开头。
- `internal/server`：HTTP 接口，Go 标准库 `net/http` 路由；`/api/health` 不需要登录。
- `internal/fileops`：删除与回收站。回收站放在**文件所在文件系统（st_dev 相同）的最上层目录**下的 `.linkscope-trash`（飞牛 Btrfs 子卷如 `/vol2/1000` 无法跨子卷 rename），用过的位置记录在 `DATA_DIR/trash-dirs.json`，扫描始终跳过该目录名；只能操作 `LS_ROOTS` 内、非根目录的路径；路径本身是软链接时删除链接而不是目标。释放空间按 (dev, inode) 统计：选中路径数 ≥ nlink 才算释放。
- `internal/torrents`：下载器（qBittorrent WebUI API / Transmission RPC，多实例）与站点识别；配置在 `DATA_DIR/downloaders.json`（含密码，0600，接口不返回密码）。`jobs/annotate.go` 把种子信息标注到扫描结果：标签 torrent / seeding / unregistered / orphan。删除时可选 `remove_torrents`：`Store.RemoveByPaths` 只移除全部文件都已删除（或已不存在）的种子，且始终不让下载器删文件（qB `deleteFiles=false`、Transmission `delete-local-data: false`）。
- `web/`：Vue 3 + Vite + TypeScript + Naive UI；构建产物 `web/dist` 通过 `go:embed` 打包进程序。

## 前端规范（web/）

界面风格参考 MoviePilot（Vuetify / Materio 后台模板）：左侧导航 + 顶部应用栏 + 卡片。组件库仍用 Naive UI（不引入 Vuetify），图标用 `@mdi/js`（按名称引入，打包只含用到的图标）。

- 目录：`components/` 组件，`views/` 页面（DashboardView 仪表盘、ScanView 扫描、ResultsView 清理结果、DownloadersView 下载器、TrashView 回收站、LogsView 日志、SettingsView 设置），`router.ts` 极简 hash 路由（`#/results`、`#/settings/update`，无 vue-router），`store.ts` 全局单例状态（配置、扫描任务、结果筛选、勾选、详情、删除；无 Pinia），`api.ts` 接口，`types.ts` 类型，`format.ts` / `badges.ts` 工具，`prefs.ts` 浏览器本地偏好（主题、默认视图、扫描预设、扫描表单草稿），`icons.ts` MDI 图标表，`style.css` 全局变量与通用样式（`.card` / `.card-head` / `.avatar` / `.set-row` 等）。
- 布局：`AppRoot.vue` 用 `<KeepAlive>` 缓存各页面（切换不丢状态，页面内需要刷新的用 `onActivated`）；`AppLayout.vue` 是导航与应用栏；删除对话框 `DeleteDialog` 挂在根组件，任何页面通过 `store.askDelete` / `deletePaths` 发起。新增页面时在 `router.ts` 的 `pages` 与 `AppRoot.vue` 的 `views` 中登记。
- 主要组件：AppLayout、FolderTable / ResultTable（远程分页 + 排序）、DetailDrawer（FolderDetail / FileDetail）、BulkBar（勾选后显示在表格上方）、DeleteDialog（预览 → 确认 → 结果，含「同时删除种子」选项）、FolderPicker、UpdatePanel。
- 主题：`index.html` 在首屏前读取 `localStorage['ls.theme']` 设置 `data-theme`；深色变量在 `style.css` 中同时写给 `prefers-color-scheme` 与 `[data-theme="dark"]`。
- 颜色只用 `style.css` 里的 CSS 变量（已含深色模式），不写死颜色；主色 `--primary`（#9155FD）用于导航选中、主按钮、焦点框、选中行。Naive UI 主题在 `App.vue` 的 themeOverrides（色值与 style.css 保持一致）。
- 卡片无边框、靠 `--shadow` 分层；页面内容用 `.page`（纵向间距 22px）与 `.grid-2`；网格列写 `minmax(0, 1fr)`，避免长路径把手机页面撑宽。清理结果页桌面端卡片占满一屏，表格用 `flex-height` 内部滚动，不要让页面出现外部滚动条。
- 弹窗用 `NModal preset="card"`，确认操作用 `useDialog()`，提示用 `useMessage()`，不用原生 alert / confirm。设置类内容用分组卡片或标签页，不要堆成一个长表单。
- 状态颜色：已整理 蓝、待清理 橙、部分游离 / 重复副本 紫、STRM 青、失效 / 已删种 / 危险操作 红、做种中 绿、孤儿文件 橙；颜色之外还需有文字标签。
- NAS 局域网多为 http，复制到剪贴板要用 `format.ts` 的 `copyText`（含降级方案）。

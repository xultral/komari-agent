# Fork Notes

本文档记录 `xultral/komari-agent` 与上游官方版本的主要差异，以及后续同步上游代码时的维护注意事项。

## 当前 fork 的目标

这个 fork 被收敛成了“监控专用 agent”：

- 保留监控采集与上报
- 移除远程终端能力
- 移除远程命令执行能力
- 移除远程 ping 任务执行能力
- 移除自动更新的实际运行路径

目标是让 agent 只做探针该做的事，不再承担远程控制面。

## 2026-10-27 上游同步状态

本轮同步基于上游 `upstream/main`（`ddbdbdc`，稳定版线 `1.5.11`），只吸收监控采集、
构建兼容性和安全清理，不整体合并上游分支。

已同步：

- CPU、网络采样和报告 JSON 编码优化
- Linux LXC uptime、Android FUSE 磁盘、macOS `nullfs` 去重
- AMD GPU sysfs 回退和 Windows NVIDIA 详细 GPU 监控
- netstatic 统计文件不可写时的警告
- `loong64` 构建、Docker `COPY --chmod`、GitHub Actions 使用 `go.mod` 指定 Go 版本
- 发布二进制启用上游的符号表和调试信息裁剪（`-s -w`）
- Linux 非 root 安装和 systemd user service，服务默认使用调用安装脚本的用户
- Windows 安装脚本自动提权，并修复用户名或安装路径包含空格时的参数转义
- Linux 安装脚本支持无 Bash 的 POSIX shell、OpenWrt `opkg`，并修正 procd/launchd 服务参数
- v2 报告直接复用原始 JSON，避免重复反序列化和序列化
- 采样改为非阻塞后，默认上报间隔同步上游调整为 3 秒，基础信息不再触发实时 CPU 采样
- WebSocket 每秒检查连接，但按配置间隔限流上报；v2 事件去重表增加 TTL 和容量上限

明确不引入：

- `server/files*.go`、`server/file_stream*.go` 及 `agent.file`
- `server/task.go` 中的远程命令执行和 ping
- `terminal/`、`update/`、版本切换和自动更新
- `server/startup_config.go` 的上报完整启动配置能力
- 上游安装脚本中的第三方 GitHub 镜像自动回退

`security_invariants_test.go` 会在运行测试时阻止上述危险目录、文件、依赖和事件处理
重新进入仓库。v2 事件分发默认拒绝未识别的方法，避免未来上游新增事件时被误执行。

Cloudflare Access 请求头属于本 fork 已有的面板连接兼容能力，不涉及远程控制，因此本轮
保留上游已移除的 CF Access Client ID/Secret 配置；这些值只会发送到已配置的面板地址。

`.gitattributes` 固定 shell、Docker 和 workflow 文件使用 LF，Windows PowerShell 脚本
使用 CRLF，避免在 Windows 检出的 `install.sh` 无法被 shell 解析。

## 与官方版的核心差异

### 1. 运行模式

当前 fork 默认运行在 monitoring-only 模式：

- `--disable-web-ssh` 仅作为兼容参数保留
- `--disable-auto-update` 仅作为兼容参数保留
- `--show-warning` 仅作为兼容参数保留

即使配置文件或环境变量中重新设置这些值，也不会恢复远程功能。

### 2. 远程功能移除

已移除或停止使用的能力包括：

- Web terminal
- 远程 exec
- 远程 ping task
- 远程控制相关的 Windows 警告辅助逻辑

当前实现中，agent 收到相关事件时会忽略，而不会执行。

### 3. 配置优先级修正

当前 fork 的配置加载顺序为：

- `config`
- `env`
- `CLI`

之后再强制应用 monitoring-only 默认值。

这样可以避免“命令行明明关了安全开关，却被配置文件重新打开”的问题。

### 4. 发布源改为 fork

已改为默认从以下仓库下载发布包：

- `xultral/komari-agent`

涉及位置：

- `install.sh`
- `install.ps1`
- ~~`update/update.go`~~ **已删除（2026-06-25）**
- `README`
- `RELEASE.md`

### 4.5. 版本管理重构（2026-06-25 新增）

为彻底消除自动更新的理论风险，已完全删除 `update` 包：

- **删除**：`update/update.go` 及整个 `update/` 目录
- **新增**：`version/version.go`，只包含版本常量
- **更新引用点**：
  - `cmd/root.go`：从 `update.CurrentVersion` 改为 `version.CurrentVersion`
  - `server/basicInfo.go`：从 `update.CurrentVersion` 改为 `version.CurrentVersion`
- **更新构建脚本版本注入路径**：
  - `.github/workflows/build.yml`
  - `.github/workflows/release.yml`
  - `.github/workflows/release-docker.yml`
  - `build_all.sh`
  - `build_all.ps1`
  - 从 `-ldflags "-X .../update.CurrentVersion=..."` 改为 `-ldflags "-X .../version.CurrentVersion=..."`

### 5. 模块路径改动

Go module 路径已从：

- `github.com/komari-monitor/komari-agent`

改为：

- `github.com/xultral/komari-agent`

对应源码 import、构建脚本、GitHub Actions 的版本注入路径也已同步调整。

## 已补充的文档

当前仓库新增或更新了以下文档：

- [readme.md](/D:/Temp/komari/komari-agent/readme.md)
- [RELEASE.md](/D:/Temp/komari/komari-agent/RELEASE.md)
- [FORK_NOTES.md](/D:/Temp/komari/komari-agent/FORK_NOTES.md)

## 后续同步上游会不会被覆盖

会不会被覆盖，取决于你怎么同步。

### 如果你直接用上游内容强制覆盖当前分支

例如做一种“拿上游 main 完全替换当前 main”的操作，那当然会把 fork 改动冲掉。

### 如果你正常做 fork 同步

也就是把上游变更 merge 或 rebase 到你自己的分支，那么不会“无提示覆盖”，而是会出现两种情况：

- 不冲突的文件自动合并
- 冲突的文件要求你手动处理

这才是推荐方式。

## 推荐同步策略

建议保留一个明确的上游 remote，然后按正常 Git 流程同步。

示例：

```bash
git remote add upstream https://github.com/komari-monitor/komari-agent.git
git fetch upstream
git checkout main
git merge upstream/main
```

如果你更喜欢线性历史，也可以用 `rebase`：

```bash
git fetch upstream
git checkout main
git rebase upstream/main
```

## 哪些文件最容易在同步时冲突

重点关注这些文件：

- `cmd/root.go`
- `server/websocket.go`
- `go.mod`
- `install.sh`
- `install.ps1`
- `build_all.sh`
- `build_all.ps1`
- `.github/workflows/build.yml`
- `.github/workflows/release.yml`
- `.github/workflows/release-docker.yml`
- `update/update.go`
- `readme.md`

这些文件正是 fork 差异最集中的地方。

## 如何降低后续同步成本

建议每次同步上游后，优先检查三类内容：

1. 远程能力是否被重新引入
2. 安装脚本的下载源是否被改回官方仓库
3. Go module 路径和构建注入路径是否仍然指向 `xultral/komari-agent`

可直接用下面的检索思路快速排查：

```bash
rg "terminal|agent.exec|agent.ping|disable-web-ssh|komari-monitor/komari-agent"
```

## 建议的长期做法

如果你准备长期维护这个 fork，最稳妥的方式不是“同步后再靠记忆修修补补”，而是把当前 fork 的关键约束当成维护规则：

- 只保留监控能力
- 不接受远程控制面回归
- 默认发布源固定到 `xultral/komari-agent`
- 每次同步上游后先跑测试，再检查差异文档中列出的关键文件

这样即使后面过了几个月，你也能快速判断哪些改动是 fork 有意为之，哪些只是上游新代码。

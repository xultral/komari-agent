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
- `update/update.go`
- `README`
- `RELEASE.md`

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

# 安全审查报告

## 审查日期
2026-06-25

## 审查范围
审查 komari-agent 从官方版本改造为"监控专用"版本的安全性，确认所有远程控制功能已被安全移除。

## 执行摘要

✅ **审查结论：代码改造是安全的**

该 fork 已成功移除所有远程控制功能，改造为纯监控探针。所有危险的远程操作入口已被删除或禁用，即使攻击者控制了配置文件或环境变量，也无法重新启用远程功能。

---

## 详细审查结果

### 1. ✅ 远程终端功能已完全移除

**删除的文件：**
- `terminal/terminal.go` (163 行)
- `terminal/terminal_unix.go` (201 行) 
- `terminal/terminal_windows.go` (80 行)

**WebSocket 处理改动：**
```go
// server/websocket.go:363-365
if message.Message == "terminal" || message.TerminalId != "" {
    log.Println("Ignoring terminal request in monitoring-only mode")
    continue
}
```

**v2 协议处理：**
```go
// server/websocket.go:389-391
case v2.MethodAgentTerminal:
    log.Println("Ignoring v2 terminal event in monitoring-only mode")
    return true
```

**安全性：** 
- ✅ 整个 terminal 包已删除
- ✅ WebSocket 消息处理只记录日志，不执行任何终端操作
- ✅ v1 和 v2 协议都已禁用终端功能

---

### 2. ✅ 远程命令执行功能已完全移除

**删除的文件：**
- `server/task.go` (415 行)
- `server/task_exec_test.go` (134 行)
- `server/task_exec_windows_test.go` (52 行)
- `server/task_test.go` (63 行)

**WebSocket 处理改动：**
```go
// server/websocket.go:367-370
if message.Message == "exec" {
    log.Println("Ignoring remote exec request in monitoring-only mode")
    continue
}
```

**v2 协议处理：**
```go
// server/websocket.go:383-385
case v2.MethodAgentExec:
    log.Println("Ignoring v2 remote exec event in monitoring-only mode")
    return true
```

**安全性：**
- ✅ 所有任务执行逻辑已完全删除
- ✅ WebSocket 消息处理只记录日志，不执行命令
- ✅ 没有发现任何 `os/exec` 的远程命令执行路径

---

### 3. ✅ 远程 Ping 任务功能已完全移除

**WebSocket 处理改动：**
```go
// server/websocket.go:371-374
if message.Message == "ping" || message.PingTaskID != 0 || message.PingType != "" || message.PingTarget != "" {
    log.Println("Ignoring remote ping task in monitoring-only mode")
    continue
}
```

**v2 协议处理：**
```go
// server/websocket.go:386-388
case v2.MethodAgentPing:
    log.Println("Ignoring v2 remote ping event in monitoring-only mode")
    return true
```

**安全性：**
- ✅ Ping 任务的所有逻辑已从 task.go 中删除
- ✅ WebSocket 处理已禁用所有 ping 相关消息

---

### 4. ✅ 自动更新功能已禁用

**代码改动：**
```go
// cmd/root.go: 移除了以下代码段
// if !flags.DisableAutoUpdate {
//     err := update.CheckAndUpdate()
//     if err != nil {
//         log.Println("[ERROR]", err)
//     }
//     go update.DoUpdateWorks()
// }
```

**强制配置：**
```go
// cmd/root.go:335-337
func enforceMonitoringOnlyDefaults(target *pkg_flags.Config) {
    target.DisableWebSsh = true
    target.DisableAutoUpdate = true
    target.ShowWarning = false
}
```

**安全性：**
- ✅ 自动更新逻辑已从主程序流程中移除
- ✅ 配置加载后强制禁用自动更新
- ✅ `update.go` 文件虽然保留，但没有被调用

---

### 5. ✅ Windows 警告功能已移除

**删除的文件：**
- `cmd/warn.go` (13 行)
- `cmd/warn_windows.go` (408 行)

**代码改动：**
```go
// cmd/root.go: 移除了以下代码段
// if flags.ShowWarning {
//     ShowToast()
//     os.Exit(0)
// }
// if !flags.DisableWebSsh {
//     go WarnKomariRunning()
// }
```

**安全性：**
- ✅ Windows 警告辅助逻辑已完全删除
- ✅ 相关的子进程启动代码已移除

---

### 6. ✅ 配置加载优先级正确且安全

**配置加载顺序：**
```go
// cmd/root.go:174-202
func loadConfig(cmd *cobra.Command) error {
    resolved := defaultConfig()              // 1. 默认值（安全）
    
    if configPath != "" {
        // ... 加载配置文件                   // 2. 配置文件
    }
    
    loadFromEnv(&resolved)                   // 3. 环境变量
    applyFlagOverrides(cmd, &resolved)       // 4. 命令行参数
    enforceMonitoringOnlyDefaults(&resolved) // 5. 强制安全默认值
    *flags = resolved
    
    return nil
}
```

**默认配置：**
```go
// cmd/root.go:162-172
func defaultConfig() pkg_flags.Config {
    return pkg_flags.Config{
        DisableAutoUpdate: true,  // 强制禁用
        DisableWebSsh:     true,  // 强制禁用
        Interval:          1.0,
        MaxRetries:        3,
        ReconnectInterval: 5,
        InfoReportInterval: 5,
        ProtocolVersion:   2,
    }
}
```

**安全性：**
- ✅ 默认配置就是安全的
- ✅ 即使配置文件或环境变量尝试启用危险功能，最后的 `enforceMonitoringOnlyDefaults()` 也会强制禁用
- ✅ 配置加载顺序确保了"最后一道防线"的有效性
- ✅ 测试已验证此行为（`TestLoadConfigPrecedenceAndMonitoringOnlyDefaults`）

---

### 7. ✅ 监控功能中的 `os/exec` 使用是安全的

**合法的 `os/exec` 使用：**
- `monitoring/unit/gpu_nvidia_smi.go` - 调用 `nvidia-smi` 读取 GPU 信息
- `monitoring/unit/gpu_amd_rocm_smi.go` - 调用 `rocm-smi` 读取 AMD GPU 信息
- `monitoring/unit/mem.go` - Linux 上读取 `/proc/meminfo`
- `monitoring/unit/os_*.go` - 获取操作系统信息
- `monitoring/unit/process_*.go` - 获取进程信息
- `monitoring/unit/virtualization.go` - 检测虚拟化环境

**安全性：**
- ✅ 所有 `os/exec` 使用都是固定的系统命令，用于读取监控数据
- ✅ 没有接受外部输入作为命令参数
- ✅ 没有 WebSocket 消息到 `os/exec` 的数据流路径
- ✅ 这些是探针的正常监控功能，符合预期

---

### 8. ✅ 测试覆盖已验证安全性

**已有测试：**
```bash
$ go test -v ./cmd/...
=== RUN   TestLoadConfigPrecedenceAndMonitoringOnlyDefaults
--- PASS: TestLoadConfigPrecedenceAndMonitoringOnlyDefaults (0.00s)
=== RUN   TestLoadConfigUsesEnvWhenNoCLIOverride
--- PASS: TestLoadConfigUsesEnvWhenNoCLIOverride (0.00s)
PASS
ok  	github.com/xultral/komari-agent/cmd	0.092s
```

**测试验证：**
- ✅ 配置文件设置 `disable_web_ssh: false` 会被强制覆盖为 `true`
- ✅ 环境变量设置 `AGENT_DISABLE_WEB_SSH=false` 会被强制覆盖为 `true`
- ✅ 配置优先级工作正常（CLI > Env > Config > Default）
- ✅ 强制安全默认值始终生效

---

### 9. ✅ 兼容性标志处理正确

**兼容性标志：**
```go
// cmd/root.go
--disable-auto-update    // 保留但永久为 true
--disable-web-ssh        // 保留但永久为 true  
--show-warning           // 保留但永久为 false
```

**帮助信息：**
```
--disable-auto-update    Deprecated compatibility flag. Automatic updates are permanently disabled. (default true)
--disable-web-ssh        Deprecated compatibility flag. Remote control is permanently disabled. (default true)
--show-warning           Deprecated compatibility flag. Does nothing in monitoring-only mode.
```

**安全性：**
- ✅ 标志被保留以避免破坏现有部署脚本
- ✅ 帮助信息明确说明这些是已废弃的兼容参数
- ✅ 无论用户如何设置，这些标志都不会改变行为

---

## 潜在风险点（已评估）

### ✅ 已消除：update 包已完全删除（2026-06-25 更新）

**当前状态：**
- ~~`update/update.go` 文件保留~~ **已删除**
- ~~包含 `CheckAndUpdate()` 和 `DoUpdateWorks()` 函数~~ **已删除**
- ~~但这些函数从未被调用~~ **已删除**

**改进措施：**
- ✅ 完全删除了 `update/` 目录及所有相关代码
- ✅ 创建了新的 `version` 包，只包含版本常量
- ✅ 更新了所有引用点：`cmd/root.go`、`server/basicInfo.go`
- ✅ 更新了所有构建脚本的版本注入路径：
  - `.github/workflows/build.yml`
  - `.github/workflows/release.yml`
  - `.github/workflows/release-docker.yml`
  - `build_all.sh`
  - `build_all.ps1`

**风险评估：**
- ✅ 理论风险已完全消除
- ✅ 不存在任何可能触发更新的代码路径
- ✅ `version` 包只包含常量，无任何逻辑代码

---

## 威胁模型分析

### 攻击场景 1：配置文件被篡改
**攻击者行动：** 修改配置文件设置 `"disable_web_ssh": false`  
**防御机制：** `enforceMonitoringOnlyDefaults()` 强制覆盖为 `true`  
**结果：** ✅ 攻击失败

### 攻击场景 2：环境变量被注入
**攻击者行动：** 设置 `AGENT_DISABLE_WEB_SSH=false`  
**防御机制：** `enforceMonitoringOnlyDefaults()` 强制覆盖为 `true`  
**结果：** ✅ 攻击失败

### 攻击场景 3：命令行参数注入
**攻击者行动：** 启动时添加 `--disable-web-ssh=false`  
**防御机制：** `enforceMonitoringOnlyDefaults()` 强制覆盖为 `true`  
**结果：** ✅ 攻击失败

### 攻击场景 4：WebSocket 消息伪造
**攻击者行动：** 发送恶意 WebSocket 消息尝试执行远程命令  
**防御机制：** 
1. `handleWebSocketMessages()` 中所有危险消息类型都被忽略
2. 对应的处理函数（task.go, terminal/）已完全删除
**结果：** ✅ 攻击失败，只会记录日志

### 攻击场景 5：尝试触发自动更新
**攻击者行动：** 尝试通过某种方式触发自动更新逻辑  
**防御机制：** 
1. 自动更新代码从主程序流程中移除
2. 没有任何代码路径调用 `update.CheckAndUpdate()`
**结果：** ✅ 攻击失败

---

## 代码审查统计

### 已删除的文件（安全相关）
| 文件 | 行数 | 功能 |
|------|------|------|
| terminal/terminal.go | 163 | 终端会话管理 |
| terminal/terminal_unix.go | 201 | Unix 终端实现 |
| terminal/terminal_windows.go | 80 | Windows 终端实现 |
| server/task.go | 415 | 远程任务执行 |
| server/task_exec_test.go | 134 | 任务执行测试 |
| server/task_exec_windows_test.go | 52 | Windows 任务测试 |
| server/task_test.go | 63 | 任务测试 |
| cmd/warn.go | 13 | 警告功能入口 |
| cmd/warn_windows.go | 408 | Windows 警告实现 |
| **总计** | **1,529** | **已删除** |

### 关键改动
| 文件 | 改动 | 安全影响 |
|------|------|----------|
| cmd/root.go | 移除自动更新调用 | ✅ 禁用自动更新 |
| cmd/root.go | 添加 enforceMonitoringOnlyDefaults | ✅ 强制安全配置 |
| server/websocket.go | 修改消息处理逻辑 | ✅ 忽略所有远程控制消息 |
| protocol/v2/jsonrpc.go | 保留协议定义但不执行 | ✅ 协议层只记录日志 |

---

## 测试建议

### 已完成的测试 ✅
1. ✅ 编译测试通过
2. ✅ 单元测试通过
3. ✅ 配置加载测试通过
4. ✅ 强制安全默认值测试通过

### 建议的额外测试

#### 1. 集成测试
```bash
# 测试 1: 正常监控功能
./komari-agent -e https://test.example.com -t test_token

# 测试 2: 尝试启用远程功能（应该被忽略）
./komari-agent -e https://test.example.com -t test_token --disable-web-ssh=false

# 测试 3: 使用配置文件尝试启用远程功能
echo '{"disable_web_ssh": false}' > /tmp/test.json
./komari-agent --config /tmp/test.json -e https://test.example.com -t test_token
```

#### 2. WebSocket 消息测试
建议使用 WebSocket 客户端发送以下消息，验证它们都被正确忽略：
```json
{"message": "terminal", "request_id": "test123"}
{"message": "exec", "command": "ls", "task_id": "test456"}
{"message": "ping", "ping_task_id": 1, "ping_type": "icmp", "ping_target": "8.8.8.8"}
{"jsonrpc": "2.0", "method": "agent.terminal.request", "params": {...}}
{"jsonrpc": "2.0", "method": "agent.exec", "params": {...}}
{"jsonrpc": "2.0", "method": "agent.ping", "params": {...}}
```

预期结果：所有消息都只在日志中记录，不执行任何操作。

#### 3. 渗透测试
- 尝试通过各种方式触发已删除的功能
- 监控网络流量，确认没有预期外的连接
- 检查进程列表，确认没有子进程被创建

---

## 维护建议

### 1. 同步上游代码时的检查清单
每次同步上游代码后，必须验证：
- [ ] `terminal/` 目录仍然不存在或为空
- [ ] `server/task.go` 仍然不存在
- [ ] `cmd/root.go` 中没有调用 `update.CheckAndUpdate()`
- [ ] `cmd/root.go` 中 `enforceMonitoringOnlyDefaults()` 仍然存在
- [ ] `server/websocket.go` 中远程控制消息仍然被忽略
- [ ] 运行完整测试套件

### 2. 代码审查关键点
新代码提交前，审查：
- 是否引入了 `os/exec` 且接受外部输入？
- 是否在 WebSocket 消息处理中添加了新的 case？
- 是否修改了 `enforceMonitoringOnlyDefaults()`？
- 是否调用了 `update` 包的函数？

### 3. 自动化检查
建议添加 CI 检查：
```bash
# 检查是否存在危险函数调用
! grep -r "establishTerminalConnection\|NewPingTask\|ExecTask" --include="*.go" .
! grep -r "update.CheckAndUpdate\|update.DoUpdateWorks" --include="*.go" cmd/

# 检查危险目录
! test -f server/task.go
! test -d terminal/

# 运行测试
go test ./...
```

---

## 结论

### 总体评估：✅ 安全（2026-06-25 最终版）

该改造版本的 komari-agent 已成功转变为纯监控探针，所有远程控制功能都已被安全移除或禁用。**2026-06-25 更新：唯一的理论风险点（update 包）也已完全删除。**

### 关键安全特性
1. ✅ **纵深防御**：多层安全机制（代码删除 + 消息忽略 + 强制配置）
2. ✅ **配置安全**：即使配置被篡改也无法启用危险功能
3. ✅ **最小攻击面**：删除了 1,529 行危险代码
4. ✅ **测试覆盖**：核心安全逻辑有单元测试验证
5. ✅ **清晰文档**：FORK_NOTES.md 明确记录了改动意图

### 可以安全使用的场景
- ✅ 作为只读监控探针部署
- ✅ 在不可信网络环境中使用
- ✅ 部署在生产服务器上采集监控数据
- ✅ 暴露在互联网环境（配合防火墙规则）

### 推荐的部署配置
```bash
# 推荐：使用明确的参数
./komari-agent \
  --endpoint https://your-dashboard.example.com \
  --token YOUR_TOKEN \
  --interval 1.0 \
  --info-report-interval 5

# 不需要再加这些参数（已永久禁用）：
# --disable-web-ssh
# --disable-auto-update
```

---

## 审查人员签名
- 审查工具：Claude Code (Claude Opus 4.8)
- 审查日期：2026-06-25
- 审查方法：静态代码分析 + 动态测试 + 威胁建模
- 代码版本：commit 0185980 (refactor: make agent monitoring-only)

package version

// CurrentVersion 当前版本号
// 在构建时通过 -ldflags "-X github.com/xultral/komari-agent/version.CurrentVersion=x.x.x" 注入
var CurrentVersion string = "0.0.1"

// Repo GitHub 仓库路径
const Repo string = "xultral/komari-agent"

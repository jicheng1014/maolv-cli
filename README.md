# maolv-cli

[毛驴短链](https://www.maolvdl.com)的 Go 命令行工具和 AI Skill。支持授权、短链创建、查询、修改和项目查询，使用账号现有的 API Token。

## 安装

无需 Go。支持 macOS、Linux、Windows 的 amd64 / arm64，使用 [GitHub Releases](https://github.com/jicheng1014/maolv-cli/releases) 的预编译程序。

macOS / Linux（需要 curl 和系统 SHA256 工具）：

```sh
curl --proto '=https' --proto-redir '=https' -fLsS https://raw.githubusercontent.com/jicheng1014/maolv-cli/v0.1.0/scripts/install.sh -o /tmp/maolv-install.sh
sh /tmp/maolv-install.sh
export PATH="$HOME/.local/bin:$PATH"
```

Windows PowerShell：

```powershell
$installer = Join-Path $env:TEMP 'maolv-install.ps1'
Invoke-WebRequest -Uri 'https://raw.githubusercontent.com/jicheng1014/maolv-cli/v0.1.0/scripts/install.ps1' -OutFile $installer
powershell.exe -NoProfile -ExecutionPolicy Bypass -File $installer
$env:Path = "$env:LOCALAPPDATA\Maolv\bin;$env:Path"
```

安装到用户目录，现有 CLI 会保留；不需要 sudo，不会修改 shell 配置或系统执行策略。安装器固定下载 `v0.1.0`，允许用环境变量 `MAOLV_VERSION` 指定其他 `vX.Y.Z` 正式版本。下载只允许 HTTPS（包括 GitHub 重定向），设置大小、超时限制，校验 SHA256 和程序版本后才安装。二进制与 `checksums.txt` 来自同一 GitHub Release，首版没有独立签名或自更新。

尚未发布的版本会下载失败，不会回退为源码编译。需要升级时，从 Releases 下载目标平台文件并按 `checksums.txt` 校验后替换现有程序；安装器本身不会覆盖旧版。

## 授权

在 [API 访问令牌](https://www.maolvdl.com/web/console/api_token) 页面创建 Token，再在本机终端运行：

```sh
maolv config set-token
maolv project list --json
```

交互输入不会回显；`config set-token` 也可从标准输入接收 Token，拒绝将 Token 作为命令参数。不要把 Token 写入日志、截图或仓库。`config status` 只显示配置状态，不显示 Token；`config unset-token` 删除本地保存的 Token。

自动化可直接设置 `MAOLV_API_TOKEN`，优先于本地配置。配置文件存放在系统用户配置目录的 `maolv/config.json`：macOS 为 `~/Library/Application Support/maolv/config.json`，Linux 为 `$XDG_CONFIG_HOME/maolv/config.json`（未设置时为 `~/.config/maolv/config.json`），Windows 为用户 AppData 的对应目录。保存 Token 后仍需用业务查询验证授权。

默认 API 为 `https://www.maolvdl.com/apic/v1/api`，仅在有意连接其他环境时设置 `MAOLV_BASE_URL`。

## 使用

```sh
maolv version --json
maolv project list --json
maolv link create https://example.com --name '产品首页' --project '官网' --json
maolv link list --page 1 --per-page 20 --json
maolv link list --project-id 123 --keyword '首页' --json
maolv link show abc123 --json
maolv link update abc123 --url https://example.com/new --name '新首页' --json
maolv help
```

`create --project` 使用项目名称，现有 API 可能自动创建同名项目；查询列表的 `--project-id` 使用项目 ID。`show`、`update` 接受短链的 `short` 值（如 `abc123`），不是完整短链 URL 或记录 ID。可用 `--short` 指定创建或修改后的短码；修改短码可能使旧地址失效。

免费账号创建的是有效期 10 分钟的体验链接；会员创建前需在官网绑定域名。修改目标 URL 仅支持服务端的 `SimpleCube` 短链类型，其他类型应按 API 返回说明处理。CLI 不会代购会员或绑定域名。

服务端需允许 Token 访问列表接口；旧服务端版本可能对 `link list` 返回 401。此时不能把 401 当作“列表为空”，应核对账号授权和服务端版本。`project list` 可单独验证项目查询权限。

所有命令支持 `--json`（可放在命令前后）。标准输出只包含 JSON：成功为 `{"data": ...}`，保留 API 原始响应；失败为 `{"error":{"code":"...","message":"...","details":...}}`。单条短链通常在 `data.cube`，列表在 `data.cubes`。退出码：`0` 成功、`2` 参数错误、`1` 其他错误。终端普通输出仅作展示，自动化使用 JSON。

创建、修改的网络超时可能发生在服务器已完成操作之后；先查询确认结果，再决定是否重试。现有 API 同时修改目标地址和其他字段时并非原子操作：其他字段校验失败时，目标地址可能已经改变。修改返回错误也需读回当前状态；短码变更成功后使用返回的 `data.cube.short` 查询，失败时先查原短码。

## AI Skill

将仓库的完整 [`skills/maolv`](skills/maolv) 目录复制到所用 AI 工具的 Skill 目录，例如 Codex 的 `~/.codex/skills/maolv`。必须保留其中的 `scripts`，首次使用可自动安装 CLI；无需预先安装 Go。

也可通过 [Vercel Skills CLI](https://github.com/vercel-labs/skills) 安装（Node.js 要求见该工具说明）：

```sh
npx skills add jicheng1014/maolv-cli --skill maolv
```

示例请求：“用毛驴短链给 https://example.com 创建短链接，放到官网项目。” Skill 会使用 CLI 的 JSON 输出；首次授权引导到官网创建 Token，Token 不会写入 Skill。更新 Skill 时替换完整目录，更新 Skill 与更新 CLI 是两个独立操作。

## 开发和发布

```sh
go test -race ./...
go vet ./...
python3 scripts/test_install.py
go build -o maolv ./cmd/maolv
```

开发版本号默认为 `dev`。正式构建使用 `-ldflags '-s -w -X main.version=v0.1.0'`。推送 `vX.Y.Z` 标签后，Release 工作流测试并构建六个平台，生成 `checksums.txt` 并发布 GitHub Release。资产命名如 `maolv_v0.1.0_darwin_arm64`、`maolv_v0.1.0_windows_amd64.exe`。

MIT License。

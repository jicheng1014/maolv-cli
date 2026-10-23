# 毛驴短链 AI Skill

在支持本地 Skill 和命令执行的 AI 工具中安装 `maolv`，即可用自然语言创建、查询和修改账号自己的短链，或查询项目。Skill 调用本仓库的 Go CLI，不提供站内聊天助手，不需要另搭服务。

[官网介绍](https://www.maolvdl.com/cli) · [CLI 安装、命令与 JSON 格式](../README.md) · [Skill 执行规范](maolv/SKILL.md)

## 安装 Skill

### 使用 Skills CLI

需要 Node.js；版本和工具安装要求见 [Vercel Skills CLI](https://github.com/vercel-labs/skills)。运行后按提示选择使用的 AI 工具与安装范围：

```sh
npx skills add jicheng1014/maolv-cli --skill maolv
```

### 手动安装完整目录

下载或克隆本仓库，将整个 `skills/maolv` 目录复制到 AI 工具支持的位置，保留 `SKILL.md` 和 `scripts`。只复制 `SKILL.md` 会缺少首次安装 CLI 所需的安装器。

| 工具 | 个人安装位置 | 当前项目安装位置 |
| --- | --- | --- |
| [Codex](https://developers.openai.com/codex/skills) | `~/.agents/skills/maolv/` | `.agents/skills/maolv/` |
| [Claude Code](https://code.claude.com/docs/en/skills) | `~/.claude/skills/maolv/` | `.claude/skills/maolv/` |

以上是工具官方文档的本地路径；已有环境可能使用其他配置位置，以实际版本和工具说明为准。个人目录适用于本机，不能据此假定云端会话也已安装。安装后重新打开会话，确认能发现 `maolv`。

也可把这段话交给支持安装本地 Skill 的 AI 工具：

> 从 https://github.com/jicheng1014/maolv-cli 安装 skills/maolv 完整目录，保留 SKILL.md 和 scripts，放入当前工具支持的 Skill 目录。先阅读 SKILL.md，再检查 maolv --help。缺少 CLI 时使用 Skill 自带安装器。账号令牌由用户在本机终端运行 maolv config set-token 输入，不在对话中索取或显示令牌。

## 首次使用与授权

Skill 先检查已有 CLI；缺少时可使用自己的安装器。支持 macOS、Linux、Windows 的 amd64 / arm64，无需 Go 或管理员权限。安装器默认下载 GitHub Release `v0.1.0`，检查 HTTPS、SHA256、程序版本及下载大小，保留已有 CLI；不会自动升级。

| 环境 | 安装器（相对 Skill 目录） | 默认程序位置 |
| --- | --- | --- |
| macOS / Linux / WSL | `scripts/install.sh` | `~/.local/bin/maolv` |
| Windows | `scripts/install.ps1` | `%LOCALAPPDATA%\Maolv\bin\maolv.exe` |

执行安装器时使用它的绝对路径，不能假定当前目录就是 Skill 目录。安装后可用程序绝对路径调用，不要求修改永久 PATH；Windows PowerShell 调用程序路径时使用 `&`。SHA256 与程序来自同一 GitHub Release，当前没有独立签名；下载或校验失败应停止，不绕过校验。

登录官网的 [API 访问令牌](https://www.maolvdl.com/web/console/api_token) 页面创建 Token，在本机终端执行：

```sh
maolv config set-token
maolv project list --json
```

第一条命令隐藏输入；第二条验证实际授权。若终端找不到 `maolv`，使用上表中的程序绝对路径。不要将 Token 放入聊天、命令参数、截图、仓库或日志；Skill 不需要读取已保存的 Token。`maolv config status --json` 仅显示配置状态。

已有 `MAOLV_API_TOKEN` 环境变量时，它优先于本地配置。环境变量失效时应更新或移除它，仅重新保存本地 Token 不会改变优先级。默认连接毛驴官网，只有明确要使用其他环境时才设置 `MAOLV_BASE_URL`。

## 用自然语言操作

完成授权后，可向 AI 工具发送：

- “使用 maolv，为 https://example.com/campaign 创建短链，名称是渠道入口，放到春季推广项目，返回完整短链。”
- “使用 maolv，查询账号最近 20 条短链，列出名称、目标地址和完整短链。”
- “使用 maolv，将短码 abc123 的目标地址改为 https://example.com/new，保留名称和短码，完成后查询确认。”

项目名称和短码以实际账号数据为准。创建参数 `--project` 使用项目名称，可能创建缺失项目；列表筛选 `--project-id` 使用项目 ID。查询和修改接受实际 `short` 字段，不接受记录 ID 或完整短链 URL，也不应仅凭 URL 最后一段猜测短码。

Skill 仅处理本次明确要求的字段。创建或修改超时、返回错误时，服务端仍可能已完成部分操作，应先查询核对，不能直接重复写入；成功修改短码后使用响应中的新 `short` 查询。完整命令及非原子修改说明见 [CLI 使用文档](../README.md#使用)。

## 账号限制与排错

- 免费账号创建的是 **10 分钟体验链接**；会员创建前需在官网绑定域名。Skill 不代购会员或自行绑定域名。
- 目标地址修改仅支持 `SimpleCube` 类型，其他类型按服务端错误说明处理。
- Token 缺少或失效时，先在本机完成授权并用项目查询验证；列表 `401` 还可能来自旧服务端未允许 Token 查询，不表示账号没有短链。
- 批量创建按条执行，保留成功结果；鉴权、配额或校验错误后停止同类失败重试，报告未完成项。
- Skill 不包含管理员操作、会员赠送或账号之外的数据权限。

## 更新和卸载

Skill 与 CLI 分别更新。通过 Skills CLI 安装时，使用其更新功能并核对变更；手动安装时，先检查本地定制，再用仓库最新的完整 `skills/maolv` 目录替换已安装副本，保留配套 `scripts`。更新后重新打开会话并检查 `maolv version --json`。

替换 Skill 不会升级已有 CLI。升级程序需从 [GitHub Releases](https://github.com/jicheng1014/maolv-cli/releases) 下载对应系统和架构的版本，按同版 `checksums.txt` 校验后替换；安装器默认 `v0.1.0`，已有程序会保留，不能通过重复安装器升级。详见 [CLI 安装说明](../README.md#安装)。

卸载 Skill 时删除其安装目录或使用安装工具的移除功能；CLI 与本地授权配置仍独立保留。如需清除 CLI 保存的授权，运行 `maolv config unset-token`。环境变量中的 Token 需在设置它的位置另行移除。

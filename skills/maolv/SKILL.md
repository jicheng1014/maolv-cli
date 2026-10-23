---
name: maolv
description: 通过毛驴短链（maolvdl）CLI 创建、查询和修改账号短链及查询项目。用户要求创建毛驴短链接、查看短链或修改目标地址时使用；后台会员赠送和管理员操作不适用。
---

# 毛驴短链

使用 `maolv` CLI 操作用户自己的毛驴短链账号。业务命令一律带 `--json`，按退出码与 `error.code` 判断结果；成功为 `{data: ...}`，失败为 `{error: {code, message, details?}}`。不要重新实现 Token 存储或 API 请求。

## 安装与授权

先检查 PATH 中的 `maolv`，再检查以下用户目录。没有 CLI 时，可运行本 Skill 自带的安装器；实际执行使用安装器绝对路径，不能假设工作目录是 Skill 目录。

| 环境 | 相对本 Skill 的安装命令 | 程序位置 |
| --- | --- | --- |
| macOS / Linux / WSL | `sh scripts/install.sh` | `~/.local/bin/maolv` |
| Windows | `powershell.exe -NoProfile -ExecutionPolicy Bypass -File scripts/install.ps1` | `%LOCALAPPDATA%\Maolv\bin\maolv.exe` |

安装后使用程序绝对路径，不修改永久 PATH。PowerShell 调用使用 `&`，如 `& "$env:LOCALAPPDATA\Maolv\bin\maolv.exe" version --json`。支持 amd64 / arm64；安装器从 GitHub Releases 下载固定 `v0.1.0`，校验 HTTPS、SHA256、版本及大小，不覆盖现有 CLI，不需要 Go 或管理员权限。macOS/Linux 需要 curl 和 SHA256 工具。下载失败说明真实原因；不得绕过校验或悄悄转为源码编译。此版本没有自更新命令，已有 CLI 不自动升级。

先运行 `maolv project list --json` 验证授权。缺少或失效 Token 时，向用户展示 [API 访问令牌](https://www.maolvdl.com/web/console/api_token) 页面，引导登录并创建 Token。已有 `MAOLV_API_TOKEN` 时直接使用，优先于配置文件；失效时提示更新环境变量，保存另一个 Token 不会覆盖环境变量。

用户可在本机终端运行 `maolv config set-token` 隐藏输入；也可经用户提供后通过标准输入交给 `maolv config set-token --json`。若选择在聊天中提供，先说明 Token 会保留在聊天记录中。不要把 Token 放入命令参数、工具可见输出、日志或仓库，不读取已保存 Token；工具不能安全地传入且不回显时，改用用户本机终端输入。保存成功后再次运行项目查询，确认授权有效才继续原任务。`config status --json` 只显示配置状态。默认使用官网，仅在用户明确要求其他环境时设置 `MAOLV_BASE_URL`。

## 创建与查询

```sh
maolv project list --json
maolv link create https://example.com --name '产品首页' --project '官网' --json
maolv link list --page 1 --per-page 20 --json
maolv link list --project-id 123 --keyword '首页' --json
maolv link show abc123 --json
```

- 保持用户指定 URL、名称、项目和自定义短码。创建参数 `--project` 是项目名称，API 可能创建缺失项目；列表过滤的 `--project-id` 是项目 ID。用户没有指定项目时省略，不自行创建项目。
- 通过 `--short` 传用户指定的短码；未指定时由服务端生成。创建成功通常从 `data.cube` 读取短链结果，向用户返回服务端给出的完整短链，不拼接假定域名。
- 免费账号创建的是有效期 10 分钟的体验链接，返回结果时明确说明期限，不能说是永久链接。会员创建前需在官网绑定域名；如缺少配置，引导到官网控制台处理，不代购会员或自行绑定域名。
- `show` 和 `update` 的位置参数必须是 `short` 字段，不能传记录 ID 或完整 URL。从列表结果提取实际 `short`；不能仅凭 URL 最后一段猜测。
- 列表通常在 `data.cubes`；有多页时继续使用 `--page` 翻页。列表 401 可能是旧服务端未允许 Token 访问该接口，不代表无短链。明确报告错误并核对授权或服务端版本。
- 批量创建逐条执行，保留成功项，汇总失败原因。网络超时可能已经创建成功，先查询核对，不自动重复创建。遇到鉴权、配额或验证错误后停止同类失败重试；限流可有限退避，仍失败则汇报剩余项。

## 修改

```sh
maolv link update abc123 --url https://example.com/new --json
maolv link update abc123 --name '新首页' --json
```

仅修改本次明确要求的字段。修改目标前用 `show` 核对当前短链；用户请求已授权的修改无需重复确认。`update --short 新短码` 可能使原地址失效，仅在用户要求改短码时使用。修改成功后再次 `show` 读取，使用返回 `data.cube.short` 作为查询键。

目标 URL 修改仅支持 `SimpleCube` 类型；其他类型按 API 返回说明处理，不声称已修改。

现有 API 同时修改目标地址及名称/短码时非原子：后续字段校验失败仍可能已经修改目标地址。所有写入失败或超时都先读回状态，不直接重试；修改短码失败时先查原 `short`，成功时查响应的 `data.cube.short`。报告已经生效的字段与失败项，不能把报错等同于全未修改。

完整实现和更新位于 [maolv-cli](https://github.com/jicheng1014/maolv-cli)。CLI 与 Skill 分开更新，更新 Skill 时保留整个目录及安装器。

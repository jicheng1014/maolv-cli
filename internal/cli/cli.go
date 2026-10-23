package cli

import (
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"regexp"
	"strconv"
	"strings"

	"github.com/jicheng1014/maolv-cli/internal/api"
	"github.com/jicheng1014/maolv-cli/internal/config"
)

func Run(args []string, in io.Reader, out, errOut io.Writer, version string) int {
	jsonOutput := false
	clean := make([]string, 0, len(args))
	for _, arg := range args {
		if arg == "--json" {
			jsonOutput = true
		} else {
			clean = append(clean, arg)
		}
	}
	data, err := run(clean, in, errOut, version)
	if err != nil {
		var failure *api.Error
		if !errors.As(err, &failure) {
			failure = &api.Error{Code: "cli_error", Message: "命令执行失败"}
		}
		if jsonOutput {
			_ = json.NewEncoder(out).Encode(map[string]any{"error": failure})
		} else {
			fmt.Fprintln(errOut, "错误:", failure.Message)
			if failure.Details != nil {
				detail, _ := json.Marshal(failure.Details)
				fmt.Fprintln(errOut, string(detail))
			}
		}
		if failure.Code == "usage_error" {
			return 2
		}
		return 1
	}
	if jsonOutput {
		if json.NewEncoder(out).Encode(map[string]any{"data": data}) != nil {
			return 1
		}
		return 0
	}
	if text, ok := data.(string); ok {
		_, err = fmt.Fprintln(out, text)
	} else {
		encoder := json.NewEncoder(out)
		encoder.SetIndent("", "  ")
		err = encoder.Encode(data)
	}
	if err != nil {
		return 1
	}
	return 0
}

func run(args []string, in io.Reader, errOut io.Writer, version string) (any, error) {
	if len(args) == 0 || (len(args) == 1 && (args[0] == "help" || args[0] == "--help" || args[0] == "-h")) {
		return usage, nil
	}
	if len(args) == 1 && args[0] == "version" {
		return map[string]string{"version": version}, nil
	}
	if len(args) >= 2 && (args[len(args)-1] == "--help" || args[len(args)-1] == "-h") {
		switch args[0] {
		case "config", "link", "project":
			return usage, nil
		}
	}
	if len(args) == 2 && args[0] == "config" {
		switch args[1] {
		case "set-token":
			if err := config.Save(in, errOut); err != nil {
				return nil, &api.Error{Code: "config_error", Message: "保存失败：" + err.Error()}
			}
			return map[string]bool{"saved": true}, nil
		case "unset-token":
			if err := config.Unset(); err != nil {
				return nil, &api.Error{Code: "config_error", Message: "无法移除已保存令牌"}
			}
			_, source, _ := config.Token()
			return map[string]any{"removed": true, "environment_override": source == "environment"}, nil
		case "status":
			token, source, err := config.Token()
			if err != nil {
				return nil, &api.Error{Code: "config_error", Message: err.Error()}
			}
			base, err := api.BaseURL()
			if err != nil {
				return nil, err
			}
			return map[string]any{"configured": config.ValidToken(token), "token_source": source, "base_url": base.String()}, nil
		}
	}
	if len(args) == 2 && args[0] == "project" && args[1] == "list" {
		return request(http.MethodGet, "/projects", nil, nil)
	}
	if len(args) < 2 || args[0] != "link" {
		return nil, usageError("未知命令；运行 maolv help 查看用法")
	}
	switch args[1] {
	case "create":
		flags := newFlags("link create")
		name, short, project := flags.String("name", "", "名称"), flags.String("short", "", "自定义短码"), flags.String("project", "", "项目名称；不存在时创建")
		if err := parse(flags, args[2:]); err != nil || flags.NArg() != 1 {
			return nil, usageError("用法: maolv link create URL [--name 名称] [--short 短码] [--project 项目名]")
		}
		if err := validateURL(flags.Arg(0)); err != nil {
			return nil, err
		}
		if err := validateOptional(flags, "name", *name); err != nil {
			return nil, err
		}
		if err := validateOptional(flags, "project", *project); err != nil {
			return nil, err
		}
		if present(flags, "short") && !validShort(*short) {
			return nil, usageError("短码需为 4–50 位字母、数字、下划线，最多含一个中间连字符")
		}
		payload := map[string]any{"links": flags.Arg(0), "category": "Cubes::SimpleCube"}
		if present(flags, "name") {
			payload["cube_name"] = *name
		}
		if present(flags, "short") {
			payload["cube_short"] = *short
		}
		if present(flags, "project") {
			payload["project_name"] = *project
		}
		return request(http.MethodPost, "/cubes", nil, payload)
	case "list":
		flags := newFlags("link list")
		page, perPage, projectID, keyword := flags.Int("page", 1, "页码"), flags.Int("per-page", 10, "每页数量"), flags.Int64("project-id", 0, "项目 ID"), flags.String("keyword", "", "关键词")
		if err := parse(flags, args[2:]); err != nil || flags.NArg() != 0 || *page < 1 || *perPage < 1 || *perPage > 100 || (present(flags, "project-id") && *projectID <= 0) {
			return nil, usageError("用法: maolv link list [--page 正整数] [--per-page 1..100] [--project-id 正整数] [--keyword 关键词]")
		}
		if err := validateOptional(flags, "keyword", *keyword); err != nil {
			return nil, err
		}
		query := url.Values{"page": {strconv.Itoa(*page)}, "per_page": {strconv.Itoa(*perPage)}}
		if *projectID > 0 {
			query.Set("project_id", strconv.FormatInt(*projectID, 10))
		}
		if *keyword != "" {
			query.Set("keyword", *keyword)
		}
		return request(http.MethodGet, "/cubes", query, nil)
	case "show":
		if len(args) != 3 || !validShort(args[2]) {
			return nil, usageError("用法: maolv link show SHORT（仅短码，不是完整短链、ID 或 slug）")
		}
		return request(http.MethodGet, "/cubes/"+args[2], nil, nil)
	case "update":
		flags := newFlags("link update")
		target, name, short := flags.String("url", "", "目标 URL"), flags.String("name", "", "名称"), flags.String("short", "", "新短码")
		if err := parse(flags, args[2:]); err != nil || flags.NArg() != 1 || !validShort(flags.Arg(0)) {
			return nil, usageError("用法: maolv link update SHORT [--url URL] [--name 名称] [--short 新短码]")
		}
		payload := map[string]any{}
		if present(flags, "url") {
			if err := validateURL(*target); err != nil {
				return nil, err
			}
			payload["link"] = *target
		}
		if present(flags, "name") {
			if err := validateOptional(flags, "name", *name); err != nil {
				return nil, err
			}
			payload["name"] = *name
		}
		if present(flags, "short") {
			if !validShort(*short) {
				return nil, usageError("新短码格式无效；运行 maolv help 查看约束")
			}
			payload["short"] = *short
		}
		if len(payload) == 0 {
			return nil, usageError("update 至少指定 --url、--name 或 --short")
		}
		client, err := api.New()
		if err != nil {
			return nil, err
		}
		path := "/cubes/" + flags.Arg(0)
		if present(flags, "url") {
			body, err := client.Do(http.MethodGet, path, nil, nil)
			if err != nil {
				return nil, err
			}
			var current struct {
				Cube struct {
					Category string `json:"category"`
				} `json:"cube"`
			}
			if json.Unmarshal(body, &current) != nil || current.Cube.Category == "" {
				return nil, &api.Error{Code: "response_error", Message: "无法确认短链类型，未执行更新"}
			}
			if current.Cube.Category != "Cubes::SimpleCube" {
				return nil, &api.Error{Code: "unsupported_operation", Message: "--url 仅支持普通短链 Cubes::SimpleCube；复杂跳转请使用网页编辑"}
			}
		}
		return client.Do(http.MethodPost, path+"/fast_modify", nil, payload)
	default:
		return nil, usageError("未知 link 命令；运行 maolv help 查看用法")
	}
}

func request(method, path string, query url.Values, payload map[string]any) (json.RawMessage, error) {
	client, err := api.New()
	if err != nil {
		return nil, err
	}
	return client.Do(method, path, query, payload)
}

func newFlags(name string) *flag.FlagSet {
	f := flag.NewFlagSet(name, flag.ContinueOnError)
	f.SetOutput(io.Discard)
	return f
}

// flag stops at positional arguments; collect options first to support URL --name NAME.
func parse(flags *flag.FlagSet, args []string) error {
	var options, positional []string
	for i := 0; i < len(args); i++ {
		arg := args[i]
		if arg == "--" {
			positional = append(positional, args[i+1:]...)
			break
		}
		if !strings.HasPrefix(arg, "-") {
			positional = append(positional, arg)
			continue
		}
		name := strings.TrimLeft(arg, "-")
		name, _, hasValue := strings.Cut(name, "=")
		if flags.Lookup(name) == nil {
			return fmt.Errorf("unknown flag")
		}
		options = append(options, arg)
		if !hasValue {
			i++
			if i >= len(args) || strings.HasPrefix(args[i], "--") {
				return fmt.Errorf("missing flag value")
			}
			options = append(options, args[i])
		}
	}
	return flags.Parse(append(append(options, "--"), positional...))
}

func present(flags *flag.FlagSet, name string) bool {
	found := false
	flags.Visit(func(f *flag.Flag) {
		if f.Name == name {
			found = true
		}
	})
	return found
}

func validateOptional(flags *flag.FlagSet, name, value string) error {
	if present(flags, name) && (strings.TrimSpace(value) == "" || len(value) > 1000 || strings.ContainsAny(value, "\x00\r\n")) {
		return usageError("--" + name + " 必须是 1–1000 字节的单行文本")
	}
	return nil
}

var shortPattern = regexp.MustCompile(`^[A-Za-z0-9_]+-?[A-Za-z0-9_]+$`)

func validShort(short string) bool {
	return len(short) >= 4 && len(short) <= 50 && shortPattern.MatchString(short)
}

func validateURL(value string) error {
	u, err := url.Parse(value)
	if err != nil || len(value) > 2000 || strings.TrimSpace(value) != value || strings.ContainsAny(value, "\r\n\x00") || u.Hostname() == "" || u.User != nil || (u.Scheme != "http" && u.Scheme != "https") {
		return usageError("URL 必须是有效 HTTP/HTTPS 地址，不含用户信息，且不超过 2000 字节")
	}
	return nil
}

func usageError(message string) error { return &api.Error{Code: "usage_error", Message: message} }

const usage = `maolv - 毛驴短链命令行客户端

用法:
  maolv help
  maolv version [--json]
  maolv config set-token [--json]       交互隐藏输入，或通过标准输入读取
  maolv config status [--json]          仅显示配置状态，不联网验证令牌
  maolv config unset-token [--json]     删除保存的令牌；环境变量需另行清除
  maolv project list [--json]
  maolv link create URL [--name 名称] [--short 短码] [--project 项目名] [--json]
  maolv link list [--page 页码] [--per-page 1..100] [--project-id ID] [--keyword 关键词] [--json]
  maolv link show SHORT [--json]
  maolv link update SHORT [--url URL] [--name 名称] [--short 新短码] [--json]

SHORT 仅接受短码（4–50 位字母、数字、下划线，最多一个中间连字符），不接受完整短链、ID 或 slug。
create --project 按项目名称归类，不存在时创建；省略时使用默认项目。
update --url 仅支持普通短链；复杂跳转请在网页编辑。
--json 可放在任意位置；成功输出 {"data":原始API响应}，失败输出 {"error":{"code":...,"message":...,"details":...}}。
退出码: 0 成功；2 参数错误；1 配置、网络或 API 错误。写入请求不自动重试。

配置:
  MAOLV_API_TOKEN  32 位小写十六进制令牌；优先于保存的配置
  MAOLV_BASE_URL   默认 https://www.maolvdl.com/apic/v1/api

API Token 请在 https://www.maolvdl.com/web/console/api_token 获取。
旧版 API 的 GET 认证使用 api_token 查询参数；CLI 不输出请求 URL 或令牌。
`

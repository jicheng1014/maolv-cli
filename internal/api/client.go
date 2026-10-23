package api

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"os"
	"strings"
	"time"

	"github.com/jicheng1014/maolv-cli/internal/config"
)

const DefaultBaseURL = "https://www.maolvdl.com/apic/v1/api"

type Error struct {
	Code    string `json:"code"`
	Message string `json:"message"`
	Details any    `json:"details,omitempty"`
}

func (e *Error) Error() string { return e.Message }

type Client struct {
	base  *url.URL
	token string
	http  *http.Client
}

func BaseURL() (*url.URL, error) {
	base := os.Getenv("MAOLV_BASE_URL")
	if base == "" {
		base = DefaultBaseURL
	}
	u, err := url.Parse(base)
	if err != nil {
		return nil, &Error{Code: "config_error", Message: "MAOLV_BASE_URL 无效"}
	}
	ip := net.ParseIP(u.Hostname())
	loopback := u.Hostname() == "localhost" || (ip != nil && ip.IsLoopback())
	if u.Hostname() == "" || u.User != nil || u.RawQuery != "" || u.ForceQuery || u.Fragment != "" || u.Opaque != "" || (u.Scheme != "https" && !(u.Scheme == "http" && loopback)) {
		return nil, &Error{Code: "config_error", Message: "MAOLV_BASE_URL 必须是无用户信息、查询参数或片段的 HTTPS API 地址；仅本机允许 HTTP"}
	}
	u.Path = strings.TrimRight(u.Path, "/")
	u.RawPath = ""
	return u, nil
}

func New() (*Client, error) {
	base, err := BaseURL()
	if err != nil {
		return nil, err
	}
	token, _, err := config.Token()
	if err != nil {
		return nil, &Error{Code: "config_error", Message: "无法读取令牌配置，请重新运行 maolv config set-token"}
	}
	if !config.ValidToken(token) {
		return nil, &Error{Code: "config_error", Message: "缺少有效 API Token；请在毛驴控制台 API 设置中创建令牌，再运行 maolv config set-token", Details: map[string]string{"token_url": "https://www.maolvdl.com/web/console/api_token"}}
	}
	return &Client{base: base, token: token, http: &http.Client{Timeout: 20 * time.Second, CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }}}, nil
}

func (c *Client) Do(method, path string, query url.Values, payload map[string]any) (json.RawMessage, error) {
	endpoint := *c.base
	endpoint.Path += path
	if query == nil {
		query = url.Values{}
	}
	var body io.Reader
	if method == http.MethodGet {
		// The existing API authenticates GET requests with api_token parameters.
		query.Set("api_token", c.token)
	} else {
		if payload == nil {
			payload = map[string]any{}
		}
		payload["api_token"] = c.token
		data, err := json.Marshal(payload)
		if err != nil {
			return nil, &Error{Code: "request_error", Message: "请求参数无法编码"}
		}
		body = bytes.NewReader(data)
	}
	endpoint.RawQuery = query.Encode()
	req, err := http.NewRequest(method, endpoint.String(), body)
	if err != nil {
		return nil, &Error{Code: "request_error", Message: "无法创建 API 请求"}
	}
	req.Header.Set("Accept", "application/json")
	req.Header.Set("User-Agent", "maolv-cli")
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	resp, err := c.http.Do(req)
	if err != nil {
		return nil, &Error{Code: "network_error", Message: "API 请求失败或超时；请检查网络和 API 地址（写入请求请先查询结果，再决定是否重试）"}
	}
	defer resp.Body.Close()
	data, err := io.ReadAll(io.LimitReader(resp.Body, (4<<20)+1))
	if err != nil {
		return nil, &Error{Code: "response_error", Message: "无法读取 API 响应；写入请求请先查询结果"}
	}
	if len(data) > 4<<20 {
		return nil, &Error{Code: "response_error", Message: "API 响应超过 4 MiB"}
	}
	var decoded any
	decoder := json.NewDecoder(bytes.NewReader(data))
	decoder.UseNumber()
	valid := json.Valid(data) && decoder.Decode(&decoded) == nil
	if valid {
		decoded = c.redact(decoded)
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		code := "http_error"
		switch resp.StatusCode {
		case 401:
			code = "authentication_failed"
		case 403:
			code = "permission_denied"
		case 404:
			code = "not_found"
		case 422:
			code = "validation_failed"
		case 429:
			code = "rate_limited"
		}
		failure := &Error{Code: code, Message: fmt.Sprintf("API 返回 HTTP %d", resp.StatusCode)}
		if object, ok := decoded.(map[string]any); ok {
			failure.Details = object["error"]
		}
		if resp.StatusCode == 401 {
			if detail, ok := failure.Details.(string); ok && detail == "token is not allowed" {
				failure.Code = "permission_denied"
				failure.Message += "；服务器尚未允许 API Token 使用此命令"
			} else {
				failure.Message += "；请检查或重新配置 API Token"
			}
		}
		if method != http.MethodGet {
			failure.Message += "；请先查询短链确认结果，再决定是否重试"
		}
		return nil, failure
	}
	if !valid || decoded == nil {
		return nil, &Error{Code: "response_error", Message: "API 未返回有效 JSON 数据"}
	}
	if object, ok := decoded.(map[string]any); ok {
		if detail, exists := object["error"]; exists && detail != nil {
			return nil, &Error{Code: "api_error", Message: "API 返回错误", Details: detail}
		}
		if code, ok := object["code"].(json.Number); ok && code != "200" && code != "201" {
			return nil, &Error{Code: "api_error", Message: "API 返回失败结果", Details: object}
		}
	}
	cleaned, err := json.Marshal(decoded)
	return cleaned, err
}

func (c *Client) redact(value any) any {
	switch v := value.(type) {
	case string:
		return strings.ReplaceAll(v, c.token, "[REDACTED]")
	case []any:
		for i := range v {
			v[i] = c.redact(v[i])
		}
		return v
	case map[string]any:
		clean := make(map[string]any, len(v))
		for key, item := range v {
			safeKey := strings.ReplaceAll(key, c.token, "[REDACTED]")
			if strings.Contains(strings.ToLower(key), "token") {
				clean[safeKey] = "[REDACTED]"
			} else {
				clean[safeKey] = c.redact(item)
			}
		}
		return clean
	default:
		return value
	}
}

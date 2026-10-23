package cli

import (
	"bytes"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/jicheng1014/maolv-cli/internal/config"
)

const testToken = "0123456789abcdef0123456789abcdef"

func invoke(t *testing.T, args ...string) (int, string, string) {
	t.Helper()
	var out, errOut bytes.Buffer
	code := Run(args, strings.NewReader(""), &out, &errOut, "v0.1.0")
	if strings.Contains(out.String()+errOut.String(), testToken) {
		t.Fatal("token leaked")
	}
	return code, out.String(), errOut.String()
}

func server(t *testing.T, handler http.HandlerFunc) {
	t.Helper()
	s := httptest.NewServer(handler)
	t.Cleanup(s.Close)
	t.Setenv("MAOLV_BASE_URL", s.URL+"/apic/v1/api")
	t.Setenv("MAOLV_API_TOKEN", testToken)
}

func jsonResponse(w http.ResponseWriter, body string) {
	w.Header().Set("Content-Type", "application/json")
	io.WriteString(w, body)
}

func TestCreateAndListContract(t *testing.T) {
	calls := 0
	server(t, func(w http.ResponseWriter, r *http.Request) {
		calls++
		if r.URL.Path != "/apic/v1/api/cubes" {
			t.Errorf("path %s", r.URL.Path)
		}
		if r.Header.Get("Authorization") != "" {
			t.Error("legacy token must not be mistaken for JWT")
		}
		switch r.Method {
		case "POST":
			if r.URL.RawQuery != "" {
				t.Error("write token must be in body")
			}
			var body map[string]any
			json.NewDecoder(r.Body).Decode(&body)
			want := map[string]any{"api_token": testToken, "links": "https://example.com/a?q=1", "cube_name": "活动", "cube_short": "camp2026", "project_name": "投放", "category": "Cubes::SimpleCube"}
			for k, v := range want {
				if body[k] != v {
					t.Errorf("incorrect %s", k)
				}
			}
			jsonResponse(w, `{"cube":{"short":"camp2026","category":"Cubes::SimpleCube"}}`)
		case "GET":
			query := r.URL.Query()
			if query.Get("api_token") != testToken || query.Get("page") != "2" || query.Get("per_page") != "25" || query.Get("project_id") != "3" || query.Get("keyword") != "活动" {
				t.Error("incorrect query mapping")
			}
			jsonResponse(w, `{"cubes":[],"total_count":0,"total_pages":0,"current_page":2}`)
		default:
			t.Errorf("unexpected method %s", r.Method)
		}
	})
	code, out, _ := invoke(t, "link", "create", "https://example.com/a?q=1", "--name", "活动", "--short=camp2026", "--project", "投放", "--json")
	if code != 0 || !strings.Contains(out, `"data":{"cube"`) {
		t.Fatalf("create: %d %s", code, out)
	}
	code, out, _ = invoke(t, "--json", "link", "list", "--page", "2", "--per-page", "25", "--project-id", "3", "--keyword", "活动")
	if code != 0 || !strings.Contains(out, `"current_page":2`) || calls != 2 {
		t.Fatalf("list: %d %s calls=%d", code, out, calls)
	}
}

func TestUpdateGuardsAndMapping(t *testing.T) {
	for _, category := range []string{"Cubes::SimpleCube", "Cubes::TimeCube"} {
		t.Run(category, func(t *testing.T) {
			reads, writes := 0, 0
			server(t, func(w http.ResponseWriter, r *http.Request) {
				if r.Method == "GET" {
					reads++
					if r.URL.Path != "/apic/v1/api/cubes/camp2026" || r.URL.Query().Get("api_token") != testToken {
						t.Error("show mapping")
					}
					jsonResponse(w, `{"cube":{"category":"`+category+`"}}`)
				} else {
					writes++
					if r.Method != "POST" || r.URL.Path != "/apic/v1/api/cubes/camp2026/fast_modify" {
						t.Error("update route")
					}
					var body map[string]any
					json.NewDecoder(r.Body).Decode(&body)
					if body["link"] != "https://example.com/new" || body["name"] != "新名" || body["short"] != "new2026" || body["api_token"] != testToken {
						t.Error("update payload")
					}
					jsonResponse(w, `{"cube":{"short":"new2026"}}`)
				}
			})
			code, out, _ := invoke(t, "link", "update", "camp2026", "--url", "https://example.com/new", "--name", "新名", "--short", "new2026", "--json")
			if reads != 1 {
				t.Errorf("reads=%d", reads)
			}
			if category == "Cubes::SimpleCube" {
				if code != 0 || writes != 1 {
					t.Fatalf("simple: %d %s writes=%d", code, out, writes)
				}
			} else if code != 1 || writes != 0 || !strings.Contains(out, "unsupported_operation") {
				t.Fatalf("complex: %d %s writes=%d", code, out, writes)
			}
		})
	}
}

func TestValidationBeforeNetwork(t *testing.T) {
	server(t, func(w http.ResponseWriter, r *http.Request) { t.Error("invalid input reached API") })
	cases := [][]string{
		{"link", "create", "javascript:alert(1)"}, {"link", "create", "https://user:pass@example.com"},
		{"link", "create", "https://example.com", "--short", "bad/short"}, {"link", "create", "https://example.com", "--name", ""},
		{"link", "create", "https://example.com", "--project-id", "4"}, {"link", "show", "https://m.example/abcd"},
		{"link", "update", "camp2026"}, {"link", "update", "camp2026", "--url", ""}, {"link", "list", "--page", "0"},
		{"link", "list", "--per-page", "101"}, {"link", "list", "--project-id", "0"}, {"project", "list", "unexpected"},
		{"version", "unexpected"}, {"config", "set-token", testToken}, {"link", "create", "https://example.com", "--name"},
	}
	for _, args := range cases {
		code, out, _ := invoke(t, append(args, "--json")...)
		if code != 2 || !strings.Contains(out, `"usage_error"`) {
			t.Errorf("args %v: %d %s", args, code, out)
		}
	}
}

func TestErrorsRedactAndDoNotFollowRedirect(t *testing.T) {
	for _, test := range []struct {
		status     int
		body, code string
	}{
		{401, `{"error":"token is not allowed"}`, "permission_denied"},
		{401, `{"error":"api token is error"}`, "authentication_failed"},
		{422, `{"error":{"token":"` + testToken + `","name":["bad ` + testToken + `"]}}`, "validation_failed"},
		{200, `{"error":"bad ` + testToken + `"}`, "api_error"},
		{200, `{"code":400,"msg":"invalid"}`, "api_error"},
		{200, `<!doctype html>`, "response_error"},
		{302, ``, "http_error"},
	} {
		t.Run(test.code+strconvStatus(test.status), func(t *testing.T) {
			calls := 0
			server(t, func(w http.ResponseWriter, r *http.Request) {
				calls++
				w.Header().Set("Location", "/redirected")
				w.WriteHeader(test.status)
				io.WriteString(w, test.body)
			})
			code, out, _ := invoke(t, "project", "list", "--json")
			if code != 1 || !strings.Contains(out, `"code":"`+test.code+`"`) || calls != 1 {
				t.Fatalf("%d %s calls=%d", code, out, calls)
			}
		})
	}
}

func strconvStatus(status int) string { data, _ := json.Marshal(status); return string(data) }

func TestNetworkErrorDoesNotLeakURLToken(t *testing.T) {
	s := httptest.NewServer(http.HandlerFunc(func(http.ResponseWriter, *http.Request) {}))
	s.Close()
	t.Setenv("MAOLV_BASE_URL", s.URL)
	t.Setenv("MAOLV_API_TOKEN", testToken)
	code, out, _ := invoke(t, "project", "list", "--json")
	if code != 1 || !strings.Contains(out, "network_error") || strings.Contains(out, s.URL) {
		t.Fatalf("%d %s", code, out)
	}
}

func TestConfigTokenLifecycle(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("XDG_CONFIG_HOME", home)
	t.Setenv("AppData", home)
	t.Setenv("MAOLV_BASE_URL", "https://www.maolvdl.com/apic/v1/api")
	original, exists := os.LookupEnv("MAOLV_API_TOKEN")
	os.Unsetenv("MAOLV_API_TOKEN")
	t.Cleanup(func() {
		if exists {
			os.Setenv("MAOLV_API_TOKEN", original)
		} else {
			os.Unsetenv("MAOLV_API_TOKEN")
		}
	})
	var out, errOut bytes.Buffer
	if code := Run([]string{"config", "set-token", "--json"}, strings.NewReader(testToken+"\n"), &out, &errOut, "dev"); code != 0 {
		t.Fatalf("save: %d %s", code, errOut.String())
	}
	path, err := config.Path()
	if err != nil {
		t.Fatal(err)
	}
	info, err := os.Stat(path)
	if err != nil {
		t.Fatal(err)
	}
	if info.Mode().Perm() != 0600 {
		t.Fatalf("config mode %v", info.Mode())
	}
	token, source, err := config.Token()
	if err != nil || token != testToken || source != "file" {
		t.Fatal("saved token unavailable")
	}
	code, text, _ := invoke(t, "config", "status", "--json")
	if code != 0 || !strings.Contains(text, `"configured":true`) {
		t.Fatalf("status: %d %s", code, text)
	}
	t.Setenv("MAOLV_API_TOKEN", strings.Repeat("a", 32))
	token, source, err = config.Token()
	if err != nil || source != "environment" || token != strings.Repeat("a", 32) {
		t.Fatal("environment precedence")
	}
	code, text, _ = invoke(t, "config", "unset-token", "--json")
	if code != 0 || !strings.Contains(text, `"environment_override":true`) {
		t.Fatalf("unset: %d %s", code, text)
	}
	if _, err = os.Stat(path); !os.IsNotExist(err) {
		t.Fatal("config still present")
	}
	if _, err = os.Stat(filepath.Dir(path)); err != nil {
		t.Fatal(err)
	}
}

func TestBaseURLValidation(t *testing.T) {
	for _, base := range []string{"http://example.com/api", "https://user:secret@example.com/api", "https://example.com/api?token=x", "https://example.com/api#fragment", "ftp://example.com/api"} {
		t.Setenv("MAOLV_BASE_URL", base)
		code, out, _ := invoke(t, "config", "status", "--json")
		if code != 1 || !strings.Contains(out, "config_error") || strings.Contains(out, "secret") {
			t.Errorf("base %s: %d %s", base, code, out)
		}
	}
}

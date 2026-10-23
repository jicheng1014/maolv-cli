package config

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"golang.org/x/term"
)

type fileConfig struct {
	Token string `json:"token"`
}

func Path() (string, error) {
	dir, err := os.UserConfigDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(dir, "maolv", "config.json"), nil
}

func Token() (token, source string, err error) {
	if token, exists := os.LookupEnv("MAOLV_API_TOKEN"); exists {
		return token, "environment", nil
	}
	path, err := Path()
	if err != nil {
		return "", "", err
	}
	data, err := os.ReadFile(path)
	if errors.Is(err, os.ErrNotExist) {
		return "", "none", nil
	}
	if err != nil {
		return "", "", fmt.Errorf("无法读取令牌配置")
	}
	var saved fileConfig
	if json.Unmarshal(data, &saved) != nil {
		return "", "", fmt.Errorf("令牌配置格式无效")
	}
	return saved.Token, "file", nil
}

func ValidToken(token string) bool {
	if len(token) != 32 {
		return false
	}
	for _, c := range token {
		if (c < '0' || c > '9') && (c < 'a' || c > 'f') {
			return false
		}
	}
	return true
}

func Save(in io.Reader, prompt io.Writer) error {
	var data []byte
	var err error
	if file, ok := in.(*os.File); ok && term.IsTerminal(int(file.Fd())) {
		fmt.Fprint(prompt, "毛驴 API Token: ")
		data, err = term.ReadPassword(int(file.Fd()))
		fmt.Fprintln(prompt)
	} else {
		data, err = io.ReadAll(io.LimitReader(in, 129))
	}
	if err != nil {
		return fmt.Errorf("读取令牌失败")
	}
	token := strings.TrimSpace(string(data))
	if len(data) > 128 || !ValidToken(token) {
		return fmt.Errorf("API Token 必须是 32 位小写十六进制字符")
	}
	path, err := Path()
	if err != nil {
		return err
	}
	if err = os.MkdirAll(filepath.Dir(path), 0700); err != nil {
		return err
	}
	tmp, err := os.CreateTemp(filepath.Dir(path), ".config-*")
	if err != nil {
		return err
	}
	defer os.Remove(tmp.Name())
	if err = tmp.Chmod(0600); err != nil {
		tmp.Close()
		return err
	}
	if err = json.NewEncoder(tmp).Encode(fileConfig{Token: token}); err != nil {
		tmp.Close()
		return err
	}
	if err = tmp.Close(); err != nil {
		return err
	}
	return os.Rename(tmp.Name(), path)
}

func Unset() error {
	path, err := Path()
	if err != nil {
		return err
	}
	err = os.Remove(path)
	if errors.Is(err, os.ErrNotExist) {
		return nil
	}
	return err
}

package codexlaunch

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"
)

// ReadDeveloper resolves trusted project/account/CLI layers through Codex's
// config/read API. No model turn or thread is created by this exchange.
func ReadDeveloper(parent context.Context, binary string, args []string) (string, error) {
	ctx, cancel := context.WithTimeout(parent, 15*time.Second)
	defer cancel()
	cwd, err := os.Getwd()
	if err != nil {
		return "", fmt.Errorf("resolve Codex launch cwd: %w", err)
	}
	options, cwd, err := configOptions(args, cwd)
	if err != nil {
		return "", err
	}
	options, queryHome, cleanup, err := readerHome(options)
	if err != nil {
		return "", err
	}
	defer cleanup()
	cmd := exec.CommandContext(ctx, binary, append(options, "app-server")...)
	if queryHome != "" {
		for _, entry := range os.Environ() {
			if !strings.HasPrefix(entry, "CODEX_HOME=") {
				cmd.Env = append(cmd.Env, entry)
			}
		}
		cmd.Env = append(cmd.Env, "CODEX_HOME="+queryHome)
	}
	stdin, err := cmd.StdinPipe()
	if err != nil {
		return "", err
	}
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return "", err
	}
	var stderr bytes.Buffer
	cmd.Stderr = &stderr
	if err := cmd.Start(); err != nil {
		return "", fmt.Errorf("start Codex config reader: %w", err)
	}
	defer func() { _ = stdin.Close(); cancel(); _ = cmd.Wait() }()
	requests := []any{
		map[string]any{"id": 0, "method": "initialize", "params": map[string]any{"clientInfo": map[string]string{"name": "professor", "version": "1"}}},
		map[string]any{"method": "initialized", "params": nil},
		map[string]any{"id": 1, "method": "config/read", "params": map[string]any{"cwd": cwd}},
	}
	encoder := json.NewEncoder(stdin)
	for _, request := range requests {
		if err := encoder.Encode(request); err != nil {
			return "", fmt.Errorf("write Codex config request: %w", err)
		}
	}
	scanner := bufio.NewScanner(stdout)
	scanner.Buffer(make([]byte, 65536), 1<<20)
	for scanner.Scan() {
		var response struct {
			ID     *int            `json:"id"`
			Error  json.RawMessage `json:"error"`
			Result *struct {
				Config *struct {
					Developer       *string `json:"developer_instructions"`
					CredentialStore string  `json:"cli_auth_credentials_store"`
				} `json:"config"`
			} `json:"result"`
		}
		if err := json.Unmarshal(scanner.Bytes(), &response); err != nil {
			return "", fmt.Errorf("decode Codex config response: %w", err)
		}
		if response.ID == nil {
			continue
		}
		if len(response.Error) > 0 && string(response.Error) != "null" {
			return "", fmt.Errorf("Codex config reader rejected request %d: %s", *response.ID, response.Error)
		}
		if *response.ID != 1 {
			continue
		}
		if response.Result == nil || response.Result.Config == nil {
			return "", fmt.Errorf("Codex config reader returned no config")
		}
		if queryHome != "" && response.Result.Config.CredentialStore != "" && response.Result.Config.CredentialStore != "file" {
			return "", fmt.Errorf("Professor profile/ignore-user-config appendix requires file credential storage; %s is unsupported because a temporary config-reader home changes keyring identity", response.Result.Config.CredentialStore)
		}
		if response.Result.Config.Developer == nil {
			return "", nil
		}
		return *response.Result.Config.Developer, nil
	}
	if err := scanner.Err(); err != nil {
		return "", fmt.Errorf("read Codex config response: %w", err)
	}
	if err := ctx.Err(); err != nil {
		return "", fmt.Errorf("Codex config reader: %w", err)
	}
	return "", fmt.Errorf("Codex config reader ended without a response: %s", strings.TrimSpace(stderr.String()))
}

func configOptions(args []string, cwd string) ([]string, string, error) {
	var result []string
	baseCwd := cwd
	for i := 0; i < len(args); i++ {
		arg := args[i]
		if arg == "--" {
			break
		}
		name, value, inline := strings.Cut(arg, "=")
		if !strings.HasPrefix(arg, "--") && len(arg) > 2 {
			switch arg[:2] {
			case "-c", "-p", "-C":
				name, value, inline = arg[:2], strings.TrimPrefix(arg[2:], "="), true
			}
		}
		switch name {
		case "-c", "--config", "-p", "--profile", "--enable", "--disable", "-C", "--cd":
			if !inline {
				i++
				if i == len(args) {
					return nil, "", fmt.Errorf("missing value for %s", name)
				}
				value = args[i]
			}
			if name == "-C" || name == "--cd" {
				if filepath.IsAbs(value) {
					cwd = value
				} else {
					cwd = filepath.Join(baseCwd, value)
				}
			} else {
				result = append(result, name, value)
			}
		case "--strict-config", "--ignore-user-config":
			result = append(result, arg)
		}
	}
	return result, cwd, nil
}

package statusline

import (
	"context"
	"encoding/json"
	"io"
	"os"
	"os/exec"
	"strings"
	"testing"
	"time"
)

func TestAppServerHandshakeCarriesJSONRPCAndReadsIDOne(t *testing.T) {
	command := exec.CommandContext(
		context.Background(),
		os.Args[0],
		"-test.run=TestGPTAppServerFixture",
		"--",
		"app-server",
	)
	command.Env = append(os.Environ(), "PFM_GPT_APP_SERVER_FIXTURE=1")
	body, err := readGPTRateLimitsCommand(command)
	if err != nil {
		t.Fatal(err)
	}
	parsed, err := parseGPTRateLimits(body)
	if err != nil || parsed["planType"] != "plus" {
		t.Fatalf("parsed=%#v err=%v", parsed, err)
	}
}

func TestGPTAppServerFixture(t *testing.T) {
	if os.Getenv("PFM_GPT_APP_SERVER_FIXTURE") != "1" {
		return
	}
	body, err := io.ReadAll(os.Stdin)
	if err != nil {
		os.Exit(91)
	}
	lines := strings.Split(strings.TrimSpace(string(body)), "\n")
	if len(lines) != 3 {
		os.Exit(92)
	}
	for _, line := range lines {
		var message map[string]any
		if json.Unmarshal([]byte(line), &message) != nil || message["jsonrpc"] != "2.0" {
			os.Exit(93)
		}
	}
	_, _ = io.WriteString(os.Stdout, `{"jsonrpc":"2.0","method":"configWarning","params":{}}`+"\n")
	_, _ = io.WriteString(os.Stdout, `{"jsonrpc":"2.0","id":1,"result":{"rateLimits":{"primary":{"usedPercent":10},"planType":"plus"}}}`+"\n")
	time.Sleep(10 * time.Second)
	os.Exit(0)
}

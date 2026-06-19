package tests

import (
	"bufio"
	"os"
	"strings"
	"sync"
	"testing"

	"github.com/gioeba/go_sdk_test/constants"
)

var dotEnvOnce sync.Once

func loadDotEnv(paths ...string) {
	for _, p := range paths {
		f, err := os.Open(p)
		if err != nil {
			continue
		}
		scanner := bufio.NewScanner(f)
		for scanner.Scan() {
			line := strings.TrimSpace(scanner.Text())
			if line == "" || strings.HasPrefix(line, "#") {
				continue
			}
			key, val, ok := strings.Cut(line, "=")
			if !ok {
				continue
			}
			key = strings.TrimSpace(key)
			val = strings.Trim(strings.TrimSpace(val), `"'`)
			if _, exists := os.LookupEnv(key); !exists {
				os.Setenv(key, val)
			}
		}
		f.Close()
		return
	}
}

func requireLive(t *testing.T) {
	t.Helper()
	dotEnvOnce.Do(func() { loadDotEnv("../.env", ".env") })
	if os.Getenv("HINKAL_LIVE") == "" {
		t.Skip("set HINKAL_LIVE=1 to run the live snapshot-server test")
	}
	constants.Mode = constants.DeploymentModeProduction
}

//go:build integration

package tests

import (
	"fmt"
	"net"
	"net/http"
	"os"
	"os/exec"
	"strings"
	"testing"
	"time"
)

func startOpenProjectContainer(t *testing.T) (baseURL, containerName string, err error) {
	t.Helper()

	hostPort, err := freeLocalPort()
	if err != nil {
		return "", "", err
	}

	secretKeyBase := os.Getenv("OPENPROJECT_BRIDGE_ITEST_SECRET_KEY_BASE")
	if secretKeyBase == "" {
		secretKeyBase = "opbridge-itest-secret-key-base-please-change-me-0123456789abcdef"
	}

	containerName = fmt.Sprintf("openproject-bridge-itest-%d", time.Now().UnixNano())
	args := []string{
		"docker", "run", "--rm", "-d",
		"--name", containerName,
		"-p", fmt.Sprintf("%d:80", hostPort),
		"-e", "OPENPROJECT_HTTPS=false",
		"-e", fmt.Sprintf("OPENPROJECT_HOST__NAME=localhost:%d", hostPort),
		"-e", "OPENPROJECT_DEFAULT__LANGUAGE=en",
		"-e", fmt.Sprintf("SECRET_KEY_BASE=%s", secretKeyBase),
		"openproject/openproject:17",
	}
	if err := runCmd(args); err != nil {
		return "", "", err
	}

	t.Cleanup(func() {
		_ = exec.Command("docker", "rm", "-f", containerName).Run()
	})

	baseURL = fmt.Sprintf("http://localhost:%d", hostPort)
	waitForOpenProject(t, baseURL)
	return baseURL, containerName, nil
}

func bootstrapOpenProjectAdmin(t *testing.T, container, login, password string) (apiKey, effectivePassword string, err error) {
	t.Helper()
	if password == "" {
		password = "admin"
	}

	ruby := `
u = User.find_by!(login: ENV.fetch("OPBRIDGE_LOGIN"))
pass = ENV.fetch("OPBRIDGE_PASSWORD")
u.password = pass
u.password_confirmation = pass
u.force_password_change = false
u.first_login = false
u.consented_at = Time.current
u.save!(validate: false)
Token::API.where(user: u).find_each(&:destroy)
t = Token::API.create!(user: u, data: { token_name: "opbridge-itest" })
puts "TOKEN=#{t.plain_value}"
`

	cmd := exec.Command(
		"docker", "exec",
		"-u", "app",
		"-e", "OPBRIDGE_LOGIN="+login,
		"-e", "OPBRIDGE_PASSWORD="+password,
		container,
		"bash", "-lc", "cd /app && bundle exec rails runner "+shellQuote(ruby),
	)
	out, err := cmd.CombinedOutput()
	if err != nil {
		return "", "", fmt.Errorf("rails runner failed: %w\n%s", err, strings.TrimSpace(string(out)))
	}
	for _, line := range strings.Split(string(out), "\n") {
		line = strings.TrimSpace(line)
		if strings.HasPrefix(line, "TOKEN=") {
			return strings.TrimPrefix(line, "TOKEN="), password, nil
		}
	}
	return "", "", fmt.Errorf("TOKEN= not found in rails runner output:\n%s", strings.TrimSpace(string(out)))
}

func resolveItestOpenProject(t *testing.T) (baseURL, apiKey string) {
	t.Helper()

	baseURL = strings.TrimSpace(os.Getenv("OPENPROJECT_BRIDGE_ITEST_URL"))
	if baseURL == "" && os.Getenv("OPENPROJECT_BRIDGE_ITEST_DOCKER") != "1" {
		t.Skip("set OPENPROJECT_BRIDGE_ITEST_URL or OPENPROJECT_BRIDGE_ITEST_DOCKER=1")
	}

	adminUser := strings.TrimSpace(os.Getenv("OPENPROJECT_BRIDGE_ITEST_USERNAME"))
	adminPass := os.Getenv("OPENPROJECT_BRIDGE_ITEST_PASSWORD")
	if adminUser == "" {
		adminUser = "admin"
	}
	if adminPass == "" {
		adminPass = "admin"
	}

	apiKey = strings.TrimSpace(os.Getenv("OPENPROJECT_BRIDGE_ITEST_API_KEY"))
	var container string

	if baseURL == "" {
		var err error
		baseURL, container, err = startOpenProjectContainer(t)
		if err != nil {
			t.Fatalf("start OpenProject: %v", err)
		}
	}

	if apiKey == "" && os.Getenv("OPENPROJECT_BRIDGE_ITEST_CREATE_API_KEY") == "1" {
		if container == "" {
			t.Fatal("OPENPROJECT_BRIDGE_ITEST_CREATE_API_KEY=1 requires Docker mode")
		}
		created, _, err := bootstrapOpenProjectAdmin(t, container, adminUser, adminPass)
		if err != nil {
			t.Fatalf("bootstrap API key: %v", err)
		}
		apiKey = created
	}
	if apiKey == "" {
		t.Skip("set OPENPROJECT_BRIDGE_ITEST_API_KEY or OPENPROJECT_BRIDGE_ITEST_CREATE_API_KEY=1 with Docker")
	}
	return baseURL, apiKey
}

func shellQuote(s string) string {
	return `'` + strings.ReplaceAll(s, `'`, `'\''`) + `'`
}

func waitForOpenProject(t *testing.T, baseURL string) {
	t.Helper()
	client := &http.Client{Timeout: 5 * time.Second}
	deadline := time.Now().Add(8 * time.Minute)
	for time.Now().Before(deadline) {
		resp, err := client.Get(baseURL + "/api/v3/projects")
		if err == nil && (resp.StatusCode == http.StatusOK || resp.StatusCode == http.StatusUnauthorized) {
			resp.Body.Close()
			time.Sleep(3 * time.Second)
			return
		}
		if resp != nil {
			resp.Body.Close()
		}
		time.Sleep(2 * time.Second)
	}
	t.Fatalf("OpenProject did not become ready: %s", baseURL)
}

func freeLocalPort() (int, error) {
	l, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		return 0, err
	}
	defer l.Close()
	return l.Addr().(*net.TCPAddr).Port, nil
}

func runCmd(args []string) error {
	cmd := exec.Command(args[0], args[1:]...)
	out, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("%s failed: %w\n%s", args[0], err, strings.TrimSpace(string(out)))
	}
	return nil
}

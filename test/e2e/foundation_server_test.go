//go:build e2e

package e2e_test

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"testing"
	"time"
)

const (
	e2eCommandTimeout = 2 * time.Minute
	e2eStartupTimeout = 20 * time.Second
)

type serverHTTPResponse struct {
	statusCode  int
	contentType string
	body        []byte
}

func TestFoundationServerImage(t *testing.T) {
	requireDockerEngine(t)

	repositoryRoot := findRepositoryRoot(t)
	resourceSuffix := strconv.Itoa(os.Getpid()) + "-" + strconv.FormatInt(time.Now().UnixNano(), 10)
	imageName := "traicr-server:e2e-" + resourceSuffix

	runDockerCommand(t, e2eCommandTimeout,
		"build",
		"--file", filepath.Join(repositoryRoot, "Dockerfile"),
		"--build-arg", "VERSION=v0.1.0-e2e",
		"--build-arg", "COMMIT=0123456789ab",
		"--build-arg", "BUILD_DATE=2026-08-22T10:00:00Z",
		"--tag", imageName,
		repositoryRoot,
	)
	t.Cleanup(func() {
		runDockerCleanupCommand(t, "image", "rm", "--force", imageName)
	})

	t.Run("contains deterministic release metadata", func(t *testing.T) {
		output := runDockerCommand(t, e2eCommandTimeout, "run", "--rm", imageName, "version")
		want := "traicr-server v0.1.0-e2e (commit 0123456789ab, built 2026-08-22T10:00:00Z)"
		if strings.TrimSpace(output) != want {
			t.Fatalf("version output = %q, want %q", strings.TrimSpace(output), want)
		}
	})

	t.Run("configures a non-root runtime user", func(t *testing.T) {
		output := runDockerCommand(t, e2eCommandTimeout, "image", "inspect", "--format", "{{.Config.User}}", imageName)
		if strings.TrimSpace(output) != "10001:10001" {
			t.Fatalf("image user = %q, want 10001:10001", strings.TrimSpace(output))
		}
	})

	t.Run("rejects startup without an admin token", func(t *testing.T) {
		output, err := executeCommand(e2eCommandTimeout, nil, "docker", "run", "--rm", imageName)
		if err == nil {
			t.Fatal("docker run succeeded without TRAICR_ADMIN_TOKEN")
		}
		if !strings.Contains(output, "server configuration: TRAICR_ADMIN_TOKEN is required") {
			t.Fatalf("startup output = %q, want missing admin token error", output)
		}
	})

	t.Run("serves health from a read-only container and stops gracefully", func(t *testing.T) {
		containerName := "traicr-server-e2e-" + resourceSuffix
		volumeName := "traicr-data-e2e-" + resourceSuffix

		runDockerCommand(t, e2eCommandTimeout, "volume", "create", volumeName)
		t.Cleanup(func() {
			runDockerCleanupCommand(t, "container", "rm", "--force", containerName)
			runDockerCleanupCommand(t, "volume", "rm", "--force", volumeName)
		})

		runDockerCommand(t, e2eCommandTimeout,
			"run",
			"--detach",
			"--name", containerName,
			"--read-only",
			"--env", "TRAICR_ADMIN_TOKEN=e2e-admin-token",
			"--mount", "type=volume,src="+volumeName+",dst=/data",
			"--publish", "127.0.0.1::8080",
			imageName,
		)

		publishedAddress := strings.TrimSpace(runDockerCommand(
			t,
			e2eCommandTimeout,
			"port",
			containerName,
			"8080/tcp",
		))
		if _, _, err := net.SplitHostPort(publishedAddress); err != nil {
			t.Fatalf("published address %q is invalid: %v", publishedAddress, err)
		}
		serverURL := "http://" + publishedAddress

		healthResponse := waitForServerHealth(t, serverURL+"/healthz", containerName)
		if healthResponse.contentType != "application/json" {
			t.Errorf("health Content-Type = %q, want application/json", healthResponse.contentType)
		}
		var healthBody struct {
			Status string `json:"status"`
		}
		if err := json.Unmarshal(healthResponse.body, &healthBody); err != nil {
			t.Fatalf("decode health response %q: %v", healthResponse.body, err)
		}
		if healthBody.Status != "ok" {
			t.Errorf("health status = %q, want ok", healthBody.Status)
		}

		rootResponse := requestServer(t, http.MethodGet, serverURL+"/")
		if rootResponse.statusCode != http.StatusNotFound {
			t.Errorf("GET / status = %d, want %d", rootResponse.statusCode, http.StatusNotFound)
		}

		postHealthResponse := requestServer(t, http.MethodPost, serverURL+"/healthz")
		if postHealthResponse.statusCode != http.StatusMethodNotAllowed {
			t.Errorf("POST /healthz status = %d, want %d", postHealthResponse.statusCode, http.StatusMethodNotAllowed)
		}

		runDockerCommand(t, e2eCommandTimeout, "stop", "--time", "10", containerName)
		exitCode := strings.TrimSpace(runDockerCommand(
			t,
			e2eCommandTimeout,
			"container",
			"inspect",
			"--format", "{{.State.ExitCode}}",
			containerName,
		))
		if exitCode != "0" {
			logs := runDockerCommand(t, e2eCommandTimeout, "logs", containerName)
			t.Fatalf("container exit code = %q, want 0; logs:\n%s", exitCode, logs)
		}
	})
}

func TestFoundationComposeConfiguration(t *testing.T) {
	requireDockerEngine(t)
	if output, err := executeCommand(e2eCommandTimeout, nil, "docker", "compose", "version"); err != nil {
		t.Fatalf("Docker Compose is required for end-to-end tests: %v\n%s", err, output)
	}

	repositoryRoot := findRepositoryRoot(t)
	composePath := filepath.Join(repositoryRoot, "compose.yaml")
	environmentWithoutToken := removeEnvironmentVariable(os.Environ(), "TRAICR_ADMIN_TOKEN")

	output, err := executeCommand(
		e2eCommandTimeout,
		environmentWithoutToken,
		"docker", "compose", "--file", composePath, "config",
	)
	if err == nil {
		t.Fatal("docker compose config succeeded without TRAICR_ADMIN_TOKEN")
	}
	if !strings.Contains(output, "TRAICR_ADMIN_TOKEN must be set") {
		t.Fatalf("compose output = %q, want missing admin token error", output)
	}

	configuredEnvironment := append(environmentWithoutToken, "TRAICR_ADMIN_TOKEN=e2e-admin-token")
	output, err = executeCommand(
		e2eCommandTimeout,
		configuredEnvironment,
		"docker", "compose", "--file", composePath, "config",
	)
	if err != nil {
		t.Fatalf("docker compose config failed: %v\n%s", err, output)
	}
	for _, requiredConfiguration := range []string{
		"read_only: true",
		"no-new-privileges:true",
		"target: /data",
	} {
		if !strings.Contains(output, requiredConfiguration) {
			t.Errorf("compose configuration does not contain %q:\n%s", requiredConfiguration, output)
		}
	}
}

func requireDockerEngine(t *testing.T) {
	t.Helper()
	if output, err := executeCommand(15*time.Second, nil, "docker", "version"); err != nil {
		if os.Getenv("CI") != "" {
			t.Fatalf("Docker is required for end-to-end tests in CI: %v\n%s", err, output)
		}
		t.Skipf("Docker is unavailable; skipping end-to-end test: %v\n%s", err, output)
	}
}

func findRepositoryRoot(t *testing.T) string {
	t.Helper()
	_, currentFile, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("cannot locate end-to-end test source")
	}
	return filepath.Clean(filepath.Join(filepath.Dir(currentFile), "..", ".."))
}

func runDockerCommand(t *testing.T, timeout time.Duration, arguments ...string) string {
	t.Helper()
	output, err := executeCommand(timeout, nil, "docker", arguments...)
	if err != nil {
		t.Fatalf("docker %s failed: %v\n%s", strings.Join(arguments, " "), err, output)
	}
	return output
}

func runDockerCleanupCommand(t *testing.T, arguments ...string) {
	t.Helper()
	output, err := executeCommand(30*time.Second, nil, "docker", arguments...)
	if err != nil && !strings.Contains(output, "No such") {
		t.Errorf("end-to-end cleanup docker %s failed: %v\n%s", strings.Join(arguments, " "), err, output)
	}
}

func executeCommand(timeout time.Duration, environment []string, name string, arguments ...string) (string, error) {
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()

	command := exec.CommandContext(ctx, name, arguments...)
	if environment != nil {
		command.Env = environment
	}
	output, err := command.CombinedOutput()
	if errors.Is(ctx.Err(), context.DeadlineExceeded) {
		return string(output), fmt.Errorf("%s command timed out after %s", name, timeout)
	}
	return string(output), err
}

func waitForServerHealth(t *testing.T, healthURL, containerName string) serverHTTPResponse {
	t.Helper()
	deadline := time.Now().Add(e2eStartupTimeout)
	var lastError error

	for time.Now().Before(deadline) {
		response, err := requestServerWithoutFailure(http.MethodGet, healthURL)
		if err == nil && response.statusCode == http.StatusOK {
			return response
		}
		if err != nil {
			lastError = err
		} else {
			lastError = fmt.Errorf("health status is %d", response.statusCode)
		}
		time.Sleep(200 * time.Millisecond)
	}

	logs := runDockerCommand(t, e2eCommandTimeout, "logs", containerName)
	t.Fatalf("server did not become healthy within %s: %v\ncontainer logs:\n%s", e2eStartupTimeout, lastError, logs)
	return serverHTTPResponse{}
}

func requestServer(t *testing.T, method, requestURL string) serverHTTPResponse {
	t.Helper()
	response, err := requestServerWithoutFailure(method, requestURL)
	if err != nil {
		t.Fatalf("%s %s failed: %v", method, requestURL, err)
	}
	return response
}

func requestServerWithoutFailure(method, requestURL string) (serverHTTPResponse, error) {
	request, err := http.NewRequest(method, requestURL, nil)
	if err != nil {
		return serverHTTPResponse{}, err
	}

	client := http.Client{Timeout: 2 * time.Second}
	response, err := client.Do(request)
	if err != nil {
		return serverHTTPResponse{}, err
	}
	defer response.Body.Close()

	body, err := io.ReadAll(response.Body)
	if err != nil {
		return serverHTTPResponse{}, err
	}
	return serverHTTPResponse{
		statusCode:  response.StatusCode,
		contentType: response.Header.Get("Content-Type"),
		body:        body,
	}, nil
}

func removeEnvironmentVariable(environment []string, variableName string) []string {
	prefix := variableName + "="
	filteredEnvironment := make([]string, 0, len(environment))
	for _, entry := range environment {
		if !strings.HasPrefix(entry, prefix) {
			filteredEnvironment = append(filteredEnvironment, entry)
		}
	}
	return filteredEnvironment
}

package tasks

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/aapng-cmd/ctf_platform/internal/models"
)

var (
	inflight, cancelInflight = context.WithCancel(context.Background())
	inflightWG               sync.WaitGroup
)

// CancelInflight stops compose up and checker runs so a platform shutdown can remove task containers.
func CancelInflight() {
	cancelInflight()
}

// WaitInflight waits until cancelled compose commands exit.
func WaitInflight(timeout time.Duration) {
	done := make(chan struct{})
	go func() {
		inflightWG.Wait()
		close(done)
	}()
	select {
	case <-done:
	case <-time.After(timeout):
	}
}

func composeBin() (string, []string, error) {
	if path, err := exec.LookPath("docker"); err == nil {
		if err := exec.Command(path, "compose", "version").Run(); err == nil {
			return path, []string{"compose"}, nil
		}
	}
	if path, err := exec.LookPath("docker-compose"); err == nil {
		return path, nil, nil
	}
	return "", nil, fmt.Errorf("docker compose is not installed on the platform host")
}

func composeFile(dir string) string {
	for _, name := range []string{"docker-compose.yml", "docker-compose.yaml", "compose.yml", "compose.yaml"} {
		p := filepath.Join(dir, name)
		if st, err := os.Stat(p); err == nil && !st.IsDir() {
			return p
		}
	}
	return filepath.Join(dir, "docker-compose.yml")
}

func composeFileArgs(dir string) []string {
	var args []string
	main := composeFile(dir)
	if fileExists(main) {
		args = append(args, "-f", main)
	}
	overlay := filepath.Join(dir, "ctfd-checker.compose.yml")
	if fileExists(overlay) {
		args = append(args, "-f", overlay)
	}
	if len(args) == 0 {
		args = []string{"-f", main}
	}
	return args
}

func projectName(dir string) string {
	abs, err := filepath.Abs(dir)
	if err != nil {
		abs = dir
	}
	sum := sha256.Sum256([]byte(abs))
	return "ctfd" + hex.EncodeToString(sum[:4])
}

func runCompose(parent context.Context, timeout time.Duration, dir string, extra ...string) (string, error) {
	return runComposeNotify(parent, timeout, dir, nil, nil, extra...)
}

func runComposeNotify(parent context.Context, timeout time.Duration, dir string, files []string, on func(string), extra ...string) (string, error) {
	bin, base, err := composeBin()
	if err != nil {
		return "", err
	}
	args := append([]string{}, base...)
	args = append(args, "-p", projectName(dir))
	if len(files) == 0 {
		args = append(args, composeFileArgs(dir)...)
	} else {
		for _, name := range files {
			path := name
			if !filepath.IsAbs(path) {
				path = filepath.Join(dir, name)
			}
			args = append(args, "-f", path)
		}
	}
	args = append(args, extra...)
	ctx, cancel := context.WithTimeout(parent, timeout)
	defer cancel()
	if parent == inflight {
		inflightWG.Add(1)
		defer inflightWG.Done()
	}
	cmd := exec.CommandContext(ctx, bin, args...)
	cmd.Dir = dir
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return "", err
	}
	stderr, err := cmd.StderrPipe()
	if err != nil {
		return "", err
	}
	if err := cmd.Start(); err != nil {
		return "", err
	}
	var buf bytes.Buffer
	var mu sync.Mutex
	var last time.Time
	write := func(p []byte) {
		mu.Lock()
		buf.Write(p)
		snap := buf.String()
		now := time.Now()
		send := on != nil && (last.IsZero() || now.Sub(last) >= 200*time.Millisecond)
		if send {
			last = now
		}
		mu.Unlock()
		if send {
			on(snap)
		}
	}
	read := func(r io.Reader) {
		tmp := make([]byte, 4096)
		for {
			n, err := r.Read(tmp)
			if n > 0 {
				chunk := append([]byte(nil), tmp[:n]...)
				write(chunk)
			}
			if err != nil {
				return
			}
		}
	}
	var wg sync.WaitGroup
	wg.Add(2)
	go func() { defer wg.Done(); read(stdout) }()
	go func() { defer wg.Done(); read(stderr) }()
	err = cmd.Wait()
	wg.Wait()
	mu.Lock()
	out := strings.TrimSpace(buf.String())
	mu.Unlock()
	if on != nil {
		on(out)
	}
	if ctx.Err() == context.DeadlineExceeded {
		return out, fmt.Errorf("docker compose timed out")
	}
	if err != nil {
		if out == "" {
			out = err.Error()
		}
		return out, fmt.Errorf("%w", err)
	}
	return out, nil
}

func Up(dir string) (string, error) {
	return UpNotify(dir, nil)
}

func UpNotify(dir string, on func(string)) (string, error) {
	return runComposeNotify(inflight, 12*time.Minute, dir, nil, on, "up", "-d", "--build", "--remove-orphans")
}

// UpChecker starts only the checker overlay, not the vulnerable service from the task archive.
func UpChecker(dir string, on func(string)) (string, error) {
	return runComposeNotify(inflight, 12*time.Minute, dir, []string{"ctfd-checker.compose.yml"}, on, "up", "-d", "--build", "--remove-orphans")
}

func Down(dir string) (string, error) {
	return runCompose(context.Background(), 90*time.Second, dir, "down", "--remove-orphans", "-t", "15")
}

// DownVolumes also removes the named volumes of the stack, including checker state.
func DownVolumes(dir string) (string, error) {
	return runCompose(context.Background(), 90*time.Second, dir, "down", "--remove-orphans", "-v", "-t", "15")
}

// DeployDirs lists compose project directories under root. Player stacks and jury files are skipped.
func DeployDirs(root string) []string {
	seen := map[string]struct{}{}
	var out []string
	_ = filepath.WalkDir(root, func(path string, d os.DirEntry, err error) error {
		if err != nil || d.IsDir() {
			return nil
		}
		base := strings.ToLower(d.Name())
		switch base {
		case "docker-compose.yml", "docker-compose.yaml", "compose.yml", "compose.yaml":
		default:
			return nil
		}
		if strings.Contains(base, "player") {
			return nil
		}
		lower := strings.ToLower(filepath.ToSlash(path))
		if strings.Contains(lower, "/jury/") || strings.Contains(lower, "/__pycache__/") || strings.Contains(lower, "/.git/") {
			return nil
		}
		dir := filepath.Dir(path)
		if _, ok := seen[dir]; ok {
			return nil
		}
		seen[dir] = struct{}{}
		out = append(out, dir)
		return nil
	})
	return out
}

func RunChecker(dir, service, action string, args ...string) (status, message, raw string) {
	if strings.TrimSpace(service) == "" {
		service = "checker"
	}
	cmdArgs := []string{"run", "--rm", "-T", "--no-deps", "--network", "host", service, action}
	cmdArgs = append(cmdArgs, args...)
	raw, err := runCompose(inflight, 2*time.Minute, dir, cmdArgs...)
	status, message = interpretChecker(raw, err)
	return status, message, raw
}

func interpretChecker(raw string, err error) (string, string) {
	text := strings.TrimSpace(raw)
	lower := strings.ToLower(text)
	code := 0
	var ee *exec.ExitError
	if errors.As(err, &ee) {
		code = ee.ExitCode()
	} else if err != nil {
		code = -1
	}
	switch code {
	case 0, 101:
		if strings.Contains(lower, "corrupt") {
			return models.CheckCorrupt, clip(text)
		}
		msg := "OK"
		if text != "" {
			msg = clip(text)
		}
		return models.CheckOK, msg
	case 102:
		return models.CheckCorrupt, clip(text)
	case 103:
		return models.CheckMumble, clip(text)
	case 104:
		return models.CheckDown, clip(text)
	default:
		if strings.Contains(lower, "corrupt") {
			return models.CheckCorrupt, clip(text)
		}
		if strings.Contains(lower, "mumble") {
			return models.CheckMumble, clip(text)
		}
		if text == "" && err != nil {
			text = err.Error()
		}
		return models.CheckDown, clip(text)
	}
}

func clip(s string) string {
	s = strings.TrimSpace(s)
	if len(s) > 240 {
		return s[:240]
	}
	return s
}

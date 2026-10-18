package tasks

import (
	"archive/zip"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestParsePackages(t *testing.T) {
	dir := t.TempDir()
	zipPath := filepath.Join(dir, "ad.zip")
	f, err := os.Create(zipPath)
	if err != nil {
		t.Fatal(err)
	}
	zw := zip.NewWriter(f)
	files := map[string]string{
		"chat/docker-compose.yml":  "services:\n  chat:\n    build: .\n    ports:\n      - \"7777:7777\"\n  checker:\n    build: ./checker\n",
		"chat/checker/checker.py":  "print('ok')\n",
		"chat/DEPLOY.md":           "Chat service for AD.\n\nMore text.\n",
		"jury/spawn.py":            "# jury\n",
		"hello/task.yml":           "name: Hello\ncategory: Web\nvalue: 150\nflag: flag{hi}\ndescription: hi there\n",
		"hello/docker-compose.yml": "services:\n  web:\n    ports:\n      - \"8080:80\"\n",
		"hello/README.md":          "# Ignored because task.yml exists\n",
	}
	for name, body := range files {
		w, err := zw.Create(name)
		if err != nil {
			t.Fatal(err)
		}
		if _, err := w.Write([]byte(body)); err != nil {
			t.Fatal(err)
		}
	}
	if err := zw.Close(); err != nil {
		t.Fatal(err)
	}
	_ = f.Close()

	root := filepath.Join(dir, "src")
	if err := Unzip(zipPath, root); err != nil {
		t.Fatal(err)
	}
	ad, err := ParseAD(root)
	if err != nil {
		t.Fatal(err)
	}
	if ad.Name != "chat" || ad.Port != 7777 || ad.CheckerService != "checker" {
		t.Fatalf("ad spec = %+v", ad)
	}
	j, err := ParseJeopardy(filepath.Join(root, "hello"))
	if err != nil {
		t.Fatal(err)
	}
	if j.Name != "Hello" || j.Value != 150 || j.Port != 8080 || j.Flags[0].Content != "flag{hi}" {
		t.Fatalf("jeopardy spec = %+v", j)
	}
}

func TestPlayerDistWithoutChecker(t *testing.T) {
	dir := t.TempDir()
	zipPath := filepath.Join(dir, "bucket.zip")
	f, err := os.Create(zipPath)
	if err != nil {
		t.Fatal(err)
	}
	zw := zip.NewWriter(f)
	w, err := zw.Create("docker-compose.yml")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := w.Write([]byte("services:\n  bucket:\n    build: .\n    ports:\n      - \"18081:80\"\n")); err != nil {
		t.Fatal(err)
	}
	if _, err := zw.Create("DEPLOY.md"); err != nil {
		t.Fatal(err)
	}
	if err := zw.Close(); err != nil {
		t.Fatal(err)
	}
	_ = f.Close()
	root := filepath.Join(dir, "src")
	if err := Unzip(zipPath, root); err != nil {
		t.Fatal(err)
	}
	spec, err := ParseAD(root)
	if err != nil {
		t.Fatal(err)
	}
	if spec.Name != "bucket" || spec.Port != 18081 || spec.Service != "bucket" || spec.CheckerService != "" {
		t.Fatalf("%+v", spec)
	}
	real := "/home/kali/my_AD_examples/AD1/player_dist/bucket/bucket.zip"
	if _, err := os.Stat(real); err != nil {
		return
	}
	root2 := filepath.Join(dir, "extracted", "src")
	if err := Unzip(real, root2); err != nil {
		t.Fatal(err)
	}
	spec, err = ParseAD(root2)
	if err != nil {
		t.Fatal(err)
	}
	if spec.Name != "bucket" || spec.Port != 18081 {
		t.Fatalf("real zip: %+v", spec)
	}
}

func TestParseCheckerOnly(t *testing.T) {
	dir := t.TempDir()
	root := filepath.Join(dir, "src")
	checker := filepath.Join(root, "checker")
	if err := os.MkdirAll(checker, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(checker, "checker.py"), []byte("class Handler(BaseHTTPRequestHandler):\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(checker, "Dockerfile"), []byte("ENV LISTEN=0.0.0.0:8091\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	spec, err := ParseAD(root)
	if err != nil {
		t.Fatal(err)
	}
	if spec.CheckerService != "" || spec.ComposeDir != root || spec.Port != 0 {
		t.Fatalf("%+v", spec)
	}
	if !strings.Contains(spec.CheckerURL, ":8091") {
		t.Fatalf("url %q", spec.CheckerURL)
	}
	body, err := os.ReadFile(filepath.Join(root, "ctfd-checker.compose.yml"))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(body), "build: ./checker") || !strings.Contains(string(body), "network_mode: host") {
		t.Fatalf("overlay:\n%s", body)
	}
}

func TestHTTPCheckerURL(t *testing.T) {
	dir := t.TempDir()
	if err := os.MkdirAll(filepath.Join(dir, "checker"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "checker", "checker.py"), []byte("class Handler(BaseHTTPRequestHandler):\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	dockerfile := "FROM python:3.12-alpine\nRUN mkdir -p /var/lib/checker\nENV STATE_PATH=/var/lib/checker/bucket.json\nENV LISTEN=0.0.0.0:8091\nENV FLAG_PREFIX=FLAG{\n"
	if err := os.WriteFile(filepath.Join(dir, "checker", "Dockerfile"), []byte(dockerfile), 0o644); err != nil {
		t.Fatal(err)
	}
	url := HTTPCheckerURL(dir, "Bucket Silo")
	if !strings.HasSuffix(url, ":8091/") {
		t.Fatal(url)
	}
	b, err := os.ReadFile(filepath.Join(dir, "ctfd-checker.compose.yml"))
	if err != nil {
		t.Fatal(err)
	}
	body := string(b)
	for _, want := range []string{"checker-state-0:/var/lib/checker", "name: ctfd-checker-bucket-silo-state-0", "LISTEN: 0.0.0.0:8091"} {
		if !strings.Contains(body, want) {
			t.Fatalf("overlay misses %q:\n%s", want, body)
		}
	}
	if strings.Contains(body, "FLAG{") {
		t.Fatalf("flag prefix must not become a volume:\n%s", body)
	}
}

func TestDeployDirsSkipsPlayerAndJury(t *testing.T) {
	root := t.TempDir()
	must := func(path, body string) {
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, []byte(body), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	must(filepath.Join(root, "chat", "docker-compose.yml"), "services: {}\n")
	must(filepath.Join(root, "chat", "player-compose.yml"), "services: {}\n")
	must(filepath.Join(root, "jury", "docker-compose.yml"), "services: {}\n")
	dirs := DeployDirs(root)
	if len(dirs) != 1 || filepath.Base(dirs[0]) != "chat" {
		t.Fatalf("dirs = %v", dirs)
	}
}

func TestReadmeFallback(t *testing.T) {
	dir := t.TempDir()
	readme := "# Crypto box\n\n## Category\nCrypto\n\n## Description\nDecrypt it\n\n## Cost\n200\n\n## Flag\nflag{readme}\n\n## Hints\nlook closer|15\n"
	if err := os.WriteFile(filepath.Join(dir, "README.md"), []byte(readme), 0o644); err != nil {
		t.Fatal(err)
	}
	spec, err := ParseJeopardy(dir)
	if err != nil {
		t.Fatal(err)
	}
	if spec.Name != "Crypto box" || spec.Category != "Crypto" || spec.Value != 200 || spec.Flag != "flag{readme}" || len(spec.Hints) != 1 {
		t.Fatalf("%+v", spec)
	}
}

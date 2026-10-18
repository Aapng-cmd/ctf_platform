package tasks

import (
	"archive/zip"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/aapng-cmd/ctf_platform/internal/models"
	"gopkg.in/yaml.v3"
)

type HintSpec struct {
	Title   string `yaml:"title"`
	Content string `yaml:"content"`
	Cost    int    `yaml:"cost"`
}

type FlagSpec struct {
	Content string `yaml:"content"`
	Type    string `yaml:"type"`
	Data    string `yaml:"data"`
}

type JeopardySpec struct {
	Name           string     `yaml:"name"`
	Category       string     `yaml:"category"`
	Description    string     `yaml:"description"`
	Value          int        `yaml:"value"`
	Flag           string     `yaml:"flag"`
	Flags          []FlagSpec `yaml:"flags"`
	Hints          []HintSpec `yaml:"hints"`
	Files          []string   `yaml:"files"`
	Attribution    string     `yaml:"attribution"`
	ConnectionInfo string     `yaml:"connection_info"`
	MaxAttempts    int        `yaml:"max_attempts"`
	Root           string     `yaml:"-"`
	ComposeDir     string     `yaml:"-"`
	ComposeFile    string     `yaml:"-"`
	Port           int        `yaml:"-"`
	Service        string     `yaml:"-"`
}

type ADSpec struct {
	Name           string
	Description    string
	Port           int
	AttackReward   int
	SLAReward      int
	DefensePenalty int
	DefenseReward  int
	DownPenalty    int
	Root           string
	ComposeDir     string
	ComposeFile    string
	Service        string
	CheckerService string
	CheckerDir     string
	CheckerURL     string
}

func Unzip(src, dest string) error {
	r, err := zip.OpenReader(src)
	if err != nil {
		return err
	}
	defer r.Close()
	dest, err = filepath.Abs(dest)
	if err != nil {
		return err
	}
	if err := os.MkdirAll(dest, 0o755); err != nil {
		return err
	}
	for _, f := range r.File {
		name := filepath.Clean(f.Name)
		if name == "." || strings.HasPrefix(name, "..") || filepath.IsAbs(name) {
			return fmt.Errorf("unsafe path in archive: %s", f.Name)
		}
		target := filepath.Join(dest, name)
		if !strings.HasPrefix(target, dest+string(os.PathSeparator)) && target != dest {
			return fmt.Errorf("unsafe path in archive: %s", f.Name)
		}
		if f.FileInfo().IsDir() {
			if err := os.MkdirAll(target, 0o755); err != nil {
				return err
			}
			continue
		}
		if err := os.MkdirAll(filepath.Dir(target), 0o755); err != nil {
			return err
		}
		in, err := f.Open()
		if err != nil {
			return err
		}
		out, err := os.OpenFile(target, os.O_CREATE|os.O_TRUNC|os.O_WRONLY, 0o644)
		if err != nil {
			_ = in.Close()
			return err
		}
		_, err = io.Copy(out, in)
		_ = out.Close()
		_ = in.Close()
		if err != nil {
			return err
		}
	}
	return nil
}

func ParseJeopardy(root string) (*JeopardySpec, error) {
	spec := &JeopardySpec{Root: root, Category: "Misc", Value: 100}
	meta := findFile(root, "task.yml", "task.yaml", "ctf.yml")
	if meta != "" {
		b, err := os.ReadFile(meta)
		if err != nil {
			return nil, err
		}
		if err := yaml.Unmarshal(b, spec); err != nil {
			return nil, fmt.Errorf("task.yml: %w", err)
		}
		spec.Root = root
	} else if readme := findFile(root, "README.md"); readme != "" {
		applyReadme(spec, readme)
	}
	compose, dir := findCompose(root, true)
	if compose != "" {
		spec.ComposeFile = compose
		spec.ComposeDir = dir
		svc, port, err := vulnService(compose)
		if err != nil {
			return nil, err
		}
		spec.Service = svc
		spec.Port = port
	}
	if strings.TrimSpace(spec.Name) == "" {
		return nil, fmt.Errorf("task name is missing: add task.yml or a README.md heading")
	}
	if spec.Flag == "" && len(spec.Flags) == 0 {
		return nil, fmt.Errorf("flag is missing in task.yml or README.md")
	}
	if spec.Flag != "" && len(spec.Flags) == 0 {
		spec.Flags = []FlagSpec{{Content: spec.Flag, Type: "static"}}
	}
	return spec, nil
}

func ParseAD(root string) (*ADSpec, error) {
	if spec, ok, err := parseHTTPChecker(root); ok || err != nil {
		return spec, err
	}
	compose, dir := findCompose(root, true)
	if compose == "" {
		return nil, fmt.Errorf("checker not found: the archive needs a checker directory with checker.py")
	}
	checkerDir := ""
	candidates := []string{
		filepath.Join(dir, "checker"),
		filepath.Join(root, "checker"),
	}
	for _, c := range candidates {
		if fileExists(filepath.Join(c, "checker.py")) || fileExists(filepath.Join(c, "Dockerfile")) {
			checkerDir = c
			break
		}
	}
	svc, port, checkerSvc, err := adServices(compose)
	if err != nil {
		return nil, err
	}
	checkerURL := ""
	if checkerDir != "" && checkerSvc == "" {
		if httpChecker(checkerDir) {
			listen := checkerListenPort(checkerDir)
			if err := writeCheckerOverlay(dir, listen, ""); err != nil {
				return nil, err
			}
			checkerURL = fmt.Sprintf("http://%s:%d/", checkerHost(), listen)
		} else {
			if err := writeCheckerOverlay(dir, 0, ""); err != nil {
				return nil, err
			}
			checkerSvc = "checker"
		}
	}
	name := filepath.Base(dir)
	if genericDir(name) {
		name = svc
	}
	desc := ""
	if b, err := os.ReadFile(filepath.Join(dir, "DEPLOY.md")); err == nil {
		desc = firstParagraph(string(b))
	}
	return &ADSpec{
		Name: name, Description: desc, Port: port,
		AttackReward: models.ADDefaultAttackReward, SLAReward: models.ADDefaultSLAReward,
		DefensePenalty: models.ADDefaultDefensePenalty, DefenseReward: models.ADDefaultDefenseReward,
		DownPenalty: models.ADDefaultDownPenalty,
		Root:        root, ComposeDir: dir, ComposeFile: compose,
		Service: svc, CheckerService: checkerSvc, CheckerDir: checkerDir, CheckerURL: checkerURL,
	}, nil
}

func parseHTTPChecker(root string) (*ADSpec, bool, error) {
	checkerDir := findHTTPChecker(root)
	if checkerDir == "" {
		return nil, false, nil
	}
	parent := filepath.Dir(checkerDir)
	listen := checkerListenPort(checkerDir)
	if err := writeCheckerOverlay(parent, listen, ""); err != nil {
		return nil, true, err
	}
	compose := composeIn(parent)
	svc, port := "", 0
	if compose != "" {
		svc, port, _ = vulnService(compose)
	}
	name := filepath.Base(parent)
	if genericDir(name) {
		if svc != "" {
			name = svc
		} else {
			name = "checker"
		}
	}
	desc := ""
	for _, file := range []string{"DEPLOY.md", "README.md"} {
		if b, err := os.ReadFile(filepath.Join(parent, file)); err == nil {
			desc = firstParagraph(string(b))
			break
		}
	}
	return &ADSpec{
		Name: name, Description: desc, Port: port,
		AttackReward: models.ADDefaultAttackReward, SLAReward: models.ADDefaultSLAReward,
		DefensePenalty: models.ADDefaultDefensePenalty, DefenseReward: models.ADDefaultDefenseReward,
		DownPenalty: models.ADDefaultDownPenalty,
		Root:        root, ComposeDir: parent, ComposeFile: compose,
		Service: svc, CheckerDir: checkerDir,
		CheckerURL: fmt.Sprintf("http://%s:%d/", checkerHost(), listen),
	}, true, nil
}

func findHTTPChecker(root string) string {
	var found string
	_ = filepath.WalkDir(root, func(path string, d os.DirEntry, err error) error {
		if err != nil || !d.IsDir() || found != "" {
			return nil
		}
		if skipDir(path) {
			return nil
		}
		if httpChecker(path) {
			found = path
			return io.EOF
		}
		return nil
	})
	return found
}

func composeIn(dir string) string {
	for _, name := range []string{"docker-compose.yml", "docker-compose.yaml", "compose.yml", "compose.yaml"} {
		path := filepath.Join(dir, name)
		if fileExists(path) && !strings.Contains(strings.ToLower(name), "player") {
			return path
		}
	}
	return ""
}

func genericDir(name string) bool {
	switch strings.ToLower(name) {
	case "", ".", "src", "tasks", "upload", "app":
		return true
	default:
		return false
	}
}

func httpChecker(dir string) bool {
	b, err := os.ReadFile(filepath.Join(dir, "checker.py"))
	if err != nil {
		return false
	}
	text := string(b)
	return strings.Contains(text, "HTTPServer") || strings.Contains(text, "BaseHTTPRequestHandler")
}

func checkerListenPort(dir string) int {
	b, err := os.ReadFile(filepath.Join(dir, "Dockerfile"))
	if err != nil {
		return 8091
	}
	for _, line := range strings.Split(string(b), "\n") {
		line = strings.TrimSpace(line)
		upper := strings.ToUpper(line)
		if strings.HasPrefix(upper, "ENV ") && strings.Contains(upper, "LISTEN=") {
			val := line[strings.Index(line, "=")+1:]
			val = strings.Trim(val, `"'`)
			if i := strings.LastIndex(val, ":"); i >= 0 {
				if n, err := strconv.Atoi(val[i+1:]); err == nil && n > 0 {
					return n
				}
			}
		}
		if strings.HasPrefix(upper, "EXPOSE ") {
			fields := strings.Fields(line)
			if len(fields) >= 2 {
				if n, err := strconv.Atoi(strings.Split(fields[1], "/")[0]); err == nil && n > 0 {
					return n
				}
			}
		}
	}
	return 8091
}

func checkerHost() string {
	if _, err := os.Stat("/.dockerenv"); err == nil {
		return "host.docker.internal"
	}
	return "127.0.0.1"
}

// HTTPCheckerURL prepares a long-running HTTP checker that lives beside compose and returns its URL.
// The checker state is kept in volumes named after service, so it survives restarts and re-uploads.
func HTTPCheckerURL(dir, service string) string {
	checkerDir := filepath.Join(dir, "checker")
	if !httpChecker(checkerDir) {
		return ""
	}
	port := checkerListenPort(checkerDir)
	if err := writeCheckerOverlay(dir, port, CheckerStateVolume(service)); err != nil {
		return ""
	}
	return fmt.Sprintf("http://%s:%d/", checkerHost(), port)
}

// CheckerStateVolume is the docker volume prefix that holds checker state for a service.
func CheckerStateVolume(service string) string {
	slug := strings.Map(func(r rune) rune {
		switch {
		case r >= 'a' && r <= 'z', r >= '0' && r <= '9', r == '-', r == '_':
			return r
		default:
			return '-'
		}
	}, strings.ToLower(strings.TrimSpace(service)))
	slug = strings.Trim(slug, "-_")
	if slug == "" {
		return ""
	}
	return "ctfd-checker-" + slug + "-state"
}

func writeCheckerOverlay(dir string, listen int, volume string) error {
	context := "./checker"
	stateFrom := filepath.Join(dir, "checker")
	if !fileExists(filepath.Join(dir, "checker", "Dockerfile")) && !fileExists(filepath.Join(dir, "checker", "checker.py")) {
		context = "."
		stateFrom = dir
	}
	path := filepath.Join(dir, "ctfd-checker.compose.yml")
	body := "services:\n  checker:\n    build: " + context + "\n    restart: unless-stopped\n    network_mode: host\n"
	if listen > 0 {
		body += fmt.Sprintf("    environment:\n      LISTEN: 0.0.0.0:%d\n", listen)
	}
	dirs := checkerStateDirs(stateFrom)
	if len(dirs) > 0 {
		body += "    volumes:\n"
		for i, d := range dirs {
			body += fmt.Sprintf("      - checker-state-%d:%s\n", i, d)
		}
		body += "volumes:\n"
		for i := range dirs {
			body += fmt.Sprintf("  checker-state-%d:\n", i)
			if volume != "" {
				body += fmt.Sprintf("    name: %s-%d\n", volume, i)
			}
		}
	}
	return os.WriteFile(path, []byte(body), 0o644)
}

// checkerStateDirs finds where the checker keeps state: VOLUME entries and absolute
// paths in ENV keys that mention state, data, db or storage.
func checkerStateDirs(checkerDir string) []string {
	b, err := os.ReadFile(filepath.Join(checkerDir, "Dockerfile"))
	if err != nil {
		return nil
	}
	seen := map[string]bool{}
	var out []string
	add := func(p string) {
		p = strings.Trim(strings.TrimSpace(p), `"'`)
		if !strings.HasPrefix(p, "/") {
			return
		}
		p = filepath.Clean(p)
		if p == "/" || seen[p] {
			return
		}
		seen[p] = true
		out = append(out, p)
	}
	for _, raw := range strings.Split(string(b), "\n") {
		line := strings.TrimSpace(raw)
		upper := strings.ToUpper(line)
		switch {
		case strings.HasPrefix(upper, "VOLUME "):
			for _, f := range strings.FieldsFunc(line[len("VOLUME "):], func(r rune) bool {
				return r == ' ' || r == ',' || r == '[' || r == ']' || r == '"' || r == '\''
			}) {
				add(f)
			}
		case strings.HasPrefix(upper, "ENV "):
			for _, kv := range envPairs(line[len("ENV "):]) {
				key := strings.ToUpper(kv[0])
				if !strings.Contains(key, "STATE") && !strings.Contains(key, "DATA") && !strings.Contains(key, "DB") && !strings.Contains(key, "STORAGE") {
					continue
				}
				val := strings.Trim(kv[1], `"'`)
				if filepath.Ext(val) != "" {
					val = filepath.Dir(val)
				}
				add(val)
			}
		}
	}
	return out
}

func envPairs(rest string) [][2]string {
	fields := strings.Fields(rest)
	if len(fields) == 0 {
		return nil
	}
	if !strings.Contains(fields[0], "=") {
		return [][2]string{{fields[0], strings.Join(fields[1:], " ")}}
	}
	var out [][2]string
	for _, f := range fields {
		if k, v, ok := strings.Cut(f, "="); ok {
			out = append(out, [2]string{k, v})
		}
	}
	return out
}

func applyReadme(spec *JeopardySpec, path string) {
	b, _ := os.ReadFile(path)
	lines := strings.Split(string(b), "\n")
	section := ""
	var buf []string
	flush := func() {
		text := strings.TrimSpace(strings.Join(buf, "\n"))
		buf = nil
		switch section {
		case "title":
			if spec.Name == "" {
				spec.Name = text
			}
		case "category":
			if text != "" {
				spec.Category = text
			}
		case "description":
			spec.Description = text
		case "cost", "value":
			if n, err := strconv.Atoi(strings.Fields(text)[0]); err == nil {
				spec.Value = n
			}
		case "flag":
			spec.Flag = text
		case "files":
			if text != "" && !strings.EqualFold(text, "N") {
				for _, p := range strings.Split(text, ";") {
					p = strings.TrimSpace(p)
					if p != "" {
						spec.Files = append(spec.Files, p)
					}
				}
			}
		case "hints":
			for _, line := range strings.Split(text, "\n") {
				parts := strings.SplitN(line, "|", 2)
				if len(parts) == 2 {
					cost, _ := strconv.Atoi(strings.TrimSpace(parts[1]))
					spec.Hints = append(spec.Hints, HintSpec{Content: strings.TrimSpace(parts[0]), Cost: cost})
				}
			}
		case "attribution", "author":
			spec.Attribution = text
		case "connection_info", "connection":
			spec.ConnectionInfo = text
		}
	}
	for _, line := range lines {
		trim := strings.TrimSpace(line)
		if strings.HasPrefix(trim, "# ") && !strings.HasPrefix(trim, "## ") {
			flush()
			section = "title"
			buf = []string{strings.TrimSpace(strings.TrimPrefix(trim, "# "))}
			continue
		}
		if strings.HasPrefix(trim, "## ") {
			flush()
			section = strings.ToLower(strings.ReplaceAll(strings.TrimPrefix(trim, "## "), " ", "_"))
			continue
		}
		if section != "" {
			buf = append(buf, line)
		}
	}
	flush()
}

func findFile(root string, names ...string) string {
	var found string
	_ = filepath.WalkDir(root, func(path string, d os.DirEntry, err error) error {
		if err != nil || d.IsDir() || found != "" {
			return nil
		}
		if skipDir(path) {
			return nil
		}
		base := strings.ToLower(d.Name())
		for _, n := range names {
			if base == strings.ToLower(n) {
				found = path
				return io.EOF
			}
		}
		return nil
	})
	return found
}

func findCompose(root string, allowAny bool) (string, string) {
	var found, dir string
	_ = filepath.WalkDir(root, func(path string, d os.DirEntry, err error) error {
		if err != nil || d.IsDir() || found != "" {
			return nil
		}
		if skipDir(path) {
			return nil
		}
		base := strings.ToLower(d.Name())
		if base != "docker-compose.yml" && base != "docker-compose.yaml" && base != "compose.yml" && base != "compose.yaml" {
			return nil
		}
		parent := filepath.Dir(path)
		if !allowAny {
			if _, _, checkerSvc, err := adServices(path); err != nil || checkerSvc == "" {
				if !fileExists(filepath.Join(parent, "checker", "checker.py")) && !fileExists(filepath.Join(parent, "checker", "Dockerfile")) {
					return nil
				}
			}
		}
		if strings.Contains(strings.ToLower(base), "player") {
			return nil
		}
		found = path
		dir = parent
		return io.EOF
	})
	return found, dir
}

func skipDir(path string) bool {
	lower := strings.ToLower(filepath.ToSlash(path))
	return strings.Contains(lower, "/jury/") || strings.Contains(lower, "/__pycache__/") || strings.Contains(lower, "/.git/")
}

func fileExists(path string) bool {
	st, err := os.Stat(path)
	return err == nil && !st.IsDir()
}

type composeDoc struct {
	Services map[string]composeService `yaml:"services"`
}

type composeService struct {
	Ports []any `yaml:"ports"`
}

func loadCompose(path string) (*composeDoc, error) {
	b, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	var c composeDoc
	if err := yaml.Unmarshal(b, &c); err != nil {
		return nil, fmt.Errorf("%s: %w", filepath.Base(path), err)
	}
	if len(c.Services) == 0 {
		return nil, fmt.Errorf("%s has no services", filepath.Base(path))
	}
	return &c, nil
}

func vulnService(path string) (string, int, error) {
	c, err := loadCompose(path)
	if err != nil {
		return "", 0, err
	}
	name := ""
	port := 0
	for svc, body := range c.Services {
		if strings.EqualFold(svc, "checker") {
			continue
		}
		if name == "" {
			name = svc
		}
		if p := firstPort(body.Ports); p > 0 && port == 0 {
			name = svc
			port = p
		}
	}
	if name == "" {
		return "", 0, fmt.Errorf("no challenge service in %s", filepath.Base(path))
	}
	return name, port, nil
}

func adServices(path string) (service string, port int, checker string, err error) {
	c, err := loadCompose(path)
	if err != nil {
		return "", 0, "", err
	}
	for svc, body := range c.Services {
		if strings.EqualFold(svc, "checker") {
			checker = svc
			continue
		}
		p := firstPort(body.Ports)
		if service == "" || (port == 0 && p > 0) {
			service = svc
			if p > 0 {
				port = p
			}
		}
	}
	if service == "" {
		return "", 0, checker, fmt.Errorf("no vuln service in %s", filepath.Base(path))
	}
	if port == 0 {
		return "", 0, checker, fmt.Errorf("published port not found in %s", filepath.Base(path))
	}
	return service, port, checker, nil
}

func firstPort(ports []any) int {
	for _, p := range ports {
		if n := hostPort(p); n > 0 {
			return n
		}
	}
	return 0
}

func hostPort(p any) int {
	switch v := p.(type) {
	case string:
		return portFromString(v)
	case int:
		return v
	case int64:
		return int(v)
	case float64:
		return int(v)
	case map[string]any:
		if pub, ok := v["published"]; ok {
			return hostPort(pub)
		}
	}
	return 0
}

func portFromString(v string) int {
	v = strings.Trim(strings.TrimSpace(v), `"'`)
	if v == "" {
		return 0
	}
	parts := strings.Split(v, ":")
	host := parts[0]
	if len(parts) >= 2 {
		host = parts[len(parts)-2]
	}
	host = strings.Split(host, "/")[0]
	n, _ := strconv.Atoi(host)
	return n
}

func firstParagraph(s string) string {
	s = strings.TrimSpace(s)
	if i := strings.Index(s, "\n\n"); i > 0 {
		s = s[:i]
	}
	if len(s) > 500 {
		s = s[:500]
	}
	return s
}

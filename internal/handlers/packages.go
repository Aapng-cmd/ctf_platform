package handlers

import (
	"fmt"
	"io"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/aapng-cmd/ctf_platform/internal/models"
	"github.com/aapng-cmd/ctf_platform/internal/tasks"
	"github.com/aapng-cmd/ctf_platform/internal/webutil"
	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

func (a *App) AdminChallengeUpload(c *gin.Context) {
	spec, root, err := a.acceptArchive(c, "jeopardy")
	if err != nil {
		a.flash(c, "danger", err.Error())
		c.Redirect(http.StatusFound, "/admin/challenges")
		return
	}
	j, ok := spec.(*tasks.JeopardySpec)
	if !ok {
		a.flash(c, "danger", "Not a Jeopardy archive")
		c.Redirect(http.StatusFound, "/admin/challenges")
		return
	}
	_ = root
	state := models.StateVisible
	status := "ready"
	if j.ComposeDir != "" {
		state = models.StateHidden
		status = "queued"
	}
	conn := j.ConnectionInfo
	if conn == "" && j.Port > 0 {
		conn = net.JoinHostPort(a.Cfg.PublicHost, strconv.Itoa(j.Port))
	}
	ch := models.Challenge{
		Name: j.Name, Category: j.Category, Description: j.Description, Value: j.Value,
		Attribution: j.Attribution, ConnectionInfo: conn, MaxAttempts: j.MaxAttempts,
		Type: models.ChallengeStandard, State: state, Function: "linear",
		ComposeDir: j.ComposeDir, DeployStatus: status,
	}
	if err := a.DB.Create(&ch).Error; err != nil {
		a.flash(c, "danger", "Could not save challenge: "+err.Error())
		c.Redirect(http.StatusFound, "/admin/challenges")
		return
	}
	for _, f := range j.Flags {
		ft := f.Type
		if ft == "" {
			ft = models.FlagStatic
		}
		a.DB.Create(&models.Flag{ChallengeID: ch.ID, Type: ft, Content: f.Content, Data: f.Data})
	}
	for _, h := range j.Hints {
		if strings.TrimSpace(h.Content) == "" {
			continue
		}
		a.DB.Create(&models.Hint{ChallengeID: ch.ID, Title: h.Title, Content: h.Content, Cost: h.Cost})
	}
	_ = a.attachTaskFiles(j.Root, j.Files, &ch.ID, nil)
	if j.ComposeDir != "" {
		go a.deployChallenge(ch.ID)
		a.flash(c, "success", "Challenge imported. Docker deploy started; status will appear in this list.")
	} else {
		a.flash(c, "success", "Challenge imported")
	}
	c.Redirect(http.StatusFound, "/admin/challenges")
}

func (a *App) deployChallenge(id uint) {
	var ch models.Challenge
	if a.DB.First(&ch, id).Error != nil || ch.ComposeDir == "" {
		return
	}
	a.DB.Model(&ch).Update("deploy_status", "building")
	log, err := tasks.UpNotify(ch.ComposeDir, a.deployProgress("challenges", id))
	status := "running"
	state := models.StateVisible
	if err != nil {
		status = "failed"
		state = models.StateHidden
		log = err.Error() + "\n" + log
	}
	a.DB.Model(&ch).Updates(map[string]any{
		"deploy_status": status,
		"deploy_log":    log,
		"state":         state,
	})
}

func (a *App) AdminChallengeRedeploy(c *gin.Context) {
	id, _ := strconv.Atoi(c.Param("id"))
	go a.deployChallenge(uint(id))
	a.flash(c, "success", "Redeploy started")
	c.Redirect(http.StatusFound, "/admin/challenges/"+strconv.Itoa(id))
}

func (a *App) AdminChallengeStop(c *gin.Context) {
	id, _ := strconv.Atoi(c.Param("id"))
	var ch models.Challenge
	if a.DB.First(&ch, id).Error == nil && ch.ComposeDir != "" {
		log, err := tasks.Down(ch.ComposeDir)
		status := "stopped"
		if err != nil {
			status = "failed"
			log = err.Error() + "\n" + log
		}
		a.DB.Model(&ch).Updates(map[string]any{"deploy_status": status, "deploy_log": log, "state": models.StateHidden})
	}
	a.flash(c, "success", "Challenge containers stopped")
	c.Redirect(http.StatusFound, "/admin/challenges/"+strconv.Itoa(id))
}

func (a *App) AdminADUpload(c *gin.Context) {
	parsed, _, err := a.acceptArchive(c, "ad")
	if err != nil {
		a.flash(c, "danger", err.Error())
		c.Redirect(http.StatusFound, "/admin/ad")
		return
	}
	spec := parsed.(*tasks.ADSpec)
	var ch models.ADChecker
	err = a.DB.Where("name = ?", spec.Name).First(&ch).Error
	fields := map[string]any{
		"name": spec.Name, "compose_dir": spec.ComposeDir, "checker_service": spec.CheckerService,
		"url": spec.CheckerURL, "deploy_status": "queued",
	}
	if err != nil {
		ch = models.ADChecker{
			Name: spec.Name, ComposeDir: spec.ComposeDir, CheckerService: spec.CheckerService,
			URL: spec.CheckerURL, DeployStatus: "queued",
		}
		if err := a.DB.Create(&ch).Error; err != nil {
			a.flash(c, "danger", "Could not save checker: "+err.Error())
			c.Redirect(http.StatusFound, "/admin/ad")
			return
		}
	} else {
		a.DB.Model(&ch).Updates(fields)
	}
	go a.deployChecker(ch.ID)
	a.flash(c, "success", "Checker imported. Docker deploy started.")
	c.Redirect(http.StatusFound, "/admin/ad")
}

func (a *App) deployChecker(id uint) {
	var ch models.ADChecker
	if a.DB.First(&ch, id).Error != nil || ch.ComposeDir == "" {
		return
	}
	a.DB.Model(&ch).Update("deploy_status", "building")
	progress := a.deployProgress("ad_checkers", id)
	url := tasks.HTTPCheckerURL(ch.ComposeDir, ch.Name)
	if url == "" {
		a.DB.Model(&ch).Updates(map[string]any{
			"deploy_status": "failed",
			"deploy_log":    "The archive has no HTTP checker. The vulnerable service is not started.",
		})
		return
	}
	a.DB.Model(&ch).Updates(map[string]any{"url": url, "checker_service": ""})
	log, err := tasks.UpChecker(ch.ComposeDir, progress)
	status := "running"
	if err != nil {
		status = "failed"
		log = err.Error() + "\n" + log
	}
	a.DB.Model(&models.ADChecker{}).Where("id = ?", id).Updates(map[string]any{
		"deploy_status": status, "deploy_log": log, "url": url,
	})
	if status == "running" {
		a.DB.Model(&models.ADService{}).Where("checker_id = ?", id).Update("checker_url", url)
	}
}

func (a *App) deployService(id uint) {
	var svc models.ADService
	if a.DB.First(&svc, id).Error != nil || svc.ComposeDir == "" {
		return
	}
	a.DB.Model(&svc).Update("deploy_status", "building")
	progress := a.deployProgress("ad_services", id)
	var log string
	var err error
	if url := tasks.HTTPCheckerURL(svc.ComposeDir, svc.Name); url != "" {
		a.DB.Model(&svc).Updates(map[string]any{"checker_url": url, "checker_service": ""})
		log, err = tasks.UpChecker(svc.ComposeDir, progress)
	} else {
		log, err = tasks.UpNotify(svc.ComposeDir, progress)
	}
	status := "running"
	if err != nil {
		status = "failed"
		log = err.Error() + "\n" + log
	}
	fields := map[string]any{"deploy_status": status, "deploy_log": log, "active": status == "running"}
	a.DB.Model(&svc).Updates(fields)
}

const shutdownNote = "stopped because the platform is shutting down"

// RestoreDeployedTasks starts again the task stacks that StopDeployedTasks stopped on the last shutdown.
func (a *App) RestoreDeployedTasks() {
	var challenges []models.Challenge
	a.DB.Select("id").Where("compose_dir <> '' AND deploy_status = ? AND deploy_log LIKE ?", "stopped", shutdownNote+"%").Find(&challenges)
	for _, ch := range challenges {
		a.deployChallenge(ch.ID)
	}
	var services []models.ADService
	a.DB.Select("id").Where("compose_dir <> '' AND deploy_status = ? AND deploy_log LIKE ?", "stopped", shutdownNote+"%").Find(&services)
	for _, svc := range services {
		a.deployService(svc.ID)
	}
	var checkers []models.ADChecker
	a.DB.Select("id").Where("compose_dir <> '' AND deploy_status = ? AND deploy_log LIKE ?", "stopped", shutdownNote+"%").Find(&checkers)
	for _, ch := range checkers {
		go a.deployChecker(ch.ID)
	}
}

func (a *App) StopDeployedTasks() {
	tasks.CancelInflight()
	tasks.WaitInflight(8 * time.Second)
	dirs := map[string]struct{}{}
	var challenges []models.Challenge
	if err := a.DB.Where("compose_dir <> ''").Find(&challenges).Error; err == nil {
		for _, ch := range challenges {
			if dirExists(ch.ComposeDir) {
				dirs[ch.ComposeDir] = struct{}{}
			}
		}
	}
	var services []models.ADService
	if err := a.DB.Where("compose_dir <> ''").Find(&services).Error; err == nil {
		for _, svc := range services {
			if dirExists(svc.ComposeDir) {
				dirs[svc.ComposeDir] = struct{}{}
			}
		}
	}
	var checkers []models.ADChecker
	if err := a.DB.Where("compose_dir <> ''").Find(&checkers).Error; err == nil {
		for _, ch := range checkers {
			if dirExists(ch.ComposeDir) {
				dirs[ch.ComposeDir] = struct{}{}
			}
		}
	}
	for _, dir := range tasks.DeployDirs(a.Cfg.TasksDir) {
		dirs[dir] = struct{}{}
	}
	var wg sync.WaitGroup
	for dir := range dirs {
		wg.Add(1)
		go func(dir string) {
			defer wg.Done()
			logText, err := tasks.Down(dir)
			status := "stopped"
			if err != nil {
				status = "failed"
				logText = err.Error() + "\n" + logText
			}
			note := shutdownNote + "\n" + logText
			a.DB.Model(&models.Challenge{}).Where("compose_dir = ?", dir).Updates(map[string]any{
				"deploy_status": status, "deploy_log": note, "state": models.StateHidden,
			})
			a.DB.Model(&models.ADService{}).Where("compose_dir = ?", dir).Updates(map[string]any{
				"deploy_status": status, "deploy_log": note, "active": false,
			})
			a.DB.Model(&models.ADChecker{}).Where("compose_dir = ?", dir).Updates(map[string]any{
				"deploy_status": status, "deploy_log": note,
			})
		}(dir)
	}
	wg.Wait()
}

func (a *App) deployProgress(table string, id uint) func(string) {
	var mu sync.Mutex
	var latest string
	var pending bool
	flush := func() {
		mu.Lock()
		text := latest
		pending = false
		mu.Unlock()
		if text == "" {
			return
		}
		a.DB.Table(table).Where("id = ?", id).Update("deploy_log", text)
	}
	return func(text string) {
		mu.Lock()
		latest = text
		if !pending {
			pending = true
			time.AfterFunc(250*time.Millisecond, flush)
		}
		mu.Unlock()
	}
}

func (a *App) AdminDeployStatus(c *gin.Context) {
	var challenges []models.Challenge
	var services []models.ADService
	var checkers []models.ADChecker
	a.DB.Select("id", "deploy_status", "deploy_log").Where("compose_dir <> '' OR deploy_status <> ''").Find(&challenges)
	a.DB.Select("id", "deploy_status", "deploy_log").Where("compose_dir <> '' OR deploy_status <> ''").Find(&services)
	a.DB.Select("id", "deploy_status", "deploy_log").Where("compose_dir <> '' OR deploy_status <> ''").Find(&checkers)
	c.Header("Cache-Control", "no-store")
	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"data": gin.H{
			"challenges": deployViews(challenges),
			"services":   serviceViews(services),
			"checkers":   checkerViews(checkers),
		},
	})
}

type deployView struct {
	ID     uint   `json:"id"`
	Status string `json:"status"`
	Log    string `json:"log"`
}

func deployViews(rows []models.Challenge) []deployView {
	out := make([]deployView, 0, len(rows))
	for _, row := range rows {
		out = append(out, deployView{ID: row.ID, Status: row.DeployStatus, Log: row.DeployLog})
	}
	return out
}

func serviceViews(rows []models.ADService) []deployView {
	out := make([]deployView, 0, len(rows))
	for _, row := range rows {
		out = append(out, deployView{ID: row.ID, Status: row.DeployStatus, Log: row.DeployLog})
	}
	return out
}

func checkerViews(rows []models.ADChecker) []deployView {
	out := make([]deployView, 0, len(rows))
	for _, row := range rows {
		out = append(out, deployView{ID: row.ID, Status: row.DeployStatus, Log: row.DeployLog})
	}
	return out
}

func dirExists(path string) bool {
	st, err := os.Stat(path)
	return err == nil && st.IsDir()
}

func (a *App) AdminADRedeploy(c *gin.Context) {
	id, _ := strconv.Atoi(c.Param("id"))
	go a.deployService(uint(id))
	a.flash(c, "success", "Redeploy started")
	c.Redirect(http.StatusFound, "/admin/ad")
}

func (a *App) AdminADCheckerRedeploy(c *gin.Context) {
	id, _ := strconv.Atoi(c.Param("id"))
	go a.deployChecker(uint(id))
	a.flash(c, "success", "Redeploy started")
	c.Redirect(http.StatusFound, "/admin/ad")
}

func (a *App) AdminADCheckerStop(c *gin.Context) {
	id, _ := strconv.Atoi(c.Param("id"))
	var ch models.ADChecker
	if a.DB.First(&ch, id).Error == nil && ch.ComposeDir != "" {
		log, err := tasks.Down(ch.ComposeDir)
		status := "stopped"
		if err != nil {
			status = "failed"
			log = err.Error() + "\n" + log
		}
		a.DB.Model(&ch).Updates(map[string]any{"deploy_status": status, "deploy_log": log})
	}
	a.flash(c, "success", "Checker stopped")
	c.Redirect(http.StatusFound, "/admin/ad")
}

func (a *App) AdminADCheckerDelete(c *gin.Context) {
	id, _ := strconv.Atoi(c.Param("id"))
	var ch models.ADChecker
	if a.DB.First(&ch, id).Error == nil && ch.ComposeDir != "" && dirExists(ch.ComposeDir) {
		_, _ = tasks.Down(ch.ComposeDir)
	}
	a.DB.Model(&models.ADService{}).Where("checker_id = ?", id).Updates(map[string]any{
		"checker_id": gorm.Expr("NULL"), "checker_url": "",
	})
	a.DB.Delete(&models.ADChecker{}, id)
	a.flash(c, "success", "Checker deleted")
	c.Redirect(http.StatusFound, "/admin/ad")
}

func (a *App) AdminADStop(c *gin.Context) {
	id, _ := strconv.Atoi(c.Param("id"))
	var svc models.ADService
	if a.DB.First(&svc, id).Error == nil && svc.ComposeDir != "" {
		log, err := tasks.Down(svc.ComposeDir)
		status := "stopped"
		if err != nil {
			status = "failed"
			log = err.Error() + "\n" + log
		}
		a.DB.Model(&svc).Updates(map[string]any{"deploy_status": status, "deploy_log": log, "active": false})
	}
	a.flash(c, "success", "Service containers stopped")
	c.Redirect(http.StatusFound, "/admin/ad")
}

func (a *App) acceptArchive(c *gin.Context, kind string) (any, string, error) {
	fh, err := c.FormFile("archive")
	if err != nil {
		return nil, "", fmt.Errorf("choose a .zip archive")
	}
	if fh.Size > 512<<20 {
		return nil, "", fmt.Errorf("archive must be under 512 MB")
	}
	ext := strings.ToLower(filepath.Ext(fh.Filename))
	if ext != ".zip" {
		return nil, "", fmt.Errorf("archive must be a .zip file")
	}
	id := webutil.RandomHex(6)
	base := filepath.Join(a.Cfg.TasksDir, kind, id)
	if err := os.MkdirAll(base, 0o755); err != nil {
		return nil, "", err
	}
	zipPath := filepath.Join(base, "upload.zip")
	if err := c.SaveUploadedFile(fh, zipPath); err != nil {
		return nil, "", err
	}
	root := filepath.Join(base, "src")
	if err := tasks.Unzip(zipPath, root); err != nil {
		return nil, "", err
	}
	switch kind {
	case "ad":
		spec, err := tasks.ParseAD(root)
		if err == nil && spec.Name == "checker" {
			if base := strings.TrimSpace(strings.TrimSuffix(filepath.Base(fh.Filename), filepath.Ext(fh.Filename))); base != "" && len(base) <= 80 {
				spec.Name = base
			}
		}
		return spec, root, err
	default:
		spec, err := tasks.ParseJeopardy(root)
		return spec, root, err
	}
}

func (a *App) attachTaskFiles(root string, rels []string, challengeID *uint, _ *uint) error {
	if err := os.MkdirAll(a.Cfg.UploadDir, 0o755); err != nil {
		return err
	}
	root, _ = filepath.Abs(root)
	for _, rel := range rels {
		rel = filepath.Clean(rel)
		src := filepath.Join(root, rel)
		abs, err := filepath.Abs(src)
		if err != nil || !strings.HasPrefix(abs, root+string(os.PathSeparator)) {
			continue
		}
		in, err := os.Open(abs)
		if err != nil {
			continue
		}
		name := filepath.Base(rel)
		stored := webutil.RandomHex(4) + "_" + name
		out, err := os.Create(filepath.Join(a.Cfg.UploadDir, stored))
		if err != nil {
			_ = in.Close()
			continue
		}
		_, _ = io.Copy(out, in)
		_ = out.Close()
		_ = in.Close()
		a.DB.Create(&models.File{Type: "challenge", Location: stored, Name: name, ChallengeID: challengeID})
	}
	return nil
}

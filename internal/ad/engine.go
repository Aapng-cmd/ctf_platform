package ad

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/aapng-cmd/ctf_platform/internal/flags"
	"github.com/aapng-cmd/ctf_platform/internal/models"
	"github.com/aapng-cmd/ctf_platform/internal/store"
	"github.com/aapng-cmd/ctf_platform/internal/tasks"
	"github.com/aapng-cmd/ctf_platform/internal/webutil"
	"gorm.io/gorm"
)

type Engine struct {
	DB    *gorm.DB
	Store *store.Store
	Flags flags.Store
	http  *http.Client
	stop  chan struct{}
	mu    sync.Mutex

	workMu   sync.Mutex
	inflight map[uint]struct{}
}

func New(db *gorm.DB, st *store.Store, fl flags.Store) *Engine {
	return &Engine{
		DB:    db,
		Store: st,
		Flags: fl,
		http:  &http.Client{Timeout: 30 * time.Second},
		stop:  make(chan struct{}),
	}
}

func (e *Engine) Start() {
	go e.loop()
}

func (e *Engine) Stop() {
	select {
	case <-e.stop:
	default:
		close(e.stop)
	}
}

func (e *Engine) loop() {
	// Wake on the round deadline. A fixed 3s poll left the finished round on
	// screen until the next tick, which is the pause the dashboard showed.
	timer := time.NewTimer(200 * time.Millisecond)
	defer timer.Stop()
	for {
		select {
		case <-e.stop:
			return
		case <-timer.C:
			wait := e.tick()
			if wait < 20*time.Millisecond {
				wait = 20 * time.Millisecond
			}
			timer.Reset(wait)
		}
	}
}

func (e *Engine) tick() time.Duration {
	e.mu.Lock()
	defer e.mu.Unlock()
	// Preparation writes no checks and settles nothing, so the score cannot move.
	if !e.Store.IsAD() || e.Store.Preparing() {
		return time.Second
	}
	e.settleFlags()
	if e.Store.ADPaused() || !e.Store.DuringCTF() {
		return time.Second
	}
	_ = EnsureInstances(e.DB)
	dur := time.Duration(e.Store.ADRoundSeconds()) * time.Second
	var round models.ADRound
	err := e.DB.Order("number DESC").First(&round).Error
	now := time.Now()
	if err != nil {
		e.startRound(1)
		return time.Second
	}
	if round.Ended == nil && now.Sub(round.Started) >= dur {
		e.finishRound(&round)
		e.startRound(round.Number + 1)
		return time.Second
	}
	if round.Ended == nil {
		e.scheduleWork(round.ID)
		left := dur - now.Sub(round.Started)
		if left < 20*time.Millisecond {
			return 20 * time.Millisecond
		}
		if left < time.Second {
			return left
		}
	}
	return time.Second
}

func (e *Engine) startRound(n int) {
	now := time.Now()
	round := models.ADRound{Number: n, Started: now}
	if err := e.DB.Create(&round).Error; err != nil {
		return
	}
	life := time.Duration(e.Store.ADFlagLifetime()*e.Store.ADRoundSeconds()) * time.Second
	var insts []models.ADInstance
	e.DB.Preload("Service").Preload("Team").Find(&insts)
	prefix := e.Store.ADFlagPrefix()
	suffix := "}"
	if strings.Contains(prefix, "{") && !strings.HasSuffix(prefix, "{") {
		suffix = ""
	}
	for _, inst := range insts {
		if !inst.Service.Active {
			continue
		}
		if inst.Team.Banned || inst.Team.Hidden {
			continue
		}
		flag := prefix + webutil.RandomHex(16) + suffix
		row := models.ADFlag{
			RoundID: round.ID, ServiceID: inst.ServiceID, TeamID: inst.TeamID,
			Flag: flag, Issued: now, Expires: now.Add(life),
		}
		if e.Flags != nil {
			_ = e.Flags.Save(&row)
		}
	}
	// Puts and checks talk to every team. Doing that here kept the new round
	// invisible until the slowest checker answered.
	e.scheduleWork(round.ID)
}

// settleFlags marks expired flags as defended when the checker saw the flag in its
// round, the service never failed while the flag was alive and nobody stole it.
func (e *Engine) settleFlags() {
	if e.Flags == nil {
		return
	}
	cutoff := time.Now().Add(-5 * time.Second)
	for _, fl := range e.Flags.Unsettled(cutoff, 1000) {
		defended := false
		if fl.PutStatus == models.CheckOK {
			var seen, failed, stolen int64
			e.DB.Model(&models.ADCheck{}).Where("round_id = ? AND team_id = ? AND service_id = ? AND status = ?",
				fl.RoundID, fl.TeamID, fl.ServiceID, models.CheckOK).Count(&seen)
			e.DB.Model(&models.ADCheck{}).Where("team_id = ? AND service_id = ? AND date >= ? AND date <= ? AND status <> ?",
				fl.TeamID, fl.ServiceID, fl.Issued, fl.Expires, models.CheckOK).Count(&failed)
			e.DB.Model(&models.ADSteal{}).Where("flag_id = ?", fl.ID).Count(&stolen)
			defended = seen > 0 && failed == 0 && stolen == 0
		}
		_ = e.Flags.Settle(fl.ID, defended)
	}
}

func (e *Engine) resolveChecker(inst *models.ADInstance) string {
	if inst == nil {
		return ""
	}
	if inst.Service.CheckerID != nil {
		var ch models.ADChecker
		if e.DB.First(&ch, *inst.Service.CheckerID).Error == nil {
			if url := strings.TrimSpace(ch.URL); url != "" {
				return url
			}
		}
	}
	if url := strings.TrimSpace(inst.Service.CheckerURL); url != "" {
		return url
	}
	if inst.Service.ComposeDir == "" {
		return ""
	}
	url := tasks.HTTPCheckerURL(inst.Service.ComposeDir, inst.Service.Name)
	if url == "" {
		return ""
	}
	inst.Service.CheckerURL = url
	inst.Service.CheckerService = ""
	_ = e.DB.Model(&models.ADService{}).Where("id = ?", inst.Service.ID).Updates(map[string]any{
		"checker_url": url, "checker_service": "",
	}).Error
	return url
}

func (e *Engine) plant(inst models.ADInstance, fl models.ADFlag) (string, string) {
	host := strings.TrimSpace(inst.Host)
	if host == "" {
		return models.CheckDown, models.ADNoHostMessage
	}
	if !webutil.AllowedServiceHost(host) {
		return models.CheckDown, "host is not allowed"
	}
	port := inst.Port
	if port == 0 {
		port = inst.Service.Port
	}
	target := host
	if port > 0 && !strings.Contains(host, ":") {
		target = net.JoinHostPort(host, fmt.Sprintf("%d", port))
	}
	if url := e.resolveChecker(&inst); url != "" {
		return e.postChecker(url, "put", inst, fl.Flag, fl.ID)
	}
	if inst.Service.ComposeDir != "" && inst.Service.CheckerService != "" {
		status, msg, raw := tasks.RunChecker(inst.Service.ComposeDir, inst.Service.CheckerService, "put", target, fmt.Sprintf("%d", fl.ID), fl.Flag, "1")
		_ = raw
		return status, msg
	}
	return models.CheckOK, "no checker put configured"
}

func (e *Engine) finishRound(round *models.ADRound) {
	now := time.Now()
	_ = e.DB.Model(round).Update("ended", now).Error
	round.Ended = &now
	e.scheduleWork(round.ID)
}

// scheduleWork runs puts and checks for a round without blocking the ticker.
// The same round is not started twice while a pass is already in flight.
func (e *Engine) scheduleWork(id uint) {
	if id == 0 {
		return
	}
	e.workMu.Lock()
	if e.inflight == nil {
		e.inflight = map[uint]struct{}{}
	}
	if _, busy := e.inflight[id]; busy {
		e.workMu.Unlock()
		return
	}
	e.inflight[id] = struct{}{}
	e.workMu.Unlock()
	go func() {
		defer func() {
			e.workMu.Lock()
			delete(e.inflight, id)
			e.workMu.Unlock()
		}()
		e.workRound(id)
	}()
}

func (e *Engine) workRound(id uint) {
	var round models.ADRound
	if err := e.DB.First(&round, id).Error; err != nil {
		return
	}
	e.plantPending(&round)
	if !round.ChecksDone {
		e.runChecks(&round)
	}
}

func (e *Engine) plantPending(round *models.ADRound) {
	if e.Flags == nil {
		return
	}
	var pending []models.ADFlag
	for _, fl := range e.Flags.ByRound(round.ID) {
		if fl.PutStatus == "" || fl.PutMessage == models.ADNoHostMessage || fl.PutStatus == models.CheckError {
			pending = append(pending, fl)
		}
	}
	parallel(len(pending), func(i int) {
		e.plantOne(round, pending[i])
	})
}

func (e *Engine) plantOne(round *models.ADRound, fl models.ADFlag) {
	var inst models.ADInstance
	if err := e.DB.Preload("Service").Preload("Team").Where("team_id = ? AND service_id = ?", fl.TeamID, fl.ServiceID).First(&inst).Error; err != nil {
		return
	}
	if strings.TrimSpace(inst.Host) == "" && fl.PutStatus != "" {
		return
	}
	status, msg := e.plant(inst, fl)
	_ = e.Flags.SetPut(fl.ID, status, msg)
	if status != models.CheckError && round.ChecksDone {
		// The host was set during the round: replace the "no host" DOWN with a real check.
		e.DB.Where("round_id = ? AND team_id = ? AND service_id = ? AND message = ?",
			round.ID, inst.TeamID, inst.ServiceID, models.ADNoHostMessage).Delete(&models.ADCheck{})
		e.checkOne(round, inst)
	}
}

func (e *Engine) runChecks(round *models.ADRound) {
	var insts []models.ADInstance
	e.DB.Preload("Service").Preload("Team").Find(&insts)
	parallel(len(insts), func(i int) {
		e.checkOne(round, insts[i])
	})
	_ = e.DB.Model(round).Update("checks_done", true).Error
	round.ChecksDone = true
}

// parallel runs fn over n items, at most 16 at a time.
func parallel(n int, fn func(int)) {
	if n <= 0 {
		return
	}
	workers := n
	if workers > 16 {
		workers = 16
	}
	sem := make(chan struct{}, workers)
	var wg sync.WaitGroup
	for i := 0; i < n; i++ {
		wg.Add(1)
		sem <- struct{}{}
		go func(i int) {
			defer wg.Done()
			defer func() { <-sem }()
			fn(i)
		}(i)
	}
	wg.Wait()
}

func (e *Engine) checkOne(round *models.ADRound, inst models.ADInstance) {
	if !inst.Service.Active || inst.Team.Banned || inst.Team.Hidden {
		return
	}
	var exists int64
	e.DB.Model(&models.ADCheck{}).Where("round_id = ? AND service_id = ? AND team_id = ?", round.ID, inst.ServiceID, inst.TeamID).Count(&exists)
	if exists > 0 {
		return
	}
	var fl models.ADFlag
	if e.Flags != nil {
		fl, _ = e.Flags.ByRoundTeamService(round.ID, inst.TeamID, inst.ServiceID)
	}
	status, msg := e.check(inst, fl)
	if status == models.CheckError {
		return
	}
	_ = e.DB.Create(&models.ADCheck{
		RoundID: round.ID, ServiceID: inst.ServiceID, TeamID: inst.TeamID,
		Status: status, Message: msg, Date: time.Now(),
	}).Error
}

type checkerReq struct {
	Action  string `json:"action"`
	TeamID  uint   `json:"team_id"`
	Host    string `json:"host"`
	Port    int    `json:"port"`
	Flag    string `json:"flag"`
	Prefix  string `json:"prefix"`
	Round   int    `json:"round"`
	Service string `json:"service"`
}

type checkerResp struct {
	Status  string `json:"status"`
	Message string `json:"message"`
}

func (e *Engine) check(inst models.ADInstance, fl models.ADFlag) (string, string) {
	host := strings.TrimSpace(inst.Host)
	port := inst.Port
	if port == 0 {
		port = inst.Service.Port
	}
	if host == "" {
		return models.CheckDown, models.ADNoHostMessage
	}
	if !webutil.AllowedServiceHost(host) {
		return models.CheckDown, "host is not allowed"
	}
	if fl.PutStatus == models.CheckError {
		return models.CheckError, fl.PutMessage
	}
	if fl.PutStatus != "" && fl.PutStatus != models.CheckOK {
		return fl.PutStatus, "flag put failed: " + fl.PutMessage
	}
	target := host
	if port > 0 && !strings.Contains(host, ":") {
		target = net.JoinHostPort(host, fmt.Sprintf("%d", port))
	}
	if url := e.resolveChecker(&inst); url != "" {
		status, msg := e.postChecker(url, "get", inst, fl.Flag, fl.ID)
		if status == models.CheckMumble && strings.Contains(strings.ToLower(msg), "bad action") {
			return e.postChecker(url, "check", inst, fl.Flag, fl.ID)
		}
		return status, msg
	}
	if inst.Service.ComposeDir != "" && inst.Service.CheckerService != "" {
		svc := inst.Service.CheckerService
		status, msg, _ := tasks.RunChecker(inst.Service.ComposeDir, svc, "get", target, fmt.Sprintf("%d", fl.ID), fl.Flag, "1")
		if status == models.CheckOK {
			return status, msg
		}
		checkStatus, checkMsg, _ := tasks.RunChecker(inst.Service.ComposeDir, svc, "check", target)
		if checkStatus == models.CheckOK || status == models.CheckDown && strings.Contains(strings.ToLower(msg), "unrecognized") {
			return checkStatus, checkMsg
		}
		return status, msg
	}
	addr := net.JoinHostPort(host, fmt.Sprintf("%d", port))
	c, err := net.DialTimeout("tcp", addr, 3*time.Second)
	if err != nil {
		return models.CheckDown, "connection failed"
	}
	_ = c.Close()
	return models.CheckOK, "tcp open"
}

func (e *Engine) postChecker(url, action string, inst models.ADInstance, flag string, flagID uint) (string, string) {
	port := inst.Port
	if port == 0 {
		port = inst.Service.Port
	}
	body, _ := json.Marshal(checkerReq{
		Action: action, TeamID: inst.TeamID, Host: inst.Host, Port: port,
		Flag: flag, Prefix: e.Store.ADFlagPrefix(), Round: int(flagID), Service: inst.Service.Name,
	})
	res, err := e.http.Post(url, "application/json", bytes.NewReader(body))
	if err != nil {
		return models.CheckError, "checker unavailable: " + err.Error()
	}
	defer res.Body.Close()
	var out checkerResp
	_ = json.NewDecoder(res.Body).Decode(&out)
	st := strings.ToUpper(strings.TrimSpace(out.Status))
	switch st {
	case models.CheckOK, models.CheckDown, models.CheckCorrupt, models.CheckMumble:
		return st, out.Message
	default:
		if res.StatusCode >= 200 && res.StatusCode < 300 {
			return models.CheckOK, out.Message
		}
		return models.CheckError, fmt.Sprintf("checker http %d", res.StatusCode)
	}
}

func EnsureInstances(db *gorm.DB) error {
	var services []models.ADService
	var teams []models.Team
	db.Where("active = ?", true).Find(&services)
	db.Where("banned = ?", false).Find(&teams)
	for _, t := range teams {
		for _, s := range services {
			var row models.ADInstance
			err := db.Where("team_id = ? AND service_id = ?", t.ID, s.ID).First(&row).Error
			if err != nil {
				_ = db.Create(&models.ADInstance{TeamID: t.ID, ServiceID: s.ID, Host: t.ADHost, Port: s.Port}).Error
				continue
			}
			updates := map[string]any{"port": s.Port}
			if t.ADHost != "" {
				updates["host"] = t.ADHost
			}
			_ = db.Model(&row).Updates(updates).Error
		}
	}
	return nil
}

func CurrentRound(db *gorm.DB) *models.ADRound {
	var r models.ADRound
	if err := db.Where("ended IS NULL").Order("number DESC").First(&r).Error; err != nil {
		if err := db.Order("number DESC").First(&r).Error; err != nil {
			return nil
		}
	}
	return &r
}

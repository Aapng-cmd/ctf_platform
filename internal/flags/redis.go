package flags

import (
	"context"
	"strconv"
	"time"

	"github.com/aapng-cmd/ctf_platform/internal/models"
	"github.com/redis/go-redis/v9"
)

const (
	keySeq  = "adflag:seq"
	keyOpen = "adflag:open"
	keyAll  = "adflag:all"
	keyDef  = "adflag:def"
)

// settleLua marks one flag settled exactly once and, when it was defended,
// increments the team/service counter the scoreboard reads.
const settleLua = `
if redis.call("HGET", KEYS[1], "settled") == "1" then
  return 0
end
local team = redis.call("HGET", KEYS[1], "team_id")
local svc = redis.call("HGET", KEYS[1], "service_id")
if not team then
  return 0
end
redis.call("HSET", KEYS[1], "settled", "1", "defended", ARGV[1])
redis.call("ZREM", KEYS[2], ARGV[2])
if ARGV[1] == "1" then
  redis.call("HINCRBY", KEYS[3], team .. ":" .. svc, 1)
end
return 1
`

// Redis is the production Store. Keys use the adflag: prefix so a shared
// instance can hold other data. AOF on the server keeps flags across restarts.
type Redis struct {
	rdb *redis.Client
}

func Open(addr, password string, db int) (*Redis, error) {
	rdb := redis.NewClient(&redis.Options{
		Addr:         addr,
		Password:     password,
		DB:           db,
		DialTimeout:  2 * time.Second,
		ReadTimeout:  3 * time.Second,
		WriteTimeout: 3 * time.Second,
	})
	var err error
	for i := 0; i < 30; i++ {
		ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
		err = rdb.Ping(ctx).Err()
		cancel()
		if err == nil {
			return &Redis{rdb: rdb}, nil
		}
		time.Sleep(time.Second)
	}
	_ = rdb.Close()
	return nil, err
}

func (r *Redis) ctx() (context.Context, context.CancelFunc) {
	return context.WithTimeout(context.Background(), 5*time.Second)
}

func valKey(flag string) string { return "adflag:v:" + flag }
func idKey(id uint) string      { return "adflag:" + strconv.FormatUint(uint64(id), 10) }
func roundKey(id uint) string   { return "adflag:r:" + strconv.FormatUint(uint64(id), 10) }
func slotKey(roundID, teamID, serviceID uint) string {
	return "adflag:k:" + strconv.FormatUint(uint64(roundID), 10) + ":" +
		strconv.FormatUint(uint64(teamID), 10) + ":" + strconv.FormatUint(uint64(serviceID), 10)
}
func svcKey(id uint) string { return "adflag:s:" + strconv.FormatUint(uint64(id), 10) }

func bit(v bool) string {
	if v {
		return "1"
	}
	return "0"
}

func (r *Redis) Save(fl *models.ADFlag) error {
	ctx, cancel := r.ctx()
	defer cancel()
	n, err := r.rdb.Incr(ctx, keySeq).Result()
	if err != nil {
		return err
	}
	fl.ID = uint(n)
	id := strconv.FormatUint(uint64(fl.ID), 10)
	ok, err := r.rdb.SetNX(ctx, valKey(fl.Flag), id, 0).Result()
	if err != nil {
		return err
	}
	if !ok {
		return ErrDuplicate
	}
	_, err = r.rdb.TxPipelined(ctx, func(p redis.Pipeliner) error {
		p.HSet(ctx, idKey(fl.ID), map[string]any{
			"id":          id,
			"round_id":    fl.RoundID,
			"service_id":  fl.ServiceID,
			"team_id":     fl.TeamID,
			"flag":        fl.Flag,
			"issued":      fl.Issued.UnixMilli(),
			"expires":     fl.Expires.UnixMilli(),
			"put_status":  fl.PutStatus,
			"put_message": fl.PutMessage,
			"settled":     bit(fl.Settled),
			"defended":    bit(fl.Defended),
		})
		p.SAdd(ctx, roundKey(fl.RoundID), id)
		p.Set(ctx, slotKey(fl.RoundID, fl.TeamID, fl.ServiceID), id, 0)
		p.SAdd(ctx, svcKey(fl.ServiceID), id)
		p.ZAdd(ctx, keyAll, redis.Z{Score: float64(fl.ID), Member: id})
		if !fl.Settled {
			p.ZAdd(ctx, keyOpen, redis.Z{Score: float64(fl.Expires.UnixMilli()), Member: id})
		}
		if fl.Defended {
			p.HIncrBy(ctx, keyDef, defField(fl.TeamID, fl.ServiceID), 1)
		}
		return nil
	})
	if err != nil {
		_ = r.rdb.Del(ctx, valKey(fl.Flag)).Err()
	}
	return err
}

func defField(team, service uint) string {
	return strconv.FormatUint(uint64(team), 10) + ":" + strconv.FormatUint(uint64(service), 10)
}

func decode(m map[string]string) (models.ADFlag, bool) {
	if len(m) == 0 || m["id"] == "" {
		return models.ADFlag{}, false
	}
	id, _ := strconv.ParseUint(m["id"], 10, 64)
	roundID, _ := strconv.ParseUint(m["round_id"], 10, 64)
	serviceID, _ := strconv.ParseUint(m["service_id"], 10, 64)
	teamID, _ := strconv.ParseUint(m["team_id"], 10, 64)
	issued, _ := strconv.ParseInt(m["issued"], 10, 64)
	expires, _ := strconv.ParseInt(m["expires"], 10, 64)
	return models.ADFlag{
		ID:         uint(id),
		RoundID:    uint(roundID),
		ServiceID:  uint(serviceID),
		TeamID:     uint(teamID),
		Flag:       m["flag"],
		Issued:     time.UnixMilli(issued),
		Expires:    time.UnixMilli(expires),
		PutStatus:  m["put_status"],
		PutMessage: m["put_message"],
		Settled:    m["settled"] == "1",
		Defended:   m["defended"] == "1",
	}, true
}

func (r *Redis) load(ctx context.Context, ids []string) []models.ADFlag {
	if len(ids) == 0 {
		return nil
	}
	cmds := make([]*redis.MapStringStringCmd, len(ids))
	_, _ = r.rdb.Pipelined(ctx, func(p redis.Pipeliner) error {
		for i, id := range ids {
			cmds[i] = p.HGetAll(ctx, "adflag:"+id)
		}
		return nil
	})
	out := make([]models.ADFlag, 0, len(ids))
	for _, cmd := range cmds {
		m, err := cmd.Result()
		if err != nil {
			continue
		}
		if fl, ok := decode(m); ok {
			out = append(out, fl)
		}
	}
	return out
}

func (r *Redis) ByValue(flag string) (models.ADFlag, bool) {
	ctx, cancel := r.ctx()
	defer cancel()
	id, err := r.rdb.Get(ctx, valKey(flag)).Result()
	if err != nil || id == "" || id == "0" {
		return models.ADFlag{}, false
	}
	m, err := r.rdb.HGetAll(ctx, "adflag:"+id).Result()
	if err != nil {
		return models.ADFlag{}, false
	}
	return decode(m)
}

func (r *Redis) ByRoundTeamService(roundID, teamID, serviceID uint) (models.ADFlag, bool) {
	ctx, cancel := r.ctx()
	defer cancel()
	id, err := r.rdb.Get(ctx, slotKey(roundID, teamID, serviceID)).Result()
	if err != nil || id == "" {
		return models.ADFlag{}, false
	}
	m, err := r.rdb.HGetAll(ctx, "adflag:"+id).Result()
	if err != nil {
		return models.ADFlag{}, false
	}
	return decode(m)
}

func (r *Redis) ByRound(roundID uint) []models.ADFlag {
	ctx, cancel := r.ctx()
	defer cancel()
	ids, err := r.rdb.SMembers(ctx, roundKey(roundID)).Result()
	if err != nil {
		return nil
	}
	return r.load(ctx, ids)
}

func (r *Redis) NeedsPut(roundID uint) []models.ADFlag {
	var out []models.ADFlag
	for _, fl := range r.ByRound(roundID) {
		if fl.PutMessage == models.ADNoHostMessage || fl.PutStatus == models.CheckError {
			out = append(out, fl)
		}
	}
	return out
}

func (r *Redis) SetPut(id uint, status, message string) error {
	ctx, cancel := r.ctx()
	defer cancel()
	return r.rdb.HSet(ctx, idKey(id), "put_status", status, "put_message", message).Err()
}

func (r *Redis) Unsettled(before time.Time, limit int) []models.ADFlag {
	ctx, cancel := r.ctx()
	defer cancel()
	if limit <= 0 {
		limit = 1000
	}
	ids, err := r.rdb.ZRangeByScore(ctx, keyOpen, &redis.ZRangeBy{
		Min:   "-inf",
		Max:   "(" + strconv.FormatInt(before.UnixMilli(), 10),
		Count: int64(limit),
	}).Result()
	if err != nil {
		return nil
	}
	return r.load(ctx, ids)
}

func (r *Redis) Settle(id uint, defended bool) error {
	ctx, cancel := r.ctx()
	defer cancel()
	return r.rdb.Eval(ctx, settleLua, []string{idKey(id), keyOpen, keyDef}, bit(defended), strconv.FormatUint(uint64(id), 10)).Err()
}

func (r *Redis) Defended() map[[2]uint]int {
	ctx, cancel := r.ctx()
	defer cancel()
	raw, err := r.rdb.HGetAll(ctx, keyDef).Result()
	if err != nil {
		return nil
	}
	out := make(map[[2]uint]int, len(raw))
	for field, val := range raw {
		n, _ := strconv.Atoi(val)
		if n <= 0 {
			continue
		}
		var team, svc uint64
		for i := 0; i < len(field); i++ {
			if field[i] == ':' {
				team, _ = strconv.ParseUint(field[:i], 10, 64)
				svc, _ = strconv.ParseUint(field[i+1:], 10, 64)
				break
			}
		}
		if team == 0 && svc == 0 {
			continue
		}
		out[[2]uint{uint(team), uint(svc)}] = n
	}
	return out
}

func (r *Redis) Recent(limit int) []models.ADFlag {
	ctx, cancel := r.ctx()
	defer cancel()
	if limit <= 0 {
		limit = 400
	}
	ids, err := r.rdb.ZRevRange(ctx, keyAll, 0, int64(limit-1)).Result()
	if err != nil {
		return nil
	}
	return r.load(ctx, ids)
}

func (r *Redis) forget(ctx context.Context, fl models.ADFlag) {
	id := strconv.FormatUint(uint64(fl.ID), 10)
	_, _ = r.rdb.Pipelined(ctx, func(p redis.Pipeliner) error {
		p.Del(ctx, idKey(fl.ID), valKey(fl.Flag), slotKey(fl.RoundID, fl.TeamID, fl.ServiceID))
		p.SRem(ctx, roundKey(fl.RoundID), id)
		p.SRem(ctx, svcKey(fl.ServiceID), id)
		p.ZRem(ctx, keyOpen, id)
		p.ZRem(ctx, keyAll, id)
		if fl.Defended {
			p.HIncrBy(ctx, keyDef, defField(fl.TeamID, fl.ServiceID), -1)
		}
		return nil
	})
}

func (r *Redis) DeleteService(serviceID uint) {
	ctx, cancel := r.ctx()
	defer cancel()
	ids, err := r.rdb.SMembers(ctx, svcKey(serviceID)).Result()
	if err != nil {
		return
	}
	for _, fl := range r.load(ctx, ids) {
		r.forget(ctx, fl)
	}
	_ = r.rdb.Del(ctx, svcKey(serviceID)).Err()
	r.dropEmptyDef(ctx)
}

func (r *Redis) dropEmptyDef(ctx context.Context) {
	raw, err := r.rdb.HGetAll(ctx, keyDef).Result()
	if err != nil {
		return
	}
	var dead []string
	for field, val := range raw {
		n, _ := strconv.Atoi(val)
		if n <= 0 {
			dead = append(dead, field)
		}
	}
	if len(dead) > 0 {
		_ = r.rdb.HDel(ctx, keyDef, dead...).Err()
	}
}

func (r *Redis) DeleteAll() {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	var cursor uint64
	for {
		keys, next, err := r.rdb.Scan(ctx, cursor, "adflag:*", 500).Result()
		if err != nil {
			return
		}
		if len(keys) > 0 {
			_ = r.rdb.Del(ctx, keys...).Err()
		}
		cursor = next
		if cursor == 0 {
			return
		}
	}
}

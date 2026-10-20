package models

import (
	"encoding/json"
	"time"
)

const (
	UserTypeUser  = "user"
	UserTypeAdmin = "admin"

	ModeUsers = "users"
	ModeTeams = "teams"

	ChallengeStandard = "standard"
	ChallengeDynamic  = "dynamic"

	StateVisible = "visible"
	StateHidden  = "hidden"

	FlagStatic = "static"
	FlagRegex  = "regex"

	SubmissionCorrect   = "correct"
	SubmissionIncorrect = "incorrect"

	UnlockHint      = "hints"
	UnlockChallenge = "challenges"

	VisibilityPublic     = "public"
	VisibilityPrivate    = "private"
	VisibilityHidden     = "hidden"
	VisibilityAdminsOnly = "admins_only"

	GameJeopardy      = "jeopardy"
	GameAttackDefense = "attack_defense"
	GameMixed         = "mixed"

	CheckOK      = "OK"
	CheckDown    = "DOWN"
	CheckCorrupt = "CORRUPT"
	CheckMumble  = "MUMBLE"
	CheckNoHost  = "NOHOST"
	// CheckError means the platform could not reach its own checker; the team is not scored for it.
	CheckError = "ERROR"

	// A down round costs ADDefaultDownPenalty per opponent, so losing the service
	// always costs more than having one flag stolen by every other team.
	ADDefaultAttackReward   = 10
	ADDefaultDefensePenalty = 10
	ADDefaultDefenseReward  = 5
	ADDefaultSLAReward      = 2
	ADDefaultDownPenalty    = 20

	ADNoHostMessage = "team host is not set"
)

// ADRewards is the event-wide Attack-Defense score. It is not stored per service.
type ADRewards struct {
	Attack  int `json:"attack"`
	Defense int `json:"defense"`
	Lost    int `json:"lost"`
	SLA     int `json:"sla"`
	Down    int `json:"down"`
}

func DefaultADRewards() ADRewards {
	return ADRewards{
		Attack:  ADDefaultAttackReward,
		Defense: ADDefaultDefenseReward,
		Lost:    ADDefaultDefensePenalty,
		SLA:     ADDefaultSLAReward,
		Down:    ADDefaultDownPenalty,
	}
}

type User struct {
	ID          uint      `gorm:"primaryKey" json:"id"`
	Email       string    `gorm:"size:128;uniqueIndex;not null" json:"email"`
	Name        string    `gorm:"size:128;uniqueIndex;not null" json:"name"`
	Password    string    `gorm:"size:255;not null" json:"-"`
	Type        string    `gorm:"size:16;default:user;index" json:"type"`
	Secret      string    `gorm:"size:64" json:"-"`
	Website     string    `gorm:"size:255" json:"website"`
	Affiliation string    `gorm:"size:255" json:"affiliation"`
	Country     string    `gorm:"size:8" json:"country"`
	Hidden      bool      `gorm:"default:false;index" json:"hidden"`
	Banned      bool      `gorm:"default:false;index" json:"banned"`
	Verified    bool      `gorm:"default:true" json:"verified"`
	TeamID      *uint     `gorm:"index" json:"team_id"`
	Team        *Team     `gorm:"constraint:OnDelete:SET NULL;" json:"team,omitempty"`
	Created     time.Time `json:"created"`
}

func (User) TableName() string { return "users" }

func (u User) IsAdmin() bool { return u.Type == UserTypeAdmin }

type Team struct {
	ID          uint      `gorm:"primaryKey" json:"id"`
	Name        string    `gorm:"size:128;uniqueIndex;not null" json:"name"`
	Email       string    `gorm:"size:128" json:"email"`
	Password    string    `gorm:"size:255;not null" json:"-"`
	Website     string    `gorm:"size:255" json:"website"`
	Affiliation string    `gorm:"size:255" json:"affiliation"`
	Country     string    `gorm:"size:8" json:"country"`
	Hidden      bool      `gorm:"default:false;index" json:"hidden"`
	Banned      bool      `gorm:"default:false;index" json:"banned"`
	CaptainID   *uint     `json:"captain_id"`
	Created     time.Time `json:"created"`
	// ADToken authenticates Attack-Defense flag submission from scripts (X-Team-Token).
	ADToken string `gorm:"size:64;index" json:"-"`
	// ADHost is the team's vulnbox. Every service of that team uses this address and the service port.
	ADHost  string `gorm:"size:255" json:"ad_host"`
	Members []User `gorm:"foreignKey:TeamID" json:"members,omitempty"`
}

func (Team) TableName() string { return "teams" }

type Challenge struct {
	ID             uint      `gorm:"primaryKey" json:"id"`
	Name           string    `gorm:"size:128;not null" json:"name"`
	Description    string    `gorm:"type:text" json:"description"`
	Attribution    string    `gorm:"size:255" json:"attribution"`
	MaxAttempts    int       `gorm:"default:0" json:"max_attempts"`
	Value          int       `json:"value"`
	Category       string    `gorm:"size:80;index" json:"category"`
	Type           string    `gorm:"size:32;default:standard" json:"type"`
	State          string    `gorm:"size:16;default:visible;index" json:"state"`
	Requirements   string    `gorm:"type:text" json:"requirements"`
	ConnectionInfo string    `gorm:"type:text" json:"connection_info"`
	Initial        int       `json:"initial"`
	Minimum        int       `json:"minimum"`
	Decay          int       `json:"decay"`
	Function       string    `gorm:"size:32;default:linear" json:"function"`
	ComposeDir     string    `gorm:"size:512" json:"-"`
	DeployStatus   string    `gorm:"size:32" json:"deploy_status"`
	DeployLog      string    `gorm:"type:text" json:"-"`
	Created        time.Time `json:"created"`
	Flags          []Flag    `gorm:"constraint:OnDelete:CASCADE;" json:"flags,omitempty"`
	Hints          []Hint    `gorm:"constraint:OnDelete:CASCADE;" json:"hints,omitempty"`
	Files          []File    `gorm:"constraint:OnDelete:CASCADE;" json:"files,omitempty"`
	Tags           []Tag     `gorm:"constraint:OnDelete:CASCADE;" json:"tags,omitempty"`
}

func (Challenge) TableName() string { return "challenges" }

func (c Challenge) RequiredIDs() []uint {
	if c.Requirements == "" {
		return nil
	}
	var ids []uint
	if err := json.Unmarshal([]byte(c.Requirements), &ids); err != nil {
		return nil
	}
	return ids
}

type Flag struct {
	ID          uint   `gorm:"primaryKey" json:"id"`
	ChallengeID uint   `gorm:"index;not null" json:"challenge_id"`
	Type        string `gorm:"size:16;default:static" json:"type"`
	Content     string `gorm:"type:text;not null" json:"content"`
	Data        string `gorm:"size:32" json:"data"`
}

func (Flag) TableName() string { return "flags" }

type Hint struct {
	ID          uint   `gorm:"primaryKey" json:"id"`
	ChallengeID uint   `gorm:"index;not null" json:"challenge_id"`
	Title       string `gorm:"size:128" json:"title"`
	Content     string `gorm:"type:text;not null" json:"content"`
	Cost        int    `json:"cost"`
}

func (Hint) TableName() string { return "hints" }

type File struct {
	ID          uint   `gorm:"primaryKey" json:"id"`
	Type        string `gorm:"size:32;default:challenge" json:"type"`
	Location    string `gorm:"size:512;not null" json:"location"`
	Name        string `gorm:"size:255" json:"name"`
	ChallengeID *uint  `gorm:"index" json:"challenge_id"`
	PageID      *uint  `gorm:"index" json:"page_id"`
}

func (File) TableName() string { return "files" }

type Tag struct {
	ID          uint   `gorm:"primaryKey" json:"id"`
	ChallengeID uint   `gorm:"index;not null" json:"challenge_id"`
	Value       string `gorm:"size:80" json:"value"`
}

func (Tag) TableName() string { return "tags" }

type Submission struct {
	ID          uint      `gorm:"primaryKey" json:"id"`
	ChallengeID uint      `gorm:"index;not null" json:"challenge_id"`
	UserID      uint      `gorm:"index;not null" json:"user_id"`
	TeamID      *uint     `gorm:"index" json:"team_id"`
	IP          string    `gorm:"size:64" json:"ip"`
	Provided    string    `gorm:"type:text" json:"provided"`
	Type        string    `gorm:"size:16;index" json:"type"`
	Date        time.Time `gorm:"index" json:"date"`
	Challenge   Challenge `gorm:"constraint:OnDelete:CASCADE;" json:"challenge,omitempty"`
	User        User      `gorm:"constraint:OnDelete:CASCADE;" json:"user,omitempty"`
}

func (Submission) TableName() string { return "submissions" }

type Solve struct {
	ID          uint      `gorm:"primaryKey" json:"id"`
	ChallengeID uint      `gorm:"uniqueIndex:idx_solve_user;uniqueIndex:idx_solve_team;index;not null" json:"challenge_id"`
	UserID      uint      `gorm:"uniqueIndex:idx_solve_user;index;not null" json:"user_id"`
	TeamID      *uint     `gorm:"uniqueIndex:idx_solve_team;index" json:"team_id"`
	IP          string    `gorm:"size:64" json:"ip"`
	Provided    string    `gorm:"type:text" json:"provided"`
	Date        time.Time `gorm:"index" json:"date"`
	Challenge   Challenge `gorm:"constraint:OnDelete:CASCADE;" json:"challenge,omitempty"`
	User        User      `gorm:"constraint:OnDelete:CASCADE;" json:"user,omitempty"`
}

func (Solve) TableName() string { return "solves" }

type Unlock struct {
	ID     uint      `gorm:"primaryKey" json:"id"`
	UserID uint      `gorm:"index;not null" json:"user_id"`
	TeamID *uint     `gorm:"index" json:"team_id"`
	Target uint      `gorm:"index;not null" json:"target"`
	Type   string    `gorm:"size:32;index" json:"type"`
	Date   time.Time `json:"date"`
}

func (Unlock) TableName() string { return "unlocks" }

type Award struct {
	ID          uint      `gorm:"primaryKey" json:"id"`
	UserID      uint      `gorm:"index;not null" json:"user_id"`
	TeamID      *uint     `gorm:"index" json:"team_id"`
	Name        string    `gorm:"size:80" json:"name"`
	Description string    `gorm:"type:text" json:"description"`
	Value       int       `json:"value"`
	Category    string    `gorm:"size:80" json:"category"`
	Icon        string    `gorm:"size:64" json:"icon"`
	Date        time.Time `json:"date"`
	User        User      `gorm:"constraint:OnDelete:CASCADE;" json:"user,omitempty"`
}

func (Award) TableName() string { return "awards" }

type Page struct {
	ID           uint   `gorm:"primaryKey" json:"id"`
	Title        string `gorm:"size:80" json:"title"`
	Route        string `gorm:"size:128;uniqueIndex" json:"route"`
	Content      string `gorm:"type:text" json:"content"`
	Draft        bool   `gorm:"default:false" json:"draft"`
	Hidden       bool   `gorm:"default:false" json:"hidden"`
	AuthRequired bool   `gorm:"default:false" json:"auth_required"`
}

func (Page) TableName() string { return "pages" }

type Notification struct {
	ID      uint      `gorm:"primaryKey" json:"id"`
	Title   string    `gorm:"size:80" json:"title"`
	Content string    `gorm:"type:text" json:"content"`
	Date    time.Time `json:"date"`
	UserID  *uint     `gorm:"index" json:"user_id"`
}

func (Notification) TableName() string { return "notifications" }

type ConfigKV struct {
	ID    uint   `gorm:"primaryKey" json:"id"`
	Key   string `gorm:"size:64;uniqueIndex;not null" json:"key"`
	Value string `gorm:"type:text" json:"value"`
}

func (ConfigKV) TableName() string { return "config" }

type Tracking struct {
	ID     uint      `gorm:"primaryKey" json:"id"`
	Type   string    `gorm:"size:32" json:"type"`
	IP     string    `gorm:"size:64" json:"ip"`
	UserID *uint     `gorm:"index" json:"user_id"`
	Date   time.Time `json:"date"`
}

func (Tracking) TableName() string { return "tracking" }

// ADChecker is a running checker process. One checker serves the same service on every team.
type ADChecker struct {
	ID             uint   `gorm:"primaryKey" json:"id"`
	Name           string `gorm:"size:80;uniqueIndex;not null" json:"name"`
	URL            string `gorm:"size:512" json:"url"`
	ComposeDir     string `gorm:"size:512" json:"-"`
	CheckerService string `gorm:"size:64" json:"checker_service"`
	DeployStatus   string `gorm:"size:32" json:"deploy_status"`
	DeployLog      string `gorm:"type:text" json:"-"`
}

func (ADChecker) TableName() string { return "ad_checkers" }

type ADService struct {
	ID             uint       `gorm:"primaryKey" json:"id"`
	Name           string     `gorm:"size:80;not null" json:"name"`
	Description    string     `gorm:"type:text" json:"description"`
	Port           int        `json:"port"`
	CheckerID      *uint      `gorm:"index" json:"checker_id"`
	Checker        *ADChecker `gorm:"foreignKey:CheckerID" json:"checker,omitempty"`
	CheckerURL     string     `gorm:"size:512" json:"checker_url"`
	AttackReward   int        `gorm:"default:10" json:"attack_reward"`
	SLAReward      int        `gorm:"default:2" json:"sla_reward"`
	DefensePenalty int        `gorm:"default:10" json:"defense_penalty"`
	DefenseReward  int        `gorm:"default:5" json:"defense_reward"`
	DownPenalty    int        `gorm:"default:20" json:"down_penalty"`
	Active         bool       `gorm:"default:true" json:"active"`
	ComposeDir     string     `gorm:"size:512" json:"-"`
	CheckerService string     `gorm:"size:64" json:"checker_service"`
	DeployStatus   string     `gorm:"size:32" json:"deploy_status"`
	DeployLog      string     `gorm:"type:text" json:"-"`
}

func (ADService) TableName() string { return "ad_services" }

type ADInstance struct {
	ID        uint      `gorm:"primaryKey" json:"id"`
	TeamID    uint      `gorm:"uniqueIndex:idx_ad_inst;index;not null" json:"team_id"`
	ServiceID uint      `gorm:"uniqueIndex:idx_ad_inst;index;not null" json:"service_id"`
	Host      string    `gorm:"size:255" json:"host"`
	Port      int       `json:"port"`
	Team      Team      `gorm:"constraint:OnDelete:CASCADE;" json:"team,omitempty"`
	Service   ADService `gorm:"constraint:OnDelete:CASCADE;" json:"service,omitempty"`
}

func (ADInstance) TableName() string { return "ad_instances" }

type ADRound struct {
	ID         uint       `gorm:"primaryKey" json:"id"`
	Number     int        `gorm:"uniqueIndex;not null" json:"number"`
	Started    time.Time  `json:"started"`
	Ended      *time.Time `json:"ended"`
	ChecksDone bool       `gorm:"default:false" json:"checks_done"`
}

func (ADRound) TableName() string { return "ad_rounds" }

// ADFlag lives in Redis, not Postgres. The struct is the document the flag store reads and writes.
type ADFlag struct {
	ID         uint      `gorm:"primaryKey" json:"id"`
	RoundID    uint      `gorm:"index;not null" json:"round_id"`
	ServiceID  uint      `gorm:"index;not null" json:"service_id"`
	TeamID     uint      `gorm:"index;not null" json:"team_id"`
	Flag       string    `gorm:"size:128;uniqueIndex;not null" json:"-"`
	Issued     time.Time `json:"issued"`
	Expires    time.Time `gorm:"index" json:"expires"`
	PutStatus  string    `gorm:"size:16" json:"put_status"`
	PutMessage string    `gorm:"size:255" json:"put_message"`
	// Settled is set once the flag expired; Defended means it lived its whole
	// lifetime on a working service without being stolen.
	Settled  bool `gorm:"default:false;index" json:"settled"`
	Defended bool `gorm:"default:false;index" json:"defended"`
}

func (ADFlag) TableName() string { return "ad_flags" }

type ADSteal struct {
	ID             uint      `gorm:"primaryKey" json:"id"`
	FlagID         uint      `gorm:"uniqueIndex:idx_ad_steal;index;not null" json:"flag_id"`
	AttackerTeamID uint      `gorm:"uniqueIndex:idx_ad_steal;index;not null" json:"attacker_team_id"`
	VictimTeamID   uint      `gorm:"index;not null" json:"victim_team_id"`
	ServiceID      uint      `gorm:"index;not null" json:"service_id"`
	RoundID        uint      `gorm:"index;not null" json:"round_id"`
	Points         int       `json:"points"`
	UserID         uint      `json:"user_id"`
	Date           time.Time `json:"date"`
	Attacker       Team      `gorm:"foreignKey:AttackerTeamID;constraint:OnDelete:CASCADE;" json:"attacker,omitempty"`
	Victim         Team      `gorm:"foreignKey:VictimTeamID;constraint:OnDelete:CASCADE;" json:"victim,omitempty"`
}

func (ADSteal) TableName() string { return "ad_steals" }

type ADCheck struct {
	ID        uint      `gorm:"primaryKey" json:"id"`
	RoundID   uint      `gorm:"index;not null" json:"round_id"`
	ServiceID uint      `gorm:"index;index:idx_ad_check_tsd,priority:2;not null" json:"service_id"`
	TeamID    uint      `gorm:"index;index:idx_ad_check_tsd,priority:1;not null" json:"team_id"`
	Status    string    `gorm:"size:16;index" json:"status"`
	Message   string    `gorm:"size:255" json:"message"`
	Date      time.Time `gorm:"index:idx_ad_check_tsd,priority:3" json:"date"`
}

func (ADCheck) TableName() string { return "ad_checks" }

type Account struct {
	ID          uint
	Name        string
	Type        string
	Hidden      bool
	Banned      bool
	Website     string
	Affiliation string
	Country     string
	Created     time.Time
	TeamID      *uint
}

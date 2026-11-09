package models

import (
	"time"
)

type Hero struct {
	Id            int      `json:"id" gorm:"primaryKey"`
	LocalizedName string   `json:"localizedName" gorm:"column:localized_name"`
	Img           string   `json:"img"`
	IconUrl       string   `json:"iconUrl"`
	Roles         []string `json:"roles" gorm:"serializer:json"` // Or however you store roles
}

// --- Enums ---

type MatchupKind int

const (
	MatchupKindVs   MatchupKind = 1
	MatchupKindWith MatchupKind = 2
)

// Assuming Bracket and Position are also enums/ints in your C# code
type Bracket int
type Position int

// --- Entities ---

type HeroMatchup struct {
	HeroId      int         `json:"heroId" gorm:"primaryKey"`
	OtherHeroId int         `json:"otherHeroId" gorm:"primaryKey"`
	Bracket     Bracket     `json:"bracket" gorm:"primaryKey"`
	Kind        MatchupKind `json:"kind" gorm:"primaryKey"`
	Games       int         `json:"games"`
	Wins        int         `json:"wins"`
	Synergy     float64     `json:"synergy"`
}

type HeroMeta struct {
	HeroId   int      `json:"heroId" gorm:"primaryKey"`
	Bracket  Bracket  `json:"bracket" gorm:"primaryKey"`
	Position Position `json:"position" gorm:"primaryKey"`
	Games    int      `json:"games"`
	Wins     int      `json:"wins"`
}

type LaneMatchup struct {
	HeroId      int      `json:"heroId" gorm:"primaryKey"`
	Position    Position `json:"position" gorm:"primaryKey"`
	OtherHeroId int      `json:"otherHeroId" gorm:"primaryKey"`
	IsWith      bool     `json:"isWith" gorm:"primaryKey"`
	Bracket     Bracket  `json:"bracket" gorm:"primaryKey"`
	Games       int      `json:"games"`
	Wins        int      `json:"wins"`
}

type MetaImport struct {
	Id             int       `json:"id" gorm:"primaryKey;autoIncrement"`
	CompletedAtUtc time.Time `json:"completedAtUtc"`
	MatchupCount   int       `json:"matchupCount"`
	MetaCount      int       `json:"metaCount"`
}

type Player struct {
	AccountId    int64     `json:"accountId" gorm:"primaryKey"`
	Name         string    `json:"name"`
	AvatarUrl    *string   `json:"avatarUrl"` // Pointer for nullable string
	RankTier     *int      `json:"rankTier"`  // Pointer for nullable int
	FetchedAtUtc time.Time `json:"fetchedAtUtc"`

	// GORM will automatically map this relationship based on the AccountId foreign key
	Heroes []PlayerHero `json:"heroes" gorm:"foreignKey:AccountId"`
}

type PlayerHero struct {
	AccountId     int64      `json:"accountId" gorm:"primaryKey"`
	HeroId        int        `json:"heroId" gorm:"primaryKey"`
	Position      Position   `json:"position" gorm:"primaryKey"`
	Games         int        `json:"games"`
	Wins          int        `json:"wins"`
	LastPlayedUtc *time.Time `json:"lastPlayedUtc"` // Pointer for nullable DateTime
}

type MetaStatusDto struct {
	Importing                   bool       `json:"importing"`
	LastImportFailed            bool       `json:"lastImportFailed"`
	RefreshEnabled              bool       `json:"refreshEnabled"` // React expects this!
	UpdatedAtUtc                *time.Time `json:"updatedAtUtc"`   // React expects this!
	MatchupCount                int        `json:"matchupCount"`
	MetaCount                   int        `json:"metaCount"`
	ManualRefreshAvailableAtUtc *time.Time `json:"manualRefreshAvailableAtUtc"`
}

type PlayerHeroDto struct {
	HeroId        int        `json:"heroId"`
	Position      Position   `json:"position"` // Make sure to use your models.Position type
	Games         int        `json:"games"`
	Wins          int        `json:"wins"`
	LastPlayedUtc *time.Time `json:"lastPlayedUtc"`
}

type PlayerDto struct {
	AccountId    int64           `json:"accountId"`
	Name         string          `json:"name"`
	AvatarUrl    *string         `json:"avatarUrl"`
	RankTier     *int            `json:"rankTier"`
	Bracket      Bracket         `json:"bracket"`
	FetchedAtUtc time.Time       `json:"fetchedAtUtc"`
	Heroes       []PlayerHeroDto `json:"heroes"`
}

type RecommendationRequest struct {
	SteamIds      []string  `json:"steamIds"`
	Allies        []*int    `json:"allies"`
	Enemies       []*int    `json:"enemies"`
	Bans          []int     `json:"bans"`
	MyPosition    *Position `json:"myPosition"`
	OpenPositions bool      `json:"openPositions"`
	Bracket       *Bracket  `json:"bracket"`
}

type RecommendationsDto struct {
	Bracket        Bracket          `json:"bracket"`
	HasComfort     bool             `json:"hasComfort"`
	Items          []Recommendation `json:"items"`
	EnemyPositions []*Position      `json:"enemyPositions"`
	Swap           *SwapDto         `json:"swap"`
	Positions      []Position       `json:"positions"`     // Used by OpenPositions
	AllyPositions  []*Position      `json:"allyPositions"` // Used by OpenPositions
}

type SwapDto struct {
	Gain    float64   `json:"gain"`
	Heroes  []*int    `json:"heroes"`
	Players []*string `json:"players"`
}

type Recommendation struct {
	HeroId       int          `json:"heroId"`
	Score        float64      `json:"score"`
	WinChance    float64      `json:"winChance"`
	Counter      float64      `json:"counter"`
	Synergy      float64      `json:"synergy"`
	Meta         float64      `json:"meta"`
	Lineup       float64      `json:"lineup"`
	Lane         float64      `json:"lane"`
	Partner      float64      `json:"partner"`
	Comfort      float64      `json:"comfort"`
	VsEnemies    []HeroImpact `json:"vsEnemies"`
	WithAllies   []HeroImpact `json:"withAllies"`
	InLane       []HeroImpact `json:"inLane"`
	LanePartners []HeroImpact `json:"lanePartners"`
	Risks        []HeroImpact `json:"risks"`
}

type Threat struct {
	Recommendation Recommendation `json:"recommendation"`
	Position       Position       `json:"position"`
}

type ScoringWeights struct {
	Counter             float64 `json:"counter"`
	Synergy             float64 `json:"synergy"`
	Meta                float64 `json:"meta"`
	Lineup              float64 `json:"lineup"`
	LineupMinTeam       int     `json:"lineupMinTeam"`
	Lane                float64 `json:"lane"`
	Partner             float64 `json:"partner"`
	Comfort             float64 `json:"comfort"`
	MatchupPrior        int     `json:"matchupPrior"`
	MetaPrior           int     `json:"metaPrior"`
	ComfortPrior        int     `json:"comfortPrior"`
	MinPositionShare    float64 `json:"minPositionShare"`
	MinComfortGames     int     `json:"minComfortGames"`
	ExperiencePoints    float64 `json:"experiencePoints"`
	ExperienceGames     int     `json:"experienceGames"`
	ComfortHalfLifeDays float64 `json:"comfortHalfLifeDays"`
}

const (
	Pos1 Position = 1
	Pos2 Position = 2
	Pos3 Position = 3
	Pos4 Position = 4
	Pos5 Position = 5
)

type DraftFeatures struct {
	Counter float64 `json:"counter"`
	Synergy float64 `json:"synergy"`
	Meta    float64 `json:"meta"`
	Lineup  float64 `json:"lineup"`
	Lane    float64 `json:"lane"`
	Partner float64 `json:"partner"`
}

type HeroImpact struct {
	HeroId int     `json:"heroId"`
	Points float64 `json:"points"`
}

type Swap struct {
	Gain    float64 `json:"gain"`
	Heroes  []int   `json:"heroes"`
	Players []int   `json:"players"`
}

var AllBrackets = []Bracket{1, 2, 3, 4, 5, 6, 7, 8} // Herald to Immortal

// --- STRATZ DTOs ---

type GraphQlResponse[T any] struct {
	Data   *T             `json:"data"`
	Errors []GraphQLError `json:"errors"`
}

type GraphQLError struct {
	Message string `json:"message"`
}

type HeroStatsData[T any] struct {
	HeroStats T `json:"heroStats"`
}

type StratzMatchup struct {
	Advantage []StratzDryad `json:"advantage"`
}

type StratzDryad struct {
	Vs   []StratzPair `json:"vs"`
	With []StratzPair `json:"with"`
}

type StratzPair struct {
	HeroId2    int      `json:"heroId2"`
	MatchCount int      `json:"matchCount"`
	WinCount   int      `json:"winCount"`
	Synergy    *float64 `json:"synergy"`
}

type StratzStats struct {
	Stats []StratzHeroPosition `json:"stats"`
}

type StratzLanes struct {
	LaneOutcome []StratzLaneOutcome `json:"laneOutcome"`
}

type StratzLaneOutcome struct {
	HeroId1       int `json:"heroId1"`
	HeroId2       int `json:"heroId2"`
	MatchCount    int `json:"matchCount"`
	MatchWinCount int `json:"matchWinCount"`
}

type StratzHeroPosition struct {
	HeroId     int    `json:"heroId"`
	Position   string `json:"position"`
	MatchCount int    `json:"matchCount"`
	WinCount   int    `json:"winCount"`
}

type StratzPlayerData struct {
	Player *StratzPlayer `json:"player"`
}

type StratzPlayer struct {
	SteamAccount *StratzSteamAccount     `json:"steamAccount"`
	P1           []StratzHeroPerformance `json:"p1"`
	P2           []StratzHeroPerformance `json:"p2"`
	P3           []StratzHeroPerformance `json:"p3"`
	P4           []StratzHeroPerformance `json:"p4"`
	P5           []StratzHeroPerformance `json:"p5"`
}

type StratzSteamAccount struct {
	Name       *string `json:"name"`
	Avatar     *string `json:"avatar"`
	SeasonRank *int    `json:"seasonRank"`
}

type StratzHeroPerformance struct {
	HeroId             int    `json:"heroId"`
	MatchCount         int    `json:"matchCount"`
	WinCount           int    `json:"winCount"`
	LastPlayedDateTime *int64 `json:"lastPlayedDateTime"`
}

type OpenDotaHero struct {
	Id            int      `json:"id"`
	Name          string   `json:"name"`
	LocalizedName string   `json:"localized_name"`
	Img           string   `json:"img"`
	Roles         []string `json:"roles"`
}

type OpenDotaPublicMatch struct {
	MatchId     int64  `json:"match_id"`
	MatchSeqNum int64  `json:"match_seq_num"`
	RadiantWin  bool   `json:"radiant_win"`
	StartTime   int64  `json:"start_time"`
	Duration    int    `json:"duration"`
	RadiantTeam string `json:"radiant_team"`
	DireTeam    string `json:"dire_team"`
}

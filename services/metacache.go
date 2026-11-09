package services

import (
	"sync"

	"dota-helper/models"

	"gorm.io/gorm"
)

// --- Core Domain Shapes ---

type PairStat struct {
	Games   int
	Wins    int
	Synergy float64
}

type HeroStat struct {
	Games int
	Wins  int
}

type HeroPosKey struct {
	HeroId int
	Pos    models.Position
}

type PairKey struct {
	HeroId      int
	OtherHeroId int
}

type LaneKey struct {
	HeroId      int
	Position    models.Position
	OtherHeroId int
}

type MetaSnapshot struct {
	Vs           map[PairKey]PairStat
	With         map[PairKey]PairStat
	Positions    map[HeroPosKey]HeroStat
	Lanes        map[LaneKey]HeroStat
	LanePartners map[LaneKey]HeroStat

	LaneTotals    map[HeroPosKey]HeroStat
	PartnerTotals map[HeroPosKey]HeroStat
}

// --- MetaCache ---

type MetaCache struct {
	DB *gorm.DB

	mu        sync.Mutex
	snapshots map[models.Bracket]*MetaSnapshot
}

func NewMetaCache(db *gorm.DB) *MetaCache {
	return &MetaCache{
		DB: db,
	}
}

// Get returns the loaded snapshots. If they are not in memory, it fetches them from SQLite.
func (c *MetaCache) Get() (map[models.Bracket]*MetaSnapshot, error) {
	c.mu.Lock()
	defer c.mu.Unlock()

	if c.snapshots != nil {
		return c.snapshots, nil
	}

	var matchups []models.HeroMatchup
	var meta []models.HeroMeta
	var lanes []models.LaneMatchup

	// Equivalent to AsNoTracking().ToListAsync()
	if err := c.DB.Find(&matchups).Error; err != nil {
		return nil, err
	}
	if err := c.DB.Find(&meta).Error; err != nil {
		return nil, err
	}
	if err := c.DB.Find(&lanes).Error; err != nil {
		return nil, err
	}

	c.snapshots = BuildMetaSnapshots(matchups, meta, lanes)
	return c.snapshots, nil
}

// Invalidate clears the cache, forcing a reload on the next Get() call.
func (c *MetaCache) Invalidate() {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.snapshots = nil
}

// BuildMetaSnapshots groups flat DB rows into nested memory maps in a single pass.
func BuildMetaSnapshots(matchups []models.HeroMatchup, meta []models.HeroMeta, lanes []models.LaneMatchup) map[models.Bracket]*MetaSnapshot {
	snapshots := make(map[models.Bracket]*MetaSnapshot)

	// Helper to get or initialize a bracket's snapshot
	getSnap := func(b models.Bracket) *MetaSnapshot {
		if s, exists := snapshots[b]; exists {
			return s
		}
		s := &MetaSnapshot{
			Vs:            make(map[PairKey]PairStat),
			With:          make(map[PairKey]PairStat),
			Positions:     make(map[HeroPosKey]HeroStat),
			Lanes:         make(map[LaneKey]HeroStat),
			LanePartners:  make(map[LaneKey]HeroStat),
			LaneTotals:    make(map[HeroPosKey]HeroStat), // Prevent nil map panic
			PartnerTotals: make(map[HeroPosKey]HeroStat), // Prevent nil map panic
		}
		snapshots[b] = s
		return s
	}

	for _, m := range matchups {
		snap := getSnap(m.Bracket)
		key := PairKey{HeroId: m.HeroId, OtherHeroId: m.OtherHeroId}
		stat := PairStat{Games: m.Games, Wins: m.Wins, Synergy: m.Synergy}

		if m.Kind == models.MatchupKindVs {
			snap.Vs[key] = stat
		} else if m.Kind == models.MatchupKindWith {
			snap.With[key] = stat
		}
	}

	for _, m := range meta {
		snap := getSnap(m.Bracket)
		key := HeroPosKey{HeroId: m.HeroId, Pos: m.Position}
		snap.Positions[key] = HeroStat{Games: m.Games, Wins: m.Wins}
	}

	for _, l := range lanes {
		snap := getSnap(l.Bracket)
		key := LaneKey{HeroId: l.HeroId, Position: l.Position, OtherHeroId: l.OtherHeroId}
		stat := HeroStat{Games: l.Games, Wins: l.Wins}

		if l.IsWith {
			snap.LanePartners[key] = stat
		} else {
			snap.Lanes[key] = stat
		}
	}

	return snapshots
}

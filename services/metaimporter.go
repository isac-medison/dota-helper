package services

import (
	"context"
	"errors"
	"fmt"
	"log"
	"time"

	"dota-helper/models"

	"gorm.io/gorm"
)

const (
	HeroesPerRequest = 32
	SaveChunk        = 500
)

type Snapshot struct {
	Matchups []models.HeroMatchup
	Meta     []models.HeroMeta
	Lanes    []models.LaneMatchup
}

type MetaImporter struct {
	DB        *gorm.DB
	Stratz    *StratzClient
	Heroes    *HeroImporter
	MetaCache *MetaCache
}

func (s *MetaImporter) ImportAsync(ctx context.Context) error {
	// A patch's new hero must be in the list before its matchups are asked for.
	if err := s.Heroes.SyncAsync(ctx); err != nil {
		return fmt.Errorf("failed to sync heroes: %w", err)
	}

	snap, err := s.FetchAsync(nil, ctx)
	if err != nil {
		return fmt.Errorf("failed to fetch snapshot: %w", err)
	}

	// Run all deletes and batch inserts inside a single transaction
	err = s.DB.Transaction(func(tx *gorm.DB) error {
		// ExecuteDeleteAsync equivalent: AllowGlobalUpdate is required by GORM to delete without a WHERE clause
		if err := tx.Session(&gorm.Session{AllowGlobalUpdate: true}).Delete(&models.HeroMatchup{}).Error; err != nil {
			return err
		}
		if err := tx.Session(&gorm.Session{AllowGlobalUpdate: true}).Delete(&models.HeroMeta{}).Error; err != nil {
			return err
		}
		if err := tx.Session(&gorm.Session{AllowGlobalUpdate: true}).Delete(&models.LaneMatchup{}).Error; err != nil {
			return err
		}

		// GORM's CreateInBatches replaces the EF Core chunking + ChangeTracker.Clear() logic
		if err := tx.CreateInBatches(snap.Matchups, SaveChunk).Error; err != nil {
			return err
		}
		if err := tx.CreateInBatches(snap.Meta, SaveChunk).Error; err != nil {
			return err
		}
		if err := tx.CreateInBatches(snap.Lanes, SaveChunk).Error; err != nil {
			return err
		}

		importRecord := models.MetaImport{
			CompletedAtUtc: time.Now().UTC(),
			MatchupCount:   len(snap.Matchups),
			MetaCount:      len(snap.Meta),
		}
		if err := tx.Create(&importRecord).Error; err != nil {
			return err
		}

		return nil
	})

	if err != nil {
		return fmt.Errorf("transaction failed: %w", err)
	}

	s.MetaCache.Invalidate()
	log.Printf("Meta import done: %d matchups, %d position rows", len(snap.Matchups), len(snap.Meta))
	return nil
}

func (s *MetaImporter) FetchAsync(week *int64, ctx context.Context) (*Snapshot, error) {
	var heroIds []int
	// Pluck extracts a single column into a slice
	if err := s.DB.Model(&models.Hero{}).Order("id").Pluck("id", &heroIds).Error; err != nil {
		return nil, err
	}

	var matchups []models.HeroMatchup
	var meta []models.HeroMeta
	var lanes []models.LaneMatchup

	for _, bracket := range models.AllBrackets {

		// 1. Fetch Matchups in chunks of HeroesPerRequest
		for i := 0; i < len(heroIds); i += HeroesPerRequest {
			end := i + HeroesPerRequest
			if end > len(heroIds) {
				end = len(heroIds)
			}
			batch := heroIds[i:end]

			result, err := s.Stratz.GetMatchupsAsync(batch, bracket, week, ctx)
			if err != nil {
				return nil, err
			}

			for heroId, dryad := range result {
				matchups = append(matchups, toMatchups(heroId, bracket, models.MatchupKindVs, dryad.Vs)...)
				matchups = append(matchups, toMatchups(heroId, bracket, models.MatchupKindWith, dryad.With)...)
			}
		}

		// 2. Fetch Position Stats
		posStats, err := s.Stratz.GetPositionStatsAsync(bracket, week, ctx)
		if err != nil {
			return nil, err
		}

		for _, row := range posStats {
			pos := StratzClient_ParsePosition(row.Position)
			if pos == nil {
				continue
			}
			meta = append(meta, models.HeroMeta{
				HeroId:   row.HeroId,
				Bracket:  bracket,
				Position: *pos,
				Games:    row.MatchCount,
				Wins:     row.WinCount,
			})
		}

		// 3. Fetch Lane Outcomes
		type LaneQuery struct {
			Position models.Position
			IsWith   bool
		}
		var queries []LaneQuery
		for _, p := range allPositions {
			queries = append(queries, LaneQuery{Position: p, IsWith: false})
			if p != models.Pos2 {
				queries = append(queries, LaneQuery{Position: p, IsWith: true})
			}
		}

		for _, q := range queries {
			outcomes, err := s.Stratz.GetLaneOutcomesAsync(bracket, q.Position, q.IsWith, week, ctx)
			if err != nil {
				return nil, err
			}

			seen := make(map[PairKey]bool)
			for _, l := range outcomes {
				if l.HeroId1 == l.HeroId2 {
					continue
				}
				key := PairKey{HeroId: l.HeroId1, OtherHeroId: l.HeroId2}
				if seen[key] {
					continue
				}
				seen[key] = true

				lanes = append(lanes, models.LaneMatchup{
					HeroId:      l.HeroId1,
					Position:    q.Position,
					OtherHeroId: l.HeroId2,
					IsWith:      q.IsWith,
					Bracket:     bracket,
					Games:       l.MatchCount,
					Wins:        l.MatchWinCount,
				})
			}
		}

		log.Printf("STRATZ %v: %d matchups, %d lane rows so far", bracket, len(matchups), len(lanes))
	}

	if len(matchups) == 0 || len(meta) == 0 || len(lanes) == 0 {
		return nil, errors.New(fmt.Sprintf(
			"STRATZ returned an empty snapshot (%d matchups, %d meta rows, %d lane rows), keeping previous data",
			len(matchups), len(meta), len(lanes)))
	}

	return &Snapshot{Matchups: matchups, Meta: meta, Lanes: lanes}, nil
}

func toMatchups(heroId int, bracket models.Bracket, kind models.MatchupKind, pairs []StratzPair) []models.HeroMatchup {
	var res []models.HeroMatchup
	seen := make(map[int]bool)

	for _, p := range pairs {
		if p.HeroId2 == heroId {
			continue
		}
		// Equivalent to DistinctBy(p => p.HeroId2)
		if seen[p.HeroId2] {
			continue
		}
		seen[p.HeroId2] = true

		synergy := 0.0
		if p.Synergy != nil {
			synergy = *p.Synergy
		}

		res = append(res, models.HeroMatchup{
			HeroId:      heroId,
			OtherHeroId: p.HeroId2,
			Bracket:     bracket,
			Kind:        kind,
			Games:       p.MatchCount,
			Wins:        p.WinCount,
			Synergy:     synergy,
		})
	}
	return res
}

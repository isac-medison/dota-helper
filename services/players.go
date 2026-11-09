package services

import (
	"context"
	"errors"
	"fmt"
	"log"
	"strconv"

	"dota-helper/models"
	"dota-helper/utils"

	"time"

	"gorm.io/gorm"
)

type Players struct {
	DB     *gorm.DB
	Stratz *StratzClient
}

const (
	cacheFor           = 24 * time.Hour
	minRefreshInterval = 1 * time.Minute
)

// Get acts as your main endpoint for React to fetch a player
func (s *Players) Get(steamId string, refresh bool) (*models.PlayerDto, error) {
	accountId, err := parseAccountId(steamId)
	if err != nil {
		return nil, errors.New("expected a Steam ID (64-bit) or a Dota friend ID")
	}

	var cached models.Player
	err = s.DB.Preload("Heroes").First(&cached, accountId).Error
	hasCached := err == nil

	if hasCached {
		age := time.Now().UTC().Sub(cached.FetchedAtUtc)
		threshold := cacheFor
		if refresh {
			threshold = minRefreshInterval
		}
		if age < threshold {
			return toPlayerDto(&cached), nil
		}
	}

	// Fetch from STRATZ
	ctx := context.Background()
	fresh, err := s.Stratz.GetPlayerAsync(accountId, ctx)

	if err != nil {
		if hasCached {
			log.Printf("STRATZ profile refresh failed for %d, serving stale cache: %v", accountId, err)
			return toPlayerDto(&cached), nil
		}
		return nil, fmt.Errorf("stratz profile refresh failed: %w", err)
	}

	if fresh == nil || fresh.SteamAccount == nil {
		return nil, errors.New("player not found on STRATZ")
	}

	// Load valid Hero IDs so we don't violate foreign keys if STRATZ has newer heroes than OpenDota
	var heroIds []int
	if err := s.DB.Model(&models.Hero{}).Pluck("id", &heroIds).Error; err != nil {
		return nil, err
	}
	knownHeroes := make(map[int]bool)
	for _, id := range heroIds {
		knownHeroes[id] = true
	}

	// Construct the new Player
	name := fresh.SteamAccount.Name
	if name == "" {
		name = fmt.Sprintf("#%d", accountId)
	}

	player := models.Player{
		AccountId:    accountId,
		Name:         name,
		FetchedAtUtc: time.Now().UTC(),
	}

	// AvatarUrl expects *string, but Avatar is string. Create a pointer to a local copy.
	if fresh.SteamAccount.Avatar != "" {
		avatarStr := fresh.SteamAccount.Avatar
		player.AvatarUrl = &avatarStr
	}

	// SeasonRank is already *int, so check against nil and assign directly.
	if fresh.SteamAccount.SeasonRank != nil {
		player.RankTier = fresh.SteamAccount.SeasonRank
	}

	// Handle RankTier if it's stored as an int; if 0, STRATZ didn't return one
	if fresh.SteamAccount.SeasonRank != nil {
		player.RankTier = fresh.SteamAccount.SeasonRank
	}

	// Map P1-P5 arrays into PlayerHeroes
	stratzPositions := [][]StratzHeroPerformance{fresh.P1, fresh.P2, fresh.P3, fresh.P4, fresh.P5}
	var playerHeroes []models.PlayerHero

	for i, rows := range stratzPositions {
		pos := models.Position(i + 1)
		seen := make(map[int]bool)

		for _, r := range rows {
			if r.MatchCount > 0 && knownHeroes[r.HeroId] && !seen[r.HeroId] {
				seen[r.HeroId] = true

				var lastPlayed *time.Time
				if r.LastPlayedDateTime != 0 {
					t := time.Unix(r.LastPlayedDateTime, 0).UTC()
					lastPlayed = &t
				}

				playerHeroes = append(playerHeroes, models.PlayerHero{
					AccountId:     accountId,
					HeroId:        r.HeroId,
					Position:      pos,
					Games:         r.MatchCount,
					Wins:          r.WinCount,
					LastPlayedUtc: lastPlayed,
				})
			}
		}
	}
	player.Heroes = playerHeroes

	// Transaction to delete old and insert new (mimicking EF Core's execute delete + add)
	err = s.DB.Transaction(func(tx *gorm.DB) error {
		if err := tx.Where("account_id = ?", accountId).Delete(&models.PlayerHero{}).Error; err != nil {
			return err
		}
		if err := tx.Where("account_id = ?", accountId).Delete(&models.Player{}).Error; err != nil {
			return err
		}
		return tx.Create(&player).Error
	})

	if err != nil {
		return nil, fmt.Errorf("failed to save player profile: %w", err)
	}

	utils.SanitizeDefaults(&player)
	return toPlayerDto(&player), nil
}

// Ensure you have a helper like this to map the entity to the DTO React expects:
func toPlayerDto(p *models.Player) *models.PlayerDto {
	// Use the capitalized name
	bracket := BracketFromRankTier(p.RankTier)

	dto := &models.PlayerDto{
		AccountId:    p.AccountId,
		Name:         p.Name,
		AvatarUrl:    p.AvatarUrl,
		RankTier:     p.RankTier,
		Bracket:      bracket,
		FetchedAtUtc: p.FetchedAtUtc,
		Heroes:       make([]models.PlayerHeroDto, len(p.Heroes)),
	}

	for i, h := range p.Heroes {
		dto.Heroes[i] = models.PlayerHeroDto{
			HeroId:        h.HeroId,
			Position:      h.Position,
			Games:         h.Games,
			Wins:          h.Wins,
			LastPlayedUtc: h.LastPlayedUtc, // Ensure this field exists on PlayerHeroDto
		}
	}
	return dto
}

// --- Helpers ---

const steamId64Base = 76561197960265728

// parseAccountId converts a 64-bit Steam ID string into a 32-bit Dota Account ID,
// or returns it as-is if it's already a 32-bit ID.
func parseAccountId(steamIdStr string) (int64, error) {
	val, err := strconv.ParseInt(steamIdStr, 10, 64)
	if err != nil {
		return 0, err
	}
	if val >= steamId64Base {
		return val - steamId64Base, nil
	}
	return val, nil
}

// BracketFromRankTier replaces Brackets.FromRankTier
func BracketFromRankTier(tier *int) models.Bracket {
	if tier == nil || *tier < 10 {
		return 0 // Unknown or uncalibrated
	}

	// Dota rank tiers: 11-15 (Herald), 21-25 (Guardian), ..., 71-75 (Divine), 80+ (Immortal)
	// Integer division drops the stars and extracts the base medal (1 through 8).
	medal := *tier / 10

	// Cap at 8 (Immortal) just in case Valve introduces weird higher tier IDs in the future
	if medal > 8 {
		medal = 8
	}

	return models.Bracket(medal)
}

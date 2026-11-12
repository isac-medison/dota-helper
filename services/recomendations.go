package services

import (
	"errors"
	"log"
	"math"
	"time"

	"dota-helper/models"
	"dota-helper/utils"
)

const Top = 10
const TeamSize = 5

type Recommendations struct {
	Players   *Players // Changed to lowercase
	MetaCache *MetaCache
}

func (s *Recommendations) Recommend(req models.RecommendationRequest) (*models.RecommendationsDto, error) {
	party, bracket, meta, err := s.load(req)
	if err != nil {
		return nil, err
	}

	steamIds := req.SteamIds
	if steamIds == nil {
		steamIds = []string{}
	}

	// Get non-null enemies (equivalent to OfType<int>())
	var enemies []int
	for _, e := range req.Enemies {
		if e != nil {
			enemies = append(enemies, *e)
		}
	}

	// Guess enemy positions
	enemyPositions := slotPositions(req.Enemies, meta)

	if req.OpenPositions {
		return s.openPositions(req, party, bracket, meta, enemies, enemyPositions), nil
	}

	// Identify my player comfort
	var player *models.PlayerDto
	if req.MyPosition != nil {
		myIndex := int(*req.MyPosition) - 1
		if myIndex >= 0 && myIndex < len(party) {
			player = party[myIndex]
		}
	}

	var comfort map[HeroPosKey]HeroStat
	if player != nil && len(player.Heroes) > 0 {
		comfort = comfortOf(player)
	}

	// Determine allies and pinned positions
	type allyPos struct {
		hero     int
		position models.Position
	}
	var allies []allyPos
	for i, hero := range req.Allies {
		pos := models.Position(i + 1)
		if hero != nil && (req.MyPosition == nil || pos != *req.MyPosition) {
			allies = append(allies, allyPos{hero: *hero, position: pos})
		}
	}

	pinned := make(map[int]models.Position)
	var allyHeroes []int
	for _, a := range allies {
		allyHeroes = append(allyHeroes, a.hero)

		idx := int(a.position) - 1
		if idx >= 0 && idx < len(steamIds) && steamIds[idx] != "" {
			pinned[a.hero] = a.position
		}
	}

	draft := DraftInput{
		Allies:     allyHeroes,
		Enemies:    enemies,
		Bans:       req.Bans,
		MyPosition: req.MyPosition,
		Pinned:     pinned,
	}

	// Get Top recommendations
	items := Recommend(draft, meta, comfort, nil)
	if len(items) > Top {
		items = items[:Top]
	}
	// Party comfort for swaps
	partyComfort := make([]map[HeroPosKey]HeroStat, len(req.Allies))
	for i := range req.Allies {
		if i < len(steamIds) && steamIds[i] != "" && i < len(party) && party[i] != nil {
			partyComfort[i] = comfortOf(party[i])
		}
	}

	var swapDto *models.SwapDto
	if swap := BestSwap(req.Allies, partyComfort, enemies, meta, nil); swap != nil {
		// Slot i gets the hero and the player that sit in slots swap.Heroes[i] and swap.Players[i] now.
		swapDto = &models.SwapDto{
			Gain:     swap.Gain,
			Allies:   make([]*int, len(swap.Heroes)),
			SteamIds: make([]*string, len(swap.Players)),
		}
		for i, hIdx := range swap.Heroes {
			if hIdx < len(req.Allies) {
				swapDto.Allies[i] = req.Allies[hIdx]
			}
		}
		for i, pIdx := range swap.Players {
			if pIdx < len(steamIds) && steamIds[pIdx] != "" {
				id := steamIds[pIdx]
				swapDto.SteamIds[i] = &id
			}
		}
	}
	dto := &models.RecommendationsDto{
		Bracket:        bracket,
		ComfortUsed:    len(comfort) > 0,
		Items:          items,
		EnemyPositions: enemyPositions,
		Swap:           swapDto,
	}
	utils.SanitizeDefaults(dto)
	return dto, nil
}

func (s *Recommendations) Threats(req models.RecommendationRequest) ([]models.Threat, error) {
	_, _, meta, err := s.load(req)
	if err != nil {
		return nil, err
	}

	var allies, enemies []int
	for _, a := range req.Allies {
		if a != nil {
			allies = append(allies, *a)
		}
	}
	for _, e := range req.Enemies {
		if e != nil {
			enemies = append(enemies, *e)
		}
	}

	draft := DraftInput{
		Allies:  allies,
		Enemies: enemies,
		Bans:    req.Bans,
	}

	threats := Threats(draft, meta, nil)
	if len(threats) > Top {
		threats = threats[:Top]
	}
	utils.SanitizeDefaults(threats)
	return threats, nil
}

// --- Internal Helpers ---

func (s *Recommendations) openPositions(req models.RecommendationRequest, party []*models.PlayerDto, bracket models.Bracket, meta *MetaSnapshot, enemies []int, enemyPositions []*models.Position) *models.RecommendationsDto {
	// The party hasn't split positions yet: every hero is scored on each position the allies' heroes
	// leave free, with the comfort of whoever in the party plays it best.
	var allies []int
	for _, a := range req.Allies {
		if a != nil {
			allies = append(allies, *a)
		}
	}

	var comforts []map[HeroPosKey]HeroStat
	for _, p := range party {
		if p != nil && len(p.Heroes) > 0 {
			comforts = append(comforts, comfortOf(p))
		}
	}
	var comfort map[HeroPosKey]HeroStat
	if len(comforts) > 0 {
		comfort = BestOfParty(comforts, nil)
	}

	picks := OnFreePositions(DraftInput{Allies: allies, Enemies: enemies, Bans: req.Bans}, meta, comfort, nil)
	if len(picks) > Top {
		picks = picks[:Top]
	}

	items := make([]models.Recommendation, len(picks))
	positions := make([]models.Position, len(picks))
	for i, p := range picks {
		items[i] = p.Recommendation
		positions[i] = p.Position
	}

	dto := &models.RecommendationsDto{
		Bracket:        bracket,
		ComfortUsed:    len(comfort) > 0,
		Items:          items,
		EnemyPositions: enemyPositions,
		Positions:      positions,
		AllyPositions:  slotPositions(req.Allies, meta),
	}
	utils.SanitizeDefaults(dto)
	return dto
}

// slotPositions guesses the position of the hero in each slot; empty slots stay nil.
func slotPositions(slots []*int, meta *MetaSnapshot) []*models.Position {
	var heroes []int
	for _, h := range slots {
		if h != nil {
			heroes = append(heroes, *h)
		}
	}
	guessed := Assign(heroes, meta, nil).Positions

	positions := make([]*models.Position, len(slots))
	next := 0
	for i, h := range slots {
		if h != nil && next < len(guessed) {
			pos := guessed[next]
			next++
			positions[i] = &pos
		}
	}
	return positions
}

// comfortOf is the player's own record by hero and position, with old games fading out.
func comfortOf(player *models.PlayerDto) map[HeroPosKey]HeroStat {
	now := time.Now().Unix()
	result := make(map[HeroPosKey]HeroStat, len(player.Heroes))
	for _, h := range player.Heroes {
		var lastPlayed *int64
		if h.LastPlayedUtc != nil {
			t := h.LastPlayedUtc.Unix()
			lastPlayed = &t
		}
		result[HeroPosKey{HeroId: h.HeroId, Pos: h.Position}] = RecentComfort(h.Games, h.Wins, lastPlayed, now, nil)
	}
	return result
}

func (s *Recommendations) load(req models.RecommendationRequest) ([]*models.PlayerDto, models.Bracket, *MetaSnapshot, error) {
	if len(req.Allies) > TeamSize || len(req.Enemies) > TeamSize {
		return nil, 0, nil, errors.New("a team has at most five heroes")
	}

	// Validate duplicates
	seen := make(map[int]bool)
	var heroCount int
	check := func(list []*int) {
		for _, h := range list {
			if h != nil {
				seen[*h] = true
				heroCount++
			}
		}
	}
	check(req.Allies)
	check(req.Enemies)
	for _, b := range req.Bans {
		seen[b] = true
		heroCount++
	}

	if heroCount != len(seen) {
		return nil, 0, nil, errors.New("a hero can only be in the draft once")
	}

	// Load Party
	steamIds := req.SteamIds
	if steamIds == nil {
		steamIds = []string{}
	}
	party := make([]*models.PlayerDto, len(steamIds))
	var totalRank, rankCount int

	for i, id := range steamIds {
		if id == "" {
			continue
		}
		player, err := s.Players.Get(id, false)
		if err != nil {
			log.Printf("Couldn't load profile %s, going on without it: %v", id, err)
			continue
		}
		party[i] = player
		if player.RankTier != nil && *player.RankTier > 0 {
			totalRank += *player.RankTier
			rankCount++
		}
	}

	// Calculate Bracket
	var bracket models.Bracket
	if req.Bracket != nil && *req.Bracket != models.BracketUnknown {
		bracket = *req.Bracket
	} else {
		var avgRank *int
		if rankCount > 0 {
			avg := int(math.Round(float64(totalRank) / float64(rankCount)))
			avgRank = &avg
		}
		bracket = BracketFromRankTier(avgRank)
	}

	metaCacheData, err := s.MetaCache.Get()
	if err != nil {
		return nil, 0, nil, err
	}

	meta, ok := metaCacheData[bracket]

	// No rank to go by, or that bracket isn't imported: take the highest bracket there is data for.
	if !ok {
		for i := len(models.AllBrackets) - 1; i >= 0 && !ok; i-- {
			if fallback, exists := metaCacheData[models.AllBrackets[i]]; exists {
				if bracket != models.BracketUnknown {
					log.Printf("Warning: Bracket %v not found, falling back to %v", bracket, models.AllBrackets[i])
				}
				meta, bracket, ok = fallback, models.AllBrackets[i], true
			}
		}

		if !ok {
			return nil, 0, nil, errors.New("STRATZ meta isn't imported yet")
		}
	}

	return party, bracket, meta, nil
}

package services

import (
	"errors"
	"log"
	"math"

	"dota-helper/models"
	"dota-helper/utils"
)

const Top = 10

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
	guessed := Assign(enemies, meta, nil).Positions
	enemyPositions := make([]*models.Position, len(req.Enemies))
	for i, h := range req.Enemies {
		if h == nil {
			enemyPositions[i] = nil
		} else {
			pos := guessed[0]
			guessed = guessed[1:]
			enemyPositions[i] = &pos
		}
	}

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
		comfort = s.comfort(player)
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
			partyComfort[i] = s.comfort(party[i])
		}
	}

	var swapDto *models.SwapDto
	if swap := BestSwap(req.Allies, partyComfort, enemies, meta, nil); swap != nil {
		swapDto = &models.SwapDto{
			Gain:    swap.Gain,
			Heroes:  make([]*int, len(swap.Heroes)),
			Players: make([]*string, len(swap.Players)),
		}
		for i, hIdx := range swap.Heroes {
			if hIdx < len(req.Allies) {
				swapDto.Heroes[i] = req.Allies[hIdx]
			}
		}
		for i, pIdx := range swap.Players {
			if pIdx < len(steamIds) {
				swapDto.Players[i] = &steamIds[pIdx]
			}
		}
	}
	dto := &models.RecommendationsDto{
		Bracket:        bracket,
		HasComfort:     comfort != nil,
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
	// ... (Similar translation for OpenPositions using standard Go slices)
	return &models.RecommendationsDto{} // Placeholder
}

func (s *Recommendations) comfort(player *models.PlayerDto) map[HeroPosKey]HeroStat {
	result := make(map[HeroPosKey]HeroStat)
	// ... (Translation of Comfort dictionary logic)
	return result
}

func (s *Recommendations) load(req models.RecommendationRequest) ([]*models.PlayerDto, models.Bracket, *MetaSnapshot, error) {
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
	if req.Bracket != nil {
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

	// FIX: If the exact requested bracket isn't in the database,
	// fallback to the first available bracket we have instead of crashing.
	if !ok {
		for fallbackBracket, fallbackMeta := range metaCacheData {
			log.Printf("Warning: Bracket %v not found, falling back to %v", bracket, fallbackBracket)
			meta = fallbackMeta
			bracket = fallbackBracket // Update to the bracket we are actually using
			ok = true
			break
		}

		// If it's STILL not ok, the map is truly empty
		if !ok {
			return nil, 0, nil, errors.New("STRATZ meta isn't imported yet")
		}
	}

	return party, bracket, meta, nil
}

package services

import (
	"dota-helper/models"
	"math"
	"sort"
)

const TopRisks = 3
const MinSwapGain = 1.5
const MoveCost = 0.5

var SlotOrders [][]int

func init() {
	SlotOrders = getOrders([]int{0, 1, 2, 3, 4})
}

func RecentComfort(games int, wins int, lastPlayedUtc *int64, nowUtc int64, w *models.ScoringWeights) HeroStat {
	if lastPlayedUtc == nil {
		return HeroStat{Games: games, Wins: wins}
	}
	if w == nil {
		w = defaultWeights()
	}

	ageSeconds := nowUtc - *lastPlayedUtc
	if ageSeconds < 0 {
		ageSeconds = 0
	}
	ageDays := float64(ageSeconds) / (24 * 3600)
	factor := math.Pow(0.5, ageDays/w.ComfortHalfLifeDays)

	return HeroStat{
		Games: int(math.Round(float64(games) * factor)),
		Wins:  int(math.Round(float64(wins) * factor)),
	}
}

type DraftInput struct {
	Allies     []int
	Enemies    []int
	Bans       []int
	MyPosition *models.Position
	Pinned     map[int]models.Position
}

func Recommend(
	draft DraftInput,
	meta *MetaSnapshot,
	comfort map[HeroPosKey]HeroStat,
	weights *models.ScoringWeights,
) []models.Recommendation {
	if weights == nil {
		weights = defaultWeights()
	}
	w := *weights

	taken := make(map[int]bool)
	for _, h := range draft.Allies {
		taken[h] = true
	}
	for _, h := range draft.Enemies {
		taken[h] = true
	}
	for _, h := range draft.Bans {
		taken[h] = true
	}

	pinned := make(map[int]models.Position)
	if draft.Pinned != nil {
		for h, p := range draft.Pinned {
			if draft.MyPosition == nil || p != *draft.MyPosition {
				pinned[h] = p
			}
		}
	}

	baseScore := Score(Features(draft.Allies, draft.Enemies, meta, w, pinned), w)
	alliesAssign := Assign(draft.Allies, meta, pinned)
	alliesLineup := alliesAssign.Fit
	allyPositions := alliesAssign.Positions

	enemyPositions := Assign(draft.Enemies, meta, nil).Positions

	var heroes []int
	heroSet := make(map[int]bool)
	for k := range meta.Positions {
		if !heroSet[k.HeroId] {
			heroSet[k.HeroId] = true
			heroes = append(heroes, k.HeroId)
		}
	}

	plays := func(hero int, p *models.Position) bool {
		atPos := At(meta.Positions, hero, p).Games
		atAll := At(meta.Positions, hero, nil).Games
		return float64(atPos) >= w.MinPositionShare*float64(atAll)
	}

	type HeroPos struct {
		HeroId int
		Pos    models.Position
	}
	var counterPicks []HeroPos

	enemyPosSet := make(map[models.Position]bool)
	for _, p := range enemyPositions {
		enemyPosSet[p] = true
	}

	for _, q := range allPositions {
		if !enemyPosSet[q] {
			for _, e := range heroes {
				if !taken[e] && plays(e, &q) {
					counterPicks = append(counterPicks, HeroPos{HeroId: e, Pos: q})
				}
			}
		}
	}

	var results []models.Recommendation

	for _, hero := range heroes {
		if taken[hero] {
			continue
		}

		heroMeta := At(meta.Positions, hero, draft.MyPosition)

		var heroComfort HeroStat
		if comfort != nil {
			heroComfort = At(comfort, hero, draft.MyPosition)
		}

		if draft.MyPosition != nil && float64(heroComfort.Games) < float64(w.MinComfortGames) && !plays(hero, draft.MyPosition) {
			continue
		}

		var vsEnemies []models.HeroImpact
		var counter float64
		for _, e := range draft.Enemies {
			pts := w.Counter * Counter(meta, hero, e, w.MatchupPrior)
			counter += pts
			vsEnemies = append(vsEnemies, models.HeroImpact{HeroId: e, Points: pts})
		}

		var withAllies []models.HeroImpact
		var synergy float64
		for _, a := range draft.Allies {
			pts := w.Synergy * Synergy(meta, hero, a, w.MatchupPrior)
			synergy += pts
			withAllies = append(withAllies, models.HeroImpact{HeroId: a, Points: pts})
		}

		metaScore := w.Meta * WinrateEdge(heroMeta, w.MetaPrior)

		var lineup float64
		if len(draft.Allies) < 5 && len(draft.Allies)+1 >= int(w.LineupMinTeam) {
			teamWithMe := append(make([]int, 0, len(draft.Allies)+1), draft.Allies...)
			teamWithMe = append(teamWithMe, hero)

			pinnedWithMe := make(map[int]models.Position)
			for k, v := range pinned {
				pinnedWithMe[k] = v
			}
			if draft.MyPosition != nil {
				pinnedWithMe[hero] = *draft.MyPosition
			}

			lineup = w.Lineup * (LineupFit(teamWithMe, meta, pinnedWithMe) - alliesLineup)
		}

		var inLane []models.HeroImpact
		var lane float64
		if draft.MyPosition != nil {
			for i, e := range draft.Enemies {
				if i < len(enemyPositions) && FaceInLane(*draft.MyPosition, enemyPositions[i]) {
					pts := w.Lane * Lane(meta, hero, *draft.MyPosition, e, enemyPositions[i], w.MatchupPrior)
					lane += pts
					inLane = append(inLane, models.HeroImpact{HeroId: e, Points: pts})
				}
			}
		}

		var lanePartners []models.HeroImpact
		var partner float64
		if draft.MyPosition != nil {
			for i, a := range draft.Allies {
				if i < len(allyPositions) && ShareLane(*draft.MyPosition, allyPositions[i]) {
					pts := w.Partner * Partner(meta, hero, *draft.MyPosition, a, allyPositions[i], w.MatchupPrior)
					partner += pts
					lanePartners = append(lanePartners, models.HeroImpact{HeroId: a, Points: pts})
				}
			}
		}

		comfortScore := calcComfortScore(heroComfort, w)

		var risks []models.HeroImpact
		seenRisks := make(map[int]bool)

		type RiskEntry struct {
			impact models.HeroImpact
		}
		var riskEntries []RiskEntry

		for _, c := range counterPicks {
			if c.HeroId == hero {
				continue
			}

			pts := w.Counter * Counter(meta, c.HeroId, hero, w.MatchupPrior)
			if draft.MyPosition != nil && FaceInLane(*draft.MyPosition, c.Pos) {
				pts += w.Lane * Lane(meta, c.HeroId, c.Pos, hero, *draft.MyPosition, w.MatchupPrior)
			}

			riskEntries = append(riskEntries, RiskEntry{impact: models.HeroImpact{HeroId: c.HeroId, Points: pts}})
		}

		sort.Slice(riskEntries, func(i, j int) bool {
			return riskEntries[i].impact.Points > riskEntries[j].impact.Points
		})

		for _, r := range riskEntries {
			if !seenRisks[r.impact.HeroId] {
				seenRisks[r.impact.HeroId] = true
				risks = append(risks, r.impact)
				if len(risks) >= TopRisks {
					break
				}
			}
		}

		score := counter + synergy + metaScore + lineup + lane + partner + comfortScore

		results = append(results, models.Recommendation{
			HeroId:       hero,
			Score:        score,
			WinChance:    WinChance(baseScore + score),
			Counter:      counter,
			Synergy:      synergy,
			Meta:         metaScore,
			Lineup:       lineup,
			Lane:         lane,
			Partner:      partner,
			Comfort:      comfortScore,
			VsEnemies:    vsEnemies,
			WithAllies:   withAllies,
			InLane:       inLane,
			LanePartners: lanePartners,
			Risks:        risks,
		})
	}

	sort.Slice(results, func(i, j int) bool {
		return results[i].Score > results[j].Score
	})

	return results
}

func OnFreePositions(
	draft DraftInput,
	meta *MetaSnapshot,
	comfort map[HeroPosKey]HeroStat,
	weights *models.ScoringWeights,
) []models.Threat {
	takenPositions := Assign(draft.Allies, meta, nil).Positions
	takenMap := make(map[models.Position]bool)
	for _, p := range takenPositions {
		takenMap[p] = true
	}

	openDraft := draft
	openDraft.Pinned = nil

	threatMap := make(map[int]models.Threat)
	for _, p := range allPositions {
		if !takenMap[p] {
			pos := p
			openDraft.MyPosition = &pos
			recs := Recommend(openDraft, meta, comfort, weights)

			for _, r := range recs {
				existing, exists := threatMap[r.HeroId]
				if !exists || r.Score > existing.Recommendation.Score {
					threatMap[r.HeroId] = models.Threat{
						Recommendation: r,
						Position:       pos,
					}
				}
			}
		}
	}

	var threats []models.Threat
	for _, t := range threatMap {
		threats = append(threats, t)
	}

	sort.Slice(threats, func(i, j int) bool {
		return threats[i].Recommendation.Score > threats[j].Recommendation.Score
	})

	return threats
}

func Threats(draft DraftInput, meta *MetaSnapshot, weights *models.ScoringWeights) []models.Threat {
	swappedDraft := DraftInput{
		Allies:  draft.Enemies,
		Enemies: draft.Allies,
		Bans:    draft.Bans,
		Pinned:  nil,
	}
	return OnFreePositions(swappedDraft, meta, nil, weights)
}

func BestOfParty(players []map[HeroPosKey]HeroStat, weights *models.ScoringWeights) map[HeroPosKey]HeroStat {
	if weights == nil {
		weights = defaultWeights()
	}
	w := *weights

	best := make(map[HeroPosKey]HeroStat)
	for _, p := range players {
		for k, stat := range p {
			current, exists := best[k]
			if !exists || calcComfortScore(stat, w) > calcComfortScore(current, w) {
				best[k] = stat
			}
		}
	}
	return best
}

func BestSwap(
	allies []*int,
	partyComfort []map[HeroPosKey]HeroStat,
	enemies []int,
	meta *MetaSnapshot,
	weights *models.ScoringWeights,
) *models.Swap {
	if weights == nil {
		weights = defaultWeights()
	}
	w := *weights

	slots := len(SlotOrders[0])
	enemyPositions := Assign(enemies, meta, nil).Positions

	onPosition := make([]map[models.Position]HeroStat, len(partyComfort))
	for i, pComfort := range partyComfort {
		if pComfort == nil {
			continue
		}
		onPosition[i] = make(map[models.Position]HeroStat)
		for _, p := range allPositions {
			var games, wins int
			for k, v := range pComfort {
				if k.Pos == p {
					games += v.Games
					wins += v.Wins
				}
			}
			onPosition[i][p] = HeroStat{Games: games, Wins: wins}
		}
	}

	heroesValue := func(order []int) float64 {
		var team []int
		pinned := make(map[int]models.Position)
		var val float64

		for s := 0; s < slots; s++ {
			if s < len(allies) && allies[order[s]] != nil {
				hero := *allies[order[s]]
				p := models.Position(s + 1)
				team = append(team, hero)
				pinned[hero] = p

				val += w.Meta * WinrateEdge(At(meta.Positions, hero, &p), w.MetaPrior)

				for eIdx, e := range enemies {
					if eIdx < len(enemyPositions) && FaceInLane(p, enemyPositions[eIdx]) {
						val += w.Lane * Lane(meta, hero, p, e, enemyPositions[eIdx], w.MatchupPrior)
					}
				}
			}
		}

		for i := 0; i < len(team); i++ {
			for j := i + 1; j < len(team); j++ {
				if ShareLane(pinned[team[i]], pinned[team[j]]) {
					val += w.Partner * Partner(meta, team[i], pinned[team[i]], team[j], pinned[team[j]], w.MatchupPrior)
				}
			}
		}

		if len(team) >= int(w.LineupMinTeam) {
			val += w.Lineup * LineupFit(team, meta, pinned)
		}
		return val
	}

	playersValue := func(heroOrder []int, playerOrder []int) float64 {
		var val float64
		for s := 0; s < slots; s++ {
			if playerOrder[s] >= len(partyComfort) || partyComfort[playerOrder[s]] == nil {
				continue
			}
			comfort := partyComfort[playerOrder[s]]
			p := models.Position(s + 1)

			var stat HeroStat
			if heroOrder[s] < len(allies) && allies[heroOrder[s]] != nil {
				stat = comfort[HeroPosKey{HeroId: *allies[heroOrder[s]], Pos: p}]
			} else {
				stat = onPosition[playerOrder[s]][p]
			}

			val += calcComfortScore(stat, w)
		}
		return val
	}

	countMoves := func(heroOrder []int, playerOrder []int) int {
		moves := 0
		for s := 0; s < slots; s++ {
			heroChanged := false
			if s < len(allies) && heroOrder[s] < len(allies) {
				if allies[heroOrder[s]] != allies[s] {
					heroChanged = true
				}
			}

			playerChanged := false
			if s < len(partyComfort) && playerOrder[s] < len(partyComfort) {
				if (partyComfort[playerOrder[s]] != nil) != (partyComfort[s] != nil) {
					playerChanged = true
				}
			}

			if heroChanged || playerChanged {
				moves++
			}
		}
		return moves
	}

	current := SlotOrders[0]
	now := heroesValue(current) + playersValue(current, current)

	bestNet := now
	bestValue := now
	bestHeroes := current
	bestPlayers := current

	for _, heroOrder := range SlotOrders {
		hVal := heroesValue(heroOrder)
		for _, playerOrder := range SlotOrders {
			val := hVal + playersValue(heroOrder, playerOrder)
			net := val - MoveCost*float64(countMoves(heroOrder, playerOrder))

			if net > bestNet+1e-9 {
				bestNet = net
				bestValue = val
				bestHeroes = heroOrder
				bestPlayers = playerOrder
			}
		}
	}

	if bestValue-now >= MinSwapGain {
		return &models.Swap{
			Gain:    bestValue - now,
			Heroes:  bestHeroes,
			Players: bestPlayers,
		}
	}

	return nil
}

func calcComfortScore(stat HeroStat, w models.ScoringWeights) float64 {
	if stat.Games == 0 {
		return 0
	}
	exp := float64(stat.Games) / float64(w.ExperienceGames)
	if exp > 1.0 {
		exp = 1.0
	}
	return w.Comfort * (WinrateEdge(stat, w.ComfortPrior) + w.ExperiencePoints*exp)
}

func getOrders(items []int) [][]int {
	if len(items) <= 1 {
		return [][]int{items}
	}
	var res [][]int
	for i, first := range items {
		rest := make([]int, 0, len(items)-1)
		rest = append(rest, items[:i]...)
		rest = append(rest, items[i+1:]...)
		for _, sub := range getOrders(rest) {
			perm := append([]int{first}, sub...)
			res = append(res, perm)
		}
	}
	return res
}

func defaultWeights() *models.ScoringWeights {
	return &models.ScoringWeights{
		Counter:             0.65,
		Synergy:             0.65,
		Meta:                0.48,
		Lineup:              0.15,
		LineupMinTeam:       4,
		Lane:                0.50,
		Partner:             0.31,
		Comfort:             0.5,
		MatchupPrior:        100,
		MetaPrior:           500,
		// The player's own winrate is noisy on a few games, so it's shrunk hard; what counts most is
		// having played the hero at all: up to Comfort*ExperiencePoints = 6 points at ExperienceGames.
		ComfortPrior:        40,
		MinPositionShare:    0.1,
		MinComfortGames:     5,
		ExperiencePoints:    12,
		ExperienceGames:     20,
		ComfortHalfLifeDays: 90,
	}
}

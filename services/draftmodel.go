package services

import (
	"math"

	"dota-helper/models"
)

const PointsPerLogit = 25.0

var allPositions = []models.Position{models.Pos1, models.Pos2, models.Pos3, models.Pos4, models.Pos5}

type AssignedDraft struct {
	Fit       float64
	Positions []models.Position
}

// Counter measures how much `hero` beats `enemy`.
func Counter(meta *MetaSnapshot, hero int, enemy int, prior int) float64 {
	vs1 := shrink(meta.Vs[PairKey{HeroId: hero, OtherHeroId: enemy}], prior)
	vs2 := shrink(meta.Vs[PairKey{HeroId: enemy, OtherHeroId: hero}], prior)
	return (vs1 - vs2) / 2.0
}

// Synergy measures how well `hero` and `ally` play together.
func Synergy(meta *MetaSnapshot, hero int, ally int, prior int) float64 {
	w1 := shrink(meta.With[PairKey{HeroId: hero, OtherHeroId: ally}], prior)
	w2 := shrink(meta.With[PairKey{HeroId: ally, OtherHeroId: hero}], prior)
	return (w1 + w2) / 2.0
}

// WinrateEdge calculates winrate above 50% in percentage points with a prior.
func WinrateEdge(stat HeroStat, prior int) float64 {
	if stat.Games == 0 && prior == 0 {
		return 0
	}
	num := float64(stat.Wins) + float64(prior)*0.5
	den := float64(stat.Games + prior)
	return ((num / den) - 0.5) * 100.0
}

// Features extracts the draft features of one team against the other.
func Features(team []int, enemies []int, meta *MetaSnapshot, w models.ScoringWeights, teamPinned map[int]models.Position) models.DraftFeatures {
	var counter float64
	for _, h := range team {
		for _, e := range enemies {
			counter += Counter(meta, h, e, w.MatchupPrior)
		}
	}

	synergy := pairs(team, meta, w) - pairs(enemies, meta, w)

	var teamMeta, enemyMeta float64
	for _, h := range team {
		teamMeta += heroMetaEdge(meta, h, w)
	}
	for _, h := range enemies {
		enemyMeta += heroMetaEdge(meta, h, w)
	}
	metaEdge := teamMeta - enemyMeta

	teamAssign := Assign(team, meta, teamPinned)
	enemyAssign := Assign(enemies, meta, nil)

	var lane float64
	for i, p := range teamAssign.Positions {
		for j, q := range enemyAssign.Positions {
			if FaceInLane(p, q) {
				lane += Lane(meta, team[i], p, enemies[j], q, w.MatchupPrior)
			}
		}
	}

	partner := partners(team, teamAssign.Positions, meta, w) - partners(enemies, enemyAssign.Positions, meta, w)

	return models.DraftFeatures{
		Counter: counter,
		Synergy: synergy,
		Meta:    metaEdge,
		Lineup:  teamAssign.Fit - enemyAssign.Fit,
		Lane:    lane,
		Partner: partner,
	}
}

// Score converts features to a single score based on weights.
func Score(f models.DraftFeatures, w models.ScoringWeights) float64 {
	return w.Counter*f.Counter +
		w.Synergy*f.Synergy +
		w.Meta*f.Meta +
		w.Lineup*f.Lineup +
		w.Lane*f.Lane +
		w.Partner*f.Partner
}

// ShareLane checks if two positions occupy the same lane.
func ShareLane(a models.Position, b models.Position) bool {
	return (a == models.Pos1 && b == models.Pos5) ||
		(a == models.Pos5 && b == models.Pos1) ||
		(a == models.Pos3 && b == models.Pos4) ||
		(a == models.Pos4 && b == models.Pos3)
}

// Partner measures how much `hero` and `ally` win more laning together.
func Partner(meta *MetaSnapshot, hero int, position models.Position, ally int, allyPosition models.Position, prior int) float64 {
	p1 := edge(meta.LanePartners, meta.PartnerTotals, hero, position, ally, prior)
	p2 := edge(meta.LanePartners, meta.PartnerTotals, ally, allyPosition, hero, prior)
	return (p1 + p2) / 2.0
}

// FaceInLane checks if mine and theirs meet in the same lane.
func FaceInLane(mine models.Position, theirs models.Position) bool {
	if mine == models.Pos2 && theirs == models.Pos2 {
		return true
	}
	if (mine == models.Pos1 || mine == models.Pos5) && (theirs == models.Pos3 || theirs == models.Pos4) {
		return true
	}
	if (mine == models.Pos3 || mine == models.Pos4) && (theirs == models.Pos1 || theirs == models.Pos5) {
		return true
	}
	return false
}

// Lane calculates how much `hero` wins more when laning against `enemy`.
func Lane(meta *MetaSnapshot, hero int, position models.Position, enemy int, enemyPosition models.Position, prior int) float64 {
	l1 := edge(meta.Lanes, meta.LaneTotals, hero, position, enemy, prior)
	l2 := edge(meta.Lanes, meta.LaneTotals, enemy, enemyPosition, hero, prior)
	return (l1 - l2) / 2.0
}

// LineupFit calculates how naturally the heroes split into different positions.
func LineupFit(team []int, meta *MetaSnapshot, pinned map[int]models.Position) float64 {
	return Assign(team, meta, pinned).Fit
}

// Assign brute-forces the best position assignment for up to 5 heroes.
func Assign(team []int, meta *MetaSnapshot, pinned map[int]models.Position) AssignedDraft {
	share := func(hero int, p models.Position) float64 {
		total := At(meta.Positions, hero, nil).Games
		if total == 0 {
			return math.Log(0.01)
		}
		posGames := float64(meta.Positions[HeroPosKey{HeroId: hero, Pos: p}].Games)
		return math.Log((posGames / float64(total)) + 0.01)
	}

	current := make([]models.Position, len(team))
	best := AssignedDraft{
		Fit:       math.Inf(-1),
		Positions: []models.Position{},
	}

	var search func(i int, used int, fit float64)
	search = func(i int, used int, fit float64) {
		if i == len(team) {
			if fit > best.Fit {
				posCopy := make([]models.Position, len(current))
				copy(posCopy, current)
				best = AssignedDraft{Fit: fit, Positions: posCopy}
			}
			return
		}

		for _, p := range allPositions {
			bit := 1 << (int(p) - 1)
			if (used & bit) != 0 {
				continue
			}

			if pinned != nil {
				if fixedP, exists := pinned[team[i]]; exists && p != fixedP {
					continue
				}
			}

			current[i] = p
			search(i+1, used|bit, fit+share(team[i], p))
		}
	}

	search(0, 0, 0)
	return best
}

// WinChance converts a score to a win probability using the sigmoid function.
func WinChance(score float64) float64 {
	return 1.0 / (1.0 + math.Exp(-score/PointsPerLogit))
}

// At returns stats on one position, or sums over all of them when position isn't known.
func At(stats map[HeroPosKey]HeroStat, hero int, position *models.Position) HeroStat {
	if position != nil {
		return stats[HeroPosKey{HeroId: hero, Pos: *position}]
	}

	var games, wins int
	for _, pos := range allPositions {
		s := stats[HeroPosKey{HeroId: hero, Pos: pos}]
		games += s.Games
		wins += s.Wins
	}
	return HeroStat{Games: games, Wins: wins}
}

// --- Private Helpers ---

func edge(stats map[LaneKey]HeroStat, totals map[HeroPosKey]HeroStat, hero int, position models.Position, other int, prior int) float64 {
	total := totals[HeroPosKey{HeroId: hero, Pos: position}]
	if total.Games == 0 {
		return 0
	}
	usual := float64(total.Wins) / float64(total.Games)
	stat := stats[LaneKey{HeroId: hero, Position: position, OtherHeroId: other}]

	num := float64(stat.Wins) + float64(prior)*usual
	den := float64(stat.Games + prior)
	return ((num / den) - usual) * 100.0
}

func heroMetaEdge(meta *MetaSnapshot, hero int, w models.ScoringWeights) float64 {
	return WinrateEdge(At(meta.Positions, hero, nil), w.MetaPrior)
}

func partners(team []int, positions []models.Position, meta *MetaSnapshot, w models.ScoringWeights) float64 {
	if len(positions) < len(team) {
		return 0
	}
	var sum float64
	for i := 0; i < len(team); i++ {
		for j := i + 1; j < len(team); j++ {
			if ShareLane(positions[i], positions[j]) {
				sum += Partner(meta, team[i], positions[i], team[j], positions[j], w.MatchupPrior)
			}
		}
	}
	return sum
}

func pairs(team []int, meta *MetaSnapshot, w models.ScoringWeights) float64 {
	var sum float64
	for i := 0; i < len(team); i++ {
		for j := i + 1; j < len(team); j++ {
			sum += Synergy(meta, team[i], team[j], w.MatchupPrior)
		}
	}
	return sum
}

func shrink(pair PairStat, prior int) float64 {
	if pair.Games+prior == 0 {
		return 0
	}
	return pair.Synergy * (float64(pair.Games) / float64(pair.Games+prior))
}

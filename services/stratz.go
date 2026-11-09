package services

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"strconv"
	"strings"
	"time"

	"dota-helper/models"
)

const (
	maxAttempts              = 4
	recentMatchesPerPosition = 500
	maxRetryDelay            = 1 * time.Minute
)

// --- STRATZ DTOs ---

type GraphQLError struct {
	Message string `json:"message"`
}

type GraphQlResponse[T any] struct {
	Data   *T             `json:"data"`
	Errors []GraphQLError `json:"errors"`
}

type HeroStatsData[T any] struct {
	HeroStats T `json:"heroStats"`
}

type StratzMatchupData struct {
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
	Stats []StratzPositionRow `json:"stats"`
}

type StratzPositionRow struct {
	HeroId     int    `json:"heroId"`
	Position   string `json:"position"`
	MatchCount int    `json:"matchCount"`
	WinCount   int    `json:"winCount"`
}

type StratzLanes struct {
	LaneOutcome []StratzLaneRow `json:"laneOutcome"`
}

type StratzLaneRow struct {
	HeroId1       int `json:"heroId1"`
	HeroId2       int `json:"heroId2"`
	MatchCount    int `json:"matchCount"`
	MatchWinCount int `json:"matchWinCount"`
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
	Name       string `json:"name"`
	Avatar     string `json:"avatar"`
	SeasonRank *int   `json:"seasonRank"`
}

type StratzHeroPerformance struct {
	HeroId             int   `json:"heroId"`
	MatchCount         int   `json:"matchCount"`
	WinCount           int   `json:"winCount"`
	LastPlayedDateTime int64 `json:"lastPlayedDateTime"`
}

// --- Client ---

type StratzClient struct {
	Http  *http.Client
	Token string
}

func (c *StratzClient) GetMatchupsAsync(heroIds []int, bracket models.Bracket, week *int64, ctx context.Context) (map[int]StratzDryad, error) {
	var aliases strings.Builder
	for _, id := range heroIds {
		aliases.WriteString(fmt.Sprintf(
			`h%d: heroVsHeroMatchup(heroId: %d, bracketBasicIds: [%s]%s, take: 200) { advantage { vs { heroId2 matchCount winCount synergy } with { heroId2 matchCount winCount synergy } } } `,
			id, id, bracketName(bracket), weekArg(week),
		))
	}

	query := fmt.Sprintf("{ heroStats { %s} }", aliases.String())

	// Deserialize into a map where keys are "h1", "h2", etc.
	data, err := queryAsync[HeroStatsData[map[string]*StratzMatchupData]](c, query, ctx)
	if err != nil {
		return nil, err
	}

	result := make(map[int]StratzDryad)
	for key, matchup := range data.HeroStats {
		if matchup != nil && len(matchup.Advantage) > 0 {
			// Extract integer from "h123"
			id, parseErr := strconv.Atoi(key[1:])
			if parseErr == nil {
				result[id] = matchup.Advantage[0]
			}
		}
	}
	return result, nil
}

func (c *StratzClient) GetPositionStatsAsync(bracket models.Bracket, week *int64, ctx context.Context) ([]StratzPositionRow, error) {
	query := fmt.Sprintf(
		`{ heroStats { stats(bracketBasicIds: [%s]%s, groupByPosition: true) { heroId position matchCount winCount } } }`,
		bracketName(bracket), weekArg(week),
	)

	data, err := queryAsync[HeroStatsData[StratzStats]](c, query, ctx)
	if err != nil {
		return nil, err
	}
	return data.HeroStats.Stats, nil
}

func (c *StratzClient) GetLaneOutcomesAsync(bracket models.Bracket, position models.Position, isWith bool, week *int64, ctx context.Context) ([]StratzLaneRow, error) {
	isWithStr := "false"
	if isWith {
		isWithStr = "true"
	}

	query := fmt.Sprintf(
		`{ heroStats { laneOutcome(isWith: %s, bracketBasicIds: [%s], positionIds: [POSITION_%d]%s) { heroId1 heroId2 matchCount matchWinCount } } }`,
		isWithStr, bracketName(bracket), position, weekArg(week),
	)

	data, err := queryAsync[HeroStatsData[StratzLanes]](c, query, ctx)
	if err != nil {
		return nil, err
	}
	return data.HeroStats.LaneOutcome, nil
}

func (c *StratzClient) GetPlayerAsync(accountId int64, ctx context.Context) (*StratzPlayer, error) {
	var positions strings.Builder
	for p := 1; p <= 5; p++ {
		positions.WriteString(fmt.Sprintf(
			`p%d: heroesPerformance(request: { positionIds: [POSITION_%d], take: %d }, take: 200) { heroId matchCount winCount lastPlayedDateTime } `,
			p, p, recentMatchesPerPosition,
		))
	}

	query := fmt.Sprintf(
		`{ player(steamAccountId: %d) { steamAccount { name avatar seasonRank } %s} }`,
		accountId, positions.String(),
	)

	data, err := queryAsync[StratzPlayerData](c, query, ctx)
	if err != nil {
		return nil, err
	}

	if data.Player == nil || data.Player.SteamAccount == nil {
		return nil, nil
	}
	return data.Player, nil
}

// --- Helpers ---

func StratzClient_ParsePosition(stratzPosition string) *models.Position {
	var p models.Position
	switch stratzPosition {
	case "POSITION_1":
		p = models.Pos1
	case "POSITION_2":
		p = models.Pos2
	case "POSITION_3":
		p = models.Pos3
	case "POSITION_4":
		p = models.Pos4
	case "POSITION_5":
		p = models.Pos5
	default:
		return nil
	}
	return &p
}

func WeekStart(utc time.Time, weeksAgo int) int64 {
	const weekSeconds = 7 * 24 * 3600
	return (utc.Unix()/weekSeconds - int64(weeksAgo)) * weekSeconds
}

func weekArg(week *int64) string {
	if week != nil {
		return fmt.Sprintf(", week: %d", *week)
	}
	return ""
}

func bracketName(bracket models.Bracket) string {
	// Assuming Bracket aligns with standard enum ints.
	// Adapt if you have specific int mappings for these values.
	switch bracket {
	case 1, 2: // HeraldGuardian (approx)
		return "HERALD_GUARDIAN"
	case 3, 4:
		return "CRUSADER_ARCHON"
	case 5, 6:
		return "LEGEND_ANCIENT"
	case 7, 8:
		return "DIVINE_IMMORTAL"
	default:
		return "DIVINE_IMMORTAL" // Default fallback
	}
}

// queryAsync is a generic function that handles JSON payload framing, Retries, 429 logic, and GraphQL error extraction.
func queryAsync[T any](c *StratzClient, query string, ctx context.Context) (*T, error) {
	reqBody, _ := json.Marshal(map[string]string{"query": query})

	for attempt := 1; ; attempt++ {
		req, err := http.NewRequestWithContext(ctx, http.MethodPost, "https://api.stratz.com/graphql", bytes.NewBuffer(reqBody))
		if err != nil {
			return nil, err
		}

		req.Header.Set("Content-Type", "application/json")
		if c.Token != "" {
			req.Header.Set("Authorization", "Bearer "+c.Token)
		}

		resp, err := c.Http.Do(req)
		if err != nil {
			if ctx.Err() != nil {
				return nil, ctx.Err() // Context canceled
			}
			return nil, err
		}
		defer resp.Body.Close()

		if resp.StatusCode == http.StatusTooManyRequests && attempt < maxAttempts {
			delay := time.Duration(10*attempt) * time.Second
			if retryAfter := resp.Header.Get("Retry-After"); retryAfter != "" {
				if seconds, err := strconv.Atoi(retryAfter); err == nil {
					delay = time.Duration(seconds) * time.Second
				}
			}

			if delay > maxRetryDelay {
				delay = maxRetryDelay
			}

			log.Printf("STRATZ rate limit hit, retrying in %v", delay)

			select {
			case <-time.After(delay):
				continue
			case <-ctx.Done():
				return nil, ctx.Err()
			}
		}

		if resp.StatusCode < 200 || resp.StatusCode >= 300 {
			return nil, fmt.Errorf("HTTP %d: %s", resp.StatusCode, resp.Status)
		}

		var body GraphQlResponse[T]
		if err := json.NewDecoder(resp.Body).Decode(&body); err != nil {
			return nil, fmt.Errorf("failed to decode STRATZ response: %w", err)
		}

		if len(body.Errors) > 0 {
			if attempt == maxAttempts {
				return nil, fmt.Errorf("STRATZ GraphQL error: %s", body.Errors[0].Message)
			}

			delay := time.Duration(10*attempt) * time.Second
			log.Printf("STRATZ GraphQL error %q, retrying in %v", body.Errors[0].Message, delay)

			select {
			case <-time.After(delay):
				continue
			case <-ctx.Done():
				return nil, ctx.Err()
			}
		}

		if body.Data == nil {
			return nil, fmt.Errorf("STRATZ returned no data")
		}

		return body.Data, nil
	}
}

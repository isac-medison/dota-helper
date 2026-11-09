package services

import (
	"context"
	"dota-helper/models"
	"encoding/json"
	"fmt"
	"net/http"
)

const openDotaBaseUrl = "https://api.opendota.com/api"

type OpenDotaClient struct {
	Http *http.Client
}

// GetHeroesAsync fetches the heroes map and flattens it into a slice.
func (c *OpenDotaClient) GetHeroesAsync(ctx context.Context) ([]models.Hero, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, openDotaBaseUrl+"/constants/heroes", nil)
	if err != nil {
		return nil, err
	}

	resp, err := c.Http.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, fmt.Errorf("OpenDota API error: %s", resp.Status)
	}

	// Equivalent to Dictionary<string, OpenDotaHero>
	var heroesMap map[string]models.Hero
	if err := json.NewDecoder(resp.Body).Decode(&heroesMap); err != nil {
		return nil, fmt.Errorf("failed to decode JSON: %w", err)
	}

	// Equivalent to .Values.ToList()
	heroes := make([]models.Hero, 0, len(heroesMap))
	for _, h := range heroesMap {
		heroes = append(heroes, h)
	}

	return heroes, nil
}

// GetPublicMatchesAsync retrieves the latest public matches, optionally paginated by lessThanMatchId.
func (c *OpenDotaClient) GetPublicMatchesAsync(lessThanMatchId *int64, ctx context.Context) ([]models.OpenDotaPublicMatch, error) {
	url := openDotaBaseUrl + "/publicMatches"
	if lessThanMatchId != nil {
		url += fmt.Sprintf("?less_than_match_id=%d", *lessThanMatchId)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, err
	}

	resp, err := c.Http.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, fmt.Errorf("OpenDota API error: %s", resp.Status)
	}

	var matches []models.OpenDotaPublicMatch
	if err := json.NewDecoder(resp.Body).Decode(&matches); err != nil {
		return nil, fmt.Errorf("failed to decode JSON: %w", err)
	}

	if matches == nil {
		matches = []models.OpenDotaPublicMatch{}
	}

	return matches, nil
}

package services

import (
	"context"
	"errors"
	"io"
	"log"
	"net/http"
	"net/url"
	"sync"

	"dota-helper/models"
	"dota-helper/utils"

	"gorm.io/gorm"
)

const iconHost = "cdn.cloudflare.steamstatic.com"

type Heroes struct {
	DB         *gorm.DB
	IconClient *http.Client

	// sync.Map is the Go equivalent to ConcurrentDictionary
	iconCache sync.Map
}

// GetHeroes retrieves all heroes from the local SQLite database.
func (s *Heroes) GetHeroes() ([]models.Hero, error) {
	var heroes []models.Hero
	err := s.DB.Order("localized_name ASC").Find(&heroes).Error
	if heroes == nil {
		heroes = []models.Hero{}
	}
	utils.SanitizeDefaults(&heroes)
	return heroes, err
}

// GetIcon fetches the hero's icon from Valve's CDN and caches it in memory.
// Returns the raw PNG bytes to the React frontend.
func (s *Heroes) GetIcon(id int) ([]byte, error) {
	var iconUrl string
	err := s.DB.Model(&models.Hero{}).Select("icon_url").Where("id = ?", id).First(&iconUrl).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, errors.New("hero not found")
		}
		return nil, err
	}

	// Validate the URL matches exactly what we expect (just like Uri.TryCreate in C#)
	uri, err := url.Parse(iconUrl)
	if err != nil || uri.Scheme != "https" || uri.Host != iconHost || uri.User != nil {
		return nil, errors.New("invalid icon url")
	}

	// Check cache
	if cached, ok := s.iconCache.Load(iconUrl); ok {
		return cached.([]byte), nil
	}

	// Fetch from CDN
	req, err := http.NewRequestWithContext(context.Background(), http.MethodGet, iconUrl, nil)
	if err != nil {
		return nil, err
	}

	resp, err := s.IconClient.Do(req)
	if err != nil {
		log.Printf("Icon of hero %d: HTTP error %v", id, err)
		return nil, errors.New("bad gateway")
	}
	defer resp.Body.Close()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		log.Printf("Icon of hero %d: CDN answered %d", id, resp.StatusCode)
		return nil, errors.New("bad gateway")
	}

	contentType := resp.Header.Get("Content-Type")
	if contentType != "image/png" {
		log.Printf("Icon of hero %d: CDN answered with type %s", id, contentType)
		return nil, errors.New("bad gateway")
	}

	iconBytes, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}

	// Save to cache
	s.iconCache.Store(iconUrl, iconBytes)

	return iconBytes, nil
}

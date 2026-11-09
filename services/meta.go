// services/meta.go
package services

import (
	"dota-helper/models"
	"dota-helper/utils"
	"errors"

	"gorm.io/gorm"
)

type Meta struct {
	DB        *gorm.DB
	Refresher *MetaRefresher
}

func (m *Meta) GetStatus() (*models.MetaStatusDto, error) {
	var lastImport models.MetaImport

	err := m.DB.Order("id desc").First(&lastImport).Error

	// 1. If it's a REAL database connection error, return it.
	// We specifically IGNORE ErrRecordNotFound.
	if err != nil && !errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, err
	}

	// 2. ALWAYS return a valid status object to the frontend, even if the DB is empty!
	status := &models.MetaStatusDto{
		Importing:        m.Refresher.Importing(),
		LastImportFailed: m.Refresher.LastImportFailed(),
		Enabled:          m.Refresher.Enabled(),
	}

	// 3. Only populate the dates and counts if a record ACTUALLY existed
	if err == nil {
		status.CompletedAtUtc = &lastImport.CompletedAtUtc
		status.MatchupCount = lastImport.MatchupCount
		status.MetaCount = lastImport.MetaCount
	}

	status.ManualRefreshAvailableAtUtc = m.Refresher.ManualRefreshAvailableAtUtc()
	utils.SanitizeDefaults(status)
	// 4. IMPORTANT: Return nil for the error!
	return status, nil
}

func (s *Meta) Refresh() error {
	success := s.Refresher.RequestRefresh()
	if !success {
		// Returning an error in Wails rejects the JS Promise.
		// This replaces HTTP 409 Conflict.
		return errors.New("Import is running, on cooldown, or Stratz:Token is not configured")
	}

	// Returning nil acts like HTTP 202 Accepted.
	return nil
}

func (m *Meta) RequestRefresh() error {
	if m.Refresher == nil {
		return errors.New("refresher is not initialized")
	}

	// Calls the method on the background worker we wrote earlier
	started := m.Refresher.RequestRefresh()
	if !started {
		// Mimics the 409 Conflict logic from your C# API
		return errors.New("already importing or on cooldown")
	}

	return nil
}

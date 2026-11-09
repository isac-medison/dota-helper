package services

import (
	"context"
	"dota-helper/models"
	"dota-helper/utils"
	"errors"

	"gorm.io/gorm"
)

type Meta struct {
	DB               *gorm.DB
	MetaImporter     *MetaImporter
	Token            string
	isImporting      bool
	lastImportFailed bool // 1. Add the failure tracker
}

func (m *Meta) GetStatus() (*models.MetaStatusDto, error) {
	var lastImport models.MetaImport

	err := m.DB.Order("id desc").First(&lastImport).Error

	if err != nil && !errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, err
	}

	status := &models.MetaStatusDto{
		Importing:        m.isImporting,
		LastImportFailed: m.lastImportFailed, // 2. Pass it to React
		RefreshEnabled:   m.Token != "",
	}

	if err == nil {
		status.UpdatedAtUtc = &lastImport.CompletedAtUtc
		status.MatchupCount = lastImport.MatchupCount
		status.MetaCount = lastImport.MetaCount
	}

	utils.SanitizeDefaults(status)
	return status, nil
}
func (m *Meta) Refresh() error {
	if m.Token == "" {
		return errors.New("Stratz:Token is not configured")
	}
	if m.isImporting {
		return errors.New("already importing")
	}

	// 1. FLIP FLAGS HERE (Synchronously, before replying to React)
	m.isImporting = true
	m.lastImportFailed = false

	go func() {
		// 2. Clear it only when the heavy lifting finishes
		defer func() { m.isImporting = false }()

		err := m.MetaImporter.ImportAsync(context.Background())
		if err != nil {
			m.lastImportFailed = true
			println("Manual import failed:", err.Error())
		}
	}()

	return nil // React gets the response instantly
}

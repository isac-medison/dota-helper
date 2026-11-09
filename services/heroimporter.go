package services

import (
	"context"
	"log"
	"strings"

	"dota-helper/models"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

const cdnBase = "https://cdn.cloudflare.steamstatic.com"

type HeroImporter struct {
	DB       *gorm.DB
	OpenDota *OpenDotaClient
}

// SyncAsync adds heroes new to the game and refreshes names and icons.
func (h *HeroImporter) SyncAsync(ctx context.Context) error {
	// Only fetch IDs to check what is known, saving memory
	var existingIds []int
	if err := h.DB.Model(&models.Hero{}).Pluck("id", &existingIds).Error; err != nil {
		return err
	}

	known := make(map[int]bool)
	for _, id := range existingIds {
		known[id] = true
	}

	heroes, err := h.OpenDota.GetHeroesAsync(ctx)
	if err != nil {
		// If we have existing data and the context isn't canceled, log and swallow the error
		if len(known) > 0 && ctx.Err() == nil {
			log.Printf("Couldn't refresh heroes from OpenDota, keeping the %d known: %v", len(known), err)
			return nil
		}
		return err
	}

	var added []string
	var toSave []models.Hero

	for _, odh := range heroes {
		if !known[odh.Id] {
			added = append(added, odh.LocalizedName)
		}

		toSave = append(toSave, models.Hero{
			Id:            odh.Id,
			LocalizedName: odh.LocalizedName,
			IconUrl:       cdnBase + strings.TrimRight(odh.Img, "?"),
			Roles:         odh.Roles, // Assuming models.Hero.Roles is a supported GORM type (e.g. pq.StringArray, or JSON serializer)
		})
	}

	if len(toSave) > 0 {
		// Bulk Upsert: Insert new records, and update existing ones (matching by Primary Key)
		err = h.DB.Clauses(clause.OnConflict{
			UpdateAll: true,
		}).CreateInBatches(&toSave, 100).Error

		if err != nil {
			return err
		}
	}

	if len(added) > 0 {
		log.Printf("Added %d heroes from OpenDota: %s", len(added), strings.Join(added, ", "))
	}

	return nil
}

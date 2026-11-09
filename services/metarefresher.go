package services

import (
	"context"
	"errors"
	"log"
	"sync"
	"time"

	"dota-helper/models"

	"gorm.io/gorm"
)

const (
	MaxAge         = 24 * time.Hour
	CheckInterval  = 1 * time.Hour
	ManualCooldown = 10 * time.Minute
)

type MetaRefresher struct {
	DB          *gorm.DB
	StratzToken string
	Importer    *MetaImporter

	mu                     sync.Mutex
	importing              bool
	lastImportFailed       bool
	lastAttemptFinishedUtc *time.Time

	// manualRequest acts like SemaphoreSlim(0, 1)
	manualRequest chan struct{}
}

func NewMetaRefresher(db *gorm.DB, stratzToken string, importer *MetaImporter) *MetaRefresher {
	return &MetaRefresher{
		DB:            db,
		StratzToken:   stratzToken,
		Importer:      importer,
		manualRequest: make(chan struct{}, 1),
	}
}

func (r *MetaRefresher) Enabled() bool {
	return r.StratzToken != ""
}

func (r *MetaRefresher) Importing() bool {
	r.mu.Lock()
	defer r.mu.Unlock()
	return r.importing
}

func (r *MetaRefresher) LastImportFailed() bool {
	r.mu.Lock()
	defer r.mu.Unlock()
	return r.lastImportFailed
}

func (r *MetaRefresher) ManualRefreshAvailableAtUtc() *time.Time {
	r.mu.Lock()
	defer r.mu.Unlock()

	if r.lastAttemptFinishedUtc == nil {
		return nil
	}
	at := r.lastAttemptFinishedUtc.Add(ManualCooldown)
	if at.After(time.Now().UTC()) {
		return &at
	}
	return nil
}

func (r *MetaRefresher) RequestRefresh() bool {
	r.mu.Lock()
	defer r.mu.Unlock()

	if !r.Enabled() || r.importing {
		return false
	}
	if r.lastAttemptFinishedUtc != nil {
		if r.lastAttemptFinishedUtc.Add(ManualCooldown).After(time.Now().UTC()) {
			return false
		}
	}

	r.importing = true

	// Non-blocking send to channel (equivalent to manualRequest.Release() if count == 0)
	select {
	case r.manualRequest <- struct{}{}:
	default:
	}

	return true
}

// Run is the equivalent of ExecuteAsync
func (r *MetaRefresher) Run(ctx context.Context) {
	if !r.Enabled() {
		log.Println("Stratz token is not configured, meta import is disabled")
		return
	}

	force := false
	for {
		attempted := false
		failed := false

		isStale, err := r.isStale()
		if err != nil {
			log.Printf("Error checking stale status: %v", err)
		}

		if force || isStale {
			attempted = true
			r.mu.Lock()
			r.importing = true
			r.mu.Unlock()

			log.Printf("Importing meta from STRATZ (manual: %v)", force)

			err = r.Importer.ImportAsync(ctx)

			if err != nil && !errors.Is(err, context.Canceled) {
				log.Printf("Meta import failed: %v", err)
				failed = true
			}

			if err != nil && !errors.Is(err, context.Canceled) {
				log.Printf("Meta import failed: %v", err)
				failed = true
			}
		}

		// Finally block equivalent
		r.mu.Lock()
		r.importing = false
		if attempted {
			now := time.Now().UTC()
			r.lastAttemptFinishedUtc = &now
			r.lastImportFailed = failed
		}
		r.mu.Unlock()

		// Wait for interval, manual request, or app shutdown
		select {
		case <-r.manualRequest:
			force = true
		case <-time.After(CheckInterval):
			force = false
		case <-ctx.Done():
			log.Println("MetaRefresher stopped")
			return
		}
	}
}

func (r *MetaRefresher) isStale() (bool, error) {
	var lastImport models.MetaImport

	// Select only the date field to save memory
	err := r.DB.Select("completed_at_utc").Order("id desc").First(&lastImport).Error

	if errors.Is(err, gorm.ErrRecordNotFound) {
		return true, nil
	}
	if err != nil {
		return false, err
	}

	return time.Now().UTC().Sub(lastImport.CompletedAtUtc) >= MaxAge, nil
}

package main

import (
	"log"

	"dota-helper/models"

	"github.com/glebarez/sqlite"
	"gorm.io/gorm"
)

func InitDB() *gorm.DB {
	// Connects to dota-helper.db and creates it if missing

	dsn := "dota-helper.db?_journal_mode=WAL&_busy_timeout=10000"

	db, err := gorm.Open(sqlite.Open(dsn), &gorm.Config{})
	if err != nil {
		log.Fatalln(err)
	}

	// (Optional) You can also execute this just to be absolutely sure:
	db.Exec("PRAGMA journal_mode=WAL;")
	db.Exec("PRAGMA synchronous=NORMAL;")

	// Entity Framework equivalent of db.Database.EnsureCreated()
	// Automatically creates or updates the table schema based on the struct
	err = db.AutoMigrate(
		&models.Hero{},
		&models.HeroMatchup{},
		&models.HeroMeta{},
		&models.LaneMatchup{},
		&models.MetaImport{},
		&models.Player{},
		&models.PlayerHero{},
	)

	if err != nil {
		log.Fatalln(err)
	}

	return db
}

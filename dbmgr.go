package main

import (
	"log"

	"dota-helper/models"

	"github.com/glebarez/sqlite"
	"gorm.io/gorm"
)

func InitDB() *gorm.DB {
	return openDB("dota-helper.db")
}

// openDB connects to the database file and creates it if missing.
func openDB(path string) *gorm.DB {
	dsn := path + "?_journal_mode=WAL&_busy_timeout=10000"

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

	if err := migrateBrackets(db); err != nil {
		log.Fatalln(err)
	}

	return db
}

// migrateBrackets converts meta imported per medal (brackets 1-8, each STRATZ group stored twice:
// 1,2 = Herald-Guardian ... 7,8 = Divine-Immortal) to one bracket per group (1-4).
func migrateBrackets(db *gorm.DB) error {
	tables := []any{&models.HeroMatchup{}, &models.HeroMeta{}, &models.LaneMatchup{}}

	return db.Transaction(func(tx *gorm.DB) error {
		for _, model := range tables {
			// By table name: GORM would turn values it writes into the model into extra conditions.
			stmt := &gorm.Statement{DB: tx}
			if err := stmt.Parse(model); err != nil {
				return err
			}
			table := stmt.Schema.Table

			var old int64
			if err := tx.Table(table).Where("bracket > ?", 4).Count(&old).Error; err != nil {
				return err
			}
			if old == 0 {
				continue
			}

			// The even medals are copies of the odd ones.
			if err := tx.Exec("DELETE FROM `"+table+"` WHERE bracket IN (2, 4, 6, 8)").Error; err != nil {
				return err
			}
			// Ascending, so a bracket never lands on one that's still there.
			for _, move := range [][2]int{{3, 2}, {5, 3}, {7, 4}} {
				if err := tx.Exec("UPDATE `"+table+"` SET bracket = ? WHERE bracket = ?", move[1], move[0]).Error; err != nil {
					return err
				}
			}
			log.Printf("Migrated %s to four brackets", table)
		}
		return nil
	})
}

package migration

import (
	"sort"
	"time"

	"gorm.io/gorm"
)

type Migration struct {
	Version int64  `gorm:"primaryKey"`
	Name    string `gorm:"not null"`
	Applied int64  `gorm:"not null"`
}

func (Migration) TableName() string { return "schema_migrations" }

type Step struct {
	Version int64
	Name    string
	Run     func(*gorm.DB) error
}

func Run(db *gorm.DB, steps []Step) error {
	if err := db.AutoMigrate(&Migration{}); err != nil {
		return err
	}
	sorted := append([]Step(nil), steps...)
	sort.Slice(sorted, func(i, j int) bool {
		return sorted[i].Version < sorted[j].Version
	})
	for _, step := range sorted {
		var count int64
		if err := db.Model(&Migration{}).Where("version = ?", step.Version).Count(&count).Error; err != nil {
			return err
		}
		if count > 0 {
			continue
		}
		if err := db.Transaction(func(tx *gorm.DB) error {
			if step.Run != nil {
				if err := step.Run(tx); err != nil {
					return err
				}
			}
			return tx.Create(&Migration{Version: step.Version, Name: step.Name, Applied: nowUnix()}).Error
		}); err != nil {
			return err
		}
	}
	return nil
}

var nowUnix = func() int64 { return time.Now().Unix() }

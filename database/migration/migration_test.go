package migration

import (
	"testing"

	"github.com/glebarez/sqlite"
	"gorm.io/gorm"
)

func TestRunAppliesStepsOnceInOrder(t *testing.T) {
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	if err != nil {
		t.Fatal(err)
	}
	applied := make([]int64, 0)
	steps := []Step{
		{Version: 2, Name: "second", Run: func(tx *gorm.DB) error { applied = append(applied, 2); return nil }},
		{Version: 1, Name: "first", Run: func(tx *gorm.DB) error { applied = append(applied, 1); return nil }},
	}
	if err := Run(db, steps); err != nil {
		t.Fatal(err)
	}
	if err := Run(db, steps); err != nil {
		t.Fatal(err)
	}
	if len(applied) != 2 || applied[0] != 1 || applied[1] != 2 {
		t.Fatalf("unexpected applied steps: %#v", applied)
	}
	var count int64
	if err := db.Model(&Migration{}).Count(&count).Error; err != nil {
		t.Fatal(err)
	}
	if count != 2 {
		t.Fatalf("expected 2 migration records, got %d", count)
	}
}

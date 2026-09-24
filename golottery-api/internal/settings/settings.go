// Package settings stores ScopeApp configuration overrides in the database.
package settings

import (
	"context"
	"fmt"
	"time"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

// Setting is one persisted configuration override.
type Setting struct {
	Key   string `gorm:"primaryKey;size:64"`
	Value string `gorm:"type:text;not null"`
	// Group is stored as category because GROUP is reserved in SQL.
	Group     string `gorm:"column:category;size:32"`
	UpdatedBy string `gorm:"size:64"`
	UpdatedAt time.Time
}

// TableName returns the settings table name.
func (Setting) TableName() string { return "settings" }

// Store reads and writes settings rows.
type Store struct {
	db *gorm.DB
}

// NewStore wraps a GORM handle.
func NewStore(db *gorm.DB) *Store {
	return &Store{db: db}
}

// Snapshot returns key -> value for rows with a non-empty value.
func (s *Store) Snapshot(ctx context.Context) (map[string]string, error) {
	var rows []Setting
	if err := s.db.WithContext(ctx).Find(&rows).Error; err != nil {
		return nil, fmt.Errorf("settings snapshot: %w", err)
	}
	out := make(map[string]string, len(rows))
	for _, row := range rows {
		if row.Value == "" {
			continue
		}
		out[row.Key] = row.Value
	}
	return out, nil
}

// List returns all settings rows.
func (s *Store) List(ctx context.Context) ([]Setting, error) {
	var rows []Setting
	if err := s.db.WithContext(ctx).Order("key").Find(&rows).Error; err != nil {
		return nil, fmt.Errorf("settings list: %w", err)
	}
	return rows, nil
}

// Set upserts a settings row. Empty values are rejected.
func (s *Store) Set(ctx context.Context, key, value, category, updatedBy string) error {
	if key == "" {
		return fmt.Errorf("settings set: key is required")
	}
	if value == "" {
		return fmt.Errorf("settings set: empty value; use unset to clear")
	}
	row := Setting{
		Key:       key,
		Value:     value,
		Group:     category,
		UpdatedBy: updatedBy,
		UpdatedAt: time.Now().UTC(),
	}
	err := s.db.WithContext(ctx).Clauses(clause.OnConflict{
		Columns:   []clause.Column{{Name: "key"}},
		DoUpdates: clause.AssignmentColumns([]string{"value", "category", "updated_by", "updated_at"}),
	}).Create(&row).Error
	if err != nil {
		return fmt.Errorf("settings set %s: %w", key, err)
	}
	return nil
}

// Unset deletes a settings row.
func (s *Store) Unset(ctx context.Context, key string) error {
	if key == "" {
		return fmt.Errorf("settings unset: key is required")
	}
	if err := s.db.WithContext(ctx).Delete(&Setting{}, "key = ?", key).Error; err != nil {
		return fmt.Errorf("settings unset %s: %w", key, err)
	}
	return nil
}

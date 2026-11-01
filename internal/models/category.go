package models

import (
	"strings"

	"gorm.io/gorm"
)

// Category is one Jeopardy section. Name is what the board shows.
type Category struct {
	Key   string
	Name  string
	Title string
}

// Categories is the fixed board order.
var Categories = []Category{
	{Key: "crypto", Name: "Криптография", Title: "Криптограф"},
	{Key: "forensics", Name: "Форензика", Title: "Следователь"},
	{Key: "osint", Name: "OSINT", Title: "Сталкер"},
	{Key: "misc", Name: "Misc", Title: "На все руки мастер"},
	{Key: "pwn", Name: "PWN", Title: "Голова"},
	{Key: "reverse", Name: "Реверс", Title: "Историк"},
	{Key: "web", Name: "Веб", Title: "Веббер"},
	{Key: "ppc", Name: "PPC", Title: "Программист"},
	{Key: "stego", Name: "Стеганография", Title: "Штирлиц"},
	{Key: "quest", Name: "Квест", Title: "Приключенец"},
}

// CanonicalCategory maps task.yml and folder names onto the board names.
// An unknown category is kept, so a custom task does not disappear.
func CanonicalCategory(raw string) string {
	s := strings.ToLower(strings.TrimSpace(raw))
	s = strings.ReplaceAll(s, "ё", "е")
	switch {
	case strings.Contains(s, "quest") || strings.Contains(s, "квест"):
		return "Квест"
	case strings.Contains(s, "stego") || strings.Contains(s, "стег"):
		return "Стеганография"
	case strings.Contains(s, "foren") || strings.Contains(s, "форен"):
		return "Форензика"
	case strings.Contains(s, "osint"):
		return "OSINT"
	case strings.Contains(s, "crypt") || strings.Contains(s, "крипт"):
		return "Криптография"
	case strings.Contains(s, "pwn"):
		return "PWN"
	case strings.Contains(s, "rev") || strings.Contains(s, "реверс"):
		return "Реверс"
	case strings.Contains(s, "ppc"):
		return "PPC"
	case strings.Contains(s, "web") || strings.Contains(s, "веб"):
		return "Веб"
	case strings.Contains(s, "misc"):
		return "Misc"
	}
	t := strings.TrimSpace(raw)
	if t == "" {
		return "Misc"
	}
	for _, c := range Categories {
		if strings.EqualFold(t, c.Name) {
			return c.Name
		}
	}
	return t
}

// SeedAchievements writes the favorite-category titles. Existing rows are renamed
// when the title changes, and nothing else is deleted.
func SeedAchievements(db *gorm.DB) error {
	if db == nil {
		return nil
	}
	for _, c := range Categories {
		var row Achievement
		err := db.Where("key = ?", c.Key).First(&row).Error
		if err != nil {
			if err := db.Create(&Achievement{Key: c.Key, Title: c.Title, Category: c.Name}).Error; err != nil {
				return err
			}
			continue
		}
		if row.Title != c.Title || row.Category != c.Name {
			if err := db.Model(&row).Updates(map[string]any{"title": c.Title, "category": c.Name}).Error; err != nil {
				return err
			}
		}
	}
	return nil
}

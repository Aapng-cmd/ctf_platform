package handlers

import (
	"fmt"
	"time"
)

type tzCity struct {
	ID     string
	En     string
	Ru     string
	Offset string
}

var tzCities = []tzCity{
	{ID: "UTC", En: "UTC", Ru: "UTC"},
	{ID: "Europe/Kaliningrad", En: "Kaliningrad", Ru: "Калининград"},
	{ID: "Europe/Moscow", En: "Moscow", Ru: "Москва"},
	{ID: "Europe/Samara", En: "Samara", Ru: "Самара"},
	{ID: "Asia/Yekaterinburg", En: "Yekaterinburg", Ru: "Екатеринбург"},
	{ID: "Asia/Omsk", En: "Omsk", Ru: "Омск"},
	{ID: "Asia/Novosibirsk", En: "Novosibirsk", Ru: "Новосибирск"},
	{ID: "Asia/Krasnoyarsk", En: "Krasnoyarsk", Ru: "Красноярск"},
	{ID: "Asia/Irkutsk", En: "Irkutsk", Ru: "Иркутск"},
	{ID: "Asia/Yakutsk", En: "Yakutsk", Ru: "Якутск"},
	{ID: "Asia/Vladivostok", En: "Vladivostok", Ru: "Владивосток"},
	{ID: "Asia/Magadan", En: "Magadan", Ru: "Магадан"},
	{ID: "Asia/Kamchatka", En: "Kamchatka", Ru: "Камчатка"},
	{ID: "Europe/London", En: "London", Ru: "Лондон"},
	{ID: "Europe/Berlin", En: "Berlin", Ru: "Берлин"},
	{ID: "Europe/Paris", En: "Paris", Ru: "Париж"},
	{ID: "Europe/Kyiv", En: "Kyiv", Ru: "Киев"},
	{ID: "Europe/Minsk", En: "Minsk", Ru: "Минск"},
	{ID: "Asia/Tbilisi", En: "Tbilisi", Ru: "Тбилиси"},
	{ID: "Asia/Dubai", En: "Dubai", Ru: "Дубай"},
	{ID: "Asia/Almaty", En: "Almaty", Ru: "Алматы"},
	{ID: "Asia/Tashkent", En: "Tashkent", Ru: "Ташкент"},
	{ID: "Asia/Kolkata", En: "Kolkata", Ru: "Калькутта"},
	{ID: "Asia/Shanghai", En: "Shanghai", Ru: "Шанхай"},
	{ID: "Asia/Singapore", En: "Singapore", Ru: "Сингапур"},
	{ID: "Asia/Tokyo", En: "Tokyo", Ru: "Токио"},
	{ID: "Australia/Sydney", En: "Sydney", Ru: "Сидней"},
	{ID: "America/New_York", En: "New York", Ru: "Нью-Йорк"},
	{ID: "America/Chicago", En: "Chicago", Ru: "Чикаго"},
	{ID: "America/Denver", En: "Denver", Ru: "Денвер"},
	{ID: "America/Los_Angeles", En: "Los Angeles", Ru: "Лос-Анджелес"},
	{ID: "America/Sao_Paulo", En: "Sao Paulo", Ru: "Сан-Паулу"},
}

func zoneOffset(id string, now time.Time) string {
	if id == "" || id == "UTC" {
		return ""
	}
	loc, err := time.LoadLocation(id)
	if err != nil {
		return ""
	}
	_, sec := now.In(loc).Zone()
	sign := "+"
	if sec < 0 {
		sign = "-"
		sec = -sec
	}
	hours := sec / 3600
	mins := (sec % 3600) / 60
	if mins == 0 {
		return fmt.Sprintf("UTC%s%d", sign, hours)
	}
	return fmt.Sprintf("UTC%s%d:%02d", sign, hours, mins)
}

func zoneChoices(selected string) []tzCity {
	if selected == "" {
		selected = "UTC"
	}
	now := time.Now()
	out := make([]tzCity, len(tzCities))
	found := false
	for i, city := range tzCities {
		city.Offset = zoneOffset(city.ID, now)
		out[i] = city
		if city.ID == selected {
			found = true
		}
	}
	if !found {
		out = append(out, tzCity{ID: selected, En: selected, Ru: selected, Offset: zoneOffset(selected, now)})
	}
	return out
}

func tzCityByID(id string) tzCity {
	for _, city := range zoneChoices(id) {
		if city.ID == id {
			return city
		}
	}
	return tzCity{ID: "UTC", En: "UTC", Ru: "UTC"}
}

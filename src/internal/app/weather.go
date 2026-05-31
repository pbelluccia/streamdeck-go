package app

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"strconv"
	"time"

	"streamdeck-go/internal/render"
)

type WeatherClient struct {
	location string
	client   *http.Client
	cache    render.Weather
	cacheAt  time.Time
}

func NewWeather(location string) *WeatherClient {
	return &WeatherClient{
		location: location,
		client:   &http.Client{Timeout: 5 * time.Second},
	}
}

func (w *WeatherClient) Current(ctx context.Context) render.Weather {
	if w.cache.OK && time.Since(w.cacheAt) < 10*time.Minute {
		return w.cache
	}
	endpoint := "https://wttr.in/" + url.PathEscape(w.location) + "?format=j1"
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, endpoint, nil)
	if err != nil {
		fmt.Println("Error creating weather request:", err)
		return w.cache
	}
	resp, err := w.client.Do(req)
	if err != nil {
		fmt.Println("Error fetching weather info:", err)
		return w.cache
	}
	defer resp.Body.Close()

	var payload struct {
		CurrentCondition []struct {
			TempC       string `json:"temp_C"`
			WeatherDesc []struct {
				Value string `json:"value"`
			} `json:"weatherDesc"`
		} `json:"current_condition"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&payload); err != nil {
		fmt.Println("Error decoding weather info:", err)
		return w.cache
	}
	if len(payload.CurrentCondition) == 0 || len(payload.CurrentCondition[0].WeatherDesc) == 0 {
		return w.cache
	}
	temp, err := strconv.ParseFloat(payload.CurrentCondition[0].TempC, 64)
	if err != nil {
		return w.cache
	}
	result := render.Weather{
		Condition:   payload.CurrentCondition[0].WeatherDesc[0].Value,
		Temperature: int(temp + 0.5),
		OK:          true,
	}
	w.cache = result
	w.cacheAt = time.Now()
	return result
}

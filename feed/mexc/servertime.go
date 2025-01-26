package mexc

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"

	"github.com/sirupsen/logrus"
	"go.uber.org/zap"
)

var log *logrus.Logger

// MexcConfig holds configuration for the MEXC API.
type MexcConfig struct {
	BaseURL    string
	Timeout    int // in seconds
	MaxRetries int
}

// TimeResponse represents the JSON response from /api/v3/time.
type TimeResponse struct {
	ServerTime int64 `json:"serverTime"` // in milliseconds
}

// MexcAPI handles interactions with the MEXC API.
type MexcAPI struct {
	BaseURL    string
	Timeout    int
	MaxRetries int
	Client     *http.Client
}

// fetchServerTime fetches the server time from MEXC API.
func (s *mexcDataFeed) fetchServerTime() (int64, error) {
	url := fmt.Sprintf("%s%s", BaseURL, fetchServerTimeURL)

	var serverTime int64
	var lastErr error

	for attempt := 1; attempt <= MaxRetries; attempt++ {
		client := &http.Client{}
		resp, err := client.Get(url)
		if err != nil {
			zap.L().Warn("Failed to fetch server time", zap.Error(err), zap.Int("attempt", attempt))
			lastErr = err
			time.Sleep(time.Duration(attempt) * time.Second) // Exponential backoff
			continue
		}

		defer resp.Body.Close()

		if resp.StatusCode != http.StatusOK {
			bodyBytes, _ := io.ReadAll(resp.Body)
			zap.L().Info("Non-200 status code", zap.Int("attempt", attempt), zap.Int("status", resp.StatusCode), zap.String("body", string(bodyBytes)))
			lastErr = fmt.Errorf("non-200 status code: %d", resp.StatusCode)
			time.Sleep(time.Duration(attempt) * time.Second)
			continue
		}

		body, err := io.ReadAll(resp.Body)
		if err != nil {
			zap.L().Info("Failed to read response body", zap.Int("attempt", attempt), zap.Error(err))
			lastErr = err
			time.Sleep(time.Duration(attempt) * time.Second)
			continue
		}

		var timeResp TimeResponse
		if err := json.Unmarshal(body, &timeResp); err != nil {
			zap.L().Info("Failed to unmarshal JSON", zap.Int("attempt", attempt), zap.Error(err))
			lastErr = err
			time.Sleep(time.Duration(attempt) * time.Second)
			continue
		}

		serverTime = timeResp.ServerTime
		return serverTime, nil
	}

	return 0, fmt.Errorf("failed to fetch server time after %d attempts: %v", MaxRetries, lastErr)
}

// GetServerTimeZone determines the probable timezone of the MEXC server.
func (s *mexcDataFeed) GetServerTimeZone() (string, error) {
	if tz != "" {
		return tz, nil
	}
	serverTimeMs, err := s.fetchServerTime()
	if err != nil {
		return "", fmt.Errorf("error fetching server time: %v", err)
	}

	// Convert serverTimeMs to time.Time
	serverTime := time.UnixMilli(serverTimeMs).UTC()

	// Get client's current UTC time
	clientTime := time.Now().UTC()

	// Calculate the time difference
	timeDiff := serverTime.Sub(clientTime)

	// Convert time difference to hours (can be fractional)
	offsetHours := timeDiff.Hours()

	// Round to the nearest quarter hour to account for timezones with 30 or 45-minute offsets
	roundedOffset := roundToQuarterHour(offsetHours)

	// Map the offset to a timezone string
	timezone, err := mapOffsetToTimezone(roundedOffset)
	if err != nil {
		return "", err
	}
	// set the timezone as the global timezone
	loc, err := time.LoadLocation(timezone)
	if err != nil {
		zap.L().Fatal("Failed to load location", zap.Error(err))
	}

	// Set the global timezone
	time.Local = loc
	tz = timezone
	return timezone, nil
}

// roundToQuarterHour rounds a float64 to the nearest 0.25.
func roundToQuarterHour(hours float64) float64 {
	return float64(int(hours*4+0.5)) / 4
}

// mapOffsetToTimezone maps a timezone offset to a probable timezone string.
// This function returns a list of possible timezones that match the offset.
// Due to multiple timezones sharing the same offset, it returns a slice.
func mapOffsetToTimezone(offset float64) (string, error) {
	offsetInt := int(offset)
	minutes := (offset - float64(offsetInt)) * 60

	switch {
	case offset == 0:
		return "UTC", nil
	case minutes == 0:
		if offset > 0 {
			return fmt.Sprintf("UTC+%d", offsetInt), nil
		}
		return fmt.Sprintf("UTC%d", offsetInt), nil
	case minutes == 15:
		if offset > 0 {
			return fmt.Sprintf("UTC+%d:15", offsetInt), nil
		}
		return fmt.Sprintf("UTC%d:15", offsetInt), nil
	case minutes == 30:
		if offset > 0 {
			return fmt.Sprintf("UTC+%d:30", offsetInt), nil
		}
		return fmt.Sprintf("UTC%d:30", offsetInt), nil
	case minutes == 45:
		if offset > 0 {
			return fmt.Sprintf("UTC+%d:45", offsetInt), nil
		}
		return fmt.Sprintf("UTC%d:45", offsetInt), nil
	default:
		return "", fmt.Errorf("unsupported timezone offset: %.2f hours", offset)
	}
}

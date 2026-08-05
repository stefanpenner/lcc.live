package synoptic

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestParseNWSObservation_ms(t *testing.T) {
	ts := time.Now().UTC().Add(-15 * time.Minute).Format(time.RFC3339)
	raw := []byte(`{
		"properties": {
			"timestamp": "` + ts + `",
			"temperature": {"value": 10.0, "unitCode": "wmoUnit:degC"},
			"dewpoint": {"value": -5.0, "unitCode": "wmoUnit:degC"},
			"windSpeed": {"value": 4.4704, "unitCode": "wmoUnit:m_s-1"},
			"windGust": {"value": 8.9408, "unitCode": "wmoUnit:m_s-1"},
			"windDirection": {"value": 270.0, "unitCode": "wmoUnit:degree_(angle)"},
			"relativeHumidity": {"value": 22.5, "unitCode": "wmoUnit:percent"}
		}
	}`)
	ws, err := ParseNWSObservation("CLN", raw)
	require.NoError(t, err)
	require.NotNil(t, ws)
	assert.Equal(t, "nws", ws.Source)
	assert.Equal(t, "CLN", *ws.CameraSourceId)
	// 10°C → 50°F
	require.NotNil(t, ws.AirTemperature)
	assert.Equal(t, "50.0", *ws.AirTemperature)
	// 4.4704 m/s ≈ 10 mph
	require.NotNil(t, ws.WindSpeedAvg)
	assert.Equal(t, "10.0", *ws.WindSpeedAvg)
	assert.Equal(t, "W", *ws.WindDirection)
	assert.Equal(t, "22", *ws.RelativeHumidity)
	assert.Greater(t, ws.LastUpdated, int64(0))
}

func TestParseNWSObservation_kmh(t *testing.T) {
	// Real NWS mountain stations often report km_h-1 — must not treat as m/s
	// (that bug turned ~28 km/h into "62 mph")
	ts := time.Now().UTC().Add(-15 * time.Minute).Format(time.RFC3339)
	raw := []byte(`{
		"properties": {
			"timestamp": "` + ts + `",
			"temperature": {"value": null, "unitCode": "wmoUnit:degC"},
			"windSpeed": {"value": 27.792, "unitCode": "wmoUnit:km_h-1"},
			"windGust": {"value": 38.448, "unitCode": "wmoUnit:km_h-1"},
			"windDirection": {"value": 270.0, "unitCode": "wmoUnit:degree_(angle)"}
		}
	}`)
	ws, err := ParseNWSObservation("AGD", raw)
	require.NoError(t, err)
	// 27.792 km/h ≈ 17.3 mph (NOT 62)
	require.NotNil(t, ws.WindSpeedAvg)
	assert.Equal(t, "17.3", *ws.WindSpeedAvg)
	require.NotNil(t, ws.WindSpeedGust)
	assert.Equal(t, "23.9", *ws.WindSpeedGust)
}

func TestNWSSpeedToMPH(t *testing.T) {
	assert.InDelta(t, 10.0, nwsSpeedToMPH(4.4704, "wmoUnit:m_s-1"), 0.05)
	assert.InDelta(t, 17.3, nwsSpeedToMPH(27.792, "wmoUnit:km_h-1"), 0.05)
	assert.InDelta(t, 20.0, nwsSpeedToMPH(20.0, "wmoUnit:mi_h-1"), 0.01)
}

func TestParseSynopticDropsStaleWind(t *testing.T) {
	fresh := time.Now().UTC().Add(-10 * time.Minute).Format("2006-01-02T15:04:05Z")
	stale := time.Now().UTC().Add(-5 * time.Hour).Format("2006-01-02T15:04:05Z")
	raw := []byte(`{
		"SUMMARY": {"RESPONSE_CODE": 1, "RESPONSE_MESSAGE": "OK"},
		"STATION": [{
			"STID": "AGD",
			"NAME": "ALTA - GUARD HOUSE",
			"OBSERVATIONS": {
				"air_temp_value_1": {"value": 70.0, "date_time": "` + fresh + `"},
				"wind_speed_value_1": {"value": 62.0, "date_time": "` + stale + `"},
				"wind_direction_value_1": {"value": 270.0, "date_time": "` + stale + `"}
			}
		}]
	}`)
	stations, err := ParseSynopticLatest(raw)
	require.NoError(t, err)
	require.Len(t, stations, 1)
	assert.Equal(t, "70.0", *stations[0].AirTemperature)
	assert.Nil(t, stations[0].WindSpeedAvg, "stale wind must be dropped")
	assert.Nil(t, stations[0].WindDirection, "stale wind direction must be dropped")
}

func TestParseSynopticLatest(t *testing.T) {
	// Fresh timestamps relative to "now" — use time near test run via RFC3339 now
	// Fixed recent-enough times: tests use absolute dates; if suite freezes, bump year.
	// Use dynamic times instead:
	now := time.Now().UTC()
	fresh := now.Add(-10 * time.Minute).Format("2006-01-02T15:04:05Z")
	raw := []byte(`{
		"SUMMARY": {"RESPONSE_CODE": 1, "RESPONSE_MESSAGE": "OK"},
		"STATION": [{
			"STID": "CLN",
			"NAME": "ALTA - COLLINS",
			"LATITUDE": "40.5763",
			"LONGITUDE": "-111.6383",
			"OBSERVATIONS": {
				"air_temp_value_1": {"value": 48.2, "date_time": "` + fresh + `"},
				"wind_speed_value_1": {"value": 12.0, "date_time": "` + fresh + `"},
				"wind_direction_value_1": {"value": 225.0, "date_time": "` + fresh + `"}
			}
		}]
	}`)
	stations, err := ParseSynopticLatest(raw)
	require.NoError(t, err)
	require.Len(t, stations, 1)
	s := stations[0]
	assert.Equal(t, "synoptic", s.Source)
	assert.Equal(t, "CLN", *s.CameraSourceId)
	assert.Equal(t, "ALTA - COLLINS", s.StationName)
	assert.Equal(t, "48.2", *s.AirTemperature)
	assert.Equal(t, "12.0", *s.WindSpeedAvg)
	assert.Equal(t, "SW", *s.WindDirection)
}

func TestDegreesToCompass(t *testing.T) {
	assert.Equal(t, "N", degreesToCompass(0))
	assert.Equal(t, "E", degreesToCompass(90))
	assert.Equal(t, "S", degreesToCompass(180))
	assert.Equal(t, "W", degreesToCompass(270))
}

func TestUniqueNonEmpty(t *testing.T) {
	got := uniqueNonEmpty([]string{"cln", "CLN", "", "SBDU1", " sbdu1 "})
	assert.Equal(t, []string{"CLN", "SBDU1"}, got)
}

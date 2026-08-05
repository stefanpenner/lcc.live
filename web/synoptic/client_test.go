package synoptic

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestParseNWSObservation(t *testing.T) {
	raw := []byte(`{
		"properties": {
			"timestamp": "2026-08-05T19:00:00+00:00",
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
	assert.Equal(t, "50.0", *ws.AirTemperature)
	// 4.4704 m/s ≈ 10 mph
	assert.Equal(t, "10.0", *ws.WindSpeedAvg)
	assert.Equal(t, "W", *ws.WindDirection)
	assert.Equal(t, "22", *ws.RelativeHumidity)
	assert.Greater(t, ws.LastUpdated, int64(0))
}

func TestParseSynopticLatest(t *testing.T) {
	raw := []byte(`{
		"SUMMARY": {"RESPONSE_CODE": 1, "RESPONSE_MESSAGE": "OK"},
		"STATION": [{
			"STID": "CLN",
			"NAME": "ALTA - COLLINS",
			"LATITUDE": "40.5763",
			"LONGITUDE": "-111.6383",
			"OBSERVATIONS": {
				"air_temp_value_1": {"value": 48.2, "date_time": "2026-08-05T18:00:00Z"},
				"wind_speed_value_1": {"value": 12.0, "date_time": "2026-08-05T18:00:00Z"},
				"wind_direction_value_1": {"value": 225.0, "date_time": "2026-08-05T18:00:00Z"}
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

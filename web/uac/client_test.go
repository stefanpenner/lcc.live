package uac

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestParseSaltLakeDanger(t *testing.T) {
	raw := []byte(`{
		"features": [
			{"properties": {"name": "Ogden", "danger": "moderate", "danger_level": 2, "link": "https://example.com/ogden"}},
			{"properties": {
				"name": "Salt Lake",
				"danger": "considerable",
				"danger_level": 3,
				"link": "https://utahavalanchecenter.org/forecast/salt-lake",
				"travel_advice": "Watch for unstable snow."
			}}
		]
	}`)

	d, err := ParseSaltLakeDanger(raw)
	require.NoError(t, err)
	require.NotNil(t, d)
	assert.Equal(t, "considerable", d.Danger)
	assert.Equal(t, 3, d.DangerLevel)
	assert.Equal(t, "https://utahavalanchecenter.org/forecast/salt-lake", d.Link)
	assert.Equal(t, "Watch for unstable snow.", d.TravelAdvice)
	assert.Greater(t, d.Updated, int64(0))
}

func TestParseSaltLakeDanger_Missing(t *testing.T) {
	raw := []byte(`{"features":[{"properties":{"name":"Ogden","danger":"low","danger_level":1}}]}`)
	_, err := ParseSaltLakeDanger(raw)
	require.Error(t, err)
}

func TestParseSaltLakeDanger_EmptyDanger(t *testing.T) {
	raw := []byte(`{"features":[{"properties":{"name":"Salt Lake","danger":"","danger_level":-1,"link":""}}]}`)
	d, err := ParseSaltLakeDanger(raw)
	require.NoError(t, err)
	assert.Equal(t, "no rating", d.Danger)
	assert.Contains(t, d.Link, "utahavalanchecenter.org")
}

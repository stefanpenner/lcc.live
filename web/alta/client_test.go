package alta

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestParseAltaStatus(t *testing.T) {
	html := []byte(`<!DOCTYPE html><html><head>
<script>window.Alta = {"DST":true,"alerts":[
  {"id":5,"name":"Summer Road Status","slug":"summer-road-status","type":"summer-road-status","status":"Open","message":null,"enabled":true},
  {"id":4,"name":"Parking Status","slug":"parking-status","type":"parking-status","status":"Available","message":null,"enabled":true},
  {"id":3,"name":"Unsupported Browser","slug":"unsupported-browser","type":"unsupported-browser","status":"Warning","message":"update","enabled":true}
],"conditions":{}};</script>
</head><body></body></html>`)

	st, err := ParseAltaStatus(html)
	require.NoError(t, err)
	require.NotNil(t, st)
	assert.Equal(t, "Available", st.ParkingStatus)
	assert.Equal(t, "Open", st.RoadStatus)
	assert.Greater(t, st.Updated, int64(0))
}

func TestParseAltaStatus_FullParking(t *testing.T) {
	html := []byte(`<script>window.Alta = {"alerts":[
  {"name":"Parking Status","slug":"parking-status","type":"parking-status","status":"Full","message":"Lots closed","enabled":true}
]};</script>`)

	st, err := ParseAltaStatus(html)
	require.NoError(t, err)
	assert.Equal(t, "Full", st.ParkingStatus)
	assert.Equal(t, "Lots closed", st.ParkingMessage)
}

func TestParseAltaStatus_Missing(t *testing.T) {
	_, err := ParseAltaStatus([]byte(`<html>no alta</html>`))
	require.Error(t, err)
}

func TestExtractJSONObject(t *testing.T) {
	raw, err := extractJSONObject([]byte(`{"a":1,"b":{"c":"}}"},"d":2} trailing`))
	require.NoError(t, err)
	assert.Equal(t, `{"a":1,"b":{"c":"}}"},"d":2}`, string(raw))
}

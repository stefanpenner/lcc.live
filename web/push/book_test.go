package push

import (
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestBookNoteAndSave(t *testing.T) {
	dir := t.TempDir()
	b, err := Open(filepath.Join(dir, "push.json"))
	require.NoError(t, err)

	kind, subs := b.Note("LCC", true)
	assert.Empty(t, kind)
	assert.Empty(t, subs)

	require.NoError(t, b.Save(Subscription{Canyon: "LCC", Endpoint: "https://push.example/1", P256dh: "k", Auth: "a"}))
	require.NoError(t, b.Save(Subscription{Canyon: "BCC", Endpoint: "https://push.example/2", P256dh: "k", Auth: "a"}))

	kind, subs = b.Note("LCC", true)
	assert.Empty(t, kind, "still closed, no second notice")

	kind, subs = b.Note("LCC", false)
	assert.Equal(t, "open", kind)
	require.Len(t, subs, 1)
	assert.Equal(t, "https://push.example/1", subs[0].Endpoint)

	reopened, err := Open(filepath.Join(dir, "push.json"))
	require.NoError(t, err)
	kind, subs = reopened.Note("LCC", true)
	assert.Equal(t, "closed", kind)
	require.Len(t, subs, 1)

	require.NoError(t, reopened.Save(Subscription{Canyon: "Provo", Endpoint: "https://push.example/1", P256dh: "k", Auth: "a"}))
	kind, subs = reopened.Note("LCC", false)
	assert.Equal(t, "open", kind)
	assert.Empty(t, subs, "endpoint moved to Provo")
}

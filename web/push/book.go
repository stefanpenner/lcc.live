package push

import (
	"encoding/json"
	"os"
	"path/filepath"
	"sync"
)

// Subscription is one browser push endpoint for one canyon.
type Subscription struct {
	Canyon   string `json:"canyon"`
	Endpoint string `json:"endpoint"`
	P256dh   string `json:"p256dh"`
	Auth     string `json:"auth"`
}

type canyonState struct {
	Seen   bool `json:"seen"`
	Closed bool `json:"closed"`
}

type fileState struct {
	Subs   []Subscription         `json:"subs"`
	Status map[string]canyonState `json:"status"`
}

// Book is the subscription file. It survives restarts when the path does.
type Book struct {
	path   string
	mu     sync.Mutex
	subs   []Subscription
	status map[string]canyonState
}

// Open loads the book at path, creating it if needed.
func Open(path string) (*Book, error) {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return nil, err
	}
	b := &Book{path: path, status: map[string]canyonState{}}
	if err := b.load(); err != nil && !os.IsNotExist(err) {
		return nil, err
	}
	return b, nil
}

func (b *Book) load() error {
	data, err := os.ReadFile(b.path)
	if err != nil {
		return err
	}
	if len(data) == 0 {
		return nil
	}
	var file fileState
	if err := json.Unmarshal(data, &file); err != nil {
		return err
	}
	b.subs = file.Subs
	if file.Status != nil {
		b.status = file.Status
	}
	return nil
}

func (b *Book) write() error {
	file := fileState{Subs: b.subs, Status: b.status}
	data, err := json.Marshal(file)
	if err != nil {
		return err
	}
	tmp := b.path + ".tmp"
	if err := os.WriteFile(tmp, data, 0o600); err != nil {
		return err
	}
	return os.Rename(tmp, b.path)
}

// Save adds or replaces a subscription by endpoint.
func (b *Book) Save(sub Subscription) error {
	b.mu.Lock()
	defer b.mu.Unlock()
	for i, old := range b.subs {
		if old.Endpoint == sub.Endpoint {
			b.subs[i] = sub
			return b.write()
		}
	}
	b.subs = append(b.subs, sub)
	return b.write()
}

// For returns the browsers subscribed to one canyon.
func (b *Book) For(canyon string) []Subscription {
	b.mu.Lock()
	defer b.mu.Unlock()
	var out []Subscription
	for _, sub := range b.subs {
		if sub.Canyon == canyon {
			out = append(out, sub)
		}
	}
	return out
}

// Remove drops the endpoint.
func (b *Book) Remove(endpoint string) error {
	b.mu.Lock()
	defer b.mu.Unlock()
	kept := b.subs[:0]
	for _, sub := range b.subs {
		if sub.Endpoint != endpoint {
			kept = append(kept, sub)
		}
	}
	b.subs = kept
	return b.write()
}

// Note records the closure reading and returns who to tell.
// kind is "closed", "open", or empty. The first reading for a canyon sends nothing.
func (b *Book) Note(canyon string, closed bool) (string, []Subscription) {
	b.mu.Lock()
	defer b.mu.Unlock()
	prev := b.status[canyon]
	kind := Notice(prev.Seen, prev.Closed, closed)
	b.status[canyon] = canyonState{Seen: true, Closed: closed}
	if err := b.write(); err != nil {
		b.status[canyon] = prev
		return "", nil
	}
	if kind == "" {
		return "", nil
	}
	var subs []Subscription
	for _, sub := range b.subs {
		if sub.Canyon == canyon {
			subs = append(subs, sub)
		}
	}
	return kind, subs
}

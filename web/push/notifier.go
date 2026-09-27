package push

import (
	"context"
	"encoding/json"
	"os"

	"github.com/stefanpenner/lcc-live/web/logger"
	"github.com/stefanpenner/lcc-live/web/store"
)

// AlertCanyon is the only drive that sends a road notice.
const AlertCanyon = "AVBH"

// Canyon is where a notice links.
type Canyon struct {
	Title string
	Path  string
}

// Notifier sends a browser push when the Apple Valley to Brian Head road closes or opens.
type Notifier struct {
	book    *Book
	pub     string
	priv    string
	subject string
	canyons map[string]Canyon
	send    func(ctx context.Context, sub Subscription, payload []byte) error
}

// New wires a book to the VAPID keys. send may be nil; then the web push sender is used.
func New(book *Book, publicKey, privateKey, subject string, canyons map[string]Canyon) *Notifier {
	n := &Notifier{
		book:    book,
		pub:     publicKey,
		priv:    privateKey,
		subject: subject,
		canyons: canyons,
	}
	n.send = func(ctx context.Context, sub Subscription, payload []byte) error {
		return sendWebPush(ctx, n.pub, n.priv, n.subject, sub, payload)
	}
	return n
}

// Enabled reports whether alerts can be subscribed to.
func (n *Notifier) Enabled() bool {
	return n != nil && n.pub != "" && n.priv != "" && n.book != nil
}

// PublicKey is the VAPID public key the browser uses to subscribe.
func (n *Notifier) PublicKey() string {
	if n == nil {
		return ""
	}
	return n.pub
}

// Save stores a browser subscription.
func (n *Notifier) Save(sub Subscription) error {
	return n.book.Save(sub)
}

// Remove drops a browser subscription.
func (n *Notifier) Remove(endpoint string) error {
	return n.book.Remove(endpoint)
}

// SendTest pushes one sample notice to the browsers subscribed to canyon.
// It does not change the saved road state. The count is how many were sent.
func (n *Notifier) SendTest(ctx context.Context, canyon string) int {
	if !n.Enabled() || canyon != AlertCanyon {
		return 0
	}
	body, err := json.Marshal(n.payload(canyon, "closed", ""))
	if err != nil {
		return 0
	}
	return n.deliver(ctx, n.book.For(canyon), body)
}

// Check compares each canyon with the last reading and pushes on a change.
func (n *Notifier) Check(ctx context.Context, s *store.Store) {
	if !n.Enabled() {
		return
	}
	for _, id := range s.CanyonIDs() {
		if id != AlertCanyon {
			continue
		}
		closed, label := Closed(s.GetRoadConditions(id), s.GetEvents(id))
		kind, subs := n.book.Note(id, closed)
		if kind == "" || len(subs) == 0 {
			continue
		}
		body, err := json.Marshal(n.payload(id, kind, label))
		if err != nil {
			continue
		}
		n.deliver(ctx, subs, body)
	}
}

func (n *Notifier) deliver(ctx context.Context, subs []Subscription, body []byte) int {
	sent := 0
	for _, sub := range subs {
		err := n.send(ctx, sub, body)
		if err == nil {
			sent++
			continue
		}
		safe := pushErr(err)
		if gone(err) {
			logger.Info("Road alert subscription ended: %s", safe.Error())
			_ = n.book.Remove(sub.Endpoint)
			continue
		}
		logger.Error(safe, "Road alert push failed: %s", safe.Error())
	}
	return sent
}

func (n *Notifier) payload(id, kind, label string) map[string]string {
	canyon := n.canyons[id]
	title := id
	if canyon.Title != "" {
		title = canyon.Title
	}
	path := canyon.Path
	if path == "" {
		path = "/"
	}
	if kind == "open" {
		return map[string]string{
			"title": title + " is open",
			"body":  "The road is open again.",
			"url":   path,
			"tag":   "lcc-road-" + id,
		}
	}
	body := label
	if body == "" {
		body = "The road is closed."
	}
	return map[string]string{
		"title": title + " is closed",
		"body":  body,
		"url":   path,
		"tag":   "lcc-road-" + id,
	}
}

// TryOpen turns alerts on when VAPID keys are set and the subscription file can be written.
func TryOpen(canyons map[string]Canyon) *Notifier {
	pub := os.Getenv("VAPID_PUBLIC_KEY")
	priv := os.Getenv("VAPID_PRIVATE_KEY")
	if pub == "" || priv == "" {
		logger.Info("Road alerts off (no VAPID keys)")
		return nil
	}
	path := os.Getenv("PUSH_DB")
	if path == "" {
		path = "/data/push.json"
	}
	book, err := Open(path)
	if err != nil {
		logger.Error(err, "Road alerts off: %v", err)
		return nil
	}
	subject := os.Getenv("VAPID_SUBJECT")
	if subject == "" {
		subject = "https://lcc.live"
	}
	logger.Info("Road alerts on")
	return New(book, pub, priv, subject, canyons)
}

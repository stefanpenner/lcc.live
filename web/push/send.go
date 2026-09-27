package push

import (
	"context"
	"errors"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"

	webpush "github.com/SherClockHolmes/webpush-go"
)

func sendWebPush(ctx context.Context, publicKey, privateKey, subject string, sub Subscription, payload []byte) error {
	resp, err := webpush.SendNotificationWithContext(ctx, payload, &webpush.Subscription{
		Endpoint: sub.Endpoint,
		Keys: webpush.Keys{
			P256dh: sub.P256dh,
			Auth:   sub.Auth,
		},
	}, &webpush.Options{
		Subscriber:      subject,
		VAPIDPublicKey:  publicKey,
		VAPIDPrivateKey: privateKey,
		TTL:             60 * 60,
		HTTPClient:      &http.Client{Timeout: 15 * time.Second},
	})
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode == http.StatusCreated || resp.StatusCode == http.StatusOK {
		return nil
	}
	raw, _ := io.ReadAll(io.LimitReader(resp.Body, 120))
	return &statusError{code: resp.StatusCode, body: oneLine(string(raw))}
}

type statusError struct {
	code int
	body string
}

func (e *statusError) Error() string {
	msg := "push endpoint returned " + strconv.Itoa(e.code)
	if e.body == "" {
		return msg
	}
	return msg + ": " + e.body
}

// pushErr is safe to log. A transport error includes the endpoint URL.
func pushErr(err error) error {
	if err == nil {
		return nil
	}
	var status *statusError
	if errors.As(err, &status) {
		return status
	}
	var target *url.Error
	if errors.As(err, &target) && target.Timeout() {
		return errors.New("push endpoint timed out")
	}
	if errors.Is(err, context.DeadlineExceeded) {
		return errors.New("push endpoint timed out")
	}
	return errors.New("push endpoint failed")
}

func oneLine(s string) string {
	s = strings.Join(strings.Fields(s), " ")
	if len(s) > 120 {
		return s[:120]
	}
	return s
}

func gone(err error) bool {
	if err == nil {
		return false
	}
	se, ok := err.(*statusError)
	if ok {
		return se.code == http.StatusNotFound || se.code == http.StatusGone
	}
	text := err.Error()
	return strings.Contains(text, " 404") || strings.Contains(text, " 410")
}

package push

import (
	"context"
	"net/http"
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
	return &statusError{code: resp.StatusCode}
}

type statusError struct{ code int }

func (e *statusError) Error() string {
	return "push endpoint returned " + strconv.Itoa(e.code)
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

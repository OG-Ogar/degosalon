package handlers

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"strings"
	"sync"
	"time"

	webpush "github.com/SherClockHolmes/webpush-go"

	"barber-booking/internal/runtimeconfig"
	"barber-booking/storage"
)

// pushNotification is the JSON payload the service worker receives. Keeping it
// small matters: a push service accepts only a few kilobytes, and a message
// alert only needs a name, a short preview and the link that opens the thread.
type pushNotification struct {
	Title string `json:"title"`
	Body  string `json:"body"`
	URL   string `json:"url"`
	Tag   string `json:"tag,omitempty"`
}

type pushConfiguration struct {
	enabled bool
	public  string
	private string
	subject string
}

var (
	pushConfigMu sync.RWMutex
	pushConfig   pushConfiguration
	pushSenderWG sync.WaitGroup
)

// InitPushNotifications reads the VAPID keys used to sign Web Push requests.
//
// Push is optional. With both keys absent, the application keeps working and
// the in-page alert still fires; this only disables alerts while the recipient
// has the app closed. A half-set pair is reported so an operator can fix it.
func InitPushNotifications() (bool, error) {
	public := strings.TrimSpace(os.Getenv("VAPID_PUBLIC_KEY"))
	private := strings.TrimSpace(os.Getenv("VAPID_PRIVATE_KEY"))
	if public == "" && private == "" {
		// Development can still exercise closed-app alerts with a temporary
		// pair. Production keys must survive restarts: otherwise every deploy
		// would invalidate the subscriptions already saved on phones.
		if runtimeconfig.Production() {
			setPushConfiguration(pushConfiguration{})
			return false, nil
		}
		generatedPrivate, generatedPublic, err := webpush.GenerateVAPIDKeys()
		if err != nil {
			setPushConfiguration(pushConfiguration{})
			return false, fmt.Errorf("generate development VAPID keys: %w", err)
		}
		public, private = generatedPublic, generatedPrivate
		slog.Warn("Web Push is using temporary development VAPID keys; set VAPID_PUBLIC_KEY and VAPID_PRIVATE_KEY")
	}
	if public == "" || private == "" {
		setPushConfiguration(pushConfiguration{})
		return false, fmt.Errorf("VAPID_PUBLIC_KEY and VAPID_PRIVATE_KEY must both be set")
	}
	if !validPushKey(public, 65) || !validPushKey(private, 32) {
		setPushConfiguration(pushConfiguration{})
		return false, fmt.Errorf("VAPID_PUBLIC_KEY and VAPID_PRIVATE_KEY are not a valid key pair")
	}

	subject := strings.TrimSpace(os.Getenv("VAPID_SUBJECT"))
	if subject == "" {
		subject = "mailto:no-reply@example.com"
	}
	if !strings.HasPrefix(subject, "mailto:") && !strings.HasPrefix(subject, "https://") {
		setPushConfiguration(pushConfiguration{})
		return false, fmt.Errorf("VAPID_SUBJECT must be a mailto: or https:// address")
	}

	setPushConfiguration(pushConfiguration{
		enabled: true,
		public:  public,
		private: private,
		subject: subject,
	})
	return true, nil
}

func setPushConfiguration(configuration pushConfiguration) {
	pushConfigMu.Lock()
	defer pushConfigMu.Unlock()
	pushConfig = configuration
}

func currentPushConfiguration() pushConfiguration {
	pushConfigMu.RLock()
	defer pushConfigMu.RUnlock()
	return pushConfig
}

// PushPublicKey is the VAPID public key the browser needs to create a push
// subscription. It is public by design and is sent only to signed-in users.
func PushPublicKey() string {
	return currentPushConfiguration().public
}

func pushEnabled() bool {
	return currentPushConfiguration().enabled
}

// sendPushAsync delivers a notification in the background. A slow or failing
// push service never holds up a booking redirect or a chat message.
func sendPushAsync(role string, userID int, notification pushNotification) {
	if !pushEnabled() || userID < 1 {
		return
	}

	payload, err := json.Marshal(notification)
	if err != nil {
		slog.Error("push_notification_encode_failed", "recipient_role", role, "recipient_id", userID, "error", err)
		return
	}

	pushSenderWG.Add(1)
	go func() {
		defer pushSenderWG.Done()

		ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
		defer cancel()

		subscriptions, err := storage.PushSubscriptionsFor(ctx, role, userID)
		if err != nil {
			slog.ErrorContext(ctx, "push_subscriptions_load_failed", "recipient_role", role, "recipient_id", userID, "error", err)
			return
		}
		if len(subscriptions) == 0 {
			return
		}

		config := currentPushConfiguration()
		for _, subscription := range subscriptions {
			// One browser may be gone without the app knowing. The push
			// service answers 404 or 410, and that registration is removed.
			response, err := webpush.SendNotificationWithContext(ctx, payload, &webpush.Subscription{
				Endpoint: subscription.Endpoint,
				Keys: webpush.Keys{
					P256dh: subscription.P256DH,
					Auth:   subscription.Auth,
				},
			}, &webpush.Options{
				Subscriber:      config.subject,
				VAPIDPublicKey:  config.public,
				VAPIDPrivateKey: config.private,
				TTL:             24 * 60 * 60,
				Urgency:         webpush.UrgencyHigh,
			})
			if response != nil {
				if response.StatusCode == http.StatusNotFound || response.StatusCode == http.StatusGone {
					if err := storage.DeletePushSubscriptionByEndpoint(ctx, subscription.Endpoint); err != nil {
						slog.ErrorContext(ctx, "push_subscription_cleanup_failed", "error", err)
					}
				}
				_ = response.Body.Close()
			}
			if err != nil {
				slog.WarnContext(ctx, "push_notification_failed", "recipient_role", role, "recipient_id", userID, "error", err)
			}
		}
	}()
}

// WaitForPushNotifications lets shutdown drain detached push sends, the same
// way booking email sends are drained.
func WaitForPushNotifications(ctx context.Context) error {
	done := make(chan struct{})
	go func() {
		pushSenderWG.Wait()
		close(done)
	}()
	select {
	case <-done:
		return nil
	case <-ctx.Done():
		return ctx.Err()
	}
}

// pushPreview turns a chat submission into one line suitable for a phone's
// notification shade. An attachment with no text still gets a useful label.
func pushPreview(text, image, audio, video string) string {
	text = strings.TrimSpace(text)
	switch {
	case text != "":
	case image != "":
		text = "Sent a photo"
	case video != "":
		text = "Sent a video"
	case audio != "":
		text = "Sent a voice note"
	default:
		text = "Sent a message"
	}
	// A notification banner shows only the first line or two. Keeping the
	// payload short also stays well inside a push service's record limit.
	runes := []rune(text)
	if len(runes) > 160 {
		text = string(runes[:160]) + "\u2026"
	}
	return text
}

// messagePush builds the notification for a chat message. The sender's name is
// in both the title and the body so it is readable even when the phone only
// shows the collapsed banner.
func messagePush(senderName, text, image, audio, video, url string, messageID int) pushNotification {
	senderName = strings.TrimSpace(senderName)
	if senderName == "" {
		senderName = "Someone"
	}
	return pushNotification{
		Title: "New message from " + senderName,
		Body:  senderName + ": " + pushPreview(text, image, audio, video),
		URL:   url,
		Tag:   fmt.Sprintf("chat-message-%d", messageID),
	}
}

// bookingPush builds the notification for a new appointment request.
func bookingPush(bookingID int, customerName, service, date, timeText string) pushNotification {
	customerName = strings.TrimSpace(customerName)
	if customerName == "" {
		customerName = "a customer"
	}
	service = strings.TrimSpace(service)
	if service == "" {
		service = "Haircut"
	}
	when := strings.TrimSpace(date)
	if clock := strings.TrimSpace(timeText); clock != "" {
		if when != "" {
			when += " at " + clock
		} else {
			when = clock
		}
	}
	body := service
	if when != "" {
		body += " · " + when
	}
	return pushNotification{
		Title: "New booking from " + customerName,
		Body:  customerName + " booked " + body,
		URL:   "/barber/bookings",
		Tag:   fmt.Sprintf("booking-%d", bookingID),
	}
}

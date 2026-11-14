package storage

import (
	"context"
	"database/sql"
	"sort"
	"time"

	"barber-booking/models"
)

// MessageAlert is the small, display-only shape behind the live chat
// notification. It carries the sender's name and a preview of what was sent,
// so the recipient can decide whether to open the conversation without the
// poll having to read the whole thread.
type MessageAlert struct {
	ID         int
	BarberID   int
	CustomerID int
	SenderRole string
	SenderName string
	Text       string
	Image      string
	Audio      string
	Video      string
	SentAt     time.Time
}

// IncomingMessagesForBarberAfter reports the newest message id in any of the
// barber's conversations and every message a customer sent after afterID.
// Messages the barber sent are deliberately absent: the sender does not need
// an alert for their own message.
func IncomingMessagesForBarberAfter(ctx context.Context, barberID, afterID int) (int, []MessageAlert, error) {
	return incomingMessagesAfter(ctx, "barber_id", barberID, models.SenderCustomer, afterID)
}

// IncomingMessagesForCustomerAfter is the customer's half of the same read:
// newest message id and every message a barber sent after afterID.
func IncomingMessagesForCustomerAfter(ctx context.Context, customerID, afterID int) (int, []MessageAlert, error) {
	return incomingMessagesAfter(ctx, "customer_id", customerID, models.SenderBarber, afterID)
}

func incomingMessagesAfter(ctx context.Context, participantColumn string, participantID int, senderRole string, afterID int) (int, []MessageAlert, error) {
	ctx, cancel := databaseContext(ctx)
	defer cancel()

	if err := ctx.Err(); err != nil {
		return 0, nil, err
	}
	if participantID < 1 || afterID < 0 {
		return 0, nil, ErrInvalidPage
	}

	if !databaseEnabled() {
		return incomingMessagesFromMemory(participantColumn, participantID, senderRole, afterID)
	}

	var latest int
	if err := db.QueryRowContext(ctx, `select coalesce(max(id),0) from public.messages where `+participantColumn+`=$1`, participantID).Scan(&latest); err != nil {
		return 0, nil, err
	}

	alerts, err := queryRecords(ctx,
		`select id,barber_id,customer_id,sender_role,sender_name,text,image,audio,video,sent_at from public.messages where `+participantColumn+`=$1 and sender_role=$2 and id>$3 order by id asc limit 25`,
		scanMessageAlert,
		participantID,
		senderRole,
		afterID,
	)
	if err != nil {
		return 0, nil, err
	}
	return latest, alerts, nil
}

func scanMessageAlert(rows *sql.Rows) (MessageAlert, error) {
	var v MessageAlert
	err := rows.Scan(&v.ID, &v.BarberID, &v.CustomerID, &v.SenderRole, &v.SenderName, &v.Text, &v.Image, &v.Audio, &v.Video, &v.SentAt)
	return v, err
}

func incomingMessagesFromMemory(participantColumn string, participantID int, senderRole string, afterID int) (int, []MessageAlert, error) {
	mu.RLock()
	defer mu.RUnlock()

	latest := 0
	alerts := make([]MessageAlert, 0)
	for _, message := range messages {
		mine := message.BarberID == participantID
		if participantColumn == "customer_id" {
			mine = message.CustomerID == participantID
		}
		if !mine {
			continue
		}
		if message.ID > latest {
			latest = message.ID
		}
		if message.SenderRole != senderRole || message.ID <= afterID {
			continue
		}
		alerts = append(alerts, MessageAlert{
			ID:         message.ID,
			BarberID:   message.BarberID,
			CustomerID: message.CustomerID,
			SenderRole: message.SenderRole,
			SenderName: message.SenderName,
			Text:       message.Text,
			Image:      message.Image,
			Audio:      message.Audio,
			Video:      message.Video,
			SentAt:     message.SentAt,
		})
	}

	sort.Slice(alerts, func(i, j int) bool { return alerts[i].ID < alerts[j].ID })
	if len(alerts) > 25 {
		alerts = alerts[len(alerts)-25:]
	}
	return latest, alerts, nil
}

// PushSubscription is one browser's Web Push registration. Endpoint is the
// push service URL; P256DH and Auth are the client keys the payload is
// encrypted with. None of these values are secrets the app can reuse to read
// a message, but they are still scoped to one signed-in account.
type PushSubscription struct {
	ID       int
	UserRole string
	UserID   int
	Endpoint string
	P256DH   string
	Auth     string
}

// SavePushSubscription stores or refreshes one browser registration. The same
// browser signing in to a second account moves the endpoint to that account
// rather than creating a duplicate that would notify both users.
func SavePushSubscription(ctx context.Context, subscription PushSubscription) error {
	ctx, cancel := databaseContext(ctx)
	defer cancel()

	if err := ctx.Err(); err != nil {
		return err
	}
	if subscription.UserRole != "barber" && subscription.UserRole != "customer" {
		return ErrForbidden
	}
	if subscription.UserID < 1 || subscription.Endpoint == "" || subscription.P256DH == "" || subscription.Auth == "" {
		return ErrInvalidPage
	}

	if databaseEnabled() {
		_, err := db.ExecContext(ctx,
			`insert into degosalon_internal.push_subscriptions(user_role,user_id,endpoint,p256dh,auth,updated_at)
			 values($1,$2,$3,$4,$5,now())
			 on conflict(endpoint) do update set
			   user_role=excluded.user_role, user_id=excluded.user_id,
			   p256dh=excluded.p256dh, auth=excluded.auth, updated_at=now()`,
			subscription.UserRole, subscription.UserID, subscription.Endpoint, subscription.P256DH, subscription.Auth)
		return err
	}

	pushMu.Lock()
	defer pushMu.Unlock()
	if pushSubscriptions == nil {
		pushSubscriptions = map[string]PushSubscription{}
	}
	subscription.ID = len(pushSubscriptions) + 1
	pushSubscriptions[subscription.Endpoint] = subscription
	return nil
}

// DeletePushSubscription removes one browser registration. It is scoped to the
// signed-in account so knowing another user's endpoint is not enough to
// unsubscribe them.
func DeletePushSubscription(ctx context.Context, role string, userID int, endpoint string) error {
	ctx, cancel := databaseContext(ctx)
	defer cancel()

	if err := ctx.Err(); err != nil {
		return err
	}
	if endpoint == "" {
		return ErrInvalidPage
	}

	if databaseEnabled() {
		_, err := db.ExecContext(ctx,
			`delete from degosalon_internal.push_subscriptions where endpoint=$1 and user_role=$2 and user_id=$3`,
			endpoint, role, userID)
		return err
	}

	pushMu.Lock()
	defer pushMu.Unlock()
	if stored, ok := pushSubscriptions[endpoint]; ok && stored.UserRole == role && stored.UserID == userID {
		delete(pushSubscriptions, endpoint)
	}
	return nil
}

// PushSubscriptionsFor lists every browser registered to one account. A user
// may have a phone and a desktop registered at the same time; each gets a
// notification.
func PushSubscriptionsFor(ctx context.Context, role string, userID int) ([]PushSubscription, error) {
	ctx, cancel := databaseContext(ctx)
	defer cancel()

	if err := ctx.Err(); err != nil {
		return nil, err
	}
	if userID < 1 {
		return nil, ErrInvalidPage
	}

	if databaseEnabled() {
		return queryRecords(ctx,
			`select id,user_role,user_id,endpoint,p256dh,auth from degosalon_internal.push_subscriptions where user_role=$1 and user_id=$2 order by id`,
			func(rows *sql.Rows) (PushSubscription, error) {
				var v PushSubscription
				err := rows.Scan(&v.ID, &v.UserRole, &v.UserID, &v.Endpoint, &v.P256DH, &v.Auth)
				return v, err
			},
			role,
			userID,
		)
	}

	pushMu.RLock()
	defer pushMu.RUnlock()
	out := make([]PushSubscription, 0)
	for _, v := range pushSubscriptions {
		if v.UserRole == role && v.UserID == userID {
			out = append(out, v)
		}
	}
	sort.Slice(out, func(i, j int) bool { return out[i].ID < out[j].ID })
	return out, nil
}

// DeletePushSubscriptionByEndpoint clears a registration the push service
// reported as gone (404 or 410). It is not scoped to a user because the push
// service has already proved the endpoint is dead.
func DeletePushSubscriptionByEndpoint(ctx context.Context, endpoint string) error {
	ctx, cancel := databaseContext(ctx)
	defer cancel()

	if err := ctx.Err(); err != nil {
		return err
	}
	if endpoint == "" {
		return nil
	}

	if databaseEnabled() {
		_, err := db.ExecContext(ctx, `delete from degosalon_internal.push_subscriptions where endpoint=$1`, endpoint)
		return err
	}

	pushMu.Lock()
	defer pushMu.Unlock()
	delete(pushSubscriptions, endpoint)
	return nil
}

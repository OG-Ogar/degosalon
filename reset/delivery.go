package reset

import (
	"context"
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"database/sql"
	"encoding/hex"
	"encoding/json"
	"errors"
	"net/mail"
	"strings"
	"time"

	"barber-booking/session"
)

var ErrDeliveryPayload = errors.New("invalid encrypted recovery delivery payload")

type deliveryPayload struct{ Recipient, Role, Token string }
type deliveryJob struct {
	digest           string
	payload          []byte
	attempts         int
	next, leaseUntil time.Time
	lease            string
}

var deliveries = map[string]deliveryJob{}

// NewDeliveryContext commits token replacement and its encrypted message in one
// transaction. A process restart never loses an accepted production request.
func NewDeliveryContext(ctx context.Context, role session.Role, userID int, recipient string, key []byte) (string, error) {
	address, err := mail.ParseAddress(recipient)
	if err != nil || address.Address != recipient || address.Name != "" || len(recipient) > 254 || strings.ContainsAny(recipient, "\r\n") || len(key) != 32 {
		return "", ErrDeliveryPayload
	}
	return issue(ctx, role, userID, recipient, key)
}

func sealDelivery(key []byte, recipient string, role session.Role, token string) ([]byte, error) {
	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, ErrDeliveryPayload
	}
	aead, err := cipher.NewGCM(block)
	if err != nil {
		return nil, ErrDeliveryPayload
	}
	nonce := make([]byte, aead.NonceSize())
	if _, err := rand.Read(nonce); err != nil {
		return nil, err
	}
	plaintext, err := json.Marshal(deliveryPayload{recipient, string(role), token})
	if err != nil {
		return nil, ErrDeliveryPayload
	}
	return aead.Seal(nonce, nonce, plaintext, []byte(fingerprint(token))), nil
}

func (job deliveryJob) open(key []byte) (deliveryPayload, error) {
	var payload deliveryPayload
	block, err := aes.NewCipher(key)
	if err != nil {
		return payload, ErrDeliveryPayload
	}
	aead, err := cipher.NewGCM(block)
	if err != nil || len(job.payload) < aead.NonceSize() {
		return payload, ErrDeliveryPayload
	}
	plaintext, err := aead.Open(nil, job.payload[:aead.NonceSize()], job.payload[aead.NonceSize():], []byte(job.digest))
	if err != nil || json.Unmarshal(plaintext, &payload) != nil || !validRole(session.Role(payload.Role)) || !validToken(payload.Token) || fingerprint(payload.Token) != job.digest {
		return deliveryPayload{}, ErrDeliveryPayload
	}
	return payload, nil
}

func claimDelivery(ctx context.Context) (deliveryJob, bool, error) {
	ctx, cancel := bounded(ctx)
	defer cancel()
	if err := ctx.Err(); err != nil {
		return deliveryJob{}, false, err
	}
	nonce := make([]byte, 16)
	if _, err := rand.Read(nonce); err != nil {
		return deliveryJob{}, false, err
	}
	lease := hex.EncodeToString(nonce)
	if database != nil {
		// Cascading deletion also removes expired ciphertext. Recheck the outer
		// expiry predicate so cleanup cannot delete a concurrently refreshed token.
		if _, err := database.ExecContext(ctx, `delete from degosalon_internal.password_resets
			where expires_at<=now() and token_digest in (select token_digest from degosalon_internal.password_resets where expires_at<=now() order by expires_at limit 100)`); err != nil {
			return deliveryJob{}, false, err
		}
		var job deliveryJob
		err := database.QueryRowContext(ctx, `with candidate as (
			select j.token_digest from degosalon_internal.reset_mail_jobs j
			join degosalon_internal.password_resets r on r.token_digest=j.token_digest
			where r.expires_at>now() and j.attempts<8 and j.next_attempt_at<=now()
			and (j.lease_until is null or j.lease_until<=now())
			order by j.next_attempt_at,j.token_digest limit 1 for update of j skip locked
		) update degosalon_internal.reset_mail_jobs j set attempts=j.attempts+1,
		lease_key=$1,lease_until=now()+interval '30 seconds'
		from candidate c where j.token_digest=c.token_digest
		returning j.token_digest,j.payload,j.attempts,j.lease_key`, lease).Scan(&job.digest, &job.payload, &job.attempts, &job.lease)
		if errors.Is(err, sql.ErrNoRows) {
			return deliveryJob{}, false, nil
		}
		return job, err == nil, err
	}
	mu.Lock()
	defer mu.Unlock()
	now := time.Now()
	for digest, job := range deliveries {
		request, exists := pending[digest]
		if !exists || request.expired() {
			delete(deliveries, digest)
			delete(pending, digest)
			continue
		}
		if job.attempts >= 8 || now.Before(job.next) || now.Before(job.leaseUntil) {
			continue
		}
		job.attempts++
		job.lease = lease
		job.leaseUntil = now.Add(30 * time.Second)
		deliveries[digest] = job
		return job, true, nil
	}
	return deliveryJob{}, false, nil
}

func finishDelivery(ctx context.Context, job deliveryJob, accepted bool) error {
	ctx, cancel := bounded(ctx)
	defer cancel()
	if err := ctx.Err(); err != nil {
		return err
	}
	delay := 30 * time.Second * time.Duration(1<<min(job.attempts-1, 4))
	if database != nil {
		if accepted {
			_, err := database.ExecContext(ctx, "delete from degosalon_internal.reset_mail_jobs where token_digest=$1 and lease_key=$2", job.digest, job.lease)
			return err
		}
		_, err := database.ExecContext(ctx, `update degosalon_internal.reset_mail_jobs
			set next_attempt_at=now()+($3 * interval '1 second'),lease_key='',lease_until=null
			where token_digest=$1 and lease_key=$2`, job.digest, job.lease, int(delay.Seconds()))
		return err
	}
	mu.Lock()
	defer mu.Unlock()
	current, exists := deliveries[job.digest]
	if !exists || current.lease != job.lease {
		return nil
	}
	if accepted {
		delete(deliveries, job.digest)
	} else {
		current.next = time.Now().Add(delay)
		current.lease = ""
		current.leaseUntil = time.Time{}
		deliveries[job.digest] = current
	}
	return nil
}

// ProcessDelivery leases one job across instances and only removes it after
// relay acceptance. A crash can deliver the same single-use link again; SMTP
// provides no exactly-once guarantee. Expired/replaced links are not retried.
func ProcessDelivery(ctx context.Context, key []byte, send func(context.Context, string, string, string) error) (bool, error) {
	if len(key) != 32 || send == nil {
		return false, ErrDeliveryPayload
	}
	job, found, err := claimDelivery(ctx)
	if err != nil || !found {
		return found, err
	}
	payload, err := job.open(key)
	if err != nil {
		return true, err
	}
	_, err = LookupContext(ctx, session.Role(payload.Role), payload.Token)
	if errors.Is(err, ErrInvalidToken) {
		return true, finishDelivery(ctx, job, true)
	}
	if err != nil {
		return true, err
	}
	err = send(ctx, payload.Recipient, payload.Role, payload.Token)
	// Keep acknowledgments independent of client/process cancellation so an
	// already accepted message is not unnecessarily retried during shutdown.
	finishErr := finishDelivery(context.Background(), job, err == nil)
	return true, errors.Join(err, finishErr)
}

package storage

import (
	"barber-booking/internal/runtimeconfig"
	"context"
	"database/sql"
	"errors"
	"regexp"
	"time"
)

type AdminAuditEvent struct {
	ID         int64     `json:"id"`
	RequestID  string    `json:"request_id"`
	Actor      string    `json:"actor"`
	Action     string    `json:"action"`
	TargetID   int       `json:"target_id"`
	Phase      string    `json:"phase"`
	HTTPStatus int       `json:"http_status"`
	CreatedAt  time.Time `json:"created_at"`
}

var auditEvents []AdminAuditEvent
var lastAuditID int64
var auditRequestID = regexp.MustCompile(`^[0-9a-f]{32}$`)
var auditActions = map[string]bool{"barber.create": true, "barber.update": true, "barber.archive": true, "barber.suspend": true, "barber.activate": true, "site.landing_image": true, "site.customer_landing_image": true}

func BeginAdminAudit(ctx context.Context, actor, action string, target int, requestID string) (int64, error) {
	ctx, cancel := databaseContext(ctx)
	defer cancel()
	if err := ctx.Err(); err != nil {
		return 0, err
	}
	if actor == "" || len(actor) > 200 || !auditActions[action] || target < 0 || !auditRequestID.MatchString(requestID) {
		return 0, errors.New("invalid administrator audit metadata")
	}
	if databaseEnabled() {
		var id int64
		err := db.QueryRowContext(ctx, `insert into degosalon_internal.admin_audit_events(request_id,actor,action,target_id,phase,http_status) values($1,$2,$3,$4,'started',0) returning id`, requestID, actor, action, target).Scan(&id)
		return id, err
	}
	if runtimeconfig.Production() {
		return 0, errors.New("durable administrator audit unavailable")
	}
	mu.Lock()
	defer mu.Unlock()
	for _, event := range auditEvents {
		if event.RequestID == requestID {
			return 0, errors.New("duplicate administrator audit request")
		}
	}
	lastAuditID++
	auditEvents = append(auditEvents, AdminAuditEvent{ID: lastAuditID, RequestID: requestID, Actor: actor, Action: action, TargetID: target, Phase: "started", CreatedAt: time.Now().UTC()})
	return lastAuditID, nil
}

func FinishAdminAudit(ctx context.Context, id int64, status, createdTarget int) error {
	ctx, cancel := databaseContext(ctx)
	defer cancel()
	if err := ctx.Err(); err != nil {
		return err
	}
	if status < 200 || status > 599 || createdTarget < 0 {
		return errors.New("invalid administrator audit status")
	}
	if databaseEnabled() {
		var inserted int64
		err := db.QueryRowContext(ctx, `insert into degosalon_internal.admin_audit_events(request_id,actor,action,target_id,phase,http_status)
   select request_id,actor,action,case when action='barber.create' and $3>0 then $3 else target_id end,'finished',$2 from degosalon_internal.admin_audit_events where id=$1 and phase='started' returning id`, id, status, createdTarget).Scan(&inserted)
		return recordError(err)
	}
	if runtimeconfig.Production() {
		return errors.New("durable administrator audit unavailable")
	}
	mu.Lock()
	defer mu.Unlock()
	for _, event := range auditEvents {
		if event.ID == id && event.Phase == "started" {
			for _, other := range auditEvents {
				if other.RequestID == event.RequestID && other.Phase == "finished" {
					return errors.New("administrator audit already finished")
				}
			}
			lastAuditID++
			event.ID = lastAuditID
			if event.Action == "barber.create" && createdTarget > 0 {
				event.TargetID = createdTarget
			}
			event.Phase = "finished"
			event.HTTPStatus = status
			event.CreatedAt = time.Now().UTC()
			auditEvents = append(auditEvents, event)
			return nil
		}
	}
	return ErrNotFound
}

// ListAdminAudit returns a bounded descending-ID page for operator inspection.
func ListAdminAudit(ctx context.Context, before int64, limit int) ([]AdminAuditEvent, error) {
	ctx, cancel := databaseContext(ctx)
	defer cancel()
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	if before < 0 || limit < 1 || limit > 200 {
		return nil, errors.New("invalid audit page")
	}
	if databaseEnabled() {
		return queryRecords(ctx, `select id,request_id,actor,action,target_id,phase,http_status,created_at from degosalon_internal.admin_audit_events where ($1::bigint=0 or id<$1) order by id desc limit $2`, func(rows *sql.Rows) (AdminAuditEvent, error) {
			var event AdminAuditEvent
			err := rows.Scan(&event.ID, &event.RequestID, &event.Actor, &event.Action, &event.TargetID, &event.Phase, &event.HTTPStatus, &event.CreatedAt)
			return event, err
		}, before, limit)
	}
	if runtimeconfig.Production() {
		return nil, errors.New("durable administrator audit unavailable")
	}
	mu.RLock()
	defer mu.RUnlock()
	result := []AdminAuditEvent{}
	for i := len(auditEvents) - 1; i >= 0 && len(result) < limit; i-- {
		if before == 0 || auditEvents[i].ID < before {
			result = append(result, auditEvents[i])
		}
	}
	return result, nil
}

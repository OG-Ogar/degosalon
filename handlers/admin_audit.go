package handlers

import (
	"barber-booking/internal/observability"
	"barber-booking/storage"
	"context"
	"log/slog"
	"net/http"
)

func adminAuditAction(r *http.Request) (string, int) {
	if r.Method != http.MethodPost {
		return "", 0
	}
	parts := pathParts(r.URL.Path)
	if len(parts) == 2 {
		switch parts[1] {
		case "landing-image":
			return "site.landing_image", 0
		case "customer-landing-image":
			return "site.customer_landing_image", 0
		}
	}
	if len(parts) == 3 && parts[1] == "barbers" && parts[2] == "new" {
		return "barber.create", 0
	}
	if len(parts) == 4 && parts[1] == "barbers" {
		id, ok := parseID(parts[2])
		if !ok {
			return "", 0
		}
		action := map[string]string{"update": "barber.update", "delete": "barber.archive", "suspend": "barber.suspend", "activate": "barber.activate"}[parts[3]]
		return action, id
	}
	return "", 0
}

// An initial durable record precedes the handler. A separate completion event
// records its HTTP result; an unmatched start means the outcome is unknown.
type adminAuditResultKey struct{}
type adminAuditResult struct{ createdID int }

func setCreatedAuditTarget(r *http.Request, id int) {
	if result, ok := r.Context().Value(adminAuditResultKey{}).(*adminAuditResult); ok {
		result.createdID = id
	}
}

func withAdminAudit(w http.ResponseWriter, r *http.Request, next http.HandlerFunc) {
	action, target := adminAuditAction(r)
	if action == "" {
		next(w, r)
		return
	}
	requestID := observability.RequestID(r.Context())
	if requestID == "" {
		var err error
		requestID, err = observability.NewRequestID()
		if err != nil {
			journeyError(w, "Service unavailable.", http.StatusServiceUnavailable)
			return
		}
		w.Header().Set("X-Request-ID", requestID)
	}
	id, err := storage.BeginAdminAudit(r.Context(), AdminUsername(), action, target, requestID)
	if err != nil {
		serverError(w, "begin administrator audit", err)
		return
	}
	result := &adminAuditResult{}
	r = r.WithContext(context.WithValue(r.Context(), adminAuditResultKey{}, result))
	response := &observability.Response{ResponseWriter: w}
	completed := false
	defer func() {
		status := response.StatusCode()
		if !completed {
			status = http.StatusInternalServerError
		}
		// Client cancellation cannot erase an already-performed action's outcome.
		ctx, cancel := context.WithTimeout(context.WithoutCancel(r.Context()), storage.DatabaseOperationTimeout)
		defer cancel()
		if err := storage.FinishAdminAudit(ctx, id, status, result.createdID); err != nil {
			slog.Error("admin_audit_incomplete", "request_id", requestID, "audit_id", id)
		}
	}()
	next(response, r)
	completed = true
}

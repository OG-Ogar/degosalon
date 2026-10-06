package handlers

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"

	"barber-booking/internal/resetmail"
	"barber-booking/internal/testdb"
	"barber-booking/models"
	"barber-booking/reset"
	"barber-booking/security"
	"barber-booking/session"
	"barber-booking/storage"
)

// Tests enqueue mail and consume it with a fake relay; no provider is contacted.
func configureRecoveryMail(t *testing.T) {
	t.Helper()
	for key, value := range map[string]string{
		"SMTP_HOST": "smtp.invalid", "SMTP_USERNAME": "test-user", "SMTP_PASSWORD": "test-password", "SMTP_FROM": "recovery@example.com", "SMTP_PORT": "587", "SMTP_TLS_MODE": "starttls", "SITE_URL": "https://salon.example.com", "RESET_MAIL_KEY": strings.Repeat("k", 64), "SHOW_RESET_LINKS": "0",
	} {
		t.Setenv(key, value)
	}
}

func TestRecoveryQueuesBothRolesAndCompletesFromDeliveredLink(t *testing.T) {
	checkQueuedRecoveryFlow(t)
}

func TestPostgresQueuedRecoveryHTTP(t *testing.T) {
	databaseURL := testdb.New(t, "../supabase/schema.sql")
	t.Setenv("DATABASE_URL", databaseURL)
	if err := storage.InitDatabase(); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = storage.CloseDatabase() })
	checkQueuedRecoveryFlow(t)
}

func checkQueuedRecoveryFlow(t *testing.T) {
	t.Helper()
	configureRecoveryMail(t)
	key, err := resetmail.DeliveryKeyFromEnvironment()
	if err != nil {
		t.Fatal(err)
	}
	config, err := resetmail.FromEnvironment()
	if err != nil {
		t.Fatal(err)
	}
	for _, flow := range []resetFlow{customerResetFlow, barberResetFlow} {
		t.Run(string(flow.Role), func(t *testing.T) {
			storage.Reset()
			reset.Clear()
			t.Cleanup(storage.Reset)
			t.Cleanup(reset.Clear)
			hash, err := security.HashPassword("old password")
			if err != nil {
				t.Fatal(err)
			}
			var id int
			if flow.Role == session.RoleCustomer {
				account, err := storage.CreateCustomer(models.Customer{FullName: "Recovery", Email: "known@example.com", PasswordHash: hash})
				if err != nil {
					t.Fatal(err)
				}
				id = account.ID
			} else {
				account, err := storage.CreateBarber(models.Barber{FullName: "Recovery", Email: "known@example.com", PasswordHash: hash})
				if err != nil {
					t.Fatal(err)
				}
				id = account.ID
			}
			oldSession, err := session.New(flow.Role, id)
			if err != nil {
				t.Fatal(err)
			}
			unknown := httptest.NewRecorder()
			forgotPassword(unknown, authFormRequest(flow.Prefix+"/forgot-password", url.Values{"email": {"unknown@example.com"}}), flow)
			if unknown.Code != http.StatusOK {
				t.Fatalf("unknown account: %d", unknown.Code)
			}
			found, err := reset.ProcessDelivery(context.Background(), key, func(context.Context, string, string, string) error {
				t.Error("mail created for unknown address")
				return nil
			})
			if err != nil || found {
				t.Fatal("unknown account queued a message")
			}
			known := httptest.NewRecorder()
			request := authFormRequest(flow.Prefix+"/forgot-password", url.Values{"email": {"known@example.com"}})
			request.Host = "attacker.example.com"
			forgotPassword(known, request, flow)
			if known.Code != http.StatusOK || strings.ReplaceAll(known.Body.String(), "known@example.com", "unknown@example.com") != unknown.Body.String() {
				t.Fatal("recovery exposed account membership")
			}
			var token string
			found, err = reset.ProcessDelivery(context.Background(), key, func(_ context.Context, to, role, got string) error {
				if to != "known@example.com" || role != string(flow.Role) {
					return errors.New("incorrect relay message")
				}
				link, err := config.ResetLink(role, got)
				if err != nil || !strings.HasPrefix(link, "https://salon.example.com"+flow.Prefix+"/reset-password?token=") {
					return errors.New("unsafe canonical recovery URL")
				}
				token = got
				return nil
			})
			if err != nil || !found || token == "" {
				t.Fatalf("queued delivery: %v %v", found, err)
			}
			response := httptest.NewRecorder()
			resetPassword(response, authFormRequest(flow.Prefix+"/reset-password", url.Values{"token": {token}, "password": {"new password"}, "confirm_password": {"new password"}}), flow)
			if response.Code != http.StatusSeeOther {
				t.Fatalf("delivered link failed: %d", response.Code)
			}
			var passwordHash string
			if flow.Role == session.RoleCustomer {
				account, err := storage.FindCustomerByEmailResult(context.Background(), "known@example.com")
				if err != nil {
					t.Fatal(err)
				}
				passwordHash = account.PasswordHash
			} else {
				account, err := storage.FindBarberByEmailResult(context.Background(), "known@example.com")
				if err != nil {
					t.Fatal(err)
				}
				passwordHash = account.PasswordHash
			}
			if !security.VerifyPassword(passwordHash, "new password") || security.VerifyPassword(passwordHash, "old password") {
				t.Fatal("mailed reset did not replace credentials")
			}
			if _, err := reset.LookupContext(context.Background(), flow.Role, token); !errors.Is(err, reset.ErrInvalidToken) {
				t.Fatal("delivered link was reusable")
			}
			if _, err := session.GetContext(context.Background(), oldSession.ID); !errors.Is(err, session.ErrNotFound) {
				t.Fatal("old session retained after mailed reset")
			}
		})
	}
}

func TestUnavailableRecoveryIsGenericAndPreservesExistingLink(t *testing.T) {
	configureRecoveryMail(t)
	t.Setenv("SMTP_HOST", "")
	storage.Reset()
	reset.Clear()
	t.Cleanup(storage.Reset)
	t.Cleanup(reset.Clear)
	account, err := storage.CreateCustomer(models.Customer{Email: "known@example.com"})
	if err != nil {
		t.Fatal(err)
	}
	token, err := reset.New(session.RoleCustomer, account.ID)
	if err != nil {
		t.Fatal(err)
	}
	for _, email := range []string{"known@example.com", "unknown@example.com"} {
		w := httptest.NewRecorder()
		CustomerForgotPasswordHandler(w, authFormRequest("/customer/forgot-password", url.Values{"email": {email}}))
		if w.Code != http.StatusServiceUnavailable || w.Header().Get("Retry-After") == "" || !strings.Contains(w.Body.String(), "temporarily unavailable") || strings.Contains(w.Body.String(), "queued for email") {
			t.Fatalf("false delivery claim: %d", w.Code)
		}
	}
	if _, err := reset.LookupContext(context.Background(), session.RoleCustomer, token); err != nil {
		t.Fatal("unavailable email invalidated existing link")
	}
}

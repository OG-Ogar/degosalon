package resetmail

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"net/http"
	"strings"
)

const brevoEndpoint = "https://api.brevo.com/v3/smtp/email"

type brevoAddress struct {
	Email string `json:"email"`
}

type brevoMessage struct {
	Sender      brevoAddress   `json:"sender"`
	To          []brevoAddress `json:"to"`
	Subject     string         `json:"subject"`
	TextContent string         `json:"textContent"`
}

func (c Config) sendBrevoMessage(ctx context.Context, recipient, subject, textContent string) error {
	payload, err := json.Marshal(brevoMessage{
		Sender:      brevoAddress{Email: c.from},
		To:          []brevoAddress{{Email: recipient}},
		Subject:     subject,
		TextContent: textContent,
	})
	if err != nil {
		return ErrDelivery
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, brevoEndpoint, bytes.NewReader(payload))
	if err != nil {
		return ErrDelivery
	}
	req.Header.Set("api-key", c.apiKey)
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json")
	client := http.Client{}
	if c.httpClient != nil {
		client = *c.httpClient
	}
	// Never forward the API key or reset token through a provider redirect.
	client.CheckRedirect = func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }
	resp, err := client.Do(req)
	if err != nil {
		return deliveryError(ctx)
	}
	defer resp.Body.Close()
	const maxReceipt = 64 << 10
	body, err := io.ReadAll(io.LimitReader(resp.Body, maxReceipt+1))
	if err != nil || ctx.Err() != nil || len(body) > maxReceipt || resp.StatusCode != http.StatusCreated {
		return deliveryError(ctx)
	}
	var receipt struct {
		MessageID string `json:"messageId"`
	}
	if json.Unmarshal(body, &receipt) != nil || strings.TrimSpace(receipt.MessageID) == "" {
		return ErrDelivery
	}
	return nil
}

func (c Config) sendBrevo(ctx context.Context, recipient, link string) error {
	return c.sendBrevoMessage(ctx, recipient, "Reset your password",
		"A password reset was requested for your account.\n\n"+link+
			"\n\nThis link expires after 30 minutes and works once. If you did not request this, you can ignore this message.\n")
}

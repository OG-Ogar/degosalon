package storage

import (
	"context"
	"testing"

	"barber-booking/models"
)

// Chat attachments are written straight to the media store, so the cleanup
// worker must treat a message's photo, voice note and video as references.
// Without this, an attachment would be deleted once the grace period passed.
func TestMessageAttachmentsAreRetainedMediaReferences(t *testing.T) {
	Reset()
	t.Cleanup(Reset)

	barber, err := CreateBarber(models.Barber{Email: "media-reference@example.com", FullName: "Barber"})
	if err != nil {
		t.Fatal(err)
	}
	customer, err := CreateCustomer(models.Customer{Email: "media-reference-customer@example.com", FullName: "Customer"})
	if err != nil {
		t.Fatal(err)
	}

	message, err := AddMessage(context.Background(), models.Message{
		BarberID:   barber.ID,
		CustomerID: customer.ID,
		SenderRole: models.SenderCustomer,
		SenderName: customer.FullName,
		Image:      mediaURI(1),
		Audio:      mediaURI(2),
		Video:      mediaURI(3),
	})
	if err != nil {
		t.Fatal(err)
	}

	for name, uri := range map[string]string{
		"photo": message.Image,
		"voice": message.Audio,
		"video": message.Video,
	} {
		if !mediaReferencedLocked(uri) {
			t.Fatalf("%s attachment %q is not retained", name, uri)
		}
	}
}

package storage

import (
	"context"
	"errors"
	"testing"

	"barber-booking/models"
)

func TestCreateSocialPostValidatesSoundtrack(t *testing.T) {
	Reset()
	t.Cleanup(Reset)

	customer, err := CreateCustomer(models.Customer{FullName: "Music Author"})
	if err != nil {
		t.Fatal(err)
	}
	base := models.SocialPost{
		AuthorRole:      models.SenderCustomer,
		AuthorID:        customer.ID,
		AuthorName:      customer.FullName,
		Text:            "A post with a soundtrack",
		BackgroundColor: "#fff0c2",
		TextStyle:       "normal",
	}

	t.Run("rejects unknown track", func(t *testing.T) {
		post := base
		post.Image = "/uploads/posts/photo.jpg"
		post.Music = "not-a-real-track"
		if _, err := CreateSocialPost(context.Background(), post); !errors.Is(err, ErrInvalidSocialPost) {
			t.Fatalf("want ErrInvalidSocialPost, got %v", err)
		}
	})

	t.Run("requires media for music", func(t *testing.T) {
		post := base
		post.Music = "sunrise-glow"
		if _, err := CreateSocialPost(context.Background(), post); !errors.Is(err, ErrInvalidSocialPost) {
			t.Fatalf("want ErrInvalidSocialPost, got %v", err)
		}
	})

	t.Run("stores a bundled track", func(t *testing.T) {
		post := base
		post.Image = "/uploads/posts/photo.jpg"
		post.Music = "sunrise-glow"
		created, err := CreateSocialPost(context.Background(), post)
		if err != nil {
			t.Fatal(err)
		}
		if created.Music != "sunrise-glow" {
			t.Fatalf("music = %q, want sunrise-glow", created.Music)
		}
	})
}

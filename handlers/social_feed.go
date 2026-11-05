package handlers

import (
	"errors"
	"fmt"
	"net/http"
	"path/filepath"
	"strings"
	"unicode/utf8"

	"barber-booking/internal/mediastore"
	"barber-booking/models"
	"barber-booking/storage"
)

const socialAttachmentLimit = mediastore.MaxFileBytes

type socialFeedView struct {
	Role       string
	ActorName  string
	ActorPhoto string
	CSRF       string
	PostAction string
	Posts      []models.SocialPost
	Statuses   []models.HairstyleStatus
	Error      string
}

func loadSocialFeed(r *http.Request, role string, actorID int, actorName, actorPhoto, csrf string) (socialFeedView, error) {
	posts, err := storage.HomeFeed(r.Context(), role, actorID)
	if err != nil {
		return socialFeedView{}, err
	}
	statuses, err := storage.HairstyleStatuses(r.Context())
	if err != nil {
		return socialFeedView{}, err
	}
	return socialFeedView{Role: role, ActorName: actorName, ActorPhoto: actorPhoto, CSRF: csrf, PostAction: "/" + role + "/dashboard", Posts: posts, Statuses: statuses}, nil
}

func createSocialPost(w http.ResponseWriter, r *http.Request, role string, actorID int, actorName, actorPhoto string, s sessionData) {
	if r.Method != http.MethodPost {
		methodNotAllowed(w, http.MethodPost)
		return
	}
	extendUploadDeadline(w)
	limitFormBody(w, r, socialAttachmentLimit+maxFormMemory)
	if !requireCSRF(w, r, s) {
		return
	}
	text := strings.TrimSpace(r.PostFormValue("text"))
	if utf8.RuneCountInString(text) > 2000 {
		journeyError(w, "Keep a post under 2,000 characters.", http.StatusBadRequest)
		return
	}
	imageURL, videoURL, err := saveSocialAttachment(r, "attachment")
	if err != nil && !errors.Is(err, errNoUpload) {
		journeyError(w, "That photo or video could not be uploaded. Use a supported JPEG, PNG, GIF, WebP, MP4, WebM or MOV file within the storage limit.", http.StatusBadRequest)
		return
	}
	if errors.Is(err, errNoUpload) {
		imageURL, videoURL = "", ""
	}
	post := models.SocialPost{AuthorRole: role, AuthorID: actorID, AuthorName: actorName, AuthorImage: actorPhoto, Text: text,
		TextColor: strings.ToLower(strings.TrimSpace(r.PostFormValue("text_color"))), TextStyle: strings.TrimSpace(r.PostFormValue("text_style")), Image: imageURL, Video: videoURL}
	if post.TextColor == "" {
		post.TextColor = "#0c1524"
	}
	if post.TextStyle == "" {
		post.TextStyle = "normal"
	}
	if text == "" && imageURL == "" && videoURL == "" {
		journeyError(w, "Write something or attach a photo/video before posting.", http.StatusBadRequest)
		return
	}
	if _, err = storage.CreateSocialPost(r.Context(), post); err != nil {
		removeChatMedia(imageURL)
		removeChatMedia(videoURL)
		if errors.Is(err, storage.ErrInvalidSocialPost) {
			journeyError(w, "Check the post text and text style, then try again.", http.StatusBadRequest)
			return
		}
		serverError(w, "create social post", err)
		return
	}
	http.Redirect(w, r, "/"+role+"/dashboard", http.StatusSeeOther)
}

// SocialPostActionHandler handles idempotent likes and bounded text comments.
func SocialPostActionHandler(w http.ResponseWriter, r *http.Request, role string, actorID int, actorName, actorPhoto string, s sessionData, postSegment, action string) {
	if r.Method != http.MethodPost {
		methodNotAllowed(w, http.MethodPost)
		return
	}
	if !requireCSRF(w, r, s) {
		return
	}
	postID, ok := parseID(postSegment)
	if !ok {
		journeyError(w, "Post not found.", http.StatusNotFound)
		return
	}
	var err error
	switch action {
	case "like":
		liked := r.PostFormValue("liked") == "true"
		err = storage.SetSocialPostLiked(r.Context(), postID, role, actorID, liked)
	case "comments":
		text := strings.TrimSpace(r.PostFormValue("comment"))
		if utf8.RuneCountInString(text) > 500 {
			journeyError(w, "Keep a comment under 500 characters.", http.StatusBadRequest)
			return
		}
		if text == "" {
			journeyError(w, "Write a comment before sending it.", http.StatusBadRequest)
			return
		}
		err = storage.AddSocialComment(r.Context(), models.SocialComment{PostID: postID, AuthorRole: role, AuthorID: actorID, AuthorName: actorName, AuthorImage: actorPhoto, Text: text})
	default:
		journeyError(w, "Page not found.", http.StatusNotFound)
		return
	}
	if err != nil {
		if errors.Is(err, storage.ErrNotFound) {
			journeyError(w, "Post not found.", http.StatusNotFound)
		} else if errors.Is(err, storage.ErrInvalidSocialComment) {
			journeyError(w, "That comment could not be sent.", http.StatusBadRequest)
		} else {
			serverError(w, "update social post", err)
		}
		return
	}
	http.Redirect(w, r, fmt.Sprintf("/%s/dashboard#post-%d", role, postID), http.StatusSeeOther)
}

func saveSocialAttachment(r *http.Request, field string) (imageURL, videoURL string, err error) {
	file, header, err := r.FormFile(field)
	if err != nil {
		if isNoUpload(err) {
			return "", "", errNoUpload
		}
		return "", "", err
	}
	ext := strings.ToLower(filepath.Ext(header.Filename))
	if allowedVideoExtensions[ext] {
		videoURL, err = socialPostVideos.store(r, file, header, socialAttachmentLimit)
		return "", videoURL, err
	}
	if !allowedImageExtensions[ext] {
		_ = file.Close()
		return "", "", fmt.Errorf("unsupported social attachment type")
	}
	imageURL, err = socialPostImages.store(r, file, header, socialAttachmentLimit)
	return imageURL, "", err
}

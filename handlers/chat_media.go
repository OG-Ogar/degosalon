package handlers

import (
	"barber-booking/internal/mediastore"
	"barber-booking/storage"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
)

// Chat attachments have their own caps: a phone photo is often well over the
// 5 MiB catalogue limit, and a short clip is far larger still. The body limit
// covers the biggest single attachment the composer can produce.
const (
	maxChatPhotoBytes      = 50 << 20
	maxChatVideoBytes      = 500 << 20
	maxChatAttachmentBytes = maxChatVideoBytes
	maxVoiceBytes          = 8 << 20
)

type voiceStore struct {
	dir     string
	urlPath string
}

var (
	chatImages = imageStore{
		dir:     "uploads/chat",
		urlPath: "/uploads/chat/",
	}

	chatVideos = videoStore{
		dir:     "uploads/chat-video",
		urlPath: "/uploads/chat-video/",
	}

	chatVoice = voiceStore{
		dir:     "uploads/chat-voice",
		urlPath: "/uploads/chat-voice/",
	}
)

// saveChatAttachment stores the composer's single attachment, which may be a
// photo or a video. The extension decides how it is validated, so the field
// name stays the same for both.
func saveChatAttachment(r *http.Request, field string) (image, video string, err error) {
	file, header, err := r.FormFile(field)
	if err != nil {
		if isNoUpload(err) {
			return "", "", errNoUpload
		}

		return "", "", fmt.Errorf("read chat attachment: %w", err)
	}

	extension := strings.ToLower(filepath.Ext(header.Filename))

	if allowedVideoExtensions[extension] {
		video, err = chatVideos.store(r, file, header, maxChatVideoBytes)

		return "", video, err
	}

	if !allowedImageExtensions[extension] {
		_ = file.Close()

		return "", "", fmt.Errorf("unsupported chat attachment type %q", extension)
	}

	image, err = chatImages.store(r, file, header, maxChatPhotoBytes)

	return image, "", err
}

// ftypBrand reports the major brand of an ISO base media file (MP4, M4A, MOV,
// 3GP). Some encoders put a small free/skip box before ftyp, so the later
// offsets are checked as well.
func ftypBrand(head []byte) (brand string, ok bool) {
	for _, at := range []int{4, 8, 12} {
		if len(head) >= at+8 && string(head[at:at+4]) == "ftyp" {
			return string(head[at+4 : at+8]), true
		}
	}

	return "", false
}

// sniffVoice identifies a recording from its first bytes, never from its
// file name. Phones record MP4/M4A (iPhone and modern Android), WebM/Opus
// (older Android) or WAV (the browser-side cross-platform fallback).
func sniffVoice(head []byte) (contentType, extension string, ok bool) {
	switch {
	case len(head) >= 4 && head[0] == 0x1A && head[1] == 0x45 && head[2] == 0xDF && head[3] == 0xA3:
		return "audio/webm", ".webm", true
	case len(head) >= 4 && string(head[:4]) == "OggS":
		return "audio/ogg", ".ogg", true
	case len(head) >= 12 && string(head[:4]) == "RIFF" && string(head[8:12]) == "WAVE":
		return "audio/wav", ".wav", true
	}

	if _, iso := ftypBrand(head); iso {
		return "audio/mp4", ".m4a", true
	}

	return "", "", false
}

func (store voiceStore) save(r *http.Request, field string) (string, error) {
	file, header, err := r.FormFile(field)
	if err != nil {
		if isNoUpload(err) {
			return "", errNoUpload
		}

		return "", fmt.Errorf("read voice upload: %w", err)
	}

	defer file.Close()

	if header.Size > maxVoiceBytes {
		return "", fmt.Errorf("voice note is larger than %d MiB", maxVoiceBytes>>20)
	}

	head := make([]byte, 64)
	read, _ := io.ReadFull(file, head)

	contentType, extension, ok := sniffVoice(head[:read])
	if !ok {
		return "", errors.New("that file is not a supported voice recording")
	}

	if _, err := file.Seek(0, io.SeekStart); err != nil {
		return "", fmt.Errorf("rewind voice upload: %w", err)
	}

	if err := os.MkdirAll(store.dir, 0o755); err != nil {
		return "", fmt.Errorf("create voice directory: %w", err)
	}

	name, err := mediaName(extension)
	if err != nil {
		return "", err
	}

	path := filepath.Join(store.dir, name)

	destination, err := os.OpenFile(path, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0o644)
	if err != nil {
		return "", fmt.Errorf("create voice file: %w", err)
	}

	if _, err := io.Copy(destination, io.LimitReader(file, maxVoiceBytes)); err != nil {
		destination.Close()
		os.Remove(path)

		return "", fmt.Errorf("save voice note: %w", err)
	}

	if err := destination.Close(); err != nil {
		os.Remove(path)

		return "", fmt.Errorf("close voice file: %w", err)
	}

	return persistMediaAs(r, store.urlPath, name, path, contentType)
}

// persistMediaAs is persistMedia with an explicit content type, because
// the standard library cannot identify audio containers reliably.
func persistMediaAs(r *http.Request, urlPath, name, path, contentType string) (string, error) {
	store, err := configuredMediaStore()
	if err != nil {
		_ = os.Remove(path)
		return "", err
	}

	if store == nil {
		return urlPath + name, nil
	}

	defer store.Close()
	defer os.Remove(path)

	file, err := os.Open(path)
	if err != nil {
		return "", errors.New("could not read media upload")
	}
	defer file.Close()

	info, err := file.Stat()
	if err != nil {
		return "", errors.New("could not read media upload")
	}

	objectPath := strings.TrimPrefix(urlPath, "/uploads/") + name

	uri, err := store.PublicURL(objectPath)
	if err != nil {
		return "", err
	}

	if err := storage.TrackMediaContext(r.Context(), uri); err != nil {
		return "", mediastore.ErrUpload
	}

	if err := store.Upload(r.Context(), objectPath, contentType, info.Size(), file); err != nil {
		return "", err
	}

	return uri, nil
}

// ChatVoiceHandler serves a locally stored voice note with the content type the
// browser needs to play it. http.FileServer guesses ".m4a" as
// application/octet-stream on many systems, which makes <audio> refuse it.
func ChatVoiceHandler(w http.ResponseWriter, r *http.Request) {
	serveChatMedia(w, r, chatVoice.dir, chatVoice.urlPath, map[string]string{
		".webm": "audio/webm",
		".ogg":  "audio/ogg",
		".oga":  "audio/ogg",
		".m4a":  "audio/mp4",
		".mp4":  "audio/mp4",
		".wav":  "audio/wav",
	})
}

// ChatVideoHandler serves a locally stored chat clip with an explicit type.
func ChatVideoHandler(w http.ResponseWriter, r *http.Request) {
	serveChatMedia(w, r, chatVideos.dir, chatVideos.urlPath, map[string]string{
		".mp4":  "video/mp4",
		".m4v":  "video/mp4",
		".mov":  "video/quicktime",
		".webm": "video/webm",
		".3gp":  "video/3gpp",
	})
}

// serveChatMedia answers one file from a chat media directory. Names are single
// path segments generated by mediaName, so nothing can walk out of the folder.
// http.ServeFile keeps range requests working, which lets audio and video seek.
func serveChatMedia(w http.ResponseWriter, r *http.Request, dir, urlPath string, contentTypes map[string]string) {
	name := strings.TrimPrefix(r.URL.Path, urlPath)

	if name == "" || name != filepath.Base(name) || name == "." || name == ".." {
		http.NotFound(w, r)

		return
	}

	contentType, ok := contentTypes[strings.ToLower(filepath.Ext(name))]
	if !ok {
		http.NotFound(w, r)

		return
	}

	w.Header().Set("Content-Type", contentType)
	w.Header().Set("X-Content-Type-Options", "nosniff")
	http.ServeFile(w, r, filepath.Join(dir, name))
}

// removeChatMedia deletes a chat photo, clip or voice note that was saved but
// whose message could not be stored.
func removeChatMedia(url string) {
	if url == "" {
		return
	}

	if strings.HasPrefix(url, "https://") {
		_ = scheduleCloudMedia(url)
		return
	}

	for _, prefix := range []struct{ urlPath, dir string }{
		{chatImages.urlPath, chatImages.dir},
		{chatVideos.urlPath, chatVideos.dir},
		{chatVoice.urlPath, chatVoice.dir},
	} {
		if !strings.HasPrefix(url, prefix.urlPath) {
			continue
		}

		name := strings.TrimPrefix(url, prefix.urlPath)
		if name == "" || strings.Contains(name, "/") || name == "." || name == ".." {
			return
		}

		_ = os.Remove(filepath.Join(prefix.dir, name))

		return
	}
}

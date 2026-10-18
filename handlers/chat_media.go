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

// maxVoiceBytes caps one voice note. Two minutes of speech is far smaller.
const maxVoiceBytes = 5 << 20

type voiceStore struct {
	dir     string
	urlPath string
}

var (
	chatImages = imageStore{
		dir:     "uploads/chat",
		urlPath: "/uploads/chat/",
	}

	chatVoice = voiceStore{
		dir:     "uploads/chat-voice",
		urlPath: "/uploads/chat-voice/",
	}
)

// sniffVoice identifies a recording from its first bytes, never from its
// file name. Phones record WebM (Android) or MP4/M4A (iPhone).
func sniffVoice(head []byte) (contentType, extension string, ok bool) {
	switch {
	case len(head) >= 4 && head[0] == 0x1A && head[1] == 0x45 && head[2] == 0xDF && head[3] == 0xA3:
		return "audio/webm", ".webm", true
	case len(head) >= 4 && string(head[:4]) == "OggS":
		return "audio/ogg", ".ogg", true
	case len(head) >= 12 && string(head[4:8]) == "ftyp":
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

// removeChatMedia deletes a chat photo or voice note that was saved but
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

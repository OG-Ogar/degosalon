package handlers

import (
	"bytes"
	"io"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
	"time"

	"barber-booking/models"
	"barber-booking/storage"
)

// chatDirs points every chat store at a throwaway directory so the tests never
// write into the repository's uploads folder.
func chatDirs(t *testing.T) {
	t.Helper()

	images, videos, voice := chatImages, chatVideos, chatVoice
	root := t.TempDir()

	chatImages.dir = filepath.Join(root, "chat")
	chatVideos.dir = filepath.Join(root, "chat-video")
	chatVoice.dir = filepath.Join(root, "chat-voice")

	t.Cleanup(func() {
		chatImages, chatVideos, chatVoice = images, videos, voice
	})
}

func webmBytes() []byte {
	return append([]byte{0x1A, 0x45, 0xDF, 0xA3}, bytes.Repeat([]byte{0x42}, 512)...)
}

func mp4Bytes() []byte {
	head := []byte{0x00, 0x00, 0x00, 0x18, 'f', 't', 'y', 'p', 'i', 's', 'o', 'm'}
	return append(head, bytes.Repeat([]byte{0x42}, 512)...)
}

func chatUpload(t *testing.T, field, name string, content []byte, text string) *http.Request {
	t.Helper()

	var body bytes.Buffer
	writer := multipart.NewWriter(&body)

	if err := writer.WriteField("csrf_token", "csrf-test"); err != nil {
		t.Fatal(err)
	}
	if err := writer.WriteField("text", text); err != nil {
		t.Fatal(err)
	}

	part, err := writer.CreateFormFile(field, name)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := part.Write(content); err != nil {
		t.Fatal(err)
	}
	if err := writer.Close(); err != nil {
		t.Fatal(err)
	}

	req := httptest.NewRequest(http.MethodPost, "/customer/chats/1", bytes.NewReader(body.Bytes()))
	req.Header.Set("Content-Type", writer.FormDataContentType())

	return req
}

func chatFixtures(t *testing.T) (models.Barber, models.Customer) {
	t.Helper()

	storage.Reset()
	t.Cleanup(storage.Reset)

	barber, err := storage.CreateBarber(models.Barber{Email: "media@example.com", FullName: "Media Barber"})
	if err != nil {
		t.Fatal(err)
	}
	customer, err := storage.CreateCustomer(models.Customer{Email: "media-customer@example.com", FullName: "Media Customer"})
	if err != nil {
		t.Fatal(err)
	}

	return barber, customer
}

// A voice note is stored even with no text, which is what "hold to record"
// sends.
func TestReadChatMessageStoresVoiceNote(t *testing.T) {
	chatDirs(t)

	req := chatUpload(t, "voice", "voice-note.webm", webmBytes(), "")
	text, image, audio, video, problem := readChatMessage(req)

	if problem != "" {
		t.Fatalf("voice note rejected: %q", problem)
	}
	if text != "" || image != "" || video != "" {
		t.Fatalf("unexpected fields: text=%q image=%q video=%q", text, image, video)
	}
	if !strings.HasPrefix(audio, chatVoice.urlPath) || !strings.HasSuffix(audio, ".webm") {
		t.Fatalf("audio = %q", audio)
	}
	if _, err := os.Stat(filepath.Join(chatVoice.dir, strings.TrimPrefix(audio, chatVoice.urlPath))); err != nil {
		t.Fatalf("voice file missing: %v", err)
	}
}

// The plus button now carries video as well as photos through the same field.
func TestReadChatMessageStoresVideoFromPhotoField(t *testing.T) {
	chatDirs(t)

	req := chatUpload(t, "photo", "clip.mp4", mp4Bytes(), "look at this")
	text, image, audio, video, problem := readChatMessage(req)

	if problem != "" {
		t.Fatalf("video rejected: %q", problem)
	}
	if text != "look at this" || image != "" || audio != "" {
		t.Fatalf("unexpected fields: text=%q image=%q audio=%q", text, image, audio)
	}
	if !strings.HasPrefix(video, chatVideos.urlPath) || !strings.HasSuffix(video, ".mp4") {
		t.Fatalf("video = %q", video)
	}
}

// A video extension with image bytes must not be stored as a video.
func TestReadChatMessageRejectsMislabelledVideo(t *testing.T) {
	chatDirs(t)

	req := chatUpload(t, "photo", "fake.mp4", []byte("not a video at all"), "")
	_, _, _, _, problem := readChatMessage(req)

	if problem == "" {
		t.Fatal("mislabelled video accepted")
	}
}

// The full handler must persist the video reference and redirect back to the
// thread, so both sides render a playable clip.
func TestCustomerChatStoresVideoAttachment(t *testing.T) {
	chatDirs(t)
	barber, customer := chatFixtures(t)

	req := chatUpload(t, "photo", "clip.mp4", mp4Bytes(), "")
	response := httptest.NewRecorder()

	CustomerChatHandler(response, req, customer, sessionData{CSRFToken: "csrf-test"}, strconv.Itoa(barber.ID))
	cleanupMultipart(req)

	if response.Code != http.StatusSeeOther {
		t.Fatalf("status = %d, want %d (body: %s)", response.Code, http.StatusSeeOther, response.Body.String())
	}

	messages, err := storage.ListConversation(req.Context(), barber.ID, customer.ID)
	if err != nil || len(messages) != 1 {
		t.Fatalf("stored messages = %d, %v", len(messages), err)
	}
	if messages[0].Video == "" || messages[0].Image != "" {
		t.Fatalf("stored message = %+v", messages[0])
	}
}

// Locally stored voice notes are served with an audio content type. Without
// this the browser treats .m4a as a download on many systems.
func TestChatVoiceHandlerServesPlayableType(t *testing.T) {
	chatDirs(t)

	if err := os.MkdirAll(chatVoice.dir, 0o755); err != nil {
		t.Fatal(err)
	}
	name := "0123456789abcdef0123456789abcdef.m4a"
	if err := os.WriteFile(filepath.Join(chatVoice.dir, name), mp4Bytes(), 0o644); err != nil {
		t.Fatal(err)
	}

	req := httptest.NewRequest(http.MethodGet, chatVoice.urlPath+name, nil)
	response := httptest.NewRecorder()
	ChatVoiceHandler(response, req)

	if response.Code != http.StatusOK {
		t.Fatalf("status = %d", response.Code)
	}
	if got := response.Header().Get("Content-Type"); got != "audio/mp4" {
		t.Fatalf("Content-Type = %q, want audio/mp4", got)
	}

	body, err := io.ReadAll(response.Body)
	if err != nil || !bytes.Equal(body, mp4Bytes()) {
		t.Fatalf("served body differs: %v", err)
	}
}

// Older Android browsers record WebM, which the composer converts to WAV so
// an iPhone can play it. The .wav file needs the matching content type too.
func TestChatVoiceHandlerServesWavFallback(t *testing.T) {
	chatDirs(t)

	if err := os.MkdirAll(chatVoice.dir, 0o755); err != nil {
		t.Fatal(err)
	}
	name := "0123456789abcdef0123456789abcdef.wav"
	body := []byte("RIFF\x24\x00\x00\x00WAVEfmt ")
	if err := os.WriteFile(filepath.Join(chatVoice.dir, name), body, 0o644); err != nil {
		t.Fatal(err)
	}

	req := httptest.NewRequest(http.MethodGet, chatVoice.urlPath+name, nil)
	response := httptest.NewRecorder()
	ChatVoiceHandler(response, req)

	if response.Code != http.StatusOK {
		t.Fatalf("status = %d", response.Code)
	}
	if got := response.Header().Get("Content-Type"); got != "audio/wav" {
		t.Fatalf("Content-Type = %q, want audio/wav", got)
	}
}

func TestChatVideoHandlerRejectsUnknownExtension(t *testing.T) {
	chatDirs(t)

	if err := os.MkdirAll(chatVideos.dir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(chatVideos.dir, "clip.webm"), webmBytes(), 0o644); err != nil {
		t.Fatal(err)
	}

	req := httptest.NewRequest(http.MethodGet, chatVideos.urlPath+"clip.webm", nil)
	response := httptest.NewRecorder()
	ChatVideoHandler(response, req)

	if response.Code != http.StatusOK {
		t.Fatalf("webm clip status = %d, want 200", response.Code)
	}

	req = httptest.NewRequest(http.MethodGet, chatVideos.urlPath+"secret.txt", nil)
	response = httptest.NewRecorder()
	ChatVideoHandler(response, req)

	if response.Code != http.StatusNotFound {
		t.Fatalf("unknown file status = %d, want 404", response.Code)
	}
}

// iPhones record QuickTime (.mov), whose ftyp brand is "qt  ". Go's generic
// sniffer calls that octet-stream, so the chat validator has its own check.
func TestSniffVideoRecognizesQuickTimeAndMp4(t *testing.T) {
	for _, test := range []struct {
		name string
		head []byte
		want string
	}{
		{"quicktime", []byte{0x00, 0x00, 0x00, 0x14, 'f', 't', 'y', 'p', 'q', 't', ' ', ' '}, "video/quicktime"},
		{"mp4", []byte{0x00, 0x00, 0x00, 0x18, 'f', 't', 'y', 'p', 'i', 's', 'o', 'm'}, "video/mp4"},
		{"3gp", []byte{0x00, 0x00, 0x00, 0x18, 'f', 't', 'y', 'p', '3', 'g', 'p', '4'}, "video/mp4"},
		{"webm", webmBytes(), "video/webm"},
		{"mp4 after free box", []byte{0x00, 0x00, 0x00, 0x08, 'f', 'r', 'e', 'e', 0x00, 0x00, 0x00, 0x18, 'f', 't', 'y', 'p', 'i', 's', 'o', 'm'}, "video/mp4"},
	} {
		t.Run(test.name, func(t *testing.T) {
			kind, ok := sniffVideo(test.head)
			if !ok || kind != test.want {
				t.Fatalf("sniffVideo = %q, %v; want %q", kind, ok, test.want)
			}
		})
	}

	if _, ok := sniffVideo([]byte("not a video")); ok {
		t.Fatal("plain text accepted as video")
	}
}

// A thread renders both media players, and the composer offers photo and
// video in the same picker.
func TestChatThreadRendersVideoAndVoicePlayers(t *testing.T) {
	if err := LoadTemplates("../templates"); err != nil {
		t.Fatal(err)
	}

	now := time.Now()
	response := httptest.NewRecorder()
	render(response, "customer/chat.html", chatView{
		Title:  "Barber",
		Action: "/customer/chats/1",
		CSRF:   "csrf-test",
		Messages: []models.Message{
			{SenderRole: models.SenderCustomer, SenderName: "Client", Video: "/uploads/chat-video/a.mp4", SentAt: now},
			{SenderRole: models.SenderBarber, SenderName: "Barber", Audio: "/uploads/chat-voice/a.webm", SentAt: now},
		},
	})

	body := response.Body.String()
	for _, want := range []string{
		`class="chat-video"`,
		"controls",
		`src="/uploads/chat-video/a.mp4"`,
		`src="/uploads/chat-voice/a.webm"`,
		`class="chat-audio"`,
		`preload="metadata"`,
		`class="chat-audio-download"`,
		`class="chat-audio-warning"`,
		`accept="image/jpeg,image/png,image/gif,image/webp,video/mp4,video/webm,video/quicktime,video/x-m4v,video/3gpp"`,
		`data-photo-preview-video`,
		`data-composer-status`,
	} {
		if !strings.Contains(body, want) {
			t.Fatalf("chat page missing %q", want)
		}
	}
	if got := strings.Count(body, "data-photo-preview hidden"); got != 1 {
		t.Fatalf("composer has %d attachment previews, want 1", got)
	}
}

func TestSniffVoiceAcceptsM4AWithLeadingBox(t *testing.T) {
	head := []byte{0x00, 0x00, 0x00, 0x08, 'f', 'r', 'e', 'e', 0x00, 0x00, 0x00, 0x18, 'f', 't', 'y', 'p', 'M', '4', 'A', ' '}
	kind, extension, ok := sniffVoice(head)
	if !ok || kind != "audio/mp4" || extension != ".m4a" {
		t.Fatalf("sniffVoice = %q, %q, %v", kind, extension, ok)
	}
}

func TestSniffVoiceAcceptsWavFallback(t *testing.T) {
	head := []byte("RIFF\x24\x00\x00\x00WAVEfmt ")
	kind, extension, ok := sniffVoice(head)
	if !ok || kind != "audio/wav" || extension != ".wav" {
		t.Fatalf("sniffVoice = %q, %q, %v", kind, extension, ok)
	}
}

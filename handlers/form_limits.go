package handlers

import (
	"errors"
	"io"
	"net/http"
	"strings"
	"time"
)

const maxMutationFormBytes = 64 << 10

// uploadWindow is how long a client may take to send one large attachment.
// The server-wide ReadTimeout is short on purpose (slow clients must not hold a
// connection), so upload routes extend it just for themselves.
const uploadWindow = 5 * time.Minute

type limitedFormBody struct {
	io.ReadCloser
	limit int64
}

// extendUploadDeadline lets an upload route wait for a large body without
// loosening the timeout for every other request. Recorders on a slow mobile
// connection can need minutes to send a clip.
func extendUploadDeadline(w http.ResponseWriter) {
	_ = http.NewResponseController(w).SetReadDeadline(time.Now().Add(uploadWindow))
}

func limitFormBody(w http.ResponseWriter, r *http.Request, limit int64) {
	r.Body = &limitedFormBody{ReadCloser: http.MaxBytesReader(w, r.Body, limit), limit: limit}
}

// parseBoundedForm must run before PostFormValue: that helper discards parsing
// errors and uses a much larger default multipart memory allowance.
func parseBoundedForm(w http.ResponseWriter, r *http.Request) bool {
	body, explicit := r.Body.(*limitedFormBody)
	multipart := strings.HasPrefix(strings.ToLower(r.Header.Get("Content-Type")), "multipart/")
	limit := int64(maxMutationFormBytes)
	if explicit {
		if multipart {
			limit = body.limit
		} else {
			limit = min(limit, body.limit)
			if limit < body.limit {
				limitFormBody(w, r, limit)
			}
		}
	} else {
		limitFormBody(w, r, limit)
	}
	if r.ContentLength > limit {
		journeyError(w, "This form is too large.", http.StatusRequestEntityTooLarge)
		return false
	}
	var err error
	if multipart {
		err = r.ParseMultipartForm(maxFormMemory)
	} else {
		err = r.ParseForm()
	}
	if err != nil {
		var large *http.MaxBytesError
		if errors.As(err, &large) {
			journeyError(w, "This form is too large.", http.StatusRequestEntityTooLarge)
		} else {
			journeyError(w, "This form could not be read. Reload and try again.", http.StatusBadRequest)
		}
		return false
	}
	return true
}

func cleanupMultipart(r *http.Request) {
	if r.MultipartForm != nil {
		_ = r.MultipartForm.RemoveAll()
	}
}

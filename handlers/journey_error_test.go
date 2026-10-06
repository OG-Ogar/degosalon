package handlers
import ("net/http"; "net/http/httptest"; "strings"; "testing")
func TestJourneyErrorPreservesStatusAndEscapesMessages(t *testing.T) {
    w := httptest.NewRecorder()
    journeyError(w, "<script>alert(1)</script>", http.StatusConflict)
    if w.Code != http.StatusConflict { t.Fatalf("status %d", w.Code) }
    if !strings.Contains(w.Header().Get("Content-Type"), "text/html") { t.Fatal("missing HTML content type") }
    body := w.Body.String()
    if strings.Contains(body, "<script>") { t.Fatal("error message can inject HTML") }
    if !strings.Contains(body, "&lt;script&gt;") || !strings.Contains(body, "Recovery options") { t.Fatal("missing escaped message or recovery") }
}

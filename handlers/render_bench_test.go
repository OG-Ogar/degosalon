package handlers

import (
	"bytes"
	"net/http"
	"net/http/httptest"
	"testing"
)

func BenchmarkViewStringFastPath(b *testing.B) {
	view := browseBarbersView{
		CSRF:          "test-csrf-token-1234567890",
		ResultsLink:   "/customer/barbers?state=Lagos",
		SelectedState: "Lagos",
		SelectedArea:  "Ikeja",
	}

	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = viewString(view, "CSRF")
		_ = viewString(view, "ResultsLink")
		_ = viewString(view, "SelectedState")
		_ = viewString(view, "SelectedArea")
	}
}

func BenchmarkViewStringCachedReflection(b *testing.B) {
	type CustomViewModel struct {
		CSRF        string
		CustomField string
	}
	view := CustomViewModel{CSRF: "custom-csrf-token", CustomField: "custom"}

	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = viewString(view, "CSRF")
	}
}

func BenchmarkRenderBufferPool(b *testing.B) {
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		buf := renderBufferPool.Get().(*bytes.Buffer)
		buf.Reset()
		buf.WriteString("<!DOCTYPE html><html><body>Render Buffer Pool Benchmark Test</body></html>")
		renderBufferPool.Put(buf)
	}
}

func BenchmarkJourneyErrorWithBufferPool(b *testing.B) {
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		w := httptest.NewRecorder()
		journeyError(w, "Benchmark test message", http.StatusInternalServerError)
	}
}


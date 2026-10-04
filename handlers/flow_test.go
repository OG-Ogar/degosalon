package handlers

import (
	"fmt"
	"io"
	"net/http"
	"net/http/cookiejar"
	"net/http/httptest"
	"net/url"
	"os"
	"regexp"
	"strings"
	"testing"
	"time"

	"barber-booking/storage"
	"barber-booking/models"
)

// TestMain parses the real templates once.
//
// A template that reads a field the handler does not pass only fails when
// it is executed, so these tests drive the handlers through the files on
// disk instead of stubbing them out. renderStatus buffers the page before
// writing it, which turns any such mistake into a 500 the tests catch.
func TestMain(m *testing.M) {
	if err := LoadTemplates("../templates"); err != nil {
		fmt.Fprintf(os.Stderr, "load templates: %v\n", err)
		os.Exit(1)
	}

	os.Exit(m.Run())
}

// csrfPattern finds the hidden token every authenticated form carries.
var csrfPattern = regexp.MustCompile(`name="csrf_token" value="([^"]*)"`)

// newServer starts the routes main.go serves, on a clean store.
func newServer(t *testing.T) *httptest.Server {
	t.Helper()

	storage.Reset()

	mux := http.NewServeMux()
	mux.HandleFunc("/explore", ExploreRouter)
	mux.HandleFunc("/explore/", ExploreRouter)
	mux.HandleFunc("/", HomeHandler)
	mux.HandleFunc("/barber/", BarberRouter)
	mux.HandleFunc("/customer/", CustomerRouter)

	server := httptest.NewServer(mux)
	t.Cleanup(server.Close)

	return server
}

// newBrowser returns a client that keeps cookies, so signing in on one
// request still counts on the next. Each browser is one signed-in user.
func newBrowser(t *testing.T) *http.Client {
	t.Helper()

	jar, err := cookiejar.New(nil)
	if err != nil {
		t.Fatalf("cookiejar.New: %v", err)
	}

	return &http.Client{Jar: jar, Timeout: 10 * time.Second}
}

// get fetches a page and fails the test unless it returns 200.
func get(t *testing.T, browser *http.Client, url string) string {
	t.Helper()

	response, err := browser.Get(url)
	if err != nil {
		t.Fatalf("GET %s: %v", url, err)
	}

	return readBody(t, "GET "+url, response)
}

// post submits a form and fails the test unless it returns 200, having
// followed any redirect the handler answered with.
func post(t *testing.T, browser *http.Client, url string, form url.Values) string {
	t.Helper()

	response, err := browser.PostForm(url, form)
	if err != nil {
		t.Fatalf("POST %s: %v", url, err)
	}

	return readBody(t, "POST "+url, response)
}

// readBody drains a response and reports a non-200 status with the body,
// which is where serverError and the form errors put their explanation.
func readBody(t *testing.T, what string, response *http.Response) string {
	t.Helper()

	defer response.Body.Close()

	body, err := io.ReadAll(response.Body)
	if err != nil {
		t.Fatalf("%s: read body: %v", what, err)
	}

	if response.StatusCode != http.StatusOK {
		t.Fatalf("%s: status %d\n%s", what, response.StatusCode, body)
	}

	return string(body)
}

// csrfToken pulls the session's token out of a rendered page.
func csrfToken(t *testing.T, what, body string) string {
	t.Helper()

	match := csrfPattern.FindStringSubmatch(body)
	if match == nil {
		t.Fatalf("%s: no csrf_token in the page", what)
	}

	if match[1] == "" {
		t.Fatalf("%s: csrf_token is empty", what)
	}

	return match[1]
}

// mustContain fails the test when a page is missing expected text.
func mustContain(t *testing.T, what, body, want string) {
	t.Helper()

	if !strings.Contains(body, want) {
		t.Fatalf("%s: page does not mention %q", what, want)
	}
}


func TestPublicExploreRequiresLocationAndShowsMatchingBarber(t *testing.T) {
	storage.Reset()
	_,err:=storage.CreateBarber(models.Barber{FullName:"Ada",PhoneNumber:"1",Email:"ada@example.com",PasswordHash:"x",ShopName:"Ada Cuts",ShopAddress:"12 Road",State:"Lagos",LocalArea:"Ikeja",Status:models.BarberActive})
	if err!=nil{t.Fatal(err)}
	server:=newServer(t)
	_,err=storage.CreateBarber(models.Barber{FullName:"Ada",PhoneNumber:"1",Email:"ada@example.com",PasswordHash:"x",ShopName:"Ada Cuts",ShopAddress:"12 Road",State:"Lagos",LocalArea:"Ikeja",Status:models.BarberActive})
	if err!=nil{t.Fatal(err)}
	browser:=newBrowser(t)
	body:=get(t,browser,server.URL+"/explore")
	mustContain(t,"explore",body,"Where do you need a barber?")
	body=get(t,browser,server.URL+"/explore?state=Lagos&area=Ikeja")
	mustContain(t,"filtered explore",body,"Ada Cuts")
}

func TestPublicExploreHidesBarberOutsideLocation(t *testing.T) {
	storage.Reset()
	_,_ = storage.CreateBarber(models.Barber{FullName:"Ada",PhoneNumber:"1",Email:"ada@example.com",PasswordHash:"x",ShopName:"Ada Cuts",ShopAddress:"12 Road",State:"Lagos",LocalArea:"Ikeja",Status:models.BarberActive})
	server:=newServer(t); browser:=newBrowser(t)
	body:=get(t,browser,server.URL+"/explore?state=Osun&area=Osogbo")
	if strings.Contains(body,"Ada Cuts"){t.Fatal("barber outside selected location was shown")}
}

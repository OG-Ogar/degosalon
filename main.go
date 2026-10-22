package main

import (
	"barber-booking/internal/observability"
	"context"
	"fmt"
	"log"
	"log/slog"
	"net"
	"net/http"
	"os"
	"os/signal"
	"path/filepath"
	"strings"
	"sync"
	"syscall"
	"time"
	_ "time/tzdata"

	"barber-booking/handlers"
	"barber-booking/internal/resetmail"
	"barber-booking/internal/runtimeconfig"
	"barber-booking/reset"
	"barber-booking/session"
	"barber-booking/storage"
)

// listenAddress is where the server binds. PORT overrides it so the app
// can run somewhere that assigns one.
func listenAddress() string {
	if port := os.Getenv("PORT"); port != "" {
		return ":" + port
	}

	return ":8080"
}

var (
	staticETagMu sync.RWMutex
	staticETags  = map[string]string{}
)

func getStaticETag(rel string) string {
	staticETagMu.RLock()
	etag, ok := staticETags[rel]
	staticETagMu.RUnlock()
	if ok {
		return etag
	}
	clean := filepath.Clean(rel)
	if strings.Contains(clean, "..") {
		return ""
	}
	filePath := filepath.Join("static", clean)
	info, err := os.Stat(filePath)
	if err != nil || info.IsDir() {
		return ""
	}
	etag = fmt.Sprintf(`W/"%x-%x"`, info.ModTime().UnixNano(), info.Size())
	staticETagMu.Lock()
	staticETags[rel] = etag
	staticETagMu.Unlock()
	return etag
}

// revalidate asks browsers to check with the server before reusing a cached
// copy. Static assets change with every deploy, and the file server answers
// 304 Not Modified when nothing changed, so this keeps a stale stylesheet or
// script from surviving a deploy without a hard refresh.
func revalidate(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Cache-Control", "no-cache")
		cleanPath := filepath.Clean(r.URL.Path)
		if strings.HasPrefix(cleanPath, "/static/") {
			rel := strings.TrimPrefix(cleanPath, "/static/")
			if etag := getStaticETag(rel); etag != "" {
				w.Header().Set("ETag", etag)
				if match := r.Header.Get("If-None-Match"); match == etag {
					w.WriteHeader(http.StatusNotModified)
					return
				}
			}
		}
		next.ServeHTTP(w, r)
	})
}

func main() {
	slog.SetDefault(slog.New(slog.NewJSONHandler(os.Stderr, nil)))
	if err := run(); err != nil {
		log.Printf("server stopped: %v", err)
		os.Exit(1)
	}
}

func run() error {
	lifecycleContext, stopSignals := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stopSignals()
	if err := runtimeconfig.Validate(); err != nil {
		return fmt.Errorf("deployment configuration: %w", err)
	}

	if err := storage.InitDatabase(); err != nil {
		return fmt.Errorf("connect to database: %w", err)
	}
	var workersDone chan struct{}
	defer func() {
		// A timed-out worker may still be using storage. Process exit releases the
		// connection; never reconfigure shared stores underneath that worker.
		if workersDone != nil {
			select {
			case <-workersDone:
			default:
				return
			}
		}
		_ = storage.CloseDatabase()
		handlers.CloseMediaStore()
	}()
	if err := handlers.ImportLegacySiteImages(context.Background()); err != nil {
		return fmt.Errorf("import legacy site media settings: %w", err)
	}

	// Templates are parsed once, at startup. A broken template used to
	// surface as a half-written page on the first request that hit it;
	// now it stops the server before it accepts any traffic.
	if err := handlers.LoadTemplates("templates"); err != nil {
		return fmt.Errorf("load templates: %w", err)
	}

	// The admin credentials come from the environment. A generated
	// password is printed once, so no account is left on a value that is
	// committed to the repository.
	generated, err := handlers.InitAdmin()
	if err != nil {
		return fmt.Errorf("initialise admin account: %w", err)
	}

	if generated != "" {
		log.Printf(
			"no ADMIN_PASSWORD set: generated one for this run\n"+
				"    admin username: %s\n"+
				"    admin password: %s",
			handlers.AdminUsername(),
			generated,
		)
	}

	// The bookable part of the day comes from the environment as well, so
	// a shop with different hours does not need a code change.
	if err := handlers.InitBookingHours(); err != nil {
		return fmt.Errorf("booking hours: %w", err)
	}

	openingDefault, closingDefault := handlers.BookingHours()
	storage.ConfigureCalendarDefaults(handlers.BookingTimezone(), openingDefault, closingDefault)

	mux := http.NewServeMux()
	health := &healthState{check: storage.CheckReadiness}
	mux.HandleFunc("/health/live", health.handle)
	mux.HandleFunc("/health/ready", health.handle)
	mux.HandleFunc("/media/preview", handlers.ImageVariantHandler)

	mux.HandleFunc("/explore", handlers.ExploreRouter)
	mux.HandleFunc("/explore/", handlers.ExploreRouter)
	for _, path := range []string{"/help", "/support", "/privacy", "/terms"} {
		mux.HandleFunc(path, handlers.PublicContentHandler)
	}
	mux.HandleFunc("/", handlers.HomeHandler)
	mux.HandleFunc("/admin/", handlers.AdminRouter)
	mux.HandleFunc("/barber/", handlers.BarberRouter)
	mux.HandleFunc("/customer/", handlers.CustomerRouter)

	// Uploaded images are served read-only from their own directory. Go's
	// FileServer resolves the path itself, so "../" in a URL cannot walk
	// out of it.
	mux.Handle(
		"/uploads/",
		http.StripPrefix("/uploads/", http.FileServer(http.Dir("uploads"))),
	)

	// Chat audio and video need an explicit content type: without it some
	// systems answer ".m4a" with application/octet-stream and the browser
	// refuses to play the note. These routes sit inside /uploads/ and win
	// because ServeMux prefers the longer pattern.
	mux.HandleFunc("/uploads/chat-voice/", handlers.ChatVoiceHandler)
	mux.HandleFunc("/uploads/chat-video/", handlers.ChatVideoHandler)

	// Catalogue clips (hairstyle and product videos) need the same explicit
	// content type, so a barber's uploaded video plays on the catalogue and
	// shop pages.
	mux.HandleFunc("/uploads/hairstyle-videos/", handlers.CatalogueVideoHandler)
	mux.HandleFunc("/uploads/product-videos/", handlers.CatalogueVideoHandler)

	mux.Handle(
		"/static/",
		revalidate(http.StripPrefix("/static/", http.FileServer(http.Dir("static")))),
	)

	server := &http.Server{
		Addr:    listenAddress(),
		Handler: observability.Requests(slog.Default(), observability.Compress(mux)),

		// Without these a slow or idle client can hold a connection open
		// indefinitely. WriteTimeout is the longest because it also
		// covers sending an upload back out.
		ReadHeaderTimeout: 10 * time.Second,
		ReadTimeout:       30 * time.Second,
		WriteTimeout:      60 * time.Second,
		IdleTimeout:       2 * time.Minute,
	}

	listener, err := net.Listen("tcp", server.Addr)
	if err != nil {
		return fmt.Errorf("listen: %w", err)
	}
	workerContext, stopWorkers := context.WithCancel(context.Background())
	var workers sync.WaitGroup
	startWorker := func(work func(context.Context)) {
		workers.Add(1)
		go func() { defer workers.Done(); work(workerContext) }()
	}
	startWorker(handlers.RunMediaCleanup)
	startWorker(func(ctx context.Context) { session.RunCleanupWorker(ctx, 15*time.Minute) })
	mailConfig, mailErr := resetmail.FromEnvironment()
	mailKey, keyErr := resetmail.DeliveryKeyFromEnvironment()
	if mailErr == nil && keyErr == nil {
		log.Print("password-recovery email worker configured")
		startWorker(func(ctx context.Context) { runResetMailWorker(ctx, mailConfig, mailKey) })
	} else {
		log.Print("password-recovery email unavailable: configure Brevo (BREVO_API_KEY/BREVO_FROM) or SMTP, HTTPS site origin, and RESET_MAIL_KEY")
	}
	workersDone = make(chan struct{})
	go func() { workers.Wait(); close(workersDone) }()
	health.ready.Store(true)

	log.Printf("barber booking server listening on http://localhost%s", server.Addr)

	opening, closing := handlers.BookingHours()
	log.Printf("bookings are taken between %s and %s", opening, closing)

	serveErr := serveUntilStopped(lifecycleContext, server, listener, health, stopWorkers, workersDone, shutdownTimeout)

	// Booking-alert goroutines are detached from the request that created
	// them. Join them before run returns, because its deferred cleanup closes
	// the database they may still need.
	notifyContext, cancelNotifications := context.WithTimeout(context.Background(), 20*time.Second)
	notificationErr := handlers.WaitForBookingNotifications(notifyContext)
	cancelNotifications()
	if notificationErr != nil {
		log.Printf("booking notification workers did not stop: %v", notificationErr)
	}

	return serveErr
}

func runResetMailWorker(ctx context.Context, config resetmail.Config, key []byte) {
	for {
		found, err := reset.ProcessDelivery(ctx, key, config.Send)
		if ctx.Err() != nil {
			return
		}
		if err != nil {
			log.Print("password-recovery email processing failed")
		}
		if found && err == nil {
			continue
		}
		timer := time.NewTimer(5 * time.Second)
		select {
		case <-ctx.Done():
			timer.Stop()
			return
		case <-timer.C:
		}
	}
}

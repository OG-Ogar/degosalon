package main

import (
	"errors"
	"log"
	"net/http"
	"os"
	"time"

	"barber-booking/handlers"
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

func main() {
	log.SetFlags(log.LstdFlags)

	if err := storage.InitDatabase(); err != nil {
		log.Fatalf("connect to database: %v", err)
	}
	defer storage.CloseDatabase()

	// Templates are parsed once, at startup. A broken template used to
	// surface as a half-written page on the first request that hit it;
	// now it stops the server before it accepts any traffic.
	if err := handlers.LoadTemplates("templates"); err != nil {
		log.Fatalf("load templates: %v", err)
	}

	// The admin credentials come from the environment. A generated
	// password is printed once, so no account is left on a value that is
	// committed to the repository.
	generated, err := handlers.InitAdmin()
	if err != nil {
		log.Fatalf("initialise admin account: %v", err)
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
		log.Fatalf("booking hours: %v", err)
	}

	mux := http.NewServeMux()

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

	mux.Handle(
		"/static/",
		http.StripPrefix("/static/", http.FileServer(http.Dir("static"))),
	)

	server := &http.Server{
		Addr:    listenAddress(),
		Handler: mux,

		// Without these a slow or idle client can hold a connection open
		// indefinitely. WriteTimeout is the longest because it also
		// covers sending an upload back out.
		ReadHeaderTimeout: 10 * time.Second,
		ReadTimeout:       30 * time.Second,
		WriteTimeout:      60 * time.Second,
		IdleTimeout:       2 * time.Minute,
	}

	log.Printf("barber booking server listening on http://localhost%s", server.Addr)

	opening, closing := handlers.BookingHours()
	log.Printf("bookings are taken between %s and %s", opening, closing)

	// A failed bind used to be printed and then ignored, so the process
	// exited 0 and looked healthy while serving nothing.
	if err := server.ListenAndServe(); err != nil &&
		!errors.Is(err, http.ErrServerClosed) {

		log.Fatalf("server stopped: %v", err)
	}
}

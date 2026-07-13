package main

import (
	"barber-booking/handlers"
	"fmt"
	"net/http"
)

func main() {
	http.HandleFunc("/", handlers.HomeHandler)

	http.HandleFunc("/admin/login", handlers.AdminLoginHandler)

	http.HandleFunc("/admin/barbers/new", handlers.NewBarberHandler)

	http.HandleFunc("/admin/barbers", handlers.ListBarbersHandler)

	http.HandleFunc("/admin/barbers/", handlers.BarberRouter)

	fmt.Println("Starting Barber Booking Server...")

	err := http.ListenAndServe(":8080", nil)
	if err != nil {
		fmt.Println(err)
	}
}

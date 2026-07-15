package main

import (
	"barber-booking/handlers"
	"fmt"
	"net/http"
)

func main() {

	http.HandleFunc("/", handlers.HomeHandler)

	http.HandleFunc("/admin/", handlers.AdminRouter)

	http.HandleFunc("/barber/", handlers.BarberRouter)

	fmt.Println("Starting Barber Booking Server...")

	err := http.ListenAndServe(":8080", nil)
	if err != nil {
		fmt.Println(err)
	}
}

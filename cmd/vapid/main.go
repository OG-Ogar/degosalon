// Command vapid prints a Web Push key pair for VAPID_PUBLIC_KEY and
// VAPID_PRIVATE_KEY. Generate it once and keep it stable: replacing the pair
// invalidates every push subscription already stored on a user's device.
package main

import (
	"fmt"
	"log"

	webpush "github.com/SherClockHolmes/webpush-go"
)

func main() {
	privateKey, publicKey, err := webpush.GenerateVAPIDKeys()
	if err != nil {
		log.Fatal(err)
	}
	fmt.Printf("VAPID_PUBLIC_KEY=%s\n", publicKey)
	fmt.Printf("VAPID_PRIVATE_KEY=%s\n", privateKey)
}

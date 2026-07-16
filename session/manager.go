package session

import (
	"crypto/rand"
	"encoding/hex"
)

type Session struct {
	ID       string
	BarberID int
}

var Sessions = map[string]Session{}

func NewSession(barberID int) Session {

	bytes := make([]byte, 16)

	rand.Read(bytes)

	id := hex.EncodeToString(bytes)

	session := Session{
		ID:       id,
		BarberID: barberID,
	}

	Sessions[id] = session

	return session
}

func GetSession(id string) (Session, bool) {

	session, found := Sessions[id]

	return session, found
}

func DeleteSession(id string) {

	delete(Sessions, id)

}
package hub

import (
	"testing"
	"time"

	jwt "github.com/dgrijalva/jwt-go"
	"github.com/stretchr/testify/assert"
)

func TestNewHub(t *testing.T) {
	h := NewHub([]byte("publisher"), []byte("subscriber"))

	assert.Equal(t, []byte("publisher"), h.publisherJWTKey)
	assert.Equal(t, []byte("subscriber"), h.subscriberJWTKey)
	assert.IsType(t, map[chan Resource]struct{}{}, h.subscribers)
	assert.IsType(t, make(chan (chan Resource)), h.newSubscribers)
	assert.IsType(t, make(chan (chan Resource)), h.removedSubscribers)
	assert.IsType(t, make(chan Resource), h.resources)
}

func createDummy() *Hub {
	return NewHub([]byte("publisher"), []byte("subscriber"))
}

func createDummyAuthorizedJWT(h *Hub, publisher bool) string {
	var key []byte
	if publisher {
		key = h.publisherJWTKey
	} else {
		key = h.subscriberJWTKey
	}

	token := jwt.New(jwt.SigningMethodHS256)

	expiresAt := time.Now().Add(time.Minute * 1).Unix()
	token.Claims = &jwt.StandardClaims{ExpiresAt: expiresAt}
	tokenString, _ := token.SignedString(key)

	return tokenString
}

func createDummyAuthorizedJWTWithTargets(h *Hub, targets []string) string {
	token := jwt.New(jwt.SigningMethodHS256)

	expiresAt := time.Now().Add(time.Minute * 1).Unix()
	token.Claims = &claims{targets, jwt.StandardClaims{ExpiresAt: expiresAt}}
	tokenString, _ := token.SignedString(h.subscriberJWTKey)

	return tokenString
}

func createDummyUnauthorizedJWT(h *Hub) string {
	token := jwt.New(jwt.SigningMethodHS256)

	expiresAt := time.Now().Add(time.Minute * 1).Unix()
	token.Claims = &jwt.StandardClaims{ExpiresAt: expiresAt}
	tokenString, _ := token.SignedString([]byte("unauthorized"))

	return tokenString
}

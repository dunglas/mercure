package hub

import (
	"testing"

	jwt "github.com/dgrijalva/jwt-go"
	"github.com/stretchr/testify/assert"
)

func TestNewHub(t *testing.T) {
	h := NewHub([]byte("publisher"), []byte("subscriber"), false)

	assert.Equal(t, []byte("publisher"), h.publisherJWTKey)
	assert.Equal(t, []byte("subscriber"), h.subscriberJWTKey)
	assert.IsType(t, map[chan *serializedUpdate]struct{}{}, h.subscribers)
	assert.IsType(t, make(chan (chan *serializedUpdate)), h.newSubscribers)
	assert.IsType(t, make(chan (chan *serializedUpdate)), h.removedSubscribers)
	assert.IsType(t, make(chan *serializedUpdate), h.updates)
}

func createDummy() *Hub {
	return NewHub([]byte("publisher"), []byte("subscriber"), false)
}

func createAnonymousDummy() *Hub {
	return NewHub([]byte("publisher"), []byte("subscriber"), true)
}

func createDummyAuthorizedJWT(h *Hub, publisher bool) string {
	var key []byte
	if publisher {
		key = h.publisherJWTKey
	} else {
		key = h.subscriberJWTKey
	}

	token := jwt.New(jwt.SigningMethodHS256)
	tokenString, _ := token.SignedString(key)

	return tokenString
}

func createDummyAuthorizedJWTWithTargets(h *Hub, targets []string) string {
	token := jwt.New(jwt.SigningMethodHS256)
	token.Claims = &claims{targets, jwt.StandardClaims{}}

	tokenString, _ := token.SignedString(h.subscriberJWTKey)

	return tokenString
}

func createDummyUnauthorizedJWT() string {
	token := jwt.New(jwt.SigningMethodHS256)
	tokenString, _ := token.SignedString([]byte("unauthorized"))

	return tokenString
}

func createDummyNoneSignedJWT() string {
	token := jwt.New(jwt.SigningMethodNone)
	// The generated token must have more than 41 chars
	token.Claims = jwt.StandardClaims{Subject: "me"}
	tokenString, _ := token.SignedString(jwt.UnsafeAllowNoneSignatureType)

	return tokenString
}

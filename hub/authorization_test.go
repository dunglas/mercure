package hub

import (
	"net/http"
	"testing"

	"github.com/stretchr/testify/assert"
)

const validEmptyHeader = "eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9.e30.gciTwpaT-wC-s73qBP7OQ_BMp9Oosm8YXvIpqYWddzY"
const validFullHeader = "eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9.eyJtZXJjdXJlIjp7InB1Ymxpc2giOlsiZm9vIiwiYmFyIl0sInN1YnNjcmliZSI6WyJmb28iLCJiYXoiXX19.pylpKeHDlAN2HHBayzufKYc6VqDfhjCuzlH72r1W6Nw"

func TestAuthorizeMultipleAuthorizationHeader(t *testing.T) {
	r, _ := http.NewRequest("GET", "http://example.com", nil)
	r.Header.Add("Authorization", validEmptyHeader)
	r.Header.Add("Authorization", validEmptyHeader)

	_, err := authorize(r, []byte{}, []string{})
	assert.EqualError(t, err, "Invalid \"Authorization\" HTTP header")
}

func TestAuthorizeAuthorizationHeaderTooShort(t *testing.T) {
	r, _ := http.NewRequest("GET", "http://example.com", nil)
	r.Header.Add("Authorization", "Bearer x")

	_, err := authorize(r, []byte{}, []string{})
	assert.EqualError(t, err, "Invalid \"Authorization\" HTTP header")
}

func TestAuthorizeAuthorizationHeaderNoBearer(t *testing.T) {
	r, _ := http.NewRequest("GET", "http://example.com", nil)
	r.Header.Add("Authorization", "Greater "+validEmptyHeader)

	_, err := authorize(r, []byte{}, []string{})
	assert.EqualError(t, err, "Invalid \"Authorization\" HTTP header")
}

func TestAuthorizeAuthorizationHeaderInvalidAlg(t *testing.T) {
	r, _ := http.NewRequest("GET", "http://example.com", nil)
	r.Header.Add("Authorization", "Bearer "+createDummyNoneSignedJWT())

	_, err := authorize(r, []byte{}, []string{})
	t.Log(err)
	assert.EqualError(t, err, "Unexpected signing method: none")
}

func TestAuthorizeAuthorizationHeaderInvalidKey(t *testing.T) {
	r, _ := http.NewRequest("GET", "http://example.com", nil)
	r.Header.Add("Authorization", "Bearer "+validEmptyHeader)

	_, err := authorize(r, []byte{}, []string{})
	assert.EqualError(t, err, "signature is invalid")
}

func TestAuthorizeAuthorizationHeaderNoContent(t *testing.T) {
	r, _ := http.NewRequest("GET", "http://example.com", nil)
	r.Header.Add("Authorization", "Bearer "+validEmptyHeader)

	claims, _ := authorize(r, []byte("!UnsecureChangeMe!"), []string{})
	assert.Nil(t, claims.Mercure.Publish)
	assert.Nil(t, claims.Mercure.Subscribe)
}

func TestAuthorizeAuthorizationHeader(t *testing.T) {
	r, _ := http.NewRequest("GET", "http://example.com", nil)
	r.Header.Add("Authorization", "Bearer "+validFullHeader)

	claims, _ := authorize(r, []byte("!UnsecureChangeMe!"), []string{})
	assert.Equal(t, []string{"foo", "bar"}, claims.Mercure.Publish)
	assert.Equal(t, []string{"foo", "baz"}, claims.Mercure.Subscribe)
}

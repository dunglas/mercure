package hub

import (
	"net/http"
	"testing"

	"github.com/spf13/viper"
	"github.com/stretchr/testify/assert"

	"github.com/dgrijalva/jwt-go"
)

const validEmptyHeader = "eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9.e30._esyynAyo2Z6PyGe0mM_SuQ3c-C7sMQJ1YxVLvlj80A"
const validFullHeader = "eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9.eyJtZXJjdXJlIjp7InB1Ymxpc2giOlsiZm9vIiwiYmFyIl0sInN1YnNjcmliZSI6WyJmb28iLCJiYXoiXX19.e7USPnr2YHHqLYSu9-jEVsynuTXGtAQUDAZuzoR8lxQ"

const validEmptyHeaderRsa = "eyJhbGciOiJSUzI1NiIsInR5cCI6IkpXVCJ9.e30.YbkSeO9GIBYedph1uSQz0Y6zp1NwDEB8O7ek3cc3Vw4Fjh6DwrJAwmXoNSqT6FhHDv14QG70qPIuyzsR0Q9nHFo7hGEqE8E85F8z3Pj5eBjHKBMJFno7jww514Vyp35c490ZHD6_d3F9PmxWrPkKezc1mcwlCegwiMJIS2CeR7k"
const validFullHeaderRsa = "eyJhbGciOiJSUzI1NiIsInR5cCI6IkpXVCJ9.eyJtZXJjdXJlIjp7InB1Ymxpc2giOlsiZm9vIiwiYmFyIl0sInN1YnNjcmliZSI6WyJmb28iLCJiYXoiXX19.B-ZBdmCbtDaP27wB_DvF9xIetQm88M2Q1d-LP2DZoEHrz6lYDuHkgXzSDnFdbLCZ653e0r_VOaKxe2Pc6R4F0ok2vksC6P5gHhqIUcQuTSlzNFyTrg4tyy4mMkcm1h85te9gkV4LR6TABfZpFPqqIS4t7fpCMxvtAkyf_RR5Fq4"

const publicKeyRsa = `-----BEGIN PUBLIC KEY-----
MIGeMA0GCSqGSIb3DQEBAQUAA4GMADCBiAKBgHVwuJsFmzsFnOkGj+OgAp4lTNqR
CF0RZSmjY+ECWOJ3sSEzQ8qtkJe61uSjr/PKmqvBxxex0YtUL7waSS4jvq3ws8Bm
WIxK2GqoAVjLjK8HzThSPQpgv2AjiEXD6iAERHeySLGjYAUgfMrVJ01J5fNSL+O+
bCd7nPuNAyYHCOOHAgMBAAE=
-----END PUBLIC KEY-----
`
const privateKeyRsa = `-----BEGIN RSA PRIVATE KEY-----
MIICWwIBAAKBgHVwuJsFmzsFnOkGj+OgAp4lTNqRCF0RZSmjY+ECWOJ3sSEzQ8qt
kJe61uSjr/PKmqvBxxex0YtUL7waSS4jvq3ws8BmWIxK2GqoAVjLjK8HzThSPQpg
v2AjiEXD6iAERHeySLGjYAUgfMrVJ01J5fNSL+O+bCd7nPuNAyYHCOOHAgMBAAEC
gYAiOKOCgMK4Ey2i9YeOQ70fiiz375UpUX1SAcuD8KQn8crKqt6RO7xLimU+ILiP
6LTjYcb7D5TI7dIvFNXIPSA9tpGbuPqzwa0aBkIoIxJkJ7vs6gHijq3kAQl3mik2
ddzL7OtdlbXG8fRnvgKsRLw2gVlv4+8C3OKmKADJR1bSQQJBAM/xS49IxjUIXMLO
77tsGd+VxpKo1jrUG5Ao9feFfSxWiFnnlDG9DOriDvguPf0WUkU08j7fC3A3AKVd
dQkkqWECQQCQlPSA96lJIUD9xCx7S46L7+e+A2EWnhyMb3u4D1EY5rZdA3/Zzc3P
68Jb8RtRryGuDvezLRcqmVJWq5X97i3nAkADeJ2wSKC2ZetWfSnXURileNSVwifB
V6UWJPjmJt5ODSu9hHYe1m8OxLNHRU5XmTXKXfXlQsfoGaLzH7pCatBBAkBraBzT
iiiaiTeszYV1+sVks85m3D/N+5udwFwaelZ2tz4Wjzj1ZuxUYAI9JzpyTjYpBjmB
RCgHn2sJs+Jzh/NVAkEAnRQKOSQRcm/o4PWNsvrqRwoqUzDcnVcEY67pKPwcnnlR
Ki0jUpg2xzzwyA+nEI6Bf6CDaHKnCqxL7x0yk2XqeA==
-----END RSA PRIVATE KEY-----
`

var hmacSigningMethod = jwt.GetSigningMethod("HS256")
var rsaSigningMethod = jwt.GetSigningMethod("RS256")

func TestAuthorizeMultipleAuthorizationHeader(t *testing.T) {
	r, _ := http.NewRequest("GET", defaultHubURL, nil)
	r.Header.Add("Authorization", validEmptyHeader)
	r.Header.Add("Authorization", validEmptyHeader)

	claims, err := authorize(r, []byte{}, hmacSigningMethod, []string{})
	assert.EqualError(t, err, "invalid \"Authorization\" HTTP header")
	assert.Nil(t, claims)
}

func TestAuthorizeMultipleAuthorizationHeaderRsa(t *testing.T) {
	r, _ := http.NewRequest("GET", defaultHubURL, nil)
	r.Header.Add("Authorization", validEmptyHeaderRsa)
	r.Header.Add("Authorization", validEmptyHeaderRsa)

	claims, err := authorize(r, []byte{}, rsaSigningMethod, []string{})
	assert.EqualError(t, err, "invalid \"Authorization\" HTTP header")
	assert.Nil(t, claims)
}

func TestAuthorizeAuthorizationHeaderTooShort(t *testing.T) {
	r, _ := http.NewRequest("GET", defaultHubURL, nil)
	r.Header.Add("Authorization", "Bearer x")

	claims, err := authorize(r, []byte{}, hmacSigningMethod, []string{})
	assert.EqualError(t, err, "invalid \"Authorization\" HTTP header")
	assert.Nil(t, claims)
}

func TestAuthorizeAuthorizationHeaderNoBearer(t *testing.T) {
	r, _ := http.NewRequest("GET", defaultHubURL, nil)
	r.Header.Add("Authorization", "Greater "+validEmptyHeader)

	claims, err := authorize(r, []byte{}, hmacSigningMethod, []string{})
	assert.EqualError(t, err, "invalid \"Authorization\" HTTP header")
	assert.Nil(t, claims)
}

func TestAuthorizeAuthorizationHeaderNoBearerRsa(t *testing.T) {
	r, _ := http.NewRequest("GET", defaultHubURL, nil)
	r.Header.Add("Authorization", "Greater "+validEmptyHeaderRsa)

	claims, err := authorize(r, []byte{}, rsaSigningMethod, []string{})
	assert.EqualError(t, err, "invalid \"Authorization\" HTTP header")
	assert.Nil(t, claims)
}

func TestAuthorizeAuthorizationHeaderInvalidAlg(t *testing.T) {
	r, _ := http.NewRequest("GET", defaultHubURL, nil)
	r.Header.Add("Authorization", "Bearer "+createDummyNoneSignedJWT())

	claims, err := authorize(r, []byte{}, hmacSigningMethod, []string{})
	assert.EqualError(t, err, "'none' signature type is not allowed")
	assert.Nil(t, claims)
}

func TestAuthorizeAuthorizationHeaderInvalidKey(t *testing.T) {
	r, _ := http.NewRequest("GET", defaultHubURL, nil)
	r.Header.Add("Authorization", "Bearer "+validEmptyHeader)

	claims, err := authorize(r, []byte{}, hmacSigningMethod, []string{})
	assert.EqualError(t, err, "signature is invalid")
	assert.Nil(t, claims)
}

func TestAuthorizeAuthorizationHeaderInvalidKeyRsa(t *testing.T) {
	r, _ := http.NewRequest("GET", defaultHubURL, nil)
	r.Header.Add("Authorization", "Bearer "+validEmptyHeaderRsa)

	claims, err := authorize(r, []byte{}, rsaSigningMethod, []string{})
	assert.EqualError(t, err, "public key error")
	assert.Nil(t, claims)
}

func TestAuthorizeAuthorizationHeaderNoContent(t *testing.T) {
	r, _ := http.NewRequest("GET", defaultHubURL, nil)
	r.Header.Add("Authorization", "Bearer "+validEmptyHeader)

	claims, err := authorize(r, []byte("!ChangeMe!"), hmacSigningMethod, []string{})
	assert.Nil(t, claims.Mercure.Publish)
	assert.Nil(t, claims.Mercure.Subscribe)
	assert.Nil(t, err)
}

func TestAuthorizeAuthorizationHeaderNoContentRsa(t *testing.T) {
	r, _ := http.NewRequest("GET", defaultHubURL, nil)
	r.Header.Add("Authorization", "Bearer "+validEmptyHeaderRsa)

	claims, err := authorize(r, []byte(publicKeyRsa), rsaSigningMethod, []string{})
	assert.Nil(t, claims.Mercure.Publish)
	assert.Nil(t, claims.Mercure.Subscribe)
	assert.Nil(t, err)
}

func TestAuthorizeAuthorizationHeader(t *testing.T) {
	r, _ := http.NewRequest("GET", defaultHubURL, nil)
	r.Header.Add("Authorization", "Bearer "+validFullHeader)

	claims, err := authorize(r, []byte("!ChangeMe!"), hmacSigningMethod, []string{})
	assert.Equal(t, []string{"foo", "bar"}, claims.Mercure.Publish)
	assert.Equal(t, []string{"foo", "baz"}, claims.Mercure.Subscribe)
	assert.Nil(t, err)
}

func TestAuthorizeAuthorizationHeaderRsa(t *testing.T) {
	r, _ := http.NewRequest("GET", defaultHubURL, nil)
	r.Header.Add("Authorization", "Bearer "+validFullHeaderRsa)

	claims, err := authorize(r, []byte(publicKeyRsa), rsaSigningMethod, []string{})
	assert.Equal(t, []string{"foo", "bar"}, claims.Mercure.Publish)
	assert.Equal(t, []string{"foo", "baz"}, claims.Mercure.Subscribe)
	assert.Nil(t, err)
}

func TestAuthorizeAuthorizationHeaderWrongAlgorithm(t *testing.T) {
	r, _ := http.NewRequest("GET", defaultHubURL, nil)
	r.Header.Add("Authorization", "Bearer "+validFullHeaderRsa)

	claims, err := authorize(r, []byte(publicKeyRsa), nil, []string{})
	assert.EqualError(t, err, "unexpected signing method: <nil>")
	assert.Nil(t, claims)
}

func TestAuthorizeCookieInvalidAlg(t *testing.T) {
	r, _ := http.NewRequest("GET", defaultHubURL, nil)
	r.AddCookie(&http.Cookie{Name: "mercureAuthorization", Value: createDummyNoneSignedJWT()})

	claims, err := authorize(r, []byte{}, hmacSigningMethod, []string{})
	assert.EqualError(t, err, "'none' signature type is not allowed")
	assert.Nil(t, claims)
}

func TestAuthorizeCookieInvalidKey(t *testing.T) {
	r, _ := http.NewRequest("GET", defaultHubURL, nil)
	r.AddCookie(&http.Cookie{Name: "mercureAuthorization", Value: validEmptyHeader})

	claims, err := authorize(r, []byte{}, hmacSigningMethod, []string{})
	assert.EqualError(t, err, "signature is invalid")
	assert.Nil(t, claims)
}

func TestAuthorizeCookieEmptyKeyRsa(t *testing.T) {
	r, _ := http.NewRequest("GET", defaultHubURL, nil)
	r.AddCookie(&http.Cookie{Name: "mercureAuthorization", Value: validEmptyHeaderRsa})

	claims, err := authorize(r, []byte{}, rsaSigningMethod, []string{})
	assert.EqualError(t, err, "public key error")
	assert.Nil(t, claims)
}

func TestAuthorizeCookieInvalidKeyRsa(t *testing.T) {
	r, _ := http.NewRequest("GET", defaultHubURL, nil)
	r.AddCookie(&http.Cookie{Name: "mercureAuthorization", Value: validEmptyHeaderRsa})

	claims, err := authorize(r, []byte(privateKeyRsa), rsaSigningMethod, []string{})
	assert.EqualError(t, err, "asn1: structure error: tags don't match (16 vs {class:0 tag:2 length:1 isCompound:false}) {optional:false explicit:false application:false private:false defaultValue:<nil> tag:<nil> stringType:0 timeType:0 set:false omitEmpty:false} AlgorithmIdentifier @2")
	assert.Nil(t, claims)
}

func TestAuthorizeCookieNoContent(t *testing.T) {
	r, _ := http.NewRequest("GET", defaultHubURL, nil)
	r.AddCookie(&http.Cookie{Name: "mercureAuthorization", Value: validEmptyHeader})

	claims, err := authorize(r, []byte("!ChangeMe!"), hmacSigningMethod, []string{})
	assert.Nil(t, claims.Mercure.Publish)
	assert.Nil(t, claims.Mercure.Subscribe)
	assert.Nil(t, err)
}

func TestAuthorizeCookieNoContentRsa(t *testing.T) {
	r, _ := http.NewRequest("GET", defaultHubURL, nil)
	r.AddCookie(&http.Cookie{Name: "mercureAuthorization", Value: validEmptyHeaderRsa})

	claims, err := authorize(r, []byte(publicKeyRsa), rsaSigningMethod, []string{})
	assert.Nil(t, claims.Mercure.Publish)
	assert.Nil(t, claims.Mercure.Subscribe)
	assert.Nil(t, err)
}

func TestAuthorizeCookie(t *testing.T) {
	r, _ := http.NewRequest("GET", defaultHubURL, nil)
	r.AddCookie(&http.Cookie{Name: "mercureAuthorization", Value: validFullHeader})

	claims, err := authorize(r, []byte("!ChangeMe!"), hmacSigningMethod, []string{})
	assert.Equal(t, []string{"foo", "bar"}, claims.Mercure.Publish)
	assert.Equal(t, []string{"foo", "baz"}, claims.Mercure.Subscribe)
	assert.Nil(t, err)
}

func TestAuthorizeCookieRsa(t *testing.T) {
	r, _ := http.NewRequest("GET", defaultHubURL, nil)
	r.AddCookie(&http.Cookie{Name: "mercureAuthorization", Value: validFullHeaderRsa})

	claims, err := authorize(r, []byte(publicKeyRsa), rsaSigningMethod, []string{})
	assert.Equal(t, []string{"foo", "bar"}, claims.Mercure.Publish)
	assert.Equal(t, []string{"foo", "baz"}, claims.Mercure.Subscribe)
	assert.Nil(t, err)
}

func TestAuthorizeCookieNoOriginNoReferer(t *testing.T) {
	r, _ := http.NewRequest("POST", defaultHubURL, nil)
	r.AddCookie(&http.Cookie{Name: "mercureAuthorization", Value: validFullHeader})

	claims, err := authorize(r, []byte("!ChangeMe!"), hmacSigningMethod, []string{})
	assert.EqualError(t, err, "an \"Origin\" or a \"Referer\" HTTP header must be present to use the cookie-based authorization mechanism")
	assert.Nil(t, claims)
}

func TestAuthorizeCookieNoOriginNoRefererRsa(t *testing.T) {
	r, _ := http.NewRequest("POST", defaultHubURL, nil)
	r.AddCookie(&http.Cookie{Name: "mercureAuthorization", Value: validFullHeaderRsa})

	claims, err := authorize(r, []byte(publicKeyRsa), rsaSigningMethod, []string{})
	assert.EqualError(t, err, "an \"Origin\" or a \"Referer\" HTTP header must be present to use the cookie-based authorization mechanism")
	assert.Nil(t, claims)
}

func TestAuthorizeCookieOriginNotAllowed(t *testing.T) {
	r, _ := http.NewRequest("POST", defaultHubURL, nil)
	r.Header.Add("Origin", "http://example.com")
	r.AddCookie(&http.Cookie{Name: "mercureAuthorization", Value: validFullHeader})

	claims, err := authorize(r, []byte("!ChangeMe!"), hmacSigningMethod, []string{"http://example.net"})
	assert.EqualError(t, err, "the origin \"http://example.com\" is not allowed to post updates")
	assert.Nil(t, claims)
}

func TestAuthorizeCookieOriginNotAllowedRsa(t *testing.T) {
	r, _ := http.NewRequest("POST", defaultHubURL, nil)
	r.Header.Add("Origin", "http://example.com")
	r.AddCookie(&http.Cookie{Name: "mercureAuthorization", Value: validFullHeaderRsa})

	claims, err := authorize(r, []byte(publicKeyRsa), rsaSigningMethod, []string{"http://example.net"})
	assert.EqualError(t, err, "the origin \"http://example.com\" is not allowed to post updates")
	assert.Nil(t, claims)
}

func TestAuthorizeCookieRefererNotAllowed(t *testing.T) {
	r, _ := http.NewRequest("POST", defaultHubURL, nil)
	r.Header.Add("Referer", "http://example.com/foo/bar")
	r.AddCookie(&http.Cookie{Name: "mercureAuthorization", Value: validFullHeader})

	claims, err := authorize(r, []byte("!ChangeMe!"), hmacSigningMethod, []string{"http://example.net"})
	assert.EqualError(t, err, "the origin \"http://example.com\" is not allowed to post updates")
	assert.Nil(t, claims)
}

func TestAuthorizeCookieRefererNotAllowedRsa(t *testing.T) {
	r, _ := http.NewRequest("POST", defaultHubURL, nil)
	r.Header.Add("Referer", "http://example.com/foo/bar")
	r.AddCookie(&http.Cookie{Name: "mercureAuthorization", Value: validFullHeaderRsa})

	claims, err := authorize(r, []byte(publicKeyRsa), rsaSigningMethod, []string{"http://example.net"})
	assert.EqualError(t, err, "the origin \"http://example.com\" is not allowed to post updates")
	assert.Nil(t, claims)
}

func TestAuthorizeCookieInvalidReferer(t *testing.T) {
	r, _ := http.NewRequest("POST", defaultHubURL, nil)
	r.Header.Add("Referer", "http://192.168.0.%31/")
	r.AddCookie(&http.Cookie{Name: "mercureAuthorization", Value: validFullHeader})

	claims, err := authorize(r, []byte("!ChangeMe!"), hmacSigningMethod, []string{"http://example.net"})
	assert.EqualError(t, err, "parse http://192.168.0.%31/: invalid URL escape \"%31\"")
	assert.Nil(t, claims)
}

func TestAuthorizeCookieInvalidRefererRsa(t *testing.T) {
	r, _ := http.NewRequest("POST", defaultHubURL, nil)
	r.Header.Add("Referer", "http://192.168.0.%31/")
	r.AddCookie(&http.Cookie{Name: "mercureAuthorization", Value: validFullHeaderRsa})

	claims, err := authorize(r, []byte(publicKeyRsa), rsaSigningMethod, []string{"http://example.net"})
	assert.EqualError(t, err, "parse http://192.168.0.%31/: invalid URL escape \"%31\"")
	assert.Nil(t, claims)
}

func TestAuthorizeCookieOriginHasPriority(t *testing.T) {
	r, _ := http.NewRequest("POST", defaultHubURL, nil)
	r.Header.Add("Origin", "http://example.net")
	r.Header.Add("Referer", "http://example.com")
	r.AddCookie(&http.Cookie{Name: "mercureAuthorization", Value: validFullHeader})

	claims, err := authorize(r, []byte("!ChangeMe!"), hmacSigningMethod, []string{"http://example.net"})
	assert.Equal(t, []string{"foo", "bar"}, claims.Mercure.Publish)
	assert.Equal(t, []string{"foo", "baz"}, claims.Mercure.Subscribe)
	assert.Nil(t, err)
}

func TestAuthorizeCookieOriginHasPriorityRsa(t *testing.T) {
	r, _ := http.NewRequest("POST", defaultHubURL, nil)
	r.Header.Add("Origin", "http://example.net")
	r.Header.Add("Referer", "http://example.com")
	r.AddCookie(&http.Cookie{Name: "mercureAuthorization", Value: validFullHeaderRsa})

	claims, err := authorize(r, []byte(publicKeyRsa), rsaSigningMethod, []string{"http://example.net"})
	assert.Equal(t, []string{"foo", "bar"}, claims.Mercure.Publish)
	assert.Equal(t, []string{"foo", "baz"}, claims.Mercure.Subscribe)
	assert.Nil(t, err)
}

func TestAuthorizedNilClaim(t *testing.T) {
	all, targets := authorizedTargets(nil, true)
	assert.False(t, all)
	assert.Empty(t, targets)
}

func TestAuthorizedTargetsPublisher(t *testing.T) {
	c := &claims{Mercure: mercureClaim{
		Publish: []string{"foo", "bar"},
	}}

	all, targets := authorizedTargets(c, true)
	assert.False(t, all)
	assert.Equal(t, map[string]struct{}{"foo": {}, "bar": {}}, targets)
}

func TestAuthorizedAllTargetsPublisher(t *testing.T) {
	c := &claims{Mercure: mercureClaim{
		Publish: []string{"*"},
	}}

	all, targets := authorizedTargets(c, true)
	assert.True(t, all)
	assert.Empty(t, targets)
}

func TestAuthorizedTargetsSubscriber(t *testing.T) {
	c := &claims{Mercure: mercureClaim{
		Subscribe: []string{"foo", "bar"},
	}}

	all, targets := authorizedTargets(c, false)
	assert.False(t, all)
	assert.Equal(t, map[string]struct{}{"foo": {}, "bar": {}}, targets)
}

func TestAuthorizedAllTargetsSubscriber(t *testing.T) {
	c := &claims{Mercure: mercureClaim{
		Subscribe: []string{"*"},
	}}

	all, targets := authorizedTargets(c, false)
	assert.True(t, all)
	assert.Empty(t, targets)
}

func TestGetJWTKeyInvalid(t *testing.T) {
	v := viper.New()
	h := createDummyWithTransportAndConfig(NewLocalTransport(), v)

	h.config.Set("publisher_jwt_key", "")
	assert.PanicsWithValue(t, "one of these configuration parameters must be defined: [publisher_jwt_key jwt_key]", func() {
		h.getJWTKey(publisherRole)
	})

	h.config.Set("subscriber_jwt_key", "")
	assert.PanicsWithValue(t, "one of these configuration parameters must be defined: [subscriber_jwt_key jwt_key]", func() {
		h.getJWTKey(subscriberRole)
	})
}

func TestGetJWTAlgorithmInvalid(t *testing.T) {
	v := viper.New()
	h := createDummyWithTransportAndConfig(NewLocalTransport(), v)

	h.config.Set("publisher_jwt_algorithm", "foo")
	assert.PanicsWithValue(t, "invalid signing method: foo", func() {
		h.getJWTAlgorithm(publisherRole)
	})

	h.config.Set("subscriber_jwt_algorithm", "foo")
	assert.PanicsWithValue(t, "invalid signing method: foo", func() {
		h.getJWTAlgorithm(subscriberRole)
	})
}

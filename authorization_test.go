package mercure

import (
	"net/http"
	"testing"

	"github.com/form3tech-oss/jwt-go"
	"github.com/stretchr/testify/assert"
)

const (
	validEmptyHeader = "eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9.e30._esyynAyo2Z6PyGe0mM_SuQ3c-C7sMQJ1YxVLvlj80A"
	validFullHeader  = "eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9.eyJtZXJjdXJlIjp7InB1Ymxpc2giOlsiZm9vIiwiYmFyIl0sInN1YnNjcmliZSI6WyJmb28iLCJiYXoiXX19.e7USPnr2YHHqLYSu9-jEVsynuTXGtAQUDAZuzoR8lxQ"
)

const (
	validEmptyHeaderRsa          = "eyJhbGciOiJSUzI1NiIsInR5cCI6IkpXVCJ9.e30.YbkSeO9GIBYedph1uSQz0Y6zp1NwDEB8O7ek3cc3Vw4Fjh6DwrJAwmXoNSqT6FhHDv14QG70qPIuyzsR0Q9nHFo7hGEqE8E85F8z3Pj5eBjHKBMJFno7jww514Vyp35c490ZHD6_d3F9PmxWrPkKezc1mcwlCegwiMJIS2CeR7k"
	validFullHeaderRsa           = "eyJhbGciOiJSUzI1NiIsInR5cCI6IkpXVCJ9.eyJtZXJjdXJlIjp7InB1Ymxpc2giOlsiZm9vIiwiYmFyIl0sInN1YnNjcmliZSI6WyJmb28iLCJiYXoiXX19.B-ZBdmCbtDaP27wB_DvF9xIetQm88M2Q1d-LP2DZoEHrz6lYDuHkgXzSDnFdbLCZ653e0r_VOaKxe2Pc6R4F0ok2vksC6P5gHhqIUcQuTSlzNFyTrg4tyy4mMkcm1h85te9gkV4LR6TABfZpFPqqIS4t7fpCMxvtAkyf_RR5Fq4"
	validFullHeaderRsaForCert    = "eyJhbGciOiJSUzI1NiIsInR5cCI6IkpXVCIsImtpZCI6IkxIbzlPMmNNUzBqbzRsQWwtRk11ayJ9.eyJodHRwczovL21lcmN1cmUucm9ja3MvIjp7InB1Ymxpc2giOlsiZm9vIiwiYmFyIl0sInN1YnNjcmliZSI6WyJmb28iLCJiYXoiXX0sImlzcyI6Imh0dHBzOi8vbWVyY3VyZS10ZXN0LmV1LmF1dGgwLmNvbS8iLCJzdWIiOiJhdXRoMHw1ZmRjM2U4OGUzYjA0YjAwNzZhNTQxM2MiLCJhdWQiOlsiaHR0cHM6Ly9tZXJjdXJlLnJvY2tzL2FwaSIsImh0dHBzOi8vbWVyY3VyZS10ZXN0LmV1LmF1dGgwLmNvbS91c2VyaW5mbyJdLCJpYXQiOjE2MDgyNzAxMTQsImV4cCI6MTYwODM1NjUxNCwiYXpwIjoiMzAxaHptQkEydnlnN2dKVmJIRUxSVEN6WXR1QmtVTnYiLCJzY29wZSI6Im9wZW5pZCBwcm9maWxlIGVtYWlsIn0.WhMkaOvIckY7PFCYs5SvIcl8OK32z7AhCCPGx0G3yF4L0nOTssXV9gAPEpEOrLbCOG3ALDxOGB4VagGnwBIYuztBsuZyPRoIwUkBZOgrIUJYS96jYTb9osPUYZ7BxNlVefFse93JmeSFTZRi6oH5lbqCEW6FUVKNlHWBBl39UK9Fg36EFtOHIJ7wZ_NX51TPvN-roCp27qIhY3atDcHYWXTKS7VjznKDKxl5G7AmyA1L7eE3vpnGiECFcLwxr9BJGVQmnPnwUKf-tY3pSBY0gFE562y15vXk3D2il43uAP4uK2sw8rcup_CmWnT3wmAdIODkwdn8zKM3GC3Y-9WEvQ"
	validFullHeaderNamespacedRsa = "eyJhbGciOiJSUzI1NiIsInR5cCI6IkpXVCJ9.eyJodHRwczovL21lcmN1cmUucm9ja3MvIjp7InB1Ymxpc2giOlsiZm9vIiwiYmFyIl0sInN1YnNjcmliZSI6WyJmb28iLCJiYXoiXX19.SmTVZkxlNbzHlaF4MfA3Fp5d1W2COmHlYPgc6SodAJOQtHh1Uxz0jkhA611w0OSwCaA8C5gqUd-GgekgHVPCBkIzV0qPmmhhJpTtotkeCX3N7oBOJOi58xXouNCNt0vnUH6xACqiZJq_FhNG9ZqP5saa4xNd1E-F1E9Vo1mFji4"
)

const publicKeyRsa = `-----BEGIN PUBLIC KEY-----
MIGeMA0GCSqGSIb3DQEBAQUAA4GMADCBiAKBgHVwuJsFmzsFnOkGj+OgAp4lTNqR
CF0RZSmjY+ECWOJ3sSEzQ8qtkJe61uSjr/PKmqvBxxex0YtUL7waSS4jvq3ws8Bm
WIxK2GqoAVjLjK8HzThSPQpgv2AjiEXD6iAERHeySLGjYAUgfMrVJ01J5fNSL+O+
bCd7nPuNAyYHCOOHAgMBAAE=
-----END PUBLIC KEY-----
`

const certificateRsa = `-----BEGIN CERTIFICATE-----
MIIDDTCCAfWgAwIBAgIJBQgDe2IFUWHIMA0GCSqGSIb3DQEBCwUAMCQxIjAgBgNV
BAMTGW1lcmN1cmUtdGVzdC5ldS5hdXRoMC5jb20wHhcNMjAxMjE4MDUyMjU2WhcN
MzQwODI3MDUyMjU2WjAkMSIwIAYDVQQDExltZXJjdXJlLXRlc3QuZXUuYXV0aDAu
Y29tMIIBIjANBgkqhkiG9w0BAQEFAAOCAQ8AMIIBCgKCAQEAxCm+LIbbfGmIAFbW
2JXm7bNb7B5QOE2gKStBQrFI3lysVRTnJQy+nYOu9NRpDZrsdVMmU9NAbR5eEZqE
oHiSk/gSPFzzRjJBzeEvWHiLzJYrxfp8op/nfQoiXk8RpAazk6ZP4KeCmYYDuZxJ
FhfhbaMkXgde0iIhPugsj5a8z13rxt3hYjyQtQTwrBFoZxG1BaopXYXoMQaUPpFy
UcCu5Imdvs2qWcjFi/hXOrJ2RdXonkuEnZrvYb/XTi1m5OL6byVKc9WLBBm5CviK
lrsy4ogD8tRIFs1DnjcibZHZJQ9QSGimO5AI2OF+8bTnBH/1aWKitaxFE0Ksy89a
pYYMOQIDAQABo0IwQDAPBgNVHRMBAf8EBTADAQH/MB0GA1UdDgQWBBRoVOwyPtFY
L963vd2EZtk0RLX93jAOBgNVHQ8BAf8EBAMCAoQwDQYJKoZIhvcNAQELBQADggEB
AAsmr3cvKCpeTginNyowkOIKRMOJFJ64l2PRxI40K2IGICHmSAemjc5KLlYQ3LpX
MYVyiar/IFtGQRjToGc2Xf4ZyQZHu+JgmkuN0iFfYCISbhfzfioqhoUziJoFZ/eb
g3R1maanbWQ/ElSl6eAB8P7CSifJY/61RdQVcmeqH/8jH6tUtiUu8vIQETxio/hs
iKF3H5HvxIdPWY+nOYwJii8zYPdQ50KBo6nbA41fbhyn35Iu5KXd9odJV1XHSlR3
DYVhT6vD41c2KSy1XMPFLp14wT1nUsH/+vgwcCK+8epw/95JrFuSq3duGzwsPg3p
Qcl5xTqY2Yao8AHNjnR1Uks=
-----END CERTIFICATE-----
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

func TestAuthorizeMultipleAuthorizationHeader(t *testing.T) {
	r, _ := http.NewRequest("GET", defaultHubURL, nil) //nolint:noctx
	r.Header.Add("Authorization", validEmptyHeader)
	r.Header.Add("Authorization", validEmptyHeader)

	claims, err := authorize(r, &jwtConfig{[]byte{}, jwt.SigningMethodHS256}, []string{})
	assert.EqualError(t, err, "invalid \"Authorization\" HTTP header")
	assert.Nil(t, claims)
}

func TestAuthorizeMultipleAuthorizationHeaderRsa(t *testing.T) {
	r, _ := http.NewRequest("GET", defaultHubURL, nil) //nolint:noctx
	r.Header.Add("Authorization", validEmptyHeaderRsa)
	r.Header.Add("Authorization", validEmptyHeaderRsa)

	claims, err := authorize(r, &jwtConfig{[]byte{}, jwt.SigningMethodRS256}, []string{})
	assert.EqualError(t, err, "invalid \"Authorization\" HTTP header")
	assert.Nil(t, claims)
}

func TestAuthorizeAuthorizationHeaderTooShort(t *testing.T) {
	r, _ := http.NewRequest("GET", defaultHubURL, nil) //nolint:noctx
	r.Header.Add("Authorization", "Bearer x")

	claims, err := authorize(r, &jwtConfig{[]byte{}, jwt.SigningMethodHS256}, []string{})
	assert.EqualError(t, err, "invalid \"Authorization\" HTTP header")
	assert.Nil(t, claims)
}

func TestAuthorizeAuthorizationHeaderNoBearer(t *testing.T) {
	r, _ := http.NewRequest("GET", defaultHubURL, nil) //nolint:noctx
	r.Header.Add("Authorization", "Greater "+validEmptyHeader)

	claims, err := authorize(r, &jwtConfig{[]byte{}, jwt.SigningMethodHS256}, []string{})
	assert.EqualError(t, err, "invalid \"Authorization\" HTTP header")
	assert.Nil(t, claims)
}

func TestAuthorizeAuthorizationHeaderNoBearerRsa(t *testing.T) {
	r, _ := http.NewRequest("GET", defaultHubURL, nil) //nolint:noctx
	r.Header.Add("Authorization", "Greater "+validEmptyHeaderRsa)

	claims, err := authorize(r, &jwtConfig{[]byte{}, jwt.SigningMethodRS256}, []string{})
	assert.EqualError(t, err, "invalid \"Authorization\" HTTP header")
	assert.Nil(t, claims)
}

func TestAuthorizeAuthorizationHeaderInvalidAlg(t *testing.T) {
	r, _ := http.NewRequest("GET", defaultHubURL, nil) //nolint:noctx
	r.Header.Add("Authorization", "Bearer "+createDummyNoneSignedJWT())

	claims, err := authorize(r, &jwtConfig{[]byte{}, jwt.SigningMethodHS256}, []string{})
	assert.EqualError(t, err, "unable to parse JWT: 'none' signature type is not allowed")
	assert.Nil(t, claims)
}

func TestAuthorizeAuthorizationHeaderInvalidKey(t *testing.T) {
	r, _ := http.NewRequest("GET", defaultHubURL, nil) //nolint:noctx
	r.Header.Add("Authorization", "Bearer "+validEmptyHeader)

	claims, err := authorize(r, &jwtConfig{[]byte{}, jwt.SigningMethodHS256}, []string{})
	assert.EqualError(t, err, "unable to parse JWT: signature is invalid")
	assert.Nil(t, claims)
}

func TestAuthorizeAuthorizationHeaderInvalidKeyRsa(t *testing.T) {
	r, _ := http.NewRequest("GET", defaultHubURL, nil) //nolint:noctx
	r.Header.Add("Authorization", "Bearer "+validEmptyHeaderRsa)

	claims, err := authorize(r, &jwtConfig{[]byte{}, jwt.SigningMethodRS256}, []string{})
	assert.EqualError(t, err, "unable to parse JWT: unable to parse RSA public key: Invalid Key: Key must be a PEM encoded PKCS1 or PKCS8 key")
	assert.Nil(t, claims)
}

func TestAuthorizeAuthorizationHeaderNoContent(t *testing.T) {
	r, _ := http.NewRequest("GET", defaultHubURL, nil) //nolint:noctx
	r.Header.Add("Authorization", "Bearer "+validEmptyHeader)

	claims, err := authorize(r, &jwtConfig{[]byte("!ChangeMe!"), jwt.SigningMethodHS256}, []string{})
	assert.Nil(t, claims.Mercure.Publish)
	assert.Nil(t, claims.Mercure.Subscribe)
	assert.Nil(t, err)
}

func TestAuthorizeAuthorizationHeaderNoContentRsa(t *testing.T) {
	r, _ := http.NewRequest("GET", defaultHubURL, nil) //nolint:noctx
	r.Header.Add("Authorization", "Bearer "+validEmptyHeaderRsa)

	claims, err := authorize(r, &jwtConfig{[]byte(publicKeyRsa), jwt.SigningMethodRS256}, []string{})
	assert.Nil(t, claims.Mercure.Publish)
	assert.Nil(t, claims.Mercure.Subscribe)
	assert.Nil(t, err)
}

func TestAuthorizeAuthorizationHeader(t *testing.T) {
	r, _ := http.NewRequest("GET", defaultHubURL, nil) //nolint:noctx
	r.Header.Add("Authorization", "Bearer "+validFullHeader)

	claims, err := authorize(r, &jwtConfig{[]byte("!ChangeMe!"), jwt.SigningMethodHS256}, []string{})
	assert.Equal(t, []string{"foo", "bar"}, claims.Mercure.Publish)
	assert.Equal(t, []string{"foo", "baz"}, claims.Mercure.Subscribe)
	assert.Nil(t, err)
}

func TestAuthorizeAuthorizationHeaderRsa(t *testing.T) {
	r, _ := http.NewRequest("GET", defaultHubURL, nil) //nolint:noctx
	r.Header.Add("Authorization", "Bearer "+validFullHeaderRsa)

	claims, err := authorize(r, &jwtConfig{[]byte(publicKeyRsa), jwt.SigningMethodRS256}, []string{})
	assert.Equal(t, []string{"foo", "bar"}, claims.Mercure.Publish)
	assert.Equal(t, []string{"foo", "baz"}, claims.Mercure.Subscribe)
	assert.Nil(t, err)
}

func TestAuthorizeAuthorizationHeaderNamespacedRsa(t *testing.T) {
	r, _ := http.NewRequest("GET", defaultHubURL, nil) //nolint:noctx
	r.Header.Add("Authorization", "Bearer "+validFullHeaderNamespacedRsa)

	claims, err := authorize(r, &jwtConfig{[]byte(publicKeyRsa), jwt.SigningMethodRS256}, []string{})
	assert.Equal(t, []string{"foo", "bar"}, claims.Mercure.Publish)
	assert.Equal(t, []string{"foo", "baz"}, claims.Mercure.Subscribe)
	assert.Nil(t, err)
}

func TestAuthorizeAuthorizationHeaderRsaWithCert(t *testing.T) {
	r, _ := http.NewRequest("GET", defaultHubURL, nil) //nolint:noctx
	r.Header.Add("Authorization", "Bearer "+validFullHeaderRsaForCert)

	claims, err := authorize(r, &jwtConfig{[]byte(certificateRsa), jwt.SigningMethodRS256}, []string{})
	assert.Equal(t, []string{"foo", "bar"}, claims.Mercure.Publish)
	assert.Equal(t, []string{"foo", "baz"}, claims.Mercure.Subscribe)
	assert.Nil(t, err)
}

func TestAuthorizeAuthorizationHeaderWrongAlgorithm(t *testing.T) {
	r, _ := http.NewRequest("GET", defaultHubURL, nil) //nolint:noctx
	r.Header.Add("Authorization", "Bearer "+validFullHeaderRsa)

	claims, err := authorize(r, &jwtConfig{[]byte(publicKeyRsa), nil}, []string{})
	assert.EqualError(t, err, "unable to parse JWT: <nil>: unexpected signing method")
	assert.Nil(t, claims)
}

func TestAuthorizeCookieInvalidAlg(t *testing.T) {
	r, _ := http.NewRequest("GET", defaultHubURL, nil) //nolint:noctx
	r.AddCookie(&http.Cookie{Name: "mercureAuthorization", Value: createDummyNoneSignedJWT()})

	claims, err := authorize(r, &jwtConfig{[]byte{}, jwt.SigningMethodHS256}, []string{})
	assert.EqualError(t, err, "unable to parse JWT: 'none' signature type is not allowed")
	assert.Nil(t, claims)
}

func TestAuthorizeCookieInvalidKey(t *testing.T) {
	r, _ := http.NewRequest("GET", defaultHubURL, nil) //nolint:noctx
	r.AddCookie(&http.Cookie{Name: "mercureAuthorization", Value: validEmptyHeader})

	claims, err := authorize(r, &jwtConfig{[]byte{}, jwt.SigningMethodHS256}, []string{})
	assert.EqualError(t, err, "unable to parse JWT: signature is invalid")
	assert.Nil(t, claims)
}

func TestAuthorizeCookieEmptyKeyRsa(t *testing.T) {
	r, _ := http.NewRequest("GET", defaultHubURL, nil) //nolint:noctx
	r.AddCookie(&http.Cookie{Name: "mercureAuthorization", Value: validEmptyHeaderRsa})

	claims, err := authorize(r, &jwtConfig{[]byte{}, jwt.SigningMethodRS256}, []string{})
	assert.EqualError(t, err, "unable to parse JWT: unable to parse RSA public key: Invalid Key: Key must be a PEM encoded PKCS1 or PKCS8 key")
	assert.Nil(t, claims)
}

func TestAuthorizeCookieInvalidKeyRsa(t *testing.T) {
	r, _ := http.NewRequest("GET", defaultHubURL, nil) //nolint:noctx
	r.AddCookie(&http.Cookie{Name: "mercureAuthorization", Value: validEmptyHeaderRsa})

	claims, err := authorize(r, &jwtConfig{[]byte(privateKeyRsa), jwt.SigningMethodRS256}, []string{})
	assert.EqualError(t, err, "unable to parse JWT: unable to parse RSA public key: asn1: structure error: tags don't match (16 vs {class:0 tag:2 length:1 isCompound:false}) {optional:false explicit:false application:false private:false defaultValue:<nil> tag:<nil> stringType:0 timeType:0 set:false omitEmpty:false} tbsCertificate @2")
	assert.Nil(t, claims)
}

func TestAuthorizeCookieNoContent(t *testing.T) {
	r, _ := http.NewRequest("GET", defaultHubURL, nil) //nolint:noctx
	r.AddCookie(&http.Cookie{Name: "mercureAuthorization", Value: validEmptyHeader})

	claims, err := authorize(r, &jwtConfig{[]byte("!ChangeMe!"), jwt.SigningMethodHS256}, []string{})
	assert.Nil(t, claims.Mercure.Publish)
	assert.Nil(t, claims.Mercure.Subscribe)
	assert.Nil(t, err)
}

func TestAuthorizeCookieNoContentRsa(t *testing.T) {
	r, _ := http.NewRequest("GET", defaultHubURL, nil) //nolint:noctx
	r.AddCookie(&http.Cookie{Name: "mercureAuthorization", Value: validEmptyHeaderRsa})

	claims, err := authorize(r, &jwtConfig{[]byte(publicKeyRsa), jwt.SigningMethodRS256}, []string{})
	assert.Nil(t, claims.Mercure.Publish)
	assert.Nil(t, claims.Mercure.Subscribe)
	assert.Nil(t, err)
}

func TestAuthorizeCookie(t *testing.T) {
	r, _ := http.NewRequest("GET", defaultHubURL, nil) //nolint:noctx
	r.AddCookie(&http.Cookie{Name: "mercureAuthorization", Value: validFullHeader})

	claims, err := authorize(r, &jwtConfig{[]byte("!ChangeMe!"), jwt.SigningMethodHS256}, []string{})
	assert.Equal(t, []string{"foo", "bar"}, claims.Mercure.Publish)
	assert.Equal(t, []string{"foo", "baz"}, claims.Mercure.Subscribe)
	assert.Nil(t, err)
}

func TestAuthorizeCookieRsa(t *testing.T) {
	r, _ := http.NewRequest("GET", defaultHubURL, nil) //nolint:noctx
	r.AddCookie(&http.Cookie{Name: "mercureAuthorization", Value: validFullHeaderRsa})

	claims, err := authorize(r, &jwtConfig{[]byte(publicKeyRsa), jwt.SigningMethodRS256}, []string{})
	assert.Equal(t, []string{"foo", "bar"}, claims.Mercure.Publish)
	assert.Equal(t, []string{"foo", "baz"}, claims.Mercure.Subscribe)
	assert.Nil(t, err)
}

func TestAuthorizeCookieNoOriginNoReferer(t *testing.T) {
	r, _ := http.NewRequest("POST", defaultHubURL, nil) //nolint:noctx
	r.AddCookie(&http.Cookie{Name: "mercureAuthorization", Value: validFullHeader})

	claims, err := authorize(r, &jwtConfig{[]byte("!ChangeMe!"), jwt.SigningMethodHS256}, []string{})
	assert.EqualError(t, err, "an \"Origin\" or a \"Referer\" HTTP header must be present to use the cookie-based authorization mechanism")
	assert.Nil(t, claims)
}

func TestAuthorizeCookieNoOriginNoRefererRsa(t *testing.T) {
	r, _ := http.NewRequest("POST", defaultHubURL, nil) //nolint:noctx
	r.AddCookie(&http.Cookie{Name: "mercureAuthorization", Value: validFullHeaderRsa})

	claims, err := authorize(r, &jwtConfig{[]byte(publicKeyRsa), jwt.SigningMethodRS256}, []string{})
	assert.EqualError(t, err, "an \"Origin\" or a \"Referer\" HTTP header must be present to use the cookie-based authorization mechanism")
	assert.Nil(t, claims)
}

func TestAuthorizeCookieOriginNotAllowed(t *testing.T) {
	r, _ := http.NewRequest("POST", defaultHubURL, nil) //nolint:noctx
	r.Header.Add("Origin", "http://example.com")
	r.AddCookie(&http.Cookie{Name: "mercureAuthorization", Value: validFullHeader})

	claims, err := authorize(r, &jwtConfig{[]byte("!ChangeMe!"), jwt.SigningMethodHS256}, []string{"http://example.net"})
	assert.EqualError(t, err, `"http://example.com": origin not allowed to post updates`)
	assert.Nil(t, claims)
}

func TestAuthorizeCookieOriginNotAllowedRsa(t *testing.T) {
	r, _ := http.NewRequest("POST", defaultHubURL, nil) //nolint:noctx
	r.Header.Add("Origin", "http://example.com")
	r.AddCookie(&http.Cookie{Name: "mercureAuthorization", Value: validFullHeaderRsa})

	claims, err := authorize(r, &jwtConfig{[]byte(publicKeyRsa), jwt.SigningMethodRS256}, []string{"http://example.net"})
	assert.EqualError(t, err, `"http://example.com": origin not allowed to post updates`)
	assert.Nil(t, claims)
}

func TestAuthorizeCookieRefererNotAllowed(t *testing.T) {
	r, _ := http.NewRequest("POST", defaultHubURL, nil) //nolint:noctx
	r.Header.Add("Referer", "http://example.com/foo/bar")
	r.AddCookie(&http.Cookie{Name: "mercureAuthorization", Value: validFullHeader})

	claims, err := authorize(r, &jwtConfig{[]byte("!ChangeMe!"), jwt.SigningMethodHS256}, []string{"http://example.net"})
	assert.EqualError(t, err, `"http://example.com": origin not allowed to post updates`)
	assert.Nil(t, claims)
}

func TestAuthorizeCookieRefererNotAllowedRsa(t *testing.T) {
	r, _ := http.NewRequest("POST", defaultHubURL, nil) //nolint:noctx
	r.Header.Add("Referer", "http://example.com/foo/bar")
	r.AddCookie(&http.Cookie{Name: "mercureAuthorization", Value: validFullHeaderRsa})

	claims, err := authorize(r, &jwtConfig{[]byte(publicKeyRsa), jwt.SigningMethodRS256}, []string{"http://example.net"})
	assert.EqualError(t, err, `"http://example.com": origin not allowed to post updates`)
	assert.Nil(t, claims)
}

func TestAuthorizeCookieInvalidReferer(t *testing.T) {
	r, _ := http.NewRequest("POST", defaultHubURL, nil) //nolint:noctx
	r.Header.Add("Referer", "http://192.168.0.%31/")
	r.AddCookie(&http.Cookie{Name: "mercureAuthorization", Value: validFullHeader})

	claims, err := authorize(r, &jwtConfig{[]byte("!ChangeMe!"), jwt.SigningMethodHS256}, []string{"http://example.net"})
	assert.EqualError(t, err, `unable to parse referer: parse "http://192.168.0.%31/": invalid URL escape "%31"`)
	assert.Nil(t, claims)
}

func TestAuthorizeCookieInvalidRefererRsa(t *testing.T) {
	r, _ := http.NewRequest("POST", defaultHubURL, nil) //nolint:noctx
	r.Header.Add("Referer", "http://192.168.0.%31/")
	r.AddCookie(&http.Cookie{Name: "mercureAuthorization", Value: validFullHeaderRsa})

	claims, err := authorize(r, &jwtConfig{[]byte(publicKeyRsa), jwt.SigningMethodRS256}, []string{"http://example.net"})
	assert.EqualError(t, err, `unable to parse referer: parse "http://192.168.0.%31/": invalid URL escape "%31"`)
	assert.Nil(t, claims)
}

func TestAuthorizeCookieOriginHasPriority(t *testing.T) {
	r, _ := http.NewRequest("POST", defaultHubURL, nil) //nolint:noctx
	r.Header.Add("Origin", "http://example.net")
	r.Header.Add("Referer", "http://example.com")
	r.AddCookie(&http.Cookie{Name: "mercureAuthorization", Value: validFullHeader})

	claims, err := authorize(r, &jwtConfig{[]byte("!ChangeMe!"), jwt.SigningMethodHS256}, []string{"http://example.net"})
	assert.Equal(t, []string{"foo", "bar"}, claims.Mercure.Publish)
	assert.Equal(t, []string{"foo", "baz"}, claims.Mercure.Subscribe)
	assert.Nil(t, err)
}

func TestAuthorizeCookieOriginHasPriorityRsa(t *testing.T) {
	r, _ := http.NewRequest("POST", defaultHubURL, nil) //nolint:noctx
	r.Header.Add("Origin", "http://example.net")
	r.Header.Add("Referer", "http://example.com")
	r.AddCookie(&http.Cookie{Name: "mercureAuthorization", Value: validFullHeaderRsa})

	claims, err := authorize(r, &jwtConfig{[]byte(publicKeyRsa), jwt.SigningMethodRS256}, []string{"http://example.net"})
	assert.Equal(t, []string{"foo", "bar"}, claims.Mercure.Publish)
	assert.Equal(t, []string{"foo", "baz"}, claims.Mercure.Subscribe)
	assert.Nil(t, err)
}

func TestAuthorizeAllOriginsAllowed(t *testing.T) {
	r, _ := http.NewRequest("POST", defaultHubURL, nil) //nolint:noctx
	r.Header.Add("Origin", "http://example.com")
	r.AddCookie(&http.Cookie{Name: "mercureAuthorization", Value: validFullHeader})

	_, err := authorize(r, &jwtConfig{[]byte("!ChangeMe!"), jwt.SigningMethodHS256}, []string{"*"})
	assert.Nil(t, err)
}

func TestCanReceive(t *testing.T) {
	s := NewTopicSelectorStore()
	assert.True(t, canReceive(s, []string{"foo", "bar"}, []string{"foo", "bar"}, true))
	assert.True(t, canReceive(s, []string{"foo", "bar"}, []string{"bar"}, true))
	assert.True(t, canReceive(s, []string{"foo", "bar"}, []string{"*"}, true))
	assert.False(t, canReceive(s, []string{"foo", "bar"}, []string{}, true))
	assert.False(t, canReceive(s, []string{"foo", "bar"}, []string{"baz"}, true))
	assert.False(t, canReceive(s, []string{"foo", "bar"}, []string{"baz", "bat"}, true))
}

func TestCanDispatch(t *testing.T) {
	s := NewTopicSelectorStore()
	assert.True(t, canDispatch(s, []string{"foo", "bar"}, []string{"foo", "bar"}))
	assert.True(t, canDispatch(s, []string{"foo", "bar"}, []string{"*"}))
	assert.False(t, canDispatch(s, []string{"foo", "bar"}, []string{}))
	assert.False(t, canDispatch(s, []string{"foo", "bar"}, []string{"foo"}))
	assert.False(t, canDispatch(s, []string{"foo", "bar"}, []string{"baz"}))
	assert.False(t, canDispatch(s, []string{"foo", "bar"}, []string{"baz", "bat"}))
}

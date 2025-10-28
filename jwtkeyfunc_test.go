package mercure

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestCreateJWTKeyfunc(t *testing.T) {
	t.Parallel()

	f, err := createJWTKeyfunc([]byte{}, "invalid")
	require.Error(t, err)
	require.Nil(t, f)
}

func TestAuthorizeAuthorizationHeaderEmptyKeyRsa(t *testing.T) {
	t.Parallel()

	keyfunc, err := createJWTKeyfunc([]byte{}, "RS256")
	require.EqualError(t, err, "unable to parse RSA public key: invalid key: Key must be a PEM encoded PKCS1 or PKCS8 key")
	require.Nil(t, keyfunc)
}

func TestAuthorizeAuthorizationHeaderInvalidKeyRsa(t *testing.T) {
	t.Parallel()

	keyfunc, err := createJWTKeyfunc([]byte(`-----BEGIN RSA PRIVATE KEY-----
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
`), "RS256")
	require.EqualError(t, err, "unable to parse RSA public key: asn1: structure error: integer too large")
	require.Nil(t, keyfunc)
}

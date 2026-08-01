package caddy

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"strings"
	"time"

	caddycmd "github.com/caddyserver/caddy/v2/cmd"
	"github.com/gofrs/uuid/v5"
	"github.com/golang-jwt/jwt/v5"
	"github.com/spf13/cobra"
)

// devIssuer, devAudience, and devExp match the example token embedded in the
// quickstart guide (docs/getting-started/quickstart.md): the hub's default
// trusted issuer, its derived resource identifier when SERVER_NAME defaults
// to localhost, and a year-2100 expiry so the token never needs regenerating
// during a local session.
const (
	devIssuer      = "https://localhost"
	devAudience    = "https://localhost/.well-known/mercure"
	devExp         = 4102444800
	devKeyEnv      = "MERCURE_PUBLISHER_JWT_KEY"
	devKeyFallback = "!ChangeThisMercureHubJWTSecretKey!"
)

// RFC 9068 claim names and the JWT access token typ value, shared between the
// flag names (deliberately named after the claim they fill) and the claims
// map, so each string literal has exactly one home.
const (
	claimIss                  = "iss"
	claimAud                  = "aud"
	claimSub                  = "sub"
	claimClientID             = "client_id"
	claimIat                  = "iat"
	claimExp                  = "exp"
	claimJTI                  = "jti"
	claimAuthorizationDetails = "authorization_details"
	accessTokenType           = "at+jwt"
)

var errMissingTokenFlag = errors.New("missing required flag")

func init() { //nolint:gochecknoinits
	caddycmd.RegisterCommand(caddycmd.Command{
		Name:  "mercure-token",
		Usage: "[--dev] --iss <issuer> --aud <audience> --key <secret-or-@file> [--publish <topic>]... [--subscribe <topic>]...",
		Short: "Mint a self-issued Mercure access token",
		Long: `
Mints an RFC 9068 JWT access token carrying an RFC 9396 authorization_details
claim, signed with the given key, and prints the compact JWS to stdout.

--publish and --subscribe accept an exact topic and may be repeated;
--publish-urlpattern and --subscribe-urlpattern accept a WHATWG URL Pattern
and may also be repeated. These mirror the "match" and "match_urlpattern"
subscribe query parameters (see docs/concepts/topics-and-matchers.md): the
grant on the token uses the same matcher vocabulary as the subscription URL
it authorizes.

--key accepts a raw HMAC secret, an @-prefixed path to a file holding one,
@- to read it from stdin, or a PEM-encoded private key (inline, from a file,
or from stdin). A PEM key requires --alg (e.g. ES256, RS256, EdDSA); a raw
secret defaults to HS256. This is the signing key, not the hub's
verification key: for HMAC they are the same secret, but for an asymmetric
issuer, issuer.<publisher|subscriber>.jwt in the Caddyfile holds the public
key, and this command needs the matching private key.

--dev fills in --iss, --aud, and --key (from ` + devKeyEnv + `, falling back
to the quickstart's default secret) to match the hub started by the quickstart's
"docker run", and grants --publish '*' --subscribe '*' if no grant flag is
given. Use it for local development only.

Example:

  caddy mercure-token --dev

  caddy mercure-token \
    --iss https://example.com --aud https://hub.example.com/.well-known/mercure \
    --key @publisher.pem --alg ES256 \
    --publish https://example.com/books/1 \
    --subscribe-urlpattern 'https://example.com/books/:id'
`,
		CobraFunc: func(cmd *cobra.Command) {
			cmd.Flags().Bool("dev", false, "fill --iss, --aud, and --key for the quickstart's local dev hub")
			cmd.Flags().String(claimIss, "", "issuer identifier (the token `iss` claim; must match a trusted issuer on the hub)")
			cmd.Flags().String(claimAud, "", "hub resource identifier (the token `aud` claim; the hub's canonical URL)")
			cmd.Flags().String("key", "", "signing key: a raw HMAC secret, @path/to/file, @- for stdin, or a PEM-encoded private key")
			cmd.Flags().String("alg", "", "signing algorithm (default HS256 for a raw secret; required for a PEM key)")
			cmd.Flags().String("kid", "", "optional `kid` header, a hint for hubs trusting more than one key per issuer")
			cmd.Flags().String(claimSub, "", "subscriber/publisher identifier (default: a random urn:uuid)")
			cmd.Flags().String("client-id", "", "RFC 9068 client_id claim (default: same as --iss)")
			cmd.Flags().Duration("ttl", time.Hour, "token lifetime (SHOULD be short-lived; ignored with --dev)")
			cmd.Flags().String("payload", "", "JSON object attached to the subscribe grant (see docs/concepts/authorization.md#subscriber-payloads)")
			cmd.Flags().Bool("pretty", false, "also print the decoded header and claims to stderr")
			cmd.Flags().StringArray("publish", nil, "exact topic to grant publish on (repeatable)")
			cmd.Flags().StringArray("publish-urlpattern", nil, "URL Pattern topic to grant publish on (repeatable)")
			cmd.Flags().StringArray("subscribe", nil, "exact topic to grant subscribe on (repeatable)")
			cmd.Flags().StringArray("subscribe-urlpattern", nil, "URL Pattern topic to grant subscribe on (repeatable)")
			cmd.RunE = caddycmd.WrapCommandFuncForCobra(cmdMercureToken)
		},
	})
}

// topicMatcher and authDetail mirror the wire shape of an authorization_details
// entry (see authorizationdetails.go); they're redeclared here rather than
// reusing the package-private mercure types, since this command only ever
// needs to marshal them, never validate or match them — the hub does that.
type topicMatcher struct {
	Match     string `json:"match"`
	MatchType string `json:"match_type,omitempty"`
}

type authDetail struct {
	Type    string         `json:"type"`
	Actions []string       `json:"actions"`
	Topics  []topicMatcher `json:"topics"`
	Payload any            `json:"payload,omitempty"`
}

const authorizationDetailType = "https://mercure.rocks/authorization-detail"

// tokenParams is cmdMercureToken's parsed and defaulted flags. Keeping it as
// one struct, rather than reading flags ad hoc throughout the function,
// keeps applyDevDefaults' job (filling gaps before validation) obviously
// separate from validation and token assembly.
type tokenParams struct {
	iss, aud, key, alg, kid string
	sub, clientID           string
	ttl                     time.Duration
	// exp, when non-zero, pins the expiry (Unix seconds) instead of deriving it
	// from ttl/dev. The quickstart's example token uses it for a long-lived
	// token that never expires mid-session; the command leaves it zero.
	exp                     int64
	dev, pretty             bool
	payload                 string
	publish, publishURL     []string
	subscribe, subscribeURL []string
}

func readTokenParams(fl caddycmd.Flags) (tokenParams, error) {
	var p tokenParams

	p.dev = fl.Bool("dev")
	p.iss = fl.String(claimIss)
	p.aud = fl.String(claimAud)
	p.key = fl.String("key")
	p.alg = fl.String("alg")
	p.kid = fl.String("kid")
	p.sub = fl.String(claimSub)
	p.clientID = fl.String("client-id")
	p.ttl = fl.Duration("ttl")
	p.payload = fl.String("payload")
	p.pretty = fl.Bool("pretty")

	var err error

	if p.publish, err = fl.GetStringArray("publish"); err != nil {
		return p, fmt.Errorf("reading --publish: %w", err)
	}

	if p.publishURL, err = fl.GetStringArray("publish-urlpattern"); err != nil {
		return p, fmt.Errorf("reading --publish-urlpattern: %w", err)
	}

	if p.subscribe, err = fl.GetStringArray("subscribe"); err != nil {
		return p, fmt.Errorf("reading --subscribe: %w", err)
	}

	if p.subscribeURL, err = fl.GetStringArray("subscribe-urlpattern"); err != nil {
		return p, fmt.Errorf("reading --subscribe-urlpattern: %w", err)
	}

	return p, nil
}

func cmdMercureToken(fl caddycmd.Flags) (int, error) {
	p, err := readTokenParams(fl)
	if err != nil {
		return 1, err
	}

	if p.dev {
		p.applyDevDefaults()
	}

	if p.iss == "" || p.aud == "" || p.key == "" {
		return 1, fmt.Errorf("%w: --iss, --aud, and --key are required (or pass --dev)", errMissingTokenFlag)
	}

	signed, claims, alg, err := mint(p)
	if err != nil {
		return 1, err
	}

	fmt.Fprintln(os.Stdout, signed)

	if p.pretty {
		if err := printPretty(alg, claims); err != nil {
			return 1, err
		}
	}

	return 0, nil
}

// mint assembles and signs the RFC 9068 access token described by p, returning
// the compact JWS plus the claim set and algorithm (for optional display). It
// is the shared core of the mercure-token command and the hub's insecure
// playground token; callers validate that iss, aud and key are set before calling.
func mint(p tokenParams) (signed string, claims jwt.MapClaims, alg string, err error) {
	details, err := p.buildAuthorizationDetails()
	if err != nil {
		return "", nil, "", err
	}

	if len(details) == 0 {
		return "", nil, "", fmt.Errorf("%w: at least one of --publish, --publish-urlpattern, --subscribe, --subscribe-urlpattern is required", errMissingTokenFlag)
	}

	now := time.Now()

	exp := p.exp
	if exp == 0 {
		exp = now.Add(p.ttl).Unix()
		if p.dev {
			exp = devExp
		}
	}

	claims = p.buildClaims(details, now, exp)

	signed, alg, err = signToken(claims, p.kid, p.key, p.alg)
	if err != nil {
		return "", nil, "", err
	}

	return signed, claims, alg, nil
}

func printPretty(alg string, claims jwt.MapClaims) error {
	pretty, err := json.MarshalIndent(claims, "", "  ")
	if err != nil {
		return fmt.Errorf("rendering --pretty output: %w", err)
	}

	fmt.Fprintf(os.Stderr, "header: {\"alg\":%q,\"typ\":%q}\nclaims: %s\n", alg, accessTokenType, pretty)

	return nil
}

// buildClaims assembles the RFC 9068 claim set. sub, client_id, iat, and jti
// are always populated, even though the hub itself only enforces iss, aud,
// and exp: RFC 9068 requires issuers to populate the rest, and this command
// aims to produce tokens any conforming validator accepts, not just this hub.
func (p *tokenParams) buildClaims(details []authDetail, now time.Time, exp int64) jwt.MapClaims {
	sub := p.sub
	if sub == "" {
		sub = "urn:uuid:" + uuid.Must(uuid.NewV4()).String()
	}

	clientID := p.clientID
	if clientID == "" {
		clientID = p.iss
	}

	return jwt.MapClaims{
		claimIss:                  p.iss,
		claimAud:                  p.aud,
		claimSub:                  sub,
		claimClientID:             clientID,
		claimIat:                  now.Unix(),
		claimExp:                  exp,
		claimJTI:                  "urn:uuid:" + uuid.Must(uuid.NewV4()).String(),
		claimAuthorizationDetails: details,
	}
}

// applyDevDefaults fills unset flags to match the hub the quickstart guide's
// "docker run" starts: the same issuer, audience, and secret, so a token
// minted here is accepted with zero configuration. It never overrides a flag
// the user actually passed.
func (p *tokenParams) applyDevDefaults() {
	if p.iss == "" {
		p.iss = devIssuer
	}

	if p.aud == "" {
		p.aud = devAudience
	}

	if p.key == "" {
		if k := os.Getenv(devKeyEnv); k != "" {
			p.key = k
		} else {
			p.key = devKeyFallback
		}
	}

	if len(p.publish) == 0 && len(p.publishURL) == 0 && len(p.subscribe) == 0 && len(p.subscribeURL) == 0 {
		p.publish = []string{"*"}
		p.subscribe = []string{"*"}
	}
}

func (p *tokenParams) buildAuthorizationDetails() ([]authDetail, error) {
	var payload any

	if p.payload != "" {
		if err := json.Unmarshal([]byte(p.payload), &payload); err != nil {
			return nil, fmt.Errorf("--payload is not valid JSON: %w", err)
		}
	}

	var details []authDetail

	if pub := matchers(p.publish, p.publishURL); len(pub) > 0 {
		details = append(details, authDetail{
			Type:    authorizationDetailType,
			Actions: []string{"publish"},
			Topics:  pub,
		})
	}

	if sub := matchers(p.subscribe, p.subscribeURL); len(sub) > 0 {
		details = append(details, authDetail{
			Type:    authorizationDetailType,
			Actions: []string{"subscribe"},
			Topics:  sub,
			Payload: payload,
		})
	}

	return details, nil
}

// matchers merges an --X (exact) and --X-urlpattern flag pair into the
// topics array of one authorization detail, tagging each entry with its
// matcher type so the two flag families can share a single grant.
func matchers(exact, urlPattern []string) []topicMatcher {
	out := make([]topicMatcher, 0, len(exact)+len(urlPattern))

	for _, m := range exact {
		out = append(out, topicMatcher{Match: m})
	}

	for _, m := range urlPattern {
		out = append(out, topicMatcher{Match: m, MatchType: "urlpattern"})
	}

	return out
}

var (
	errPEMKeyNeedsAlg       = errors.New("a PEM-encoded key requires --alg to be set explicitly (for example ES256, RS256, or EdDSA)")
	errUnexpectedSigningAlg = errors.New("unsupported signing algorithm")
)

// readKeyMaterial resolves --key: a literal secret, an @-prefixed file path,
// or @- for stdin. Reading from a file or stdin trims one trailing newline,
// since `echo secret > file` and heredocs commonly add one and a raw HMAC
// secret has no delimiter of its own to separate it from that newline. A PEM
// key is unaffected: encoding/pem already tolerates surrounding whitespace.
func readKeyMaterial(raw string) ([]byte, error) {
	after, ok := strings.CutPrefix(raw, "@")
	if !ok {
		return []byte(raw), nil
	}

	var (
		content []byte
		err     error
	)

	if after == "-" {
		content, err = io.ReadAll(os.Stdin)
		if err != nil {
			return nil, fmt.Errorf("reading --key from stdin: %w", err)
		}
	} else {
		content, err = os.ReadFile(after)
		if err != nil {
			return nil, fmt.Errorf("reading --key file: %w", err)
		}
	}

	content = bytes.TrimSuffix(content, []byte("\n"))
	content = bytes.TrimSuffix(content, []byte("\r"))

	return content, nil
}

// signToken resolves --key (a literal secret, an @-prefixed file reference,
// or inline PEM) and --alg, signs claims, and returns the compact JWS and the
// algorithm used. It mirrors the PEM-detection and default-HS256 rules the
// hub itself applies to a verifier's static key (see normalizeJWT and
// createJWTKeyfunc in mercure.go and jwtkeyfunc.go) — but parses a *private*
// key, since this command signs rather than verifies. The signing method is
// never returned on its own (only bundled into the final JWS string), so it
// doesn't leak jwt.SigningMethod, a third-party interface, across a function
// boundary.
func signToken(claims jwt.MapClaims, kid, raw, alg string) (string, string, error) {
	material, err := readKeyMaterial(raw)
	if err != nil {
		return "", "", err
	}

	if strings.HasPrefix(strings.TrimSpace(string(material)), pemPrefix) {
		if alg == "" {
			return "", "", errPEMKeyNeedsAlg
		}

		if strings.HasPrefix(alg, "HS") {
			return "", "", errPEMKeyHMACAlgorithm
		}
	} else if alg == "" {
		alg = defaultJWTAlgorithm
	}

	method := jwt.GetSigningMethod(alg)
	if method == nil {
		return "", "", fmt.Errorf("%w: %q", jwt.ErrHashUnavailable, alg)
	}

	key, err := signingKey(method, material)
	if err != nil {
		return "", "", err
	}

	token := jwt.NewWithClaims(method, claims)
	token.Header["typ"] = accessTokenType

	if kid != "" {
		token.Header["kid"] = kid
	}

	signed, err := token.SignedString(key)
	if err != nil {
		return "", "", fmt.Errorf("signing token: %w", err)
	}

	return signed, alg, nil
}

// signingKey parses material into the private key type golang-jwt expects
// for method: the raw secret itself for HMAC, or a parsed PEM private key for
// every asymmetric family this command's --alg allowlist can select.
func signingKey(method jwt.SigningMethod, material []byte) (any, error) {
	switch method.(type) {
	case *jwt.SigningMethodHMAC:
		return material, nil
	case *jwt.SigningMethodRSA, *jwt.SigningMethodRSAPSS:
		key, err := jwt.ParseRSAPrivateKeyFromPEM(material)
		if err != nil {
			return nil, fmt.Errorf("parsing RSA private key: %w", err)
		}

		return key, nil
	case *jwt.SigningMethodECDSA:
		key, err := jwt.ParseECPrivateKeyFromPEM(material)
		if err != nil {
			return nil, fmt.Errorf("parsing EC private key: %w", err)
		}

		return key, nil
	case *jwt.SigningMethodEd25519:
		key, err := jwt.ParseEdPrivateKeyFromPEM(material)
		if err != nil {
			return nil, fmt.Errorf("parsing Ed private key: %w", err)
		}

		return key, nil
	default:
		return nil, fmt.Errorf("%T: %w", method, errUnexpectedSigningAlg)
	}
}

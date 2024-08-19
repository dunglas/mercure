package caddy

import (
	"bytes"
	"encoding/gob"
	"strconv"

	"github.com/caddyserver/caddy/v2"
	"github.com/caddyserver/caddy/v2/caddyconfig/caddyfile"
	"github.com/dunglas/mercure"
)

func init() { //nolint:gochecknoinits
	caddy.RegisterModule(Bolt{})
}

type Bolt struct {
	Path             string  `json:"path,omitempty"`
	BucketName       string  `json:"bucket_name,omitempty"`
	Size             uint64  `json:"size,omitempty"`
	CleanupFrequency float64 `json:"cleanup_frequency,omitempty"`

	transport    *mercure.BoltTransport
	transportKey string
}

// CaddyModule returns the Caddy module information.
func (Bolt) CaddyModule() caddy.ModuleInfo {
	return caddy.ModuleInfo{
		ID:  "http.handlers.mercure.bolt",
		New: func() caddy.Module { return new(Bolt) },
	}
}

func (b *Bolt) GetTransport() mercure.Transport { //nolint:ireturn
	return b.transport
}

// Provision provisions b's configuration.
//
//nolint:wrapcheck
func (b *Bolt) Provision(ctx caddy.Context) error {
	var key bytes.Buffer
	if err := gob.NewEncoder(&key).Encode(b); err != nil {
		return err
	}
	b.transportKey = key.String()

	destructor, _, err := transport.LoadOrNew(b.transportKey, func() (caddy.Destructor, error) {
		t, err := mercure.NewBoltTransport(ctx.Logger(), b.Path, b.BucketName, b.Size, b.CleanupFrequency)
		if err != nil {
			return nil, err
		}

		return transportDestructor[*mercure.BoltTransport]{transport: t}, nil
	})
	if err != nil {
		return err
	}

	b.transport = destructor.(transportDestructor[*mercure.BoltTransport]).transport

	return nil
}

//nolint:wrapcheck
func (b *Bolt) Cleanup() error {
	_, err := transport.Delete(b.transportKey)

	return err
}

// UnmarshalCaddyfile sets up the handler from Caddyfile tokens.
//
//nolint:wrapcheck
func (b *Bolt) UnmarshalCaddyfile(d *caddyfile.Dispenser) error {
	for d.Next() {
		for d.NextBlock(0) {
			switch d.Val() {
			case "path":
				if !d.NextArg() {
					return d.ArgErr()
				}

				b.Path = d.Val()

			case "bucket_name":
				if !d.NextArg() {
					return d.ArgErr()
				}

				b.BucketName = d.Val()

			case "cleanup_frequency":
				if !d.NextArg() {
					return d.ArgErr()
				}

				f, e := strconv.ParseFloat(d.Val(), 64)
				if e != nil {
					return e
				}

				b.CleanupFrequency = f

			case "size":
				if !d.NextArg() {
					return d.ArgErr()
				}

				s, e := strconv.ParseUint(d.Val(), 10, 64)
				if e != nil {
					return e
				}

				b.Size = s
			}
		}
	}

	return nil
}

var (
	_ caddy.Provisioner     = (*Bolt)(nil)
	_ caddy.CleanerUpper    = (*Bolt)(nil)
	_ caddyfile.Unmarshaler = (*Bolt)(nil)
)

package caddy

import (
	"bytes"
	"encoding/gob"
	"strconv"

	"github.com/caddyserver/caddy/v2"
	"github.com/caddyserver/caddy/v2/caddyconfig/caddyfile"
	"github.com/dunglas/mercure"
)

func init() {
	caddy.RegisterModule(Redis{})
}

type Redis struct {
	address            string
	username           string
	password           string
	subscribersSize    int
	dispatcherPoolSize int
	redisChannel       string

	transport    *mercure.RedisTransport
	transportKey string
}

// CaddyModule returns the Caddy module information.
func (Redis) CaddyModule() caddy.ModuleInfo {
	return caddy.ModuleInfo{
		ID:  "http.handlers.mercure.redis",
		New: func() caddy.Module { return new(Redis) },
	}
}

func (r *Redis) GetTransport() mercure.Transport {
	return r.transport
}

// Provision provisions redis configuration.
func (r *Redis) Provision(ctx caddy.Context) error {
	var key bytes.Buffer
	if err := gob.NewEncoder(&key).Encode(r); err != nil {
		return err
	}
	r.transportKey = key.String()

	destructor, _, err := TransportUsagePool.LoadOrNew(r.transportKey, func() (caddy.Destructor, error) {
		t, err := mercure.NewRedisTransport(ctx.Logger(), r.address, r.username, r.password, r.subscribersSize, r.dispatcherPoolSize, r.redisChannel)
		if err != nil {
			return nil, err
		}

		return TransportDestructor[*mercure.RedisTransport]{Transport: t}, nil
	})
	if err != nil {
		return err
	}

	r.transport = destructor.(TransportDestructor[*mercure.RedisTransport]).Transport

	return nil
}

func (r *Redis) Cleanup() error {
	_, err := TransportUsagePool.Delete(r.transportKey)

	return err
}

// UnmarshalCaddyfile sets up the handler from Caddyfile tokens.
func (r *Redis) UnmarshalCaddyfile(d *caddyfile.Dispenser) error {
	replacer := caddy.NewReplacer()
	for d.Next() {
		for d.NextBlock(0) {
			switch d.Val() {
			case "address":
				if !d.NextArg() {
					return d.ArgErr()
				}

				r.address = replacer.ReplaceKnown(d.Val(), "")

			case "username":
				if !d.NextArg() {
					return d.ArgErr()
				}

				r.username = replacer.ReplaceKnown(d.Val(), "")

			case "password":
				if !d.NextArg() {
					return d.ArgErr()
				}

				r.password = replacer.ReplaceKnown(d.Val(), "")

			case "subscribers_size":
				if !d.NextArg() {
					return d.ArgErr()
				}

				s, e := strconv.Atoi(replacer.ReplaceKnown(d.Val(), ""))
				if e != nil {
					return e
				}

				r.subscribersSize = s

			case "dispatcher_pool_size":
				if !d.NextArg() {
					return d.ArgErr()
				}

				s, e := strconv.Atoi(replacer.ReplaceKnown(d.Val(), ""))
				if e != nil {
					return e
				}

				r.dispatcherPoolSize = s

			case "redis_channel":
				if !d.NextArg() {
					return d.ArgErr()
				}

				r.redisChannel = replacer.ReplaceKnown(d.Val(), "")
			}
		}
	}

	return nil
}

var (
	_ caddy.Provisioner     = (*Redis)(nil)
	_ caddy.CleanerUpper    = (*Redis)(nil)
	_ caddyfile.Unmarshaler = (*Redis)(nil)
)

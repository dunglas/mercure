package caddy

import (
	"bytes"
	"encoding/gob"
	"fmt"
	"strconv"

	"github.com/caddyserver/caddy/v2"
	"github.com/caddyserver/caddy/v2/caddyconfig/caddyfile"
	"github.com/dunglas/mercure"
)

func init() {
	caddy.RegisterModule(Redis{})
}

type Redis struct {
	Address         string `json:"address,omitempty"`
	Username        string `json:"username,omitempty"`
	Password        string `json:"password,omitempty"`
	SubscribersSize int    `json:"subscribers_size,omitempty"`
	RedisChannel    string `json:"redis_channel,omitempty"`
	// IAM authentication for Google Cloud Memorystore
	UseIAMAuth      bool   `json:"use_iam_auth,omitempty"`
	ProjectID       string `json:"project_id,omitempty"`

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
		var t *mercure.RedisTransport
		var err error

		if r.UseIAMAuth {
			// Use IAM authentication
			if r.ProjectID == "" {
				return nil, fmt.Errorf("project_id is required when using IAM authentication")
			}
			t, err = mercure.NewRedisTransportWithIAMAddress(ctx.Logger(), r.Address, r.ProjectID, r.SubscribersSize, r.RedisChannel)
		} else {
			// Use traditional authentication
			t, err = mercure.NewRedisTransport(ctx.Logger(), r.Address, r.Username, r.Password, r.SubscribersSize, r.RedisChannel)
		}

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

				r.Address = replacer.ReplaceKnown(d.Val(), "")

			case "username":
				if !d.NextArg() {
					return d.ArgErr()
				}

				r.Username = replacer.ReplaceKnown(d.Val(), "")

			case "password":
				if !d.NextArg() {
					return d.ArgErr()
				}

				r.Password = replacer.ReplaceKnown(d.Val(), "")

			case "subscribers_size":
				if !d.NextArg() {
					return d.ArgErr()
				}

				s, e := strconv.Atoi(replacer.ReplaceKnown(d.Val(), ""))
				if e != nil {
					return e
				}

				r.SubscribersSize = s

			case "redis_channel":
				if !d.NextArg() {
					return d.ArgErr()
				}

				r.RedisChannel = replacer.ReplaceKnown(d.Val(), "")

			case "use_iam_auth":
				if !d.NextArg() {
					return d.ArgErr()
				}

				useIAM, err := strconv.ParseBool(replacer.ReplaceKnown(d.Val(), ""))
				if err != nil {
					return fmt.Errorf("invalid use_iam_auth value: %w", err)
				}

				r.UseIAMAuth = useIAM

			case "project_id":
				if !d.NextArg() {
					return d.ArgErr()
				}

				r.ProjectID = replacer.ReplaceKnown(d.Val(), "")
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

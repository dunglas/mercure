package mercure_test

import (
	"context"
	"log"
	"net/http"

	"github.com/dunglas/mercure"
)

func Example() {
	ctx := context.Background()

	h, err := mercure.NewHub(
		ctx,
		mercure.WithIssuers([]mercure.Issuer{{
			Identifier: "https://example.com",
			Publisher:  mercure.Static{Key: []byte("!ChangeMe!"), Algorithm: "HS256"},
			Subscriber: mercure.Static{Key: []byte("!ChangeMe!"), Algorithm: "HS256"},
		}}),
		mercure.WithResourceIdentifier("https://example.com/.well-known/mercure"),
	)
	if err != nil {
		log.Fatal(err)
	}

	defer func() {
		if err := h.Stop(ctx); err != nil {
			panic(err)
		}
	}()

	http.Handle("/.well-known/mercure", h)
	log.Panic(http.ListenAndServe(":8080", nil))
}

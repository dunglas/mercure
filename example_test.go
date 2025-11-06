package mercure_test

import (
	"context"
	"log"
	"net/http"

	"github.com/dunglas/mercure"
)

//nolint:gosec
func Example() {
	ctx := context.Background()

	h, err := mercure.NewHub(
		ctx,
		mercure.WithPublisherJWT([]byte("!ChangeMe!"), "HS256"),
		mercure.WithSubscriberJWT([]byte("!ChangeMe!"), "HS256"),
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

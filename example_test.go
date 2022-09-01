package mercure_test

import (
	"log"
	"net/http"

	"github.com/dunglas/mercure"
)

//nolint:gosec
func Example() {
	h, err := mercure.NewHub(
		mercure.WithPublisherJWT([]byte("!ChangeMe!"), "HS256"),
		mercure.WithSubscriberJWT([]byte("!ChangeMe!"), "HS256"),
	)
	if err != nil {
		log.Fatal(err)
	}
	defer h.Stop()

	http.Handle("/.well-known/mercure", h)
	log.Panic(http.ListenAndServe(":8080", nil))
}

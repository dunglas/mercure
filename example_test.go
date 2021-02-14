package mercure_test

import (
	"log"
	"net/http"

	"github.com/dunglas/mercure"
)

func Example() {
	h, err := mercure.NewHub(
		mercure.WithPublisherJWT([]byte("!ChangeMe!"), "HS256"),
		mercure.WithSubscriberJWT([]byte("!ChangeMe!"), "HS256"),
	)
	if err != nil {
		log.Fatal(err)
	}

	http.Handle("/.well-known/mercure", h)
	log.Fatal(http.ListenAndServe(":8080", nil))
}

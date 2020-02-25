package main

import (
	"fmt"
	"github.com/google/uuid"
	"github.com/gorilla/handlers"
	"github.com/gorilla/mux"
	"github.com/minio/minio-go/v6"
	log "github.com/sirupsen/logrus"
	"github.com/urfave/cli/v2"
	"net/http"
	"net/url"
	"os"
	"time"
)

func getMinioClient() *minio.Client {
	mc, err := minio.New(os.Getenv("MINIO_HOST"), os.Getenv("MINIO_ACCESS"), os.Getenv("MINIO_SECRET"), os.Getenv("MINIO_INSECURE") != "true")
	if err != nil {
		log.WithFields(log.Fields{"error": err.Error()}).Fatal("Failed to connect to minio")
	}
	return mc
}

func applyMiddleware(handler http.Handler) http.Handler {
	handler = handlers.LoggingHandler(os.Stdout, handler)

	return handler
}

func pasteHandler(mc *minio.Client) http.Handler {
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		key := uuid.New().String()
		ttl := 10 * time.Minute

		presignedUrl, err := mc.PresignedPutObject(os.Getenv("MINIO_BUCKET"), key, ttl)
		if err != nil {
			w.WriteHeader(http.StatusInternalServerError)
			log.Error(err)
			return
		}
		returnUrl := fmt.Sprintf("%s/%s", r.Host, key)
		h := w.Header()
		h.Set("Location", presignedUrl.String())
		h.Set("X-GET-URL", returnUrl)
		w.WriteHeader(http.StatusTemporaryRedirect)
		fmt.Fprintf(w, returnUrl)
	})
	return applyMiddleware(handler)
}

func getHandler(mc *minio.Client) http.Handler {
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		ttl := 10 * time.Minute
		reqParams := make(url.Values)
		presignedUrl, err := mc.PresignedGetObject(os.Getenv("MINIO_BUCKET"), r.URL.Path, ttl, reqParams)
		if err != nil {
			w.WriteHeader(http.StatusNotFound)
			return
		}
		http.Redirect(w, r, presignedUrl.String(), http.StatusSeeOther)
	})
	return applyMiddleware(handler)
}

func serve() {
	mc := getMinioClient()

	r := mux.NewRouter()

	r.Methods("PUT").Handler(pasteHandler(mc))
	r.Methods("GET").Handler(getHandler(mc))

	http.ListenAndServe(":3000", r)
}

func main() {
	app := &cli.App{
		Name:  "mcbin",
		Usage: "Minio pasteboard",
		Commands: []*cli.Command{
			{
				Name:  "serve",
				Usage: "serve mcbin",
				Action: func(c *cli.Context) error {
					serve()
					return nil
				},
			},
		},
	}

	err := app.Run(os.Args)
	if err != nil {
		log.Fatal(err)
	}
}

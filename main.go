package main

import (
	"fmt"
	"github.com/google/uuid"
	"github.com/gorilla/handlers"
	"github.com/gorilla/mux"
	"github.com/minio/minio-go/v6"
	log "github.com/sirupsen/logrus"
	"github.com/urfave/cli/v2"
	"io"
	"net/http"
	"os"
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
		data := http.MaxBytesReader(w, r.Body, 10000000)
		n, err := mc.PutObject(os.Getenv("MINIO_BUCKET"), key, data, -1, minio.PutObjectOptions{})
		if err != nil {
			w.WriteHeader(http.StatusInternalServerError)
			log.Error(err)
			return
		}
		log.Debug(n)
		fmt.Fprintf(w, "%s/%s", r.Host, key)
	})
	return applyMiddleware(handler)
}

func getHandler(mc *minio.Client) http.Handler {
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		object, err := mc.GetObject(os.Getenv("MINIO_BUCKET"), r.URL.Path, minio.GetObjectOptions{})
		if err != nil {
			w.WriteHeader(http.StatusNotFound)
			return
		}
		w.Header().Set("Content-Type", "text/plain")
		n, err := io.Copy(w, object)
		if err != nil {
			w.WriteHeader(http.StatusInternalServerError)
			log.Error(err)
			return
		}
		log.Debug(n)
	})
	return applyMiddleware(handler)
}

func serve() {
	mc := getMinioClient()

	r := mux.NewRouter()

	r.Methods("POST").Handler(pasteHandler(mc))
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

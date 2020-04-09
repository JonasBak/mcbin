package main

import (
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"fmt"
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

func randomString(n uint) string {
	b := make([]byte, n)
	rand.Read(b)
	return fmt.Sprintf("%x", b)
}

func signString(s, secret string) string {
	mac := hmac.New(sha256.New, []byte(secret))
	mac.Write([]byte(s))
	return base64.StdEncoding.EncodeToString(mac.Sum(nil))
}

func validate(s, secret, v string) bool {
	toValidate, err := base64.StdEncoding.DecodeString(v)
	if err != nil {
		return false
	}
	mac := hmac.New(sha256.New, []byte(secret))
	mac.Write([]byte(s))
	expected := mac.Sum(nil)
	return hmac.Equal(toValidate, expected)
}

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

func handleRedirection(key string, mc *minio.Client, w http.ResponseWriter, r *http.Request) {
	ttl := 2 * time.Minute

	presignedUrl, err := mc.PresignedPutObject(os.Getenv("MINIO_BUCKET"), key, ttl)
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		log.Error(err)
		return
	}
	h := w.Header()
	h.Set("Location", presignedUrl.String())
	w.WriteHeader(http.StatusTemporaryRedirect)
}

func pasteHandler(mc *minio.Client, secret string) http.Handler {
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		key := randomString(16)
		signed := signString(key, secret)

		returnUrl := fmt.Sprintf("%s/%s", r.Host, key)
		h := w.Header()
		h.Set("X-GET-URL", returnUrl)
		h.Set("X-SIGNED", signed)
		handleRedirection(key, mc, w, r)
		fmt.Fprintf(w, returnUrl)
	})
	return applyMiddleware(handler)
}

func editHandler(mc *minio.Client, secret string) http.Handler {
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		key := mux.Vars(r)["key"]
		signed := r.Header.Get("X-SIGNED")
		if !validate(key, secret, signed) {
			log.Info("Failed to validate HMAC")
			w.WriteHeader(http.StatusBadRequest)
			return
		}
		handleRedirection(key, mc, w, r)
	})
	return applyMiddleware(handler)
}

func getHandler(mc *minio.Client) http.Handler {
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		ttl := 2 * time.Minute
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
	signingSecret := os.Getenv("SIGNING_SECRET")
	if signingSecret == "" {
		signingSecret = randomString(32)
		log.Debugf("Created new signing secret: %s", signingSecret)
	}
	mc := getMinioClient()

	r := mux.NewRouter()

	r.Methods("GET").Handler(getHandler(mc))
	r.Methods("PUT").Path("/{key}").Handler(editHandler(mc, signingSecret))
	r.Methods("PUT").Handler(pasteHandler(mc, signingSecret))

	http.ListenAndServe(":3000", r)
}

func main() {
	log.SetLevel(log.DebugLevel)
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

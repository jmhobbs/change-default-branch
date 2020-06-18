package main

import (
	"context"
	"encoding/json"
	"flag"
	"log"
	"net/http"
	"os"

	rice "github.com/GeertJohan/go.rice"
	"github.com/google/go-github/v32/github"
	"github.com/gorilla/sessions"
	"github.com/peterbourgon/ff/v3"
	"golang.org/x/oauth2"

	"github.com/gorilla/csrf"
	"github.com/gorilla/mux"
)

var (
	store     *sessions.CookieStore
	conf      *oauth2.Config
	templates *rice.Box
	files     *rice.Box
)

const (
	SessionTokenKey string = "token"
	SessionName     string = "change-branch"
)

func main() {
	fs := flag.NewFlagSet("change-branch", flag.ExitOnError)

	var (
		listen       *string = fs.String("listen", "localhost:9090", "http listen address")
		sessionKey   *string = fs.String("session-key", "", "session secret (32 bytes, random, and secret)")
		csrfKey      *string = fs.String("csrf-key", "", "CSRF secret (32 bytes, random, and secret)")
		clientID     *string = fs.String("client-id", "", "Github OAuth2 client ID")
		clientSecret *string = fs.String("client-secret", "", "Github OAuth2 client secret")
		dev          *bool   = fs.Bool("dev", false, "dev mode (insecure)")
		_                    = fs.String("config", "", "config file (optional)")
	)

	ff.Parse(fs, os.Args[1:],
		ff.WithEnvVarNoPrefix(),
		ff.WithConfigFileFlag("config"),
		ff.WithConfigFileParser(ff.PlainParser),
	)

	templates = rice.MustFindBox("templates")
	files = rice.MustFindBox("files")

	store = sessions.NewCookieStore([]byte(*sessionKey))

	conf = &oauth2.Config{
		ClientID:     *clientID,
		ClientSecret: *clientSecret,
		Scopes:       []string{"repo"},
		Endpoint: oauth2.Endpoint{
			AuthURL:  "https://github.com/login/oauth/authorize",
			TokenURL: "https://github.com/login/oauth/access_token",
		},
	}

	options := []csrf.Option{
		csrf.CookieName("csrf"),
		csrf.FieldName("csrf-token"),
		csrf.ErrorHandler(CSRFFailureHandler()),
	}
	if *dev {
		log.Println("DEV MODE ON - INSECURE CSRF")
		options = append(options, csrf.Secure(false))
	}

	csrfMiddleware := csrf.Protect([]byte(*csrfKey), options...)

	r := mux.NewRouter()
	r.NotFoundHandler = http.HandlerFunc(ContentHandler)

	authRouter := r.PathPrefix("/auth").Subrouter()

	authRouter.HandleFunc("/callback", AuthCallbackHandler)
	authRouter.HandleFunc("/redirect", AuthRedirectHandler)

	reposRouter := r.PathPrefix("/repos").Subrouter()
	reposRouter.Use(csrfMiddleware)

	reposRouter.HandleFunc("/list", RepositoriesListHandler).Methods("GET")
	reposRouter.HandleFunc("/processing", RepositoryProcessingHandler).Methods("POST")
	reposRouter.HandleFunc("/convert", RepositoryConvertHandler).Methods("POST")

	log.Printf("Listening on %s", *listen)
	log.Fatal(http.ListenAndServe(*listen, r))
}

func clientFromSession(session *sessions.Session) (*github.Client, error) {
	var tok oauth2.Token

	err := json.Unmarshal(session.Values[SessionTokenKey].([]byte), &tok)
	if err != nil {
		return nil, err
	}

	ts := conf.TokenSource(context.Background(), &tok)
	tc := oauth2.NewClient(context.Background(), ts)
	return github.NewClient(tc), nil
}

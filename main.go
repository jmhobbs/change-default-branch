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
		clientID     *string = fs.String("client-id", "", "Github OAuth2 client ID")
		clientSecret *string = fs.String("client-secret", "", "Github OAuth2 client secret")
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

	http.HandleFunc("/auth/callback", AuthCallbackHandler)
	http.HandleFunc("/auth/redirect", AuthRedirectHandler)
	http.HandleFunc("/repositories", RepositoriesListHandler)
	http.HandleFunc("/processing", RepositoryProcessingHandler)
	http.HandleFunc("/repositories/convert", RepositoryConvertHandler)
	http.HandleFunc("/", ContentHandler)

	log.Printf("Listening on %s", *listen)
	log.Fatal(http.ListenAndServe(*listen, nil))
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

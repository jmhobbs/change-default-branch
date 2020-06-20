package main

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"log"
	"net/http"

	"github.com/google/go-github/v32/github"
	"golang.org/x/oauth2"
)

type AuthContextKey string

const (
	ContextIsAuthorized AuthContextKey = "is-authorized"
	ContextOAuthToken   AuthContextKey = "oauth-token"
)

const (
	SessionTokenKey string = "token"
	SessionName     string = "change-default-branch"
)

func authDecoratorMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		session, _ := store.Get(r, SessionName)
		token, ok := session.Values[SessionTokenKey]

		ctx := context.WithValue(r.Context(), ContextIsAuthorized, ok)
		ctx = context.WithValue(ctx, ContextOAuthToken, token)

		next.ServeHTTP(w, r.WithContext(ctx))
	})
}

func authRequiredMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		v := r.Context().Value(ContextIsAuthorized)
		isAuthorized, ok := v.(bool)
		if !ok || !isAuthorized {
			http.Redirect(w, r, "/auth/error", 302) // todo: this error page
			return
		}

		next.ServeHTTP(w, r)
	})
}

func getClient(r *http.Request) (oauth2.TokenSource, *github.Client, error) {
	var tok oauth2.Token

	v := r.Context().Value(ContextOAuthToken)
	token, ok := v.([]byte)
	if !ok {
		return nil, nil, errors.New("invalid oauth token")
	}

	err := json.Unmarshal(token, &tok)
	if err != nil {
		return nil, nil, err
	}

	ts := conf.TokenSource(context.Background(), &tok)
	tc := oauth2.NewClient(context.Background(), ts)

	return ts, github.NewClient(tc), nil
}

// AuthCallbackHandler processes OAuth callbacks from Github
func AuthCallbackHandler(w http.ResponseWriter, r *http.Request) {
	tok, err := conf.Exchange(context.Background(), r.URL.Query().Get("code"))
	if err != nil {
		log.Printf("error exchanging code for token: %v", err)
		http.Error(w, "Internal Server Error", http.StatusInternalServerError)
		return
	}

	session, err := store.Get(r, SessionName)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	tokenBytes, err := json.Marshal(tok)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	session.Values[SessionTokenKey] = tokenBytes

	err = session.Save(r, w)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	http.Redirect(w, r, "/repos/list", 302)
}

func AuthRedirectHandler(w http.ResponseWriter, r *http.Request) {
	url := conf.AuthCodeURL("state", oauth2.AccessTypeOnline)
	http.Redirect(w, r, url, 302)
}

func AuthErrorHandler(w http.ResponseWriter, r *http.Request) {
	session, err := store.Get(r, SessionName)
	if err != nil {
		log.Println(err)
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	session.Options.MaxAge = -1

	err = session.Save(r, w)
	if err != nil {
		log.Println(err)
		http.Error(w, "Internal Server Error", http.StatusInternalServerError)
		return
	}

	f, err := files.Open("auth-error.html")
	if err != nil {
		http.Error(w, "Internal Server Error", http.StatusInternalServerError)
		return
	}
	defer f.Close()

	io.Copy(w, f)
}

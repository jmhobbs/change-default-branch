package main

import (
	"context"
	"encoding/json"
	"log"
	"net/http"

	"golang.org/x/oauth2"
)

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

	btok, err := json.Marshal(tok)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	session.Values["token"] = btok

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

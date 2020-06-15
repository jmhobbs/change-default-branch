package main

import (
	"context"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"log"
	"net/http"
	"os"

	"github.com/go-git/go-git/v5"
	"github.com/go-git/go-git/v5/config"
	"github.com/go-git/go-git/v5/plumbing"
	"github.com/google/go-github/v32/github"
	"github.com/gorilla/sessions"
	"golang.org/x/oauth2"
)

var store = sessions.NewCookieStore([]byte(os.Getenv("SESSION_KEY")))
var conf *oauth2.Config

func main() {
	conf = &oauth2.Config{
		ClientID:     os.Getenv("CLIENT_ID"),
		ClientSecret: os.Getenv("CLIENT_SECRET"),
		Scopes:       []string{"repo"},
		Endpoint: oauth2.Endpoint{
			AuthURL:  "https://github.com/login/oauth/authorize",
			TokenURL: "https://github.com/login/oauth/access_token",
		},
		RedirectURL: "http://localhost:9090/auth/callback",
	}

	http.HandleFunc("/auth/callback", func(w http.ResponseWriter, r *http.Request) {
		// todo: accept header for json response?
		log.Println("query params:", r.URL.Query())

		tok, err := conf.Exchange(context.Background(), r.URL.Query().Get("code"))
		if err != nil {
			log.Fatal(err)
		}
		log.Println(tok)

		session, err := store.Get(r, "change-branch")
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

		http.Redirect(w, r, "/repositories", 302)
	})

	http.HandleFunc("/auth/redirect", func(w http.ResponseWriter, r *http.Request) {
		url := conf.AuthCodeURL("state", oauth2.AccessTypeOnline)
		log.Println("redirecting to:", url)
		http.Redirect(w, r, url, 302)
	})

	http.HandleFunc("/repositories", func(w http.ResponseWriter, r *http.Request) {
		session, err := store.Get(r, "change-branch")
		if err != nil {
			log.Fatal(err)
		}
		client, err := clientFromSession(session)
		if err != nil {
			log.Fatal(err)
		}

		repos, _, err := client.Repositories.List(context.Background(), "jmhobbs", nil)
		if err != nil {
			log.Fatal(err)
		}

		fmt.Fprint(w, `<!doctype html><html><body><h1>Repositories</h1><form method="POST" action="/repositories/convert"><ul>`)
		for _, repo := range repos {
			fmt.Fprintf(w, `<li><input type="checkbox" name="repository[]" value="%d">%s</li>`, repo.GetID(), repo.GetFullName())
		}
		fmt.Fprint(w, `</ul><input type="submit" value="Convert"></form></body></html>`)
	})

	http.HandleFunc("/repositories/convert", func(w http.ResponseWriter, r *http.Request) {
		session, err := store.Get(r, "change-branch")
		if err != nil {
			log.Fatal(err)
		}
		client, err := clientFromSession(session)
		if err != nil {
			log.Fatal(err)
		}

		// get the repo
		repo, _, err := client.Repositories.GetByID(context.TODO(), 161702810)
		if err != nil {
			log.Fatal(err)
		}

		// is it alreafy on the right branch?
		if *repo.DefaultBranch == "prime" {
			log.Println("nothing to do!")
			fmt.Fprint(w, "nothing to do!")
			return
		}

		owner := repo.GetOwner()
		name := owner.GetLogin()

		log.Println(repo)
		log.Println("owner name:", name)

		// check the branch exists
		_, response, err := client.Repositories.GetBranch(context.TODO(), name, repo.GetName(), "prime")
		if err != nil {
			if response.StatusCode == 404 {
				// create the branch
				log.Println("need to create the branch")
				// 1. get the branch for default branch
				// 2. get it's RepositoryCommit
				// 3. install deploy key
				// 4. get the sha
				// 5. create a new branch
				dir, err := ioutil.TempDir("", "change-branch")
				if err != nil {
					log.Fatal(err)
				}
				defer os.RemoveAll(dir)

				log.Println(repo.GetSSHURL())
				log.Println(dir)

				clone, err := git.PlainClone(dir, true, &git.CloneOptions{
					URL:        repo.GetSSHURL(),
					NoCheckout: true,
				})
				if err != nil {
					log.Fatal(err)
				}

				headRef, err := clone.Head()
				if err != nil {
					log.Fatal(err)
				}

				ref := plumbing.NewHashReference("refs/heads/prime", headRef.Hash())
				err = clone.Storer.SetReference(ref)
				if err != nil {
					log.Fatal(err)
				}

				// 6. push it up
				err = clone.Push(&git.PushOptions{
					RemoteName: "origin",
					RefSpecs: []config.RefSpec{
						config.RefSpec("refs/heads/prime:refs/heads/prime"),
					},
				})
				if err != nil && err != git.NoErrAlreadyUpToDate {
					log.Fatal("error pushing:", err)
				}
			} else {
				log.Fatal("error cloning:", err)
			}
		}
		// update the default branch on the repo
		repo.DefaultBranch = github.String("prime")

		_, _, err = client.Repositories.Edit(context.TODO(), name, repo.GetName(), repo)
		if err != nil {
			log.Fatal(err)
		}
		// success
		fmt.Fprintf(w, "shit dawg we did it")
	})

	http.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		session, _ := store.Get(r, "change-branch")
		if session.Values["token"] != nil {
			fmt.Fprint(w, `<!doctype html><html><body><a href="/repositories">See Repositories</a></body></html>`)
		} else {
			fmt.Fprint(w, `<!doctype html><html><body><a href="/auth/redirect">Log In</a></body></html>`)
		}
	})

	log.Println("http://127.0.0.1:9090/")
	log.Fatal(http.ListenAndServe("127.0.0.1:9090", nil))
}

func clientFromSession(session *sessions.Session) (*github.Client, error) {
	var tok oauth2.Token

	err := json.Unmarshal(session.Values["token"].([]byte), &tok)
	if err != nil {
		return nil, err
	}

	ts := conf.TokenSource(context.Background(), &tok)
	tc := oauth2.NewClient(context.Background(), ts)
	return github.NewClient(tc), nil
}

// todo: middleware that inflates request.context

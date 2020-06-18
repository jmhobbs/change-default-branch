package main

import (
	"context"
	"encoding/json"
	"fmt"
	"html/template"
	"log"
	"net/http"
	"strings"
	"time"

	"github.com/google/go-github/v32/github"
	"github.com/gorilla/csrf"
)

func templateFromFile(path string) (*template.Template, error) {
	templateString, err := templates.String(path)
	if err != nil {
		return nil, err
	}
	return template.New(path).Funcs(template.FuncMap{
		"json": func(str string) template.JS {
			return template.JS(str)
		},
	}).Parse(templateString)
}

// todo: memoize these
func getUsersRepositories(client *github.Client) ([]*github.Repository, error) {
	opt := &github.RepositoryListOptions{
		Visibility:  "all",
		Affiliation: "owner",
		ListOptions: github.ListOptions{PerPage: 100},
	}

	var repos []*github.Repository
	for {
		page, resp, err := client.Repositories.List(context.Background(), "", opt)
		if err != nil {
			return nil, err
		}
		repos = append(repos, page...)

		if resp.NextPage == 0 {
			break
		}
		opt.Page = resp.NextPage

	}

	return repos, nil
}

type repository struct {
	Name          string
	FullName      string
	DefaultBranch string
	Fork          bool
	Description   string
	Private       bool
	Archived      bool
}

func RepositoriesListHandler(w http.ResponseWriter, r *http.Request) {
	w.Header().Add("cache-control", "private, no-cache, no-store")

	session, err := store.Get(r, "change-branch")
	if err != nil {
		http.Error(w, "Internal Server Error", http.StatusInternalServerError)
		return
	}
	client, err := clientFromSession(session)
	if err != nil {
		// todo: redirect to auth, make this middleware
		http.Error(w, "Internal Server Error", http.StatusInternalServerError)
		return
	}

	tmpl, err := templateFromFile("repositories.html.tmpl")
	if err != nil {
		log.Println(err)
		http.Error(w, "Internal Server Error", http.StatusInternalServerError)
		return
	}

	repos, err := getUsersRepositories(client)
	if err != nil {
		log.Println(err)
		http.Error(w, "Internal Server Error", http.StatusInternalServerError)
		return
	}

	tmplData := []repository{}
	for _, repo := range repos {
		tmplData = append(tmplData, repository{
			Name:          repo.GetName(),
			FullName:      repo.GetFullName(),
			DefaultBranch: repo.GetDefaultBranch(),
			Fork:          repo.GetFork(),
			Description:   repo.GetDescription(),
			Private:       repo.GetPrivate(),
			Archived:      repo.GetArchived(),
		})
	}

	err = tmpl.Execute(w, map[string]interface{}{csrf.TemplateTag: csrf.TemplateField(r), "Repositories": tmplData})
	if err != nil {
		log.Println("error executing template:", err)
		http.Error(w, "Internal Server Error", http.StatusInternalServerError)
		return
	}
}

func RepositoryProcessingHandler(w http.ResponseWriter, r *http.Request) {
	w.Header().Add("cache-control", "private, no-cache, no-store")

	tmpl, err := templateFromFile("processing.html.tmpl")
	if err != nil {
		http.Error(w, "Internal Server Error", http.StatusInternalServerError)
		return
	}

	err = r.ParseForm()
	if err != nil {
		http.Error(w, "Internal Server Error", http.StatusInternalServerError)
		return
	}

	branch := r.FormValue("default_branch")
	repos := r.Form["repository[]"]

	// todo: validation

	data := map[string]interface{}{
		"CSRFToken":    csrf.Token(r),
		"Branch":       branch,
		"Repositories": repos,
	}

	encoded, err := json.Marshal(data)
	if err != nil {
		http.Error(w, "Internal Server Error", http.StatusInternalServerError)
		return
	}

	data["Data"] = string(encoded)

	err = tmpl.Execute(w, data)
	if err != nil {
		http.Error(w, "Internal Server Error", http.StatusInternalServerError)
		return
	}
}

func RepositoryConvertHandler(w http.ResponseWriter, r *http.Request) {
	w.Header().Add("cache-control", "private, no-cache, no-store")

	session, err := store.Get(r, "change-branch")
	if err != nil {
		http.Error(w, "Internal Server Error", http.StatusInternalServerError)
		return
	}
	client, err := clientFromSession(session)
	if err != nil {
		http.Error(w, "Internal Server Error", http.StatusInternalServerError)
		return
	}

	err = r.ParseForm()
	if err != nil {
		http.Error(w, "Internal Server Error", http.StatusInternalServerError)
		return
	}

	branch := strings.TrimSpace(r.FormValue("branch"))
	repo := strings.TrimSpace(r.FormValue("repository"))

	if len(branch) == 0 {
		http.Error(w, "New branch name is invalid.", http.StatusBadRequest)
		return
	}

	if len(repo) == 0 {
		http.Error(w, "No repository specified.", http.StatusBadRequest)
		return
	}

	split := strings.SplitN(repo, "/", 2)
	if len(split) != 2 {
		http.Error(w, "Invalid repository.", http.StatusBadRequest)
		return
	}

	owner := split[0]
	repo = split[1]

	logs, err := changeBranch(client, owner, repo, branch)
	if err != nil {
		log.Println(err)
		http.Error(w, strings.Join(logs, "\n"), http.StatusInternalServerError)
		return
	}

	fmt.Fprint(w, strings.Join(logs, "\n"))
}

func changeBranch(client *github.Client, owner, name, branch string) ([]string, error) {
	logs := []string{
		fmt.Sprintf("Changing branch to %q.", branch),
		fmt.Sprintf("Getting the repository %q.", name),
	}

	repo, _, err := client.Repositories.Get(context.TODO(), owner, name)
	if err != nil {
		return logs, err
	}

	// is it already on the right branch?
	if repo.GetDefaultBranch() == branch {
		logs = append(logs, "Default branch is already set, so nothing to do!")
		return logs, nil
	}

	logs = append(logs, "Checking if branch exists.")

	// check the branch exists
	_, response, err := client.Repositories.GetBranch(context.TODO(), owner, name, branch)
	if err != nil {
		if response.StatusCode == 404 {
			logs = append(logs, "Branch not found, creating a git ref for it.")

			logs = append(logs, fmt.Sprintf("Getting the current default branch %q to lookup SHA.", repo.GetDefaultBranch()))
			defaultBranch, _, err := client.Repositories.GetBranch(context.TODO(), owner, name, repo.GetDefaultBranch())
			if err != nil {
				return logs, err
			}

			ref := fmt.Sprintf("refs/heads/%s", branch)

			logs = append(logs, fmt.Sprintf("Creating %q from %s", ref, defaultBranch.GetCommit().GetSHA()))
			_, _, err = client.Git.CreateRef(context.TODO(), owner, name, &github.Reference{
				Ref: github.String(ref),
				Object: &github.GitObject{
					SHA: defaultBranch.Commit.SHA,
				},
			})
			if err != nil {
				return logs, err
			}
			// sleep here because sometimes the next call results in a missing branch error, let it sync up
			time.Sleep(5 * time.Second)
		} else {
			return logs, err
		}
	} else {
		logs = append(logs, "Branch exists.")
	}

	// update the default branch on the repo
	repo.DefaultBranch = github.String(branch)

	logs = append(logs, "Updating repository default branch.")
	_, resp, err := client.Repositories.Edit(context.TODO(), owner, name, repo)
	if err != nil {
		// todo: parse error from github api
		logs = append(logs, fmt.Sprintf("Error from Github API: %s", resp.Status))
	}

	// todo: catch 422 Validation Failed here and offer to retry in the app

	return logs, err
}

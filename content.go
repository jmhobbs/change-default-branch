package main

import (
	"io"
	"log"
	"mime"
	"net/http"
	"os"
	"strings"
)

func ContentHandler(w http.ResponseWriter, r *http.Request) {
	file := strings.TrimPrefix(r.URL.Path, "/")

	// special case for index
	if file == "" {
		file = "index.html"
	}

	f, err := files.Open(file)
	if err != nil {
		if os.IsNotExist(err) {
			file = file + ".html"
			f, err = files.Open(file)
			if err != nil {
				if os.IsNotExist(err) {
					f, err = files.Open("not-found.html")
					if err != nil {
						log.Printf("error loading 404 page: %v", err)
						http.Error(w, "Internal Server Error", http.StatusInternalServerError)
						return
					}
					defer f.Close()

					w.WriteHeader(http.StatusNotFound)
					io.Copy(w, f)
					return
				} else {
					http.Error(w, "Internal Server Error", http.StatusInternalServerError)
				}
				return
			}
		} else {
			http.Error(w, "Internal Server Error", http.StatusInternalServerError)
			return
		}
	}
	defer f.Close()

	contentType := "text/html"
	split := strings.Split(file, ".")
	if len(split) > 1 {
		extension := split[len(split)-1]
		detectedContentType := mime.TypeByExtension("." + extension)
		if detectedContentType != "" {
			contentType = detectedContentType
		}
	}

	w.Header().Add("content-type", contentType)
	w.Header().Add("cache-control", "public, max-age=86400")
	io.Copy(w, f)
}

func CSRFFailureHandler() http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		f, err := files.Open("csrf-failure.html")
		if err != nil {
			log.Println(err)
			http.Error(w, "Internal Server Error", http.StatusInternalServerError)
			return
		}
		defer f.Close()
		w.WriteHeader(http.StatusForbidden)
		io.Copy(w, f)
	})
}

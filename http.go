package main

import (
	"html/template"
	"io"
	"log"
	"net/http"
	"os"
	"path/filepath"

	assetfs "github.com/elazarl/go-bindata-assetfs"

	"github.com/tdewolff/minify"
	"github.com/tdewolff/minify/css"
	"github.com/tdewolff/minify/html"
	"github.com/tdewolff/minify/js"
	"github.com/tdewolff/minify/svg"
)

func rootPageTemplate(res *assetfs.AssetFS) *template.Template {
	mini := minify.New()
	mini.AddFunc("text/css", css.Minify)
	mini.AddFunc("text/html", html.Minify)
	mini.AddFunc("text/html", html.Minify)
	mini.AddFunc("image/svg+xml", svg.Minify)
	mini.AddFunc("text/javascript", js.Minify)

	content, err := res.Asset("static/index.htm")
	if err != nil {
		log.Fatal(err)
	}

	tmplMinified, err := mini.String("text/html", string(content))
	if err != nil {
		log.Fatal(err)
	}

	return template.Must(template.New("").Parse(tmplMinified))
}

// getpage handles the request to show the webpage
func getRootPage(w http.ResponseWriter, r *http.Request) {
	if r.URL.Path != "/" {
		http.NotFound(w, r)
		return
	}

	path := r.URL.Query().Get("p")
	data, err := listDirectory(path)

	if err != nil {
		log.Println(err)
		http.Error(w, "Bad Request", http.StatusBadRequest)
		return
	}

	rootPage.Execute(w, data)
}

// newfolder handles web requests to create a new folder
func newfolder(w http.ResponseWriter, r *http.Request) {
	err := r.ParseMultipartForm(2 ^ 16)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	log.Println("Creating new folder")

	currentPath := r.FormValue("path")
	folderName := r.FormValue("foldername")

	if currentPath == "" || folderName == "" {
		http.Error(w, "Invalid form data", http.StatusBadRequest)
		return
	}

	dirPath := filepath.Join(*basePath, currentPath, folderName)
	if !isPathValid(dirPath) {
		log.Println("Invalid dir path: " + dirPath)
		http.Error(w, "Bad Request", http.StatusBadRequest)
		return
	}

	if err = os.Mkdir(dirPath, 0755); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	w.WriteHeader(http.StatusOK)
}

// upload handles upload requests
func upload(w http.ResponseWriter, r *http.Request) {
	if err := r.ParseMultipartForm(1024 ^ 4); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	log.Println("Receiving upload")
	m := r.MultipartForm

	dirPath := m.Value["path"][0]

	for i := range m.File {
		//for each fileheader, get a handle to the actual file
		uploadStream, err := m.File[i][0].Open()
		if err != nil {
			log.Println("Unable to retrieve file: " + i)
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		defer uploadStream.Close()

		path := filepath.Join(*basePath, dirPath, m.File[i][0].Filename)
		if !isPathValid(path) {
			http.Error(w, "Bad Request", http.StatusBadRequest)
			return
		}

		// create destination file making sure the path is writeable.
		dstFile, err := os.Create(path)
		if err != nil {
			log.Println("Could not create destination file: " + path)
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		defer dstFile.Close()

		//copy the uploaded file to the destination file
		if _, err := io.Copy(dstFile, uploadStream); err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}

		log.Println("Uploaded: " + m.File[i][0].Filename)
	}

	w.WriteHeader(http.StatusOK)
}

// getFile provides a download of the requested file
func getFile(w http.ResponseWriter, r *http.Request) {
	path := r.URL.Query().Get("p")
	path = filepath.Join(*basePath, path)

	if !isPathValid(path) {
		log.Println("Acces to path not allowed:" + path)
		http.Error(w, "Forbidden", http.StatusForbidden)
		return
	}

	log.Printf("Download '%v'", path)

	// Send file always as download
	w.Header().Set("Content-Disposition", "attachment; filename=\""+filepath.Base(path)+"\"")
	http.ServeFile(w, r, path)
}

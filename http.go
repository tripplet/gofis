package main

import (
	"html/template"
	"io"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"strings"
)

func rootPageTemplate(rootPageTemple string) *template.Template {
	return template.Must(template.New("").Parse(rootPageTemple))
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

	log.Println("Recevied request to creat a new folder from:", r.RemoteAddr)

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

	log.Println("Receiving upload from:", r.RemoteAddr)
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

		// Fix path from MISE, because MSIE sends filename with complete local path not only the filename
		filename := m.File[i][0].Filename
		lastBackslash := strings.LastIndex(filename, "\\")
		if lastBackslash != -1 {
			filename = filename[lastBackslash+1:]
		}

		path := filepath.Join(*basePath, dirPath, filename)
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

		log.Println("Uploaded: " + path)
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

	log.Printf("Download '%v' from '%s'", path, r.RemoteAddr)

	// Send file always as download
	w.Header().Set("Content-Disposition", "attachment; filename="+strconv.Quote(filepath.Base(path)))
	w.Header().Set("Content-Type", "application/octet-stream")

	http.ServeFile(w, r, path)
}

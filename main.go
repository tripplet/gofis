package main

import (
	"errors"
	"flag"
	"html/template"
	"io/ioutil"
	"log"
	"net/http"
	"path/filepath"
	"strings"

	"github.com/dustin/go-humanize"

	assetfs "github.com/elazarl/go-bindata-assetfs"

	"github.com/rjeczalik/notify"
)

type file struct {
	Name     string
	Type     string
	Modified string
	Size     string
}

type directory struct {
	Name     string
	Type     string
	Modified string
}

type pageData struct {
	DirectoryName string
	IsNotTopMost  bool
	DirectoryPath string
	Files         []file
	Directories   []directory
	ParentDir     string
}

var basePath *string
var name *string
var rootPage *template.Template

func main() {
	// Parse command line parameters
	port := flag.String("p", "80", "Port for http server")
	basePath = flag.String("d", ".", "Directory to be shared")
	name = flag.String("n", "Shared", "Name of main directory")
	flag.Parse()

	if *basePath == "." {
		var err error
		*basePath, err = filepath.Abs(".")
		if err != nil {
			log.Fatal(err)
		}
	}

	log.Println("Path:", *basePath)

	// Make the channel buffered to ensure no event is dropped. Notify will drop
	// an event if the receiver is not able to keep up the sending pace.
	fsEvents := make(chan notify.EventInfo, 1)

	// Set up a watchpoint listening for events within a directory tree rooted
	// at current working directory. Dispatch remove events to c.
	if err := notify.Watch(*basePath+"...", fsEvents, notify.Create, notify.Remove, notify.Rename); err != nil {
		log.Fatal(err)
	}
	defer notify.Stop(fsEvents)
	startNotifyWsClients(fsEvents)

	res := &assetfs.AssetFS{Asset: Asset, AssetDir: AssetDir, AssetInfo: AssetInfo, Prefix: ""}
	rootPage = rootPageTemplate(res)

	http.Handle("/static/", http.FileServer(res))
	http.HandleFunc("/", getRootPage)
	http.HandleFunc("/getfile", getFile)
	http.HandleFunc("/ws", ws)
	http.HandleFunc("/upload", upload)
	http.HandleFunc("/newfolder", newfolder)

	log.Println("Starting server on :" + *port)
	if err := http.ListenAndServe(":"+*port, nil); err != nil {
		log.Fatalln("Error: Could not create server, quitting.")
	}
}

// isPathValid checks whether a given path is valid and below the basePath.
func isPathValid(path string) bool {
	if path != "." {
		var err error
		path, err = filepath.Rel(*basePath, path)
		if err != nil {
			log.Printf("Invalid path: %v", path)
			return false
		}
	}

	if strings.HasPrefix(path, "..") {
		log.Printf("Invalid path: %v", path)
		return false
	}

	return true
}

// listDirectory returns the content of the directory as pageData struct or and error if the path is invalid or not allowed.
func listDirectory(path string) (pageData, error) {
	if path == "" {
		path = "."
	}

	// Get diretory name for display in webpage
	dirName := filepath.Base(path)
	if dirName == "." {
		dirName = *name
	}

	directoryPath := filepath.Join(*basePath, path)

	if !isPathValid(directoryPath) {
		return pageData{}, errors.New("Access not allowed")
	}

	// Get contents of directory
	entries, err := ioutil.ReadDir(directoryPath)
	if err != nil {
		return pageData{}, err
	}

	var foundFiles []file
	var foundDirs []directory
	for _, f := range entries {
		if f.IsDir() {
			foundDirs = append(foundDirs, directory{Name: f.Name(), Modified: f.ModTime().Format("Mon 02. January 2006 15:04"), Type: "Dir"})
		} else {
			foundFiles = append(foundFiles, file{Name: f.Name(), Modified: f.ModTime().Format("Mon 02. January 2006 15:04"), Type: "File", Size: humanize.Bytes(uint64(f.Size()))})
		}
	}

	return pageData{
		DirectoryName: dirName,
		IsNotTopMost:  path != ".",
		Files:         foundFiles,
		Directories:   foundDirs,
		DirectoryPath: path,
		ParentDir:     filepath.Dir(path),
	}, nil
}

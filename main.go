package main

import (
	"embed"
	"errors"
	"flag"
	"fmt"
	"html/template"
	"io/fs"
	"io/ioutil"
	"log"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"strings"

	"github.com/dustin/go-humanize"
	"github.com/rjeczalik/notify"
)

//go:generate go run gen.go

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

//go:embed out
var embeddedFS embed.FS

func main() {
	// Parse command line parameters
	port := flag.Int("p", 80, "Port for http server")
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

	rootPage = loadRootTemplate()

	subFs, err := fs.Sub(embeddedFS, "out")
	if err != nil {
		log.Fatal(err)
	}

	http.Handle("/static/", http.FileServer(http.FS(subFs)))
	http.HandleFunc("/", getRootPage)
	http.HandleFunc("/getfile", getFile)
	http.HandleFunc("/ws", ws)
	http.HandleFunc("/upload", upload)
	http.HandleFunc("/newfolder", newfolder)

	log.Println("Starting server on Port", *port)
	portPostfix := ""
	if *port != 80 {
		portPostfix = fmt.Sprintf(":%d", *port)
	}

	for _, name := range getLocalNames() {
		log.Printf("Access via http://%s%s", name, portPostfix)
	}

	if err := http.ListenAndServe(fmt.Sprintf(":%d", *port), nil); err != nil {
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

func loadRootTemplate() *template.Template {
	templateBytes, err := embeddedFS.ReadFile("out/static/index.htm")
	if err != nil {
		log.Fatal(err)
	}

	return rootPageTemplate(string(templateBytes))
}

func getLocalNames() []string {
	hostname, err := os.Hostname()
	if err != nil {
		log.Fatal(err)
	}

	names := []string{hostname}

	ifaces, err := net.Interfaces()
	if err != nil {
		log.Fatal(err)
	}

	for _, iface := range ifaces {
		addrs, err := iface.Addrs()
		if err != nil {
			log.Fatal(err)
		}

		for _, addr := range addrs {
			switch v := addr.(type) {
			case *net.IPNet:
				if !v.IP.IsLoopback() && v.IP.To4() != nil {
					names = append(names, v.IP.String())
				}

			case *net.IPAddr:
				if !v.IP.IsLoopback() && v.IP.To4() != nil {
					names = append(names, v.IP.String())
				}
			}
		}
	}

	return names
}

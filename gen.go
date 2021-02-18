// Ignore file as this is only used during 'go generate'
// +build ignore

package main

import (
	"fmt"
	"os"
	"path"
	"path/filepath"
	"regexp"

	"github.com/tdewolff/minify/v2"
	"github.com/tdewolff/minify/v2/css"
	"github.com/tdewolff/minify/v2/html"
	"github.com/tdewolff/minify/v2/js"
)

func main() {
	minifier := minify.New()
	minifier.AddFunc("text/css", css.Minify)
	minifier.AddFunc("text/html", html.Minify)
	minifier.AddFuncRegexp(regexp.MustCompile("^(application|text)/(x-)?(java|ecma)script$"), js.Minify)

	fmt.Println("Generating files:")

	files, _ := filepath.Glob("static/*")
	for _, goFile := range files {
		fmt.Println(goFile)

		fileContent, err := os.ReadFile(goFile)
		if err != nil {
			panic(err)
		}

		var minified []byte

		ext := path.Ext(goFile)
		if ext == ".htm" || ext == ".html" {
			minified, err = minifier.Bytes("text/html", fileContent)
			if err != nil {
				panic(err)
			}
		} else {
			minified = fileContent
		}

		a := filepath.Join("out", goFile)

		err = os.MkdirAll(filepath.Dir(a), 0777)
		if err != nil {
			panic(err)
		}

		fmt.Println(path.Dir(a))
		fmt.Println(a)

		err = os.WriteFile(a, minified, 0666)
		if err != nil {
			panic(err)
		}
	}
}

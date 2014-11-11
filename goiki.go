package main

import (
	// stdlib
	"flag"
	"fmt"
	"io/ioutil"
	"log"
	"net"
	"net/http"
	"path/filepath"
	"regexp"
	"text/template"

	// external
	"github.com/russross/blackfriday"
)

const (
	GOIKIVERSION = "0.0.1"
)

var (
	displayVersion bool
	config         Config
)

type Config struct {
	Port    int
	Host    string
	DataDir string
}

type Page struct {
	Title string
	Body  string
}

func init() {
	flag.IntVar(&config.Port, "port", 4567, "Bind port")
	flag.StringVar(&config.Host, "host", "0.0.0.0", "Hostname or IP address to listen on")
	flag.StringVar(&config.DataDir, "data-dir", "./data", "Directory for page data")
	flag.BoolVar(&displayVersion, "version", false, "Display version and exit")
}

func (p *Page) save() error {
	filename := buildFilename(p.Title)
	return ioutil.WriteFile(filename, []byte(p.Body), 0600)
}

func buildFilename(title string) string {
	return filepath.Join(config.DataDir, (title + ".txt"))
}

func loadPage(title string) (*Page, error) {
	filename := buildFilename(title)
	body, err := ioutil.ReadFile(filename)
	if err != nil {
		return nil, err
	}
	return &Page{Title: title, Body: string(body)}, nil
}

func processLinks(content []byte) []byte {
	re := regexp.MustCompile(`\[([^\]]+)]\(\)`)
	return re.ReplaceAllFunc(content, func(match []byte) []byte {
		return re.ReplaceAll(match, []byte("[$1]($1)"))
	})
}

func viewHandler(w http.ResponseWriter, r *http.Request, title string) {
	p, err := loadPage(title)
	if err != nil {
		http.Redirect(w, r, "/edit/"+title, http.StatusFound)
		return
	}

	content := []byte(p.Body)
	content = processLinks(content)
	content = blackfriday.MarkdownCommon(content)
	p.Body = string(content)

	renderTemplate(w, "view", p)
}

func editHandler(w http.ResponseWriter, r *http.Request, title string) {
	p, err := loadPage(title)
	if err != nil {
		p = &Page{Title: title}
	}
	renderTemplate(w, "edit", p)
}

func saveHandler(w http.ResponseWriter, r *http.Request, title string) {
	body := r.FormValue("body")
	p := &Page{Title: title, Body: body}
	err := p.save()
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	http.Redirect(w, r, "/view/"+title, http.StatusFound)
}

var templates = template.Must(template.ParseFiles("templates/edit.html", "templates/view.html"))

func renderTemplate(w http.ResponseWriter, tmpl string, p *Page) {
	err := templates.ExecuteTemplate(w, tmpl+".html", p)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
	}
}

var validPath = regexp.MustCompile("^/(edit|save|view)/([a-zA-Z0-9]+)$")

func makeHandler(fn func(http.ResponseWriter, *http.Request, string)) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		m := validPath.FindStringSubmatch(r.URL.Path)
		if m == nil {
			http.NotFound(w, r)
			return
		}
		fn(w, r, m[2])
	}
}

func main() {
	flag.Parse()

	// Display version and exit
	if displayVersion {
		fmt.Println("Goiki", GOIKIVERSION)
		return
	}

	http.Handle("/static/", http.FileServer(http.Dir("./")))
	http.HandleFunc("/view/", makeHandler(viewHandler))
	http.HandleFunc("/edit/", makeHandler(editHandler))
	http.HandleFunc("/save/", makeHandler(saveHandler))

	l, err := net.Listen("tcp", fmt.Sprintf("%s:%d", config.Host, config.Port))
	if err != nil {
		log.Fatal(err)
	}

	s := &http.Server{}
	s.Serve(l)
	return
}

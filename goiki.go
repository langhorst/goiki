package main

import (
	// stdlib
	"bytes"
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
	"github.com/VictorLowther/go-git/git"
	"github.com/russross/blackfriday"
)

const (
	GOIKIVERSION = "0.0.1"
)

var (
	displayVersion bool
	config         Config
	templates      *template.Template
	validPath      *regexp.Regexp
	validLink      *regexp.Regexp
	repo           *git.Repo
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

	templates = template.Must(template.ParseFiles("templates/edit.html", "templates/view.html"))
	validPath = regexp.MustCompile("^/(edit|save|view)/([a-zA-Z0-9/]+)$")
	validLink = regexp.MustCompile(`\[([^\]]+)]\(\)`)
}

func (p *Page) save() error {
	filename := buildFilename(p.Title)
	err := ioutil.WriteFile(filepath.Join(config.DataDir, filename), []byte(p.Body), 0600)
	if err != nil {
		return err
	}

	stdOut, stdErr := gitAdd(filename)
	if stdErr.Len() > 0 {
		return fmt.Errorf("Unable to add %s", filename)
	}
	log.Println("add:", stdOut)

	message := fmt.Sprintf("Update %s", filename)
	stdOut, stdErr = gitCommit(message)
	if stdErr.Len() > 0 {
		return fmt.Errorf("Unable to commit message: %s", message)
	}
	log.Println("commit:", stdOut)

	return nil
}

func buildFilename(title string) string {
	return title + ".txt"
}

func loadPage(title string, revision string) (*Page, error) {
	filename := buildFilename(title)
	if len(revision) == 0 {
		revision = "HEAD"
	}
	body, err := gitShow(filename, revision)
	if err.Len() > 0 {
		return &Page{Title: title, Body: body.String()}, fmt.Errorf("Unable to load page content from %s at %s\n", filename, revision)
	}
	return &Page{Title: title, Body: body.String()}, nil
}

func gitShow(file string, revision string) (out, err *bytes.Buffer) {
	res, out, err := repo.Git("show", fmt.Sprintf("%s:%s", revision, file))
	runErr := res.Run()
	if runErr != nil {
		log.Println("Unable to load revision %s from %s, error: %v\n", revision, file, runErr)
	}
	return
}

func gitAdd(file string) (out, err *bytes.Buffer) {
	res, out, err := repo.Git("add", file)
	runErr := res.Run()
	if runErr != nil {
		log.Printf("Unable to add file %s, error: %v\n", file, runErr)
	}
	return
}

func gitCommit(message string) (out, err *bytes.Buffer) {
	res, out, err := repo.Git("commit", "-m", message)
	runErr := res.Run()
	if runErr != nil {
		log.Println("Unable to commit message %s, error: %v\n", message, runErr)
	}
	return
}

func gitLog(file string) (out, err *bytes.Buffer) {
	res, out, err := repo.Git("log", "--pretty=format:%h %ad %s", "--date=relative", file)
	runErr := res.Run()
	if runErr != nil {
		log.Println("Unable to load log of %s, error: %v\n", file, runErr)
	}
	return
}

func processLinks(content []byte) []byte {
	return validLink.ReplaceAllFunc(content, func(match []byte) []byte {
		return validLink.ReplaceAll(match, []byte("[$1]($1)"))
	})
}

func viewHandler(w http.ResponseWriter, r *http.Request, title string) {
	p, err := loadPage(title, "HEAD")
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
	p, err := loadPage(title, "HEAD")
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

func renderTemplate(w http.ResponseWriter, tmpl string, p *Page) {
	err := templates.ExecuteTemplate(w, tmpl+".html", p)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
	}
}

func makeHandler(fn func(http.ResponseWriter, *http.Request, string)) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		path := r.URL.Path
		if path == "/" {
			path = "/view/FrontPage"
		}
		fmt.Println(path)
		m := validPath.FindStringSubmatch(path)
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

	// Load repo
	var err error
	if repo, err = git.Open(config.DataDir); err != nil {
		log.Fatalf("Unable to open the repo at %s. Please check to make sure it exists and is initialized.\n%v", config.DataDir, err)
	}

	// Define routes
	http.Handle("/static/", http.FileServer(http.Dir("./")))
	http.HandleFunc("/", makeHandler(viewHandler))
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

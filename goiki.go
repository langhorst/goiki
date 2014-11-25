package main

import (
	// stdlib
	"flag"
	"fmt"
	"io/ioutil"
	"log"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"regexp"
	"text/template"

	// external
	"github.com/VictorLowther/go-git/git"
	auth "github.com/abbot/go-http-auth"
	"github.com/russross/blackfriday"
)

const (
	GOIKIVERSION = "0.0.1"
)

var (
	configFile     string
	displayVersion bool
	conf           config
	templates      *template.Template
	validPath      *regexp.Regexp
	validLink      *regexp.Regexp
	authenticator  *auth.BasicAuth
)

type Page struct {
	Author      author
	Title       string
	Body        string
	Description string
	Site        config
	Revisions   []gitRevision
}

type SearchPage struct {
	Title   string
	Results []gitResult
	Site    config
}

func (p *Page) save() error {
	filename := fileName(p.Title)
	datapath := dataPath(conf.DataDir, filename)

	err := os.MkdirAll(filepath.Dir(datapath), 0777)
	if err != nil {
		return err
	}
	err = ioutil.WriteFile(datapath, []byte(p.Body), 0600)
	if err != nil {
		return err
	}

	_, err = gitAdd(filename)
	if err != nil {
		return err
	}

	message := p.Description
	if len(message) == 0 {
		message = fmt.Sprintf("Update %s", filename)
	}
	stdout, err := gitCommit(message, p.Author)
	if err != nil {
		return err
	}
	log.Println(stdout)

	return nil
}

func fileName(title string) string {
	return title + ".txt"
}

func dataPath(dir string, file string) string {
	return filepath.Join(dir, file)
}

func loadPage(title string, revision string) (*Page, error) {
	filename := fileName(title)
	if len(revision) == 0 {
		revision = "HEAD"
	}
	body, err := gitShow(filename, revision)
	if err != nil {
		return &Page{
			Title: title,
			Body:  body.String(),
			Site:  conf,
		}, fmt.Errorf("Unable to load page content from %s at %s\n", filename, revision)
	}
	return &Page{Title: title, Body: body.String(), Site: conf}, nil
}

func processLinks(content []byte, link *regexp.Regexp) []byte {
	return link.ReplaceAllFunc(content, func(match []byte) []byte {
		return link.ReplaceAll(match, []byte("[$1]($1)"))
	})
}

func renderTemplate(w http.ResponseWriter, tmpl string, p *Page) {
	err := templates.ExecuteTemplate(w, tmpl, p)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
	}
}

func renderSearchTemplate(w http.ResponseWriter, tmpl string, p *SearchPage) {
	err := templates.ExecuteTemplate(w, tmpl, p)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
	}
}

func makeHandler(fn func(http.ResponseWriter, *http.Request, string)) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		path := r.URL.Path
		log.Println(path)
		if path == "/" {
			http.Redirect(w, r, "/view/FrontPage", http.StatusFound)
			return
		}
		m := validPath.FindStringSubmatch(path)
		if m == nil {
			http.NotFound(w, r)
			return
		}
		fn(w, r, m[2])
	}
}

func makeAuthHandler(fn func(http.ResponseWriter, *auth.AuthenticatedRequest, string)) auth.AuthenticatedHandlerFunc {
	return func(w http.ResponseWriter, r *auth.AuthenticatedRequest) {
		path := r.URL.Path
		log.Println(path)
		m := validPath.FindStringSubmatch(path)
		if m == nil {
			http.NotFound(w, &r.Request)
			return
		}
		fn(w, r, m[2])
	}
}

func viewHandler(w http.ResponseWriter, r *http.Request, title string) {
	revision := r.FormValue("revision")
	if revision == "" {
		revision = "HEAD"
	}

	p, err := loadPage(title, revision)
	if err != nil {
		http.Redirect(w, r, "/edit/"+title, http.StatusFound)
		return
	}

	content := []byte(p.Body)
	content = processLinks(content, validLink)
	content = blackfriday.MarkdownCommon(content)
	p.Body = string(content)

	renderTemplate(w, "view", p)
}

func editHandler(w http.ResponseWriter, r *auth.AuthenticatedRequest, title string) {
	revision := r.FormValue("revision")
	if revision == "" {
		revision = "HEAD"
	}

	p, err := loadPage(title, revision)
	if err != nil {
		p = &Page{Title: title, Site: conf}
	}

	renderTemplate(w, "edit", p)
}

func saveHandler(w http.ResponseWriter, r *auth.AuthenticatedRequest, title string) {
	body := r.FormValue("body")
	description := r.FormValue("description")
	user := conf.Auth[r.Username]
	author := author{Name: user.Name, Email: user.Email}
	p := &Page{Title: title, Body: body, Description: description, Author: author}
	err := p.save()
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	http.Redirect(w, &r.Request, "/view/"+title, http.StatusFound)
}

func historyHandler(w http.ResponseWriter, r *http.Request, title string) {
	revisions, _ := gitLog(fileName(title))
	p := &Page{Title: title, Body: "", Site: conf, Revisions: revisions}
	renderTemplate(w, "history", p)
}

func searchHandler(w http.ResponseWriter, r *http.Request) {
	search := r.FormValue("search")
	results, err := gitGrep(search)
	if err != nil {
		log.Println("error in search", err)
	}
	sp := &SearchPage{Title: "Search", Results: results, Site: conf}
	renderSearchTemplate(w, "search", sp)
}

// Initialize the configuration options, templates and URL and link regexes.
func init() {
	flag.StringVar(&configFile, "config", "./goiki.toml", "Location of configuration file")
	flag.BoolVar(&displayVersion, "version", false, "Display version and exit")

	flag.Parse()

	var err error
	conf, err = loadConfig(configFile)
	if err != nil {
		panic(err)
	}

	conf.loadAuth()

	authenticator = auth.NewBasicAuthenticator(serviceAddress(conf.Host, conf.Port), secret)
	templates = template.Must(template.ParseFiles("templates/_header.html", "templates/_footer.html", "templates/edit.html", "templates/view.html", "templates/history.html", "templates/search.html"))
	validPath = regexp.MustCompile("^/(edit|save|view|history)/([a-zA-Z0-9/]+)$")
	validLink = regexp.MustCompile(`\[([^\]]+)]\(\)`)
}

func secret(username, realm string) string {
	return conf.Auth[username].Password
}

func serviceAddress(host string, port int) string {
	return fmt.Sprintf("%s:%d", host, port)
}

func main() {
	// Display version and exit (flag -version)
	if displayVersion {
		fmt.Println("Goiki", GOIKIVERSION)
		return
	}

	// Load repo
	var err error
	if repo, err = git.Open(conf.DataDir); err != nil {
		log.Fatalf("Unable to open the repo at %s. Please check to make sure it exists and is initialized.\n%v", conf.DataDir, err)
	}

	// Define routes
	//http.HandleFunc("/", authenticator.Wrap(authHandler))
	http.Handle("/static/", http.FileServer(http.Dir("./")))
	http.HandleFunc("/search/", searchHandler)
	http.HandleFunc("/", makeHandler(viewHandler))
	http.HandleFunc("/view/", makeHandler(viewHandler))
	http.HandleFunc("/history/", makeHandler(historyHandler))

	http.HandleFunc("/edit/", authenticator.Wrap(makeAuthHandler(editHandler)))
	http.HandleFunc("/save/", authenticator.Wrap(makeAuthHandler(saveHandler)))

	l, err := net.Listen("tcp", serviceAddress(conf.Host, conf.Port))
	if err != nil {
		log.Fatal(err)
	}

	s := &http.Server{}
	s.Serve(l)
	return
}

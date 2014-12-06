package main

import (
	// stdlib
	"encoding/base64"
	"flag"
	"fmt"
	"io/ioutil"
	"log"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"text/template"

	// external
	"github.com/VictorLowther/go-git/git"
	auth "github.com/abbot/go-http-auth"
	"github.com/russross/blackfriday"
)

const (
	GOIKIVERSION = "0.2.0"
)

var (
	configFile     string
	displayConfig  bool
	displayVersion bool
	templateFiles  map[string]string
	conf           config
	templates      *template.Template
	validPath      *regexp.Regexp
	validLink      *regexp.Regexp
	authenticator  *auth.BasicAuth
)

type page struct {
	SiteName    string
	Title       string
	Author      author
	Body        string
	Description string
	Revisions   []pageRevision
}

type searchPage struct {
	SiteName string
	Title    string
	Results  []searchResult
}

type historyPage struct {
	SiteName  string
	Title     string
	Revisions []pageRevision
}

func (p *page) save() error {
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
	return title + "." + conf.FileExtension
}

func dataPath(dir string, file string) string {
	return filepath.Join(dir, file)
}

func loadPage(title string, revision string) (*page, error) {
	filename := fileName(title)
	if len(revision) == 0 {
		revision = "HEAD"
	}
	body, err := gitShow(filename, revision)
	if err != nil {
		return &page{
			Title:    title,
			Body:     body.String(),
			SiteName: conf.Name,
		}, fmt.Errorf("Unable to load page content from %s at %s\n", filename, revision)
	}
	return &page{Title: title, Body: body.String(), SiteName: conf.Name}, nil
}

func processLinks(content []byte, link *regexp.Regexp) []byte {
	return link.ReplaceAllFunc(content, func(match []byte) []byte {
		return link.ReplaceAll(match, []byte("[$1]($1)"))
	})
}

/*
TODO: How to combine the three functions? With use of an interface for the page?
*/

func renderTemplate(w http.ResponseWriter, tmpl string, p *page) {
	err := templates.ExecuteTemplate(w, tmpl, p)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
	}
}

func renderHistoryTemplate(w http.ResponseWriter, tmpl string, p *historyPage) {
	err := templates.ExecuteTemplate(w, tmpl, p)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
	}
}

func renderSearchTemplate(w http.ResponseWriter, tmpl string, p *searchPage) {
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
			path = "/view/"
		}
		if path[len(path)-1:len(path)] == "/" {
			viewIndex := path + conf.IndexPage
			http.Redirect(w, r, viewIndex, http.StatusFound)
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
		p = &page{Title: title, SiteName: conf.Name}
	}

	renderTemplate(w, "edit", p)
}

func saveHandler(w http.ResponseWriter, r *auth.AuthenticatedRequest, title string) {
	body := r.FormValue("body")
	description := r.FormValue("description")
	user := conf.Auth[r.Username]
	author := author{Name: user.Name, Email: user.Email}
	p := &page{Title: title, Body: body, Description: description, Author: author}
	err := p.save()
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	http.Redirect(w, &r.Request, "/view/"+title, http.StatusFound)
}

func historyHandler(w http.ResponseWriter, r *http.Request, title string) {
	revisions, _ := gitLog(fileName(title))
	p := &historyPage{Title: title, Revisions: revisions, SiteName: conf.Name}
	renderHistoryTemplate(w, "history", p)
}

func searchHandler(w http.ResponseWriter, r *http.Request) {
	search := r.FormValue("search")
	results, err := gitGrep(search)
	if err != nil {
		log.Println("error in search", err)
	}
	p := &searchPage{Title: "Search", Results: results, SiteName: conf.Name}
	renderSearchTemplate(w, "search", p)
}

func bundleHandler(w http.ResponseWriter, r *http.Request) {
	p := strings.TrimLeft(r.URL.Path, "/")
	f, ok := _bundle[p]
	if ok {
		w.Write([]byte(f))
	} else {
		http.NotFound(w, r)
	}
}

func defaultConfig() string {
	return _bundle["goiki.toml"]
}

func loadBundle() {
	for file, content := range _bundle {
		c, err := base64.StdEncoding.DecodeString(content)
		if err != nil {
			log.Printf("Error during loadBundle(): %v", err)
		}
		_bundle[file] = string(c)
	}
}

// Initialize the configuration options, templates and URL and link regexes.
func init() {
	flag.StringVar(&configFile, "c", "", "Specify a configuration file (use default config otherwise)")
	flag.BoolVar(&displayConfig, "d", false, "Display default configuration and exit")
	flag.BoolVar(&displayVersion, "v", false, "Display version and exit")

	loadBundle()

	templateFiles = map[string]string{"header": "_header.html", "footer": "_footer.html", "edit": "edit.html",
		"history": "history.html", "search": "search.html", "view": "view.html"}
	validPath = regexp.MustCompile("^/(edit|save|view|history)/([a-zA-Z0-9/_-]+)$")
	validLink = regexp.MustCompile(`\[([^\]]+)]\(\)`)
}

func secret(username, realm string) string {
	return conf.Auth[username].Password
}

func serviceAddress(host string, port int) string {
	return fmt.Sprintf("%s:%d", host, port)
}

func main() {
	flag.Parse()

	// Display version and exit (flag -version)
	if displayVersion {
		fmt.Println("Goiki", GOIKIVERSION)
		return
	}

	// Display the default configuration set and exit (flag -default)
	if displayConfig {
		fmt.Println(defaultConfig())
		return
	}

	// Load the configuration. Use the default embedded configuration unless
	// a config file was specified at the command line.
	var err error
	if len(configFile) == 0 {
		conf, err = loadConfig(defaultConfig())
	} else {
		conf, err = loadConfigFromFile(configFile)
	}

	if err != nil {
		fmt.Printf("FATAL: Unable to load configuration: %v\n", err)
		return
	}

	// Load authentication from the config and run the authenticator.
	conf.loadAuth()
	authenticator = auth.NewBasicAuthenticator(serviceAddress(conf.Host, conf.Port), secret)

	// Load the templates. Use the default embedded templates unless a directory
	// of templates is specified in configuration.
	if len(conf.TemplateDir) == 0 {
		templates = template.Must(template.New("goiki").Parse(""))
		for _, file := range templateFiles {
			data, _ := ioutil.ReadFile(filepath.Join("templates", file))
			template.Must(templates.Parse(string(data)))
		}
	} else {
		templateLocations := make([]string, len(templateFiles))
		i := 0
		for _, file := range templateFiles {
			templateLocations[i] = filepath.Join(conf.TemplateDir, file)
			i += 1
		}
		templates = template.Must(template.ParseFiles(templateLocations...))
	}

	// Load the repository.
	if repo, err = git.Open(conf.DataDir); err != nil {
		log.Fatalf("Unable to open the repo at %s. Please check to make sure it exists and is initialized.\n%v", conf.DataDir, err)
	}

	// Static routes
	// If a static directory is provided in the configuration, use that; otherwise
	// use the embedded static content
	if len(conf.StaticDir) > 0 {
		http.Handle("/static/", http.StripPrefix("/static/", http.FileServer(http.Dir(conf.StaticDir))))
	} else {
		http.Handle("/static/", http.FileServer(FS(false)))
	}

	// Unathenticated routes
	http.HandleFunc("/search/", searchHandler)
	http.HandleFunc("/", makeHandler(viewHandler))
	http.HandleFunc("/view/", makeHandler(viewHandler))
	http.HandleFunc("/history/", makeHandler(historyHandler))

	// Authenticated routes
	http.HandleFunc("/edit/", authenticator.Wrap(makeAuthHandler(editHandler)))
	http.HandleFunc("/save/", authenticator.Wrap(makeAuthHandler(saveHandler)))

	address := serviceAddress(conf.Host, conf.Port)

	log.Printf("Listening on %v\n", address)

	l, err := net.Listen("tcp", address)
	if err != nil {
		log.Fatal(err)
	}

	s := &http.Server{}
	s.Serve(l)
	return
}

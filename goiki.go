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
	"os"
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
	Name    string
}

type Author struct {
	Name  string
	Email string
}

type Revision struct {
	Object      string
	Title       string
	Description string
	Author      Author
	Timestamp   string
}

type Page struct {
	Author      Author
	Title       string
	Body        string
	Description string
	Site        Config
	Revisions   []Revision
}


func (p *Page) save() error {
	filename := fileName(p.Title)
	datapath := dataPath(config.DataDir, filename)

	err := os.MkdirAll(filepath.Dir(datapath), 0777)
	if err != nil {
		return err
	}
	err = ioutil.WriteFile(datapath, []byte(p.Body), 0600)
	if err != nil {
		return err
	}

	stdOut, err := gitAdd(filename)
	if err != nil {
		return err
	}
	log.Println("add:", stdOut)

	message := p.Description
	if len(message) == 0 {
		message = fmt.Sprintf("Update %s", filename)
	}
	stdOut, err = gitCommit(message)
	if err != nil {
		return err
	}
	log.Println("commit:", stdOut)

	return nil
}

func fileName(title string) string {
	return title + ".txt"
}

func dataPath(dir string, file string) string {
	return filepath.Join(dir, file)
}

func gitExec(command string, args ...string) (out *bytes.Buffer, err error) {
	err = nil
	res, out, stderr := repo.Git(command, args...)
	runErr := res.Run()
	if runErr != nil {
		return out, runErr
	} else if stderr.Len() > 0 {
		return out, fmt.Errorf(stderr.String())
	}
	return
}

func gitShow(file string, revision string) (out *bytes.Buffer, err error) {
	return gitExec("show", fmt.Sprintf("%s:%s", revision, file))
}

func gitAdd(file string) (out *bytes.Buffer, err error) {
	return gitExec("add", file)
}

func gitCommit(message string) (out *bytes.Buffer, err error) {
	return gitExec("commit", "-m", message)
}

func gitLog(file string) (out *bytes.Buffer, err error) {
	return gitExec("log", "--pretty=format:%h %ad %s", "--date=relative", file)
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
			Site:  config,
		}, fmt.Errorf("Unable to load page content from %s at %s\n", filename, revision)
	}
	return &Page{Title: title, Body: body.String(), Site: config}, nil
}

func parseLog(bytes []byte) *Revision {
	line := string(bytes)
	re := regexp.MustCompile(`(.{0,7}) (\d+ \w+ ago) (.*)`)
	matches := re.FindStringSubmatch(line)
	if len(matches) == 4 {
		return &Revision{Object: matches[1], Timestamp: matches[2], Description: matches[3]}
	}
	return nil
	/* TODO
	   we want to show more information in the log; author for example
	   and then gather up the log, parsing it into Revisions (or rename to Log)
	   then we can use the historyHandler to display all available revisions and
	   provide links to show that content.

	   also look into what it takes to do diffs on the material as it should be
	   similar
	*/
}

// Process wiki links (empty Markdown link syntax)
func processLinks(content []byte, link *regexp.Regexp) []byte {
	return link.ReplaceAllFunc(content, func(match []byte) []byte {
		return link.ReplaceAll(match, []byte("[$1]($1)"))
	})
}

// Render a template to the given ResponseWriter along with the template name
// and Page object.
func renderTemplate(w http.ResponseWriter, tmpl string, p *Page) {
	err := templates.ExecuteTemplate(w, tmpl+".html", p)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
	}
}

// Make a handler.
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

// Handle viewing pages. Load a page revision and render it to the response
// writer. If the page is not found, redirect to the edit page for the given
// title to create a new page.
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

// Handle editing pages. Load a page revision and render the raw content to
// the response writer. If the page is not found, we simply provide an empty
// page in order to create/save a new page.
func editHandler(w http.ResponseWriter, r *http.Request, title string) {
	revision := r.FormValue("revision")
	if revision == "" {
		revision = "HEAD"
	}

	p, err := loadPage(title, revision)
	if err != nil {
		p = &Page{Title: title}
	}

	renderTemplate(w, "edit", p)
}

// Handle saving pages.
func saveHandler(w http.ResponseWriter, r *http.Request, title string) {
	body := r.FormValue("body")
	description := r.FormValue("description")
	p := &Page{Title: title, Body: body, Description: description}
	err := p.save()
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	http.Redirect(w, r, "/view/"+title, http.StatusFound)
}

// Handle viewing page history.
func historyHandler(w http.ResponseWriter, r *http.Request, title string) {
	author := Author{Name: "Anonymous", Email: ""}
	stdOut, logErr := gitLog(fileName(title))
	if logErr != nil {
		log.Println(logErr)
	}
	var bytes []byte
	var err error
	revisions := make([]Revision, 0)
	for err == nil {
		bytes, err = stdOut.ReadBytes('\n')
		revision := parseLog(bytes)
		if revision == nil {
			continue
		}
		revision.Author = author
		revision.Title = title
		revisions = append(revisions, *revision)
	}
	p := &Page{Title: title, Body: "", Site: config, Revisions: revisions, Author: author}
	renderTemplate(w, "history", p)
}

// Initialize the configuration options, templates and URL and link regexes.
func init() {
	flag.StringVar(&config.Name, "name", "Goiki", "Wiki name")
	flag.IntVar(&config.Port, "port", 4567, "Bind port")
	flag.StringVar(&config.Host, "host", "0.0.0.0", "Hostname or IP address to listen on")
	flag.StringVar(&config.DataDir, "data-dir", "./data", "Directory for page data")
	flag.BoolVar(&displayVersion, "version", false, "Display version and exit")

	templates = template.Must(template.ParseFiles("templates/edit.html", "templates/view.html", "templates/history.html"))
	validPath = regexp.MustCompile("^/(edit|save|view|history)/([a-zA-Z0-9/]+)$")
	validLink = regexp.MustCompile(`\[([^\]]+)]\(\)`)
}

// Main. Parse the configuration flags and start the web server based on those
// configuration flags.
func main() {
	flag.Parse()

	// Display version and exit
	if displayVersion {
		fmt.Println("Goiki", GOIKIVERSION)
		return
	}

	log.Println(config)

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
	http.HandleFunc("/history/", makeHandler(historyHandler))

	l, err := net.Listen("tcp", fmt.Sprintf("%s:%d", config.Host, config.Port))
	if err != nil {
		log.Fatal(err)
	}

	s := &http.Server{}
	s.Serve(l)
	return
}

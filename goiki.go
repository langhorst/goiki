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
	repo           *git.Repo
	authenticator  *auth.BasicAuth
	users          map[string]user
)

/*
type Config struct {
	Name        string
	Port        int
	Host        string
	DataDir     string
	TemplateDir string
	Users       map[string]User
	AuthFile    string
	Name        string
}
*/

/*
type User struct {
	Username string
	Password string
	Name     string
	Email    string
}
*/

type Author struct {
	Name  string
	Email string
}

func (a *Author) String() string {
	return fmt.Sprintf("%s <%s>", a.Name, a.Email)
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
	Site        config
	Revisions   []Revision
}

type Result struct {
	Title   string
	Content string
}

type SearchPage struct {
	Title   string
	Results []Result
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

func gitCommit(message string, author Author) (out *bytes.Buffer, err error) {
	if author.String() == "" {
		return gitExec("commit", "-m", message)
	}
	return gitExec("commit", "-m", message, "--author", author.String())
}

func gitLog(file string) (out *bytes.Buffer, err error) {
	return gitExec("log", "--pretty=format:%h %an <%ae> %ad %s", "--date=relative", file)
}

func gitGrep(keyword string) (out *bytes.Buffer, err error) {
	log.Println("grep keyword", keyword)
	return gitExec("grep", "--ignore-case", keyword)
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

func parseLog(bytes []byte) *Revision {
	line := string(bytes)
	re := regexp.MustCompile(`(.{0,7}) (.+) (<.+>) (\d+ \w+ ago) (.*)`)
	matches := re.FindStringSubmatch(line)
	if len(matches) == 6 {
		return &Revision{Object: matches[1], Author: Author{Name: matches[2], Email: matches[3]}, Timestamp: matches[4], Description: matches[5]}
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

func parseGrepOutput(output *bytes.Buffer) []Result {
	var err error
	var bytes []byte
	results := make([]Result, 0)

	log.Println("grep output", output.String())
	re := regexp.MustCompile(`(.+)\.txt:(.*)`)
	for err == nil {
		bytes, err = output.ReadBytes('\n')
		matches := re.FindStringSubmatch(string(bytes))
		if len(matches) == 3 {
			results = append(results, Result{Title: matches[1], Content: matches[2]})
		}
	}
	return results
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
	user := users[r.Username]
	author := Author{Name: user.Name, Email: user.Email}
	p := &Page{Title: title, Body: body, Description: description, Author: author}
	err := p.save()
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	http.Redirect(w, &r.Request, "/view/"+title, http.StatusFound)
}

func historyHandler(w http.ResponseWriter, r *http.Request, title string) {
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
		revision.Title = title
		revisions = append(revisions, *revision)
	}
	p := &Page{Title: title, Body: "", Site: conf, Revisions: revisions}
	renderTemplate(w, "history", p)
}

func searchHandler(w http.ResponseWriter, r *http.Request) {
	search := r.FormValue("search")
	grep, err := gitGrep(search)
	if err != nil {
		log.Println("error in search", err)
	}

	results := parseGrepOutput(grep)
	sp := &SearchPage{Title: "Search", Results: results, Site: conf}
	renderSearchTemplate(w, "search", sp)
}

// Initialize the configuration options, templates and URL and link regexes.
func init() {
	//flag.StringVar(&conf.AuthFile, "auth", "./auth.json", "File containing user authentication")
	flag.StringVar(&configFile, "config", "./goiki.toml", "Location of configuration file")
	flag.BoolVar(&displayVersion, "version", false, "Display version and exit")

	flag.Parse()

	//users, _ = loadAuth(conf.AuthFile)
	conf, err := loadConfig(configFile)
	if err != nil {
		panic(err)
	}

	authenticator = auth.NewBasicAuthenticator(serviceAddress(conf.Host, conf.Port), secret)
	templates = template.Must(template.ParseFiles("templates/_header.html", "templates/_footer.html", "templates/edit.html", "templates/view.html", "templates/history.html", "templates/search.html"))
	validPath = regexp.MustCompile("^/(edit|save|view|history)/([a-zA-Z0-9/]+)$")
	validLink = regexp.MustCompile(`\[([^\]]+)]\(\)`)
}

func secret(username, realm string) string {
	return users[username].Password
}

func serviceAddress(host string, port int) string {
	return fmt.Sprintf("%s:%d", host, port)
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

	log.Println(conf)

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

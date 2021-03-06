package server

import (
	"html/template"
	"net/http"
	"path"
	"strings"
	"time"
	"io/ioutil"
	"regexp"

	"github.com/Unknwon/com"
	"gopkg.in/inconshreveable/log15.v2"
)

// Server is built-in http server address
type Server struct {
	dstDir string
	prefix string
}

// New create new server on dstDir
func New(dstDir string) *Server {
	s := &Server{
		dstDir: dstDir,
	}
	s.SetPrefix("")
	return s
}

// SetPrefix set prefix to trim url
func (s *Server) SetPrefix(prefix string) {
	if prefix == "" {
		prefix = "/"
	}
	s.prefix = prefix
}

// GetPrefix get prefix
func (s *Server) GetPrefix() string {
	return s.prefix
}

func (s *Server) serveFile(w http.ResponseWriter, r *http.Request, file string) bool {
	if com.IsFile(file) {
		log15.Debug("Server|Dest|%s", file)

		if !strings.HasSuffix(file, ".html") {
			http.ServeFile(w, r, file)
			return true
		}

		replacedData, err := readAndReplacedFile(file)
		if err != nil {
			log15.Error(err.Error())
			return false
		}
		tmpl, err := buildTemplate(file, replacedData)
		if err != nil {
			log15.Error(err.Error())
			return false
		}
		tmpl.Execute(w, nil)
		if respW, ok := w.(*responseWriter); ok {
			respW.status = 200
		}
		return true
	}
	return false
}

func (s *Server) serveFiles(w http.ResponseWriter, r *http.Request, param string) bool {
	ext := path.Ext(param)
	if ext == "" || ext == "." {
		// /xyz -> /xyz.html
		if !strings.HasSuffix(param, "/") {
			if s.serveFile(w, r, path.Join(s.dstDir, s.prefix, param+".html")) {
				return true
			}
		}
		// /xyz/ -> /xyz/index.html
		if s.serveFile(w, r, path.Join(s.dstDir, s.prefix, param, "index.html")) {
			return true
		}
		// /xyz/ -> /xzy.html
		param = strings.TrimSuffix(param, "/")
		if s.serveFile(w, r, path.Join(s.dstDir, s.prefix, param+".html")) {
			return true
		}
	}
	if s.serveFile(w, r, path.Join(s.dstDir, s.prefix, param)) {
		return true
	}
	return false
}

// ServeHTTP implement http.Handler
func (s *Server) ServeHTTP(rw http.ResponseWriter, r *http.Request) {
	w := &responseWriter{
		ResponseWriter: rw,
		startTime:      time.Now(),
	}

	defer func() {
		if err := recover(); err != nil {
			w.error = err
			if w.status == 0 {
				http.Error(w, http.StatusText(http.StatusServiceUnavailable), http.StatusServiceUnavailable)
			}
		}
		logger(w, r)
	}()

	param := r.URL.Path
	if param == "favicon.ico" || param == "robots.txt" {
		if !s.serveFiles(w, r, param) {
			http.NotFound(w, r)
		}
		return
	}
	if !strings.HasPrefix(param, s.prefix) {
		http.Redirect(w, r, s.prefix, 302)
		return
	}
	param = strings.TrimPrefix(param, s.prefix)
	s.serveFiles(w, r, param)
}

// Run run http server on addr
func (s *Server) Run(addr string) {
	log15.Info("Server|Start|%s", addr)
	if err := http.ListenAndServe(addr, s); err != nil {
		log15.Crit("Server|Start|%s", err.Error())
	}
}

type responseWriter struct {
	http.ResponseWriter
	status    int
	startTime time.Time
	error     interface{}
}

func (r *responseWriter) WriteHeader(status int) {
	r.status = status
	r.ResponseWriter.WriteHeader(status)
}

type notes []string

func (n notes) GenNote() string {
	if len(n) == 0 {
		return ""
	}
	t := time.Now().Unix()
	i := t%int64(len(n))
	return n[i]
}

func readNotes() notes {
	data, err := ioutil.ReadFile("source/phrase/phrase.md")
	if err != nil {
		log15.Error(err.Error())
		return []string{}
	}
	c := regexp.MustCompile("\r?\n\r?\n")
	res := c.Split(string(data), -1)
	return notes(res)
}

func readAndReplacedFile(file string) (string, error) {
	data, err := ioutil.ReadFile(file)
	if err != nil {
		return "", err
	}

	s := string(data)
	reg := regexp.MustCompile(`style="color:darkgrey\S*\s*\S*\s*\S*?div`)
	s = reg.ReplaceAllString(s, "style=\"color:darkgrey\">" + ns.GenNote() + "</div")
	return s, nil
}

func buildTemplate(file, content string) (*template.Template, error) {
	tpl := template.New(file)
	_, err := tpl.Parse(content)
	if err != nil {
		return nil, err
	}
	return tpl, nil
}

var ns notes

func init() {
	ns = readNotes()
}

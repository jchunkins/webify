package main

import (
	"flag"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"slices"
	"strings"

	"log/slog"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
	"github.com/go-chi/cors"
	"github.com/go-chi/httplog/v3"
	"github.com/go-chi/traceid"
	"github.com/golang-cz/devslog"
	"golang.org/x/term"
)

var (
	flags  = flag.NewFlagSet("webify", flag.ExitOnError)
	port   = flags.String("port", "3000", "http server port")
	host   = flags.String("host", "0.0.0.0", "http server hostname")
	dir    = flags.String("dir", ".", "directory to serve")
	cache  = flags.Bool("cache", false, "enable Cache-Control for content")
	debug  = flags.Bool("debug", false, "Debug mode, printing all network request details")
	echo   = flags.Bool("echo", false, "Echo back request body, useful for debugging")
	silent = flags.Bool("silent", false, "Do not output any logs")
	banner = flags.Bool("no-banner", false, "Do not output banner")
	level  = flags.String("log-level", "info", `Set the logging level:
  debug: log both request starts & responses (incl. OPTIONS)
  info: log responses (excl. OPTIONS)
  warn: log 4xx and 5xx responses only (except for 429)
  error: log 5xx responses only
`)
)

func main() {
	flags.Parse(os.Args[1:])

	// Setup params
	addr := fmt.Sprintf("%s:%s", *host, *port)
	cwd, _ := os.Getwd()
	if *dir == "" || *dir == "." {
		*dir = cwd
	} else {
		if (*dir)[0:1] != "/" {
			*dir = filepath.Join(cwd, *dir)
		}
		if _, err := os.Stat(*dir); os.IsNotExist(err) {
			fmt.Printf("Error: %s\n", err.Error())
			os.Exit(1)
		}
	}

	// Print banner
	if !*banner {
		fmt.Printf("================================================================================\n")
		fmt.Printf("Serving:  %s\n", *dir)
		fmt.Printf("URL:      http://%s\n", addr)
		if *cache {
			fmt.Printf("Cache:    on\n")
		} else {
			fmt.Printf("Cache:    off\n")
		}
		fmt.Printf("================================================================================\n")
		fmt.Printf("\n")
	}

	// check if we're running in localhost mode
	isLocalhost := os.Getenv("ENV") == "localhost"

	// run with "concise" when debug is false
	logFormat := httplog.SchemaECS.Concise(!*debug)

	logger := slog.New(logHandler(isLocalhost, &slog.HandlerOptions{
		AddSource:   !isLocalhost,
		ReplaceAttr: logFormat.ReplaceAttr,
	}))

	// Setup http router with file server
	r := chi.NewRouter()

	// as long as we're not running in silent mode, setup the logger
	if !*silent {
		r.Use(httplog.RequestLogger(logger, &httplog.Options{
			Level:          getLevel(*level),
			Schema:         logFormat,
			LogRequestBody: func(req *http.Request) bool { return *echo }, // log body if echo is enabled
			LogExtraAttrs: func(req *http.Request, reqBody string, respStatus int) []slog.Attr {
				// if debug is enabled...
				if *debug {
					// get the "constants" for ignore & obfuscate headers
					ignoreHeaders := getIgnoreHeaders()
					obfuscateHeaders := getObfuscateHeaders()

					attr := []slog.Attr{}
					for header, values := range req.Header {
						lcHeader := strings.ToLower(header)
						// convert the headers into slog.Attr as long as its a header we should NOT ignore
						if !slices.Contains(ignoreHeaders, lcHeader) {
							// potentially obfuscate the output for sensitive values
							if slices.Contains(obfuscateHeaders, lcHeader) {
								attr = append(attr, slog.String(header, "[REDACTED]"))
							} else {
								attr = append(attr, slog.String(header, strings.Join(values, ",")))
							}
						}
					}

					// place all the headers into their own group
					return []slog.Attr{
						{
							Key:   "headers",
							Value: slog.GroupValue(attr...),
						},
					}
				} else {
					// don't add anything for non-debug
					return []slog.Attr{}
				}
			},
		}))
	}

	// either use cache control (or not)
	if *cache {
		r.Use(CacheControl)
	} else {
		r.Use(middleware.NoCache)
	}

	// setup CORS
	cors := cors.New(cors.Options{
		// AllowedOrigins: []string{"https://foo.com"}, // Use this to allow specific origin hosts
		AllowedOrigins: []string{"*"},
		// AllowOriginFunc:  func(r *http.Request, origin string) bool { return true },
		AllowedMethods:   []string{"GET", "POST", "PUT", "DELETE", "OPTIONS"},
		AllowedHeaders:   []string{"Accept", "Authorization", "Content-Type", "X-CSRF-Token"},
		ExposedHeaders:   []string{"Link"},
		AllowCredentials: true,
		MaxAge:           300, // Maximum value not ignored by any of major browsers
	})
	r.Use(cors.Handler)

	// either go into echo mode or setup the file server
	if *echo {
		r.HandleFunc("/*", echoHandler)
	} else {
		FileServer(r, "/", http.Dir(*dir))
	}

	// Serve it up!
	err := http.ListenAndServe(addr, r)
	if err != nil {
		fmt.Printf("Error: %s\n", err.Error())
		os.Exit(1)
	}
}

// getLevel converts logLevelStr to a slog.Level
func getLevel(logLevelStr string) (level slog.Level) {
	switch strings.ToLower(logLevelStr) {
	case "debug":
		level = slog.LevelDebug // log both request starts & responses (incl. OPTIONS)
	case "info":
		level = slog.LevelInfo // log responses (excl. OPTIONS)
	case "warn":
		level = slog.LevelWarn // log 4xx and 5xx responses only (except for 429)
	case "error":
		level = slog.LevelError // log 5xx responses only
	default:
		slog.Warn("Invalid log level specified, defaulting to info", "level", logLevelStr)
		level = slog.LevelInfo
	}
	return
}

// logHandler either sets up the pretty printed logs or JSON logging.
// The variable isLocalhost controls which mode we run in. The variable handlerOpts
// is supplied for either scenario.
func logHandler(isLocalhost bool, handlerOpts *slog.HandlerOptions) slog.Handler {
	if isLocalhost {
		// Pretty logs for localhost development.
		return devslog.NewHandler(os.Stdout, &devslog.Options{
			SortKeys:           true,
			MaxErrorStackTrace: 5,
			MaxSlicePrintSize:  20,
			HandlerOptions:     handlerOpts,
			NoColor:            !term.IsTerminal(int(os.Stdout.Fd())), // if we're not in a terminal don't use color
		})
	}

	// JSON logs for production with "traceId".
	return traceid.LogHandler(
		slog.NewJSONHandler(os.Stdout, handlerOpts),
	)
}

// FileServer conveniently sets up a http.FileServer handler to serve
// static files from a http.FileSystem.
func FileServer(r chi.Router, path string, root http.FileSystem) {
	if strings.ContainsAny(path, "{}*") {
		panic("FileServer does not permit URL parameters.")
	}

	fs := http.StripPrefix(path, http.FileServer(root))

	if path != "/" && path[len(path)-1] != '/' {
		r.Get(path, http.RedirectHandler(path+"/", http.StatusMovedPermanently).ServeHTTP)
		path += "/"
	}
	path += "*"

	r.Head(path, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		fs.ServeHTTP(w, r)
	}))
	r.Get(path, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		fs.ServeHTTP(w, r)
	}))
}

func CacheControl(h http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Cache-Control", "max-age=31536000")
		h.ServeHTTP(w, r)
	})
}

// getIgnoreHeaders returns the headers we don't need to print since they are handled by logFormat
func getIgnoreHeaders() []string {
	return []string{"referer", "user-agent"}
}

// getObfuscateHeaders returns headers that should have their values obfuscated in the logs
func getObfuscateHeaders() []string {
	return []string{"authorization"}
}

// echoHandler simply repeats the body from the request in a response.
func echoHandler(w http.ResponseWriter, r *http.Request) {
	body, _ := io.ReadAll(r.Body)
	w.WriteHeader(200)
	w.Write(body)
}

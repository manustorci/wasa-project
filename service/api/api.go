/*
Package api exposes the main API engine. All HTTP APIs are handled here - so-called "business logic" should be here, or
in a dedicated package (if that logic is complex enough).

To use this package, you should create a new instance with New() passing a valid Config. The resulting Router will have
the Router.Handler() function that returns a handler that can be used in a http.Server (or in other middlewares).

Example:

	// Create the API router
	apirouter, err := api.New(api.Config{
		Logger:   logger,
		Database: appdb,
	})
	if err != nil {
		logger.WithError(err).Error("error creating the API server instance")
		return fmt.Errorf("error creating the API server instance: %w", err)
	}
	router := apirouter.Handler()

	// ... other stuff here, like middleware chaining, etc.

	// Create the API server
	apiserver := http.Server{
		Addr:              cfg.Web.APIHost,
		Handler:           router,
		ReadTimeout:       cfg.Web.ReadTimeout,
		ReadHeaderTimeout: cfg.Web.ReadTimeout,
		WriteTimeout:      cfg.Web.WriteTimeout,
	}

	// Start the service listening for requests in a separate goroutine
	apiserver.ListenAndServe()

See the `main.go` file inside the `cmd/webapi` for a full usage example.
*/
package api

import (
	"errors"
	"net/http"
	"strings"
	"wasa-project/service/api/session"
	"wasa-project/service/database"

	"github.com/julienschmidt/httprouter"
	"github.com/sirupsen/logrus"
)

// Config is used to provide dependencies and configuration to the New function.
type Config struct {
	// Logger where log entries are sent
	Logger logrus.FieldLogger
	// Database is the instance of database.AppDatabase where data are saved
	Database database.AppDatabase
}

// Router is the package API interface representing an API handler builder
type Router interface {
	// Handler returns an HTTP handler for APIs provided in this package
	Handler() http.Handler

	// Close terminates any resource used in the package
	Close() error
}

func fixPath(path string) string {
	return strings.ReplaceAll(path, "{id}", ":id")
}

// New returns a new Router instance
func New(cfg Config) (Router, error) {
	// Check if the configuration is correct
	if cfg.Logger == nil {
		return nil, errors.New("logger is required")
	}
	if cfg.Database == nil {
		return nil, errors.New("database is required")
	}

	// Create a new router where we will register HTTP endpoints. The server will pass requests to this router to be
	// handled.
	router := httprouter.New()
	router.RedirectTrailingSlash = true
	router.RedirectFixedPath = false

	// conf the route on the router
	const api = "/v1"

	sessHandler := session.NewHandler(cfg.Database)

	router.POST(fixPath(api+"/session"), sessHandler.DoLogin)
	router.POST(fixPath(api+"/conversations/{id}/messages"), sessHandler.SendMessage)
	router.POST(fixPath(api+"/conversations"), sessHandler.CreateConversation)
	router.POST(fixPath(api+"/groups/{id}/members"), sessHandler.AddUserToConversation)
	router.POST(fixPath(api+"/messages"), sessHandler.SendDirectMessage)
	router.POST(fixPath(api+"/messages/{id}/forward"), sessHandler.ForwardMessage)
	router.POST(fixPath(api+"/messages/{id}/comments"), sessHandler.CommentMessage)

	router.PUT(fixPath(api+"/groups/{id}/name"), sessHandler.SetGroupName)
	router.PUT(fixPath(api+"/me/username"), sessHandler.SetMyUserName)
	router.PUT(fixPath(api+"/me/photo"), sessHandler.SetMyPhoto)
	router.PUT(fixPath(api+"/groups/{id}/photo"), sessHandler.SetGroupPhoto)

	router.DELETE(fixPath(api+"/messages/{id}/comments"), sessHandler.UncommentMessage)
	router.DELETE(fixPath(api+"/groups/{id}/members"), sessHandler.LeaveGroup)
	router.DELETE(fixPath(api+"/messages/{id}"), sessHandler.DeleteMessage)

	router.GET(fixPath(api+"/me/conversations"), sessHandler.GetMyConversations)
	router.GET(fixPath(api+"/user/{id}"), sessHandler.GetUserByID)
	router.GET(fixPath(api+"/conversations/{id}"), sessHandler.GetConversation)
	router.GET(fixPath(api+"/users"), sessHandler.ListUsers)

	router.ServeFiles("/uploads/*filepath", http.Dir("uploads"))

	return &_router{
		router:     router,
		baseLogger: cfg.Logger,
		db:         cfg.Database,
	}, nil
}

type _router struct {
	router *httprouter.Router

	// baseLogger is a logger for non-requests contexts, like goroutines or background tasks not started by a request.
	// Use context logger if available (e.g., in requests) instead of this logger.
	baseLogger logrus.FieldLogger

	db database.AppDatabase
}

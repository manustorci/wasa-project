package api

import (
	"net/http"
)

// Handler returns an instance of httprouter.Router that handle APIs registered here
func (rt *_router) Handler() http.Handler {
	// Register routes
	rt.router.GET("/", rt.getHelloWorld)
	rt.router.GET("/context", rt.wrap(rt.getContextReply))
	// Special routes
	rt.router.GET("/liveness", rt.liveness)

	//sessHandler := session.NewHandler(cfg.Database)

	rt.router.POST("/session", rt.wrap(rt.DoLogin))
	rt.router.POST("/conversations/{id}/messages", rt.wrap(rt.SendMessage))
	rt.router.POST("/conversations", rt.wrap(rt.CreateConversation))
	rt.router.POST("/groups/{id}/members", rt.wrap(rt.AddUserToConversation))
	rt.router.POST("/messages", rt.wrap(rt.SendDirectMessage))
	rt.router.POST("/messages/{id}/forward", rt.wrap(rt.ForwardMessage))
	rt.router.POST("/messages/{id}/comments", rt.wrap(rt.CommentMessage))

	rt.router.PUT("/groups/{id}/name", rt.wrap(rt.SetGroupName))
	rt.router.PUT("/me/username", rt.wrap(rt.SetMyUserName))
	rt.router.PUT("/me/photo", rt.wrap(rt.SetMyPhoto))
	rt.router.PUT("/groups/{id}/photo", rt.wrap(rt.SetGroupPhoto))

	rt.router.DELETE("/messages/{id}/comments", rt.wrap(rt.UncommentMessage))
	rt.router.DELETE("/messages/{id}", rt.wrap(rt.DeleteMessage))
	rt.router.DELETE("/groups/{id}/members", rt.wrap(rt.LeaveGroup))

	rt.router.GET("/me/conversations", rt.wrap(rt.GetMyConversations))
	rt.router.GET("/user/{id}", rt.wrap(rt.GetUserByID))
	rt.router.GET("/conversations/{id}", rt.wrap(rt.GetConversation))

	rt.router.ServeFiles("/uploads/*filepath", http.Dir("uploads"))

	return rt.router
}

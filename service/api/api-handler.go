package api

import (
	"net/http"
)

// Handler returns an instance of httprouter.Router that handle APIs registered here
func (rt *_router) Handler() http.Handler {

	// --- Base routes ---
	rt.router.GET("/", rt.getHelloWorld)
	rt.router.GET("/context", rt.wrap(rt.getContextReply))
	rt.router.GET("/liveness", rt.liveness)

	// --- Session ---
	rt.router.POST("/session", rt.wrap(rt.DoLogin))

	// --- Conversations / Chats ---
	rt.router.POST("/conversations", rt.wrap(rt.CreateConversation))
	rt.router.GET("/conversations/:id", rt.wrap(rt.GetConversation))
	rt.router.POST("/conversations/:id/messages", rt.wrap(rt.SendMessage))
	rt.router.GET("/me/conversations", rt.wrap(rt.GetMyConversations))

	// --- Groups ---
	rt.router.POST("/groups/:id/members", rt.wrap(rt.AddUserToConversation))
	rt.router.PUT("/groups/:id/name", rt.wrap(rt.SetGroupName))
	rt.router.PUT("/groups/:id/photo", rt.wrap(rt.SetGroupPhoto))
	rt.router.DELETE("/groups/:id/members", rt.wrap(rt.LeaveGroup))

	// --- Messages ---
	rt.router.POST("/messages", rt.wrap(rt.SendDirectMessage))
	rt.router.POST("/messages/:id/forward", rt.wrap(rt.ForwardMessage))
	rt.router.POST("/messages/:id/comments", rt.wrap(rt.CommentMessage))
	rt.router.DELETE("/messages/:id/comments", rt.wrap(rt.UncommentMessage))
	rt.router.DELETE("/messages/:id", rt.wrap(rt.DeleteMessage))

	// --- Users ---
	rt.router.PUT("/me/username", rt.wrap(rt.SetMyUserName))
	rt.router.PUT("/me/photo", rt.wrap(rt.SetMyPhoto))
	rt.router.GET("/user/:id", rt.wrap(rt.GetUserByID))

	// --- Static files ---
	rt.router.ServeFiles("/uploads/*filepath", http.Dir("uploads"))

	return rt.router
}

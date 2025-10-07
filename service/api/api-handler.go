package api

import (
	"net/http"
)

// Handler returns an instance of httprouter.Router that handle APIs registered here
func (rt *_router) Handler() http.Handler {

	// --- Base routes ---
	rt.router.GET("/v1/", rt.getHelloWorld)
	rt.router.GET("/v1/context", rt.wrap(rt.getContextReply))
	rt.router.GET("/v1/liveness", rt.liveness)

	// --- Session ---
	rt.router.POST("/v1/session", rt.wrap(rt.DoLogin))

	// --- Conversations / Chats ---
	rt.router.POST("/v1/conversations", rt.wrap(rt.CreateConversation))
	rt.router.GET("/v1/conversations/:id", rt.wrap(rt.GetConversation))
	rt.router.POST("/v1/conversations/:id/messages", rt.wrap(rt.SendMessage))
	rt.router.GET("/v1/me/conversations", rt.wrap(rt.GetMyConversations))

	// --- Groups ---
	rt.router.POST("/v1/groups/:id/members", rt.wrap(rt.AddUserToConversation))
	rt.router.PUT("/v1/groups/:id/name", rt.wrap(rt.SetGroupName))
	rt.router.PUT("/v1/groups/:id/photo", rt.wrap(rt.SetGroupPhoto))
	rt.router.DELETE("/v1/groups/:id/members", rt.wrap(rt.LeaveGroup))

	// --- Messages ---
	rt.router.POST("/v1/messages", rt.wrap(rt.SendDirectMessage))
	rt.router.POST("/v1/messages/:id/forward", rt.wrap(rt.ForwardMessage))
	rt.router.POST("/v1/messages/:id/comments", rt.wrap(rt.CommentMessage))
	rt.router.DELETE("/v1/messages/:id/comments", rt.wrap(rt.UncommentMessage))
	rt.router.DELETE("/v1/messages/:id", rt.wrap(rt.DeleteMessage))

	// --- Users ---
	rt.router.PUT("/v1/me/username", rt.wrap(rt.SetMyUserName))
	rt.router.PUT("/v1/me/photo", rt.wrap(rt.SetMyPhoto))
	rt.router.GET("/v1/user/:id", rt.wrap(rt.GetUserByID))

	// --- Static files ---
	rt.router.ServeFiles("/v1/uploads/*filepath", http.Dir("uploads"))

	return rt.router
}

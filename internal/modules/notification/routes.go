package notification

import "github.com/gin-gonic/gin"

func RegisterRoutes(rg *gin.RouterGroup, handler *Handler, authGuard gin.HandlerFunc) {
	notifications := rg.Group("/notifications")
	notifications.Use(authGuard)

	notifications.GET("", handler.GetNotifications)
	notifications.GET("/unread-count", handler.GetUnreadCount)
	notifications.PATCH("/read-all", handler.MarkAllNotificationsRead)
	notifications.PATCH("/:id/read", handler.MarkNotificationRead)
	notifications.POST("/token", handler.RegisterToken)
	notifications.DELETE("/token", handler.DeleteToken)
	notifications.POST("/toggle", handler.TogglePushNotification)
}

func RegisterInternalRoutes(rg *gin.RouterGroup, handler *Handler, internalAuth gin.HandlerFunc) {
	internal := rg.Group("/notifications")
	internal.Use(internalAuth)

	internal.POST("/send", handler.SendNotification)
}

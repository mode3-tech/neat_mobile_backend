package auth

import "github.com/gin-gonic/gin"

func RegisterRoutes(rg *gin.RouterGroup, handler *AuthHandler, authGuard gin.HandlerFunc, loginMiddlewares ...gin.HandlerFunc) {

	auth := rg.Group("/auth")

	{
		auth.POST("/register", handler.Register)

		loginHandlers := append(loginMiddlewares, handler.Login)
		auth.POST("/login", loginHandlers...)

		auth.POST("/verify-device", handler.VerifyDevice)
		auth.POST("/verify-new-device", handler.VerifyNewDevice)
		auth.POST("/refresh", handler.RefreshAccessToken)
		auth.POST("/validate-bvn", handler.VerifyBVN)
		auth.POST("/validate-nin", handler.VerifyNIN)
		auth.POST("/forgot-password", handler.ForgotPassword)
		auth.POST("/reset-password", handler.ResetPassword)

		// Protected route
		auth.POST("/logout", authGuard, handler.Logout)
	}
}

package middleware

import (
	"log"
	appErr "neat_mobile_app_backend/internal/errors"
	"neat_mobile_app_backend/internal/response"
	"strings"

	"github.com/gin-gonic/gin"
)

func DeviceValidator(validator DeviceBindingChecker) gin.HandlerFunc {
	return func(c *gin.Context) {
		deviceID := strings.TrimSpace(c.Request.Header.Get("X-Device-ID"))
		if deviceID == "" {
			log.Println("device middleware: device id is missing")
			mapped := response.MapError(appErr.ErrUnauthorized)
			c.AbortWithStatusJSON(mapped.Status, response.APIResponse[any]{
				Status: "error",
				Error:  &mapped.Error,
			})
			return
		}

		mobileUserID := strings.TrimSpace(c.GetString(UserIDContextKey))
		if mobileUserID == "" {
			log.Println("device middleware: mobile user id is missing")
			mapped := response.MapError(appErr.ErrUnauthorized)
			c.AbortWithStatusJSON(mapped.Status, response.APIResponse[any]{
				Status: "error",
				Error:  &mapped.Error,
			})
			return
		}

		_, err := validator.VerifyUserDevice(c.Request.Context(), mobileUserID, deviceID)
		if err != nil {
			log.Println("device middleware: verify user device failed")
			mapped := response.MapError(err)
			c.AbortWithStatusJSON(mapped.Status, response.APIResponse[any]{
				Status: "error",
				Error:  &mapped.Error,
			})
			return
		}
		c.Set(DeviceIDContextKey, deviceID)
		c.Next()
	}
}

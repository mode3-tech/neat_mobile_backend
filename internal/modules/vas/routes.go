package vas

import "github.com/gin-gonic/gin"

func RegisterRoutes(rg *gin.RouterGroup, authGuard, deviceValidator gin.HandlerFunc, handler *Handler) {
	vas := rg.Group("/vas", authGuard, deviceValidator)
	{
		vas.POST("/airtime", handler.GetAirtime)
		vas.POST("/data", handler.GetData)
		vas.POST("/electricity/validate", handler.ValidateElectricity)
		vas.POST("/electricity/pay", handler.PayElectricity)
		vas.POST("/cable/validate", handler.ValidateCable)
		vas.POST("/cable/pay", handler.PayCable)
	}
}

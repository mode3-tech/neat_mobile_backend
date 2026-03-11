package loanproduct

import (
	"net/http"

	"github.com/gin-gonic/gin"
)

type Handler struct {
	service *Service
}

func NewHandler(service *Service) *Handler {
	return &Handler{service: service}
}

func (h Handler) GetLoanProducts(c *gin.Context) {
	loanProducts, err := h.service.GetAllLoanProducts(c.Request.Context())

	if err != nil {
		c.AbortWithStatusJSON(http.StatusInternalServerError, gin.H{"error": "something went wrong, please try again"})
	}

	c.JSON(http.StatusOK, gin.H{"message": "loan products fetch was successful", "products": loanProducts})
}

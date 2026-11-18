package wallet

import (
	"testing"

	"github.com/gin-gonic/gin"
)

func TestRegisterV2AccountStatementRoutes(t *testing.T) {
	gin.SetMode(gin.TestMode)
	router := gin.New()
	pass := func(c *gin.Context) { c.Next() }

	RegisterV2AccountStatementRoutes(router.Group(""), &Handler{}, pass, pass)

	routes := map[string]bool{}
	for _, route := range router.Routes() {
		routes[route.Method+" "+route.Path] = true
	}
	for _, want := range []string{
		"POST /account/statement/",
		"GET /account/statement/:statement_id",
	} {
		if !routes[want] {
			t.Errorf("missing route %q", want)
		}
	}
}

func TestRegisterV2BankRoutes(t *testing.T) {
	gin.SetMode(gin.TestMode)
	router := gin.New()
	pass := func(c *gin.Context) { c.Next() }

	RegisterV2BankRoutes(router.Group(""), &Handler{}, pass, pass)

	routes := map[string]bool{}
	for _, route := range router.Routes() {
		routes[route.Method+" "+route.Path] = true
	}
	for _, want := range []string{
		"GET /wallet/banks",
		"GET /wallet/bank/details",
	} {
		if !routes[want] {
			t.Errorf("missing route %q", want)
		}
	}
}

package router

import (
	"github.com/QuantumNous/new-api/controller"
	"github.com/QuantumNous/new-api/middleware"

	"github.com/gin-gonic/gin"
)

// SetVietQRRouter exposes the two callbacks VietQR needs at the flat paths its
// merchant portal produces from a base URL + base path pair.
//
// With base URL https://dgai.vn and base path "vqr" the portal builds:
//
//	https://dgai.vn/vqr/api/token_generate
//	https://dgai.vn/vqr/bank/api/transaction-sync
//
// Keeping these at the root avoids a nested base path in the portal form.
func SetVietQRRouter(router *gin.Engine) {
	group := router.Group("/vqr")
	group.Use(middleware.GlobalAPIRateLimit())
	limit := middleware.AnonymousRequestBodyLimit()
	group.POST("/api/token_generate", limit, controller.VietQRTokenGenerate)
	group.POST("/bank/api/transaction-sync", limit, controller.VietQRTransactionSync)
}

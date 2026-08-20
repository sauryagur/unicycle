// internal/routes/routes.go
package routes

import (
	"github.com/gin-gonic/gin"
)

// SetupRoutes configures all the application routes
func SetupRoutes(router *gin.Engine) {
	// API version group
	v1 := router.Group("/v1")
	{
		// Health check
		v1.GET("/health", func(c *gin.Context) {
			c.JSON(200, gin.H{
				"status": "healthy",
			})
		})

		// Auth routes
		auth := v1.Group("/auth")
		{
			auth.POST("/google", func(c *gin.Context) {
				// TODO: Implement Google OAuth
				c.JSON(200, gin.H{"message": "Google auth"})
			})
			auth.POST("/refresh", func(c *gin.Context) {
				// TODO: Implement token refresh
				c.JSON(200, gin.H{"message": "Token refreshed"})
			})
			auth.POST("/logout", func(c *gin.Context) {
				// TODO: Implement logout
				c.JSON(204, nil)
			})
			auth.GET("/me", func(c *gin.Context) {
				// TODO: Get current user
				c.JSON(200, gin.H{"message": "User profile"})
			})
		}

		// Bike routes
		bikes := v1.Group("/bikes")
		{
			bikes.GET("", func(c *gin.Context) {
				// TODO: List bikes
				c.JSON(200, gin.H{"message": "List bikes"})
			})
			bikes.GET("/:bike_id", func(c *gin.Context) {
				// TODO: Get bike details
				c.JSON(200, gin.H{"message": "Get bike"})
			})
			bikes.GET("/:bike_id/status", func(c *gin.Context) {
				// TODO: Get real-time bike status
				c.JSON(200, gin.H{"message": "Get bike status"})
			})
		}

		// Ride routes
		rides := v1.Group("/rides")
		{
			rides.POST("/start", func(c *gin.Context) {
				// TODO: Start ride
				c.JSON(201, gin.H{"message": "Ride started"})
			})
			rides.GET("/:ride_id", func(c *gin.Context) {
				// TODO: Get ride details
				c.JSON(200, gin.H{"message": "Get ride"})
			})
			rides.GET("/current", func(c *gin.Context) {
				// TODO: Get current ride
				c.JSON(200, gin.H{"message": "Get current ride"})
			})
			rides.GET("/history", func(c *gin.Context) {
				// TODO: Get ride history
				c.JSON(200, gin.H{"message": "Get ride history"})
			})
			rides.POST("/:ride_id/end", func(c *gin.Context) {
				// TODO: End ride offline
				c.JSON(200, gin.H{"message": "Ride ended"})
			})
		}

		// Wallet routes
		wallet := v1.Group("/wallet")
		{
			wallet.GET("/balance", func(c *gin.Context) {
				// TODO: Get wallet balance
				c.JSON(200, gin.H{"message": "Get balance"})
			})
			wallet.POST("/topup", func(c *gin.Context) {
				// TODO: Top up wallet
				c.JSON(200, gin.H{"message": "Wallet topped up"})
			})
			wallet.GET("/transactions", func(c *gin.Context) {
				// TODO: Get transactions
				c.JSON(200, gin.H{"message": "Get transactions"})
			})
		}

		// Report routes
		reports := v1.Group("/reports")
		{
			reports.POST("", func(c *gin.Context) {
				// TODO: Create report
				c.JSON(201, gin.H{"message": "Report created"})
			})
			reports.GET("/:report_id", func(c *gin.Context) {
				// TODO: Get report
				c.JSON(200, gin.H{"message": "Get report"})
			})
		}

		// Admin routes
		admin := v1.Group("/admin")
		{
			admin.GET("/fleet", func(c *gin.Context) {
				// TODO: Get fleet status
				c.JSON(200, gin.H{"message": "Fleet status"})
			})
			admin.GET("/routers", func(c *gin.Context) {
				// TODO: Get routers
				c.JSON(200, gin.H{"message": "Get routers"})
			})
			admin.POST("/bikes/:bike_id/disable", func(c *gin.Context) {
				// TODO: Disable bike
				c.JSON(200, gin.H{"message": "Bike disabled"})
			})
			admin.POST("/bikes/:bike_id/enable", func(c *gin.Context) {
				// TODO: Enable bike
				c.JSON(200, gin.H{"message": "Bike enabled"})
			})
			admin.GET("/reports", func(c *gin.Context) {
				// TODO: List reports
				c.JSON(200, gin.H{"message": "List reports"})
			})
			admin.POST("/reports/:report_id/resolve", func(c *gin.Context) {
				// TODO: Resolve report
				c.JSON(200, gin.H{"message": "Report resolved"})
			})
		}
	}
}

package main

func initializeRoutes() {
	// Check Health
	router.GET("/ping", checkHealth)
	router.POST("/login", login)

	authorized := router.Group("/", authenticate())

	authorized.GET("/foods", getFoods)
	authorized.POST("/carts", createCart)
	authorized.PATCH("/carts/:cart_id", addFood)
	authorized.POST("/orders", placeOrder)
	authorized.GET("/orders", getOrders)
	authorized.GET("/admin/orders", getAllOrders)
}

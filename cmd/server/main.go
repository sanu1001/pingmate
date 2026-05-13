package main

import (
	"log"

	"github.com/sanu1001/pingmate/config"

	"github.com/gin-gonic/gin"
)

func main() {
	// ─── STAGE 1: Bootstrap ──────────────────────────────
	config.Load()

	config.ConnectDB()
	defer config.DB.Close()

	config.ConnectRedis()
	defer config.Redis.Close()

	// ─── STAGE 2: (empty for now — wiring comes as we build layers) ───

	// ─── STAGE 3: Router ─────────────────────────────────
	r := gin.Default()

	r.GET("/health", func(c *gin.Context) {
		c.JSON(200, gin.H{
			"status":  "ok",
			"service": "PingMate",
		})
	})

	// v1 group — auth and reminder routes register here as we build them
	v1 := r.Group("/api/v1")
	_ = v1 // placeholder until handlers exist

	// ─── STAGE 4: Start ──────────────────────────────────
	log.Printf("PingMate running on :%s", config.App.Port)
	if err := r.Run(":" + config.App.Port); err != nil {
		log.Fatalf("Server failed: %v", err)
	}
}
package main

import (
	"log"
	"time"

	"github.com/sanu1001/pingmate/config"
	"github.com/sanu1001/pingmate/internal/handlers"
	"github.com/sanu1001/pingmate/internal/middleware"
	"github.com/sanu1001/pingmate/internal/repository"
	"github.com/sanu1001/pingmate/internal/scheduler"
	"github.com/sanu1001/pingmate/internal/services"

	"github.com/gin-gonic/gin"
)

func main() {
	// ─── STAGE 1: Bootstrap dependencies ─────────────────
	config.Load()

	config.ConnectDB()
	defer config.DB.Close()

	config.ConnectRedis()
	defer config.Redis.Close()

	// ─── STAGE 2: Build repositories (need: config.DB) ───
	userRepo := repository.NewUserRepo(config.DB)
	reminderRepo := repository.NewReminderRepo(config.DB)

	// ─── STAGE 3: Build services (need: repos + Redis) ───
	authSvc := services.NewAuthService(userRepo, config.Redis)
	reminderSvc := services.NewReminderService(reminderRepo)

	// ─── STAGE 4: Build handlers (need: services) ────────
	authHandler := handlers.NewAuthHandler(authSvc)
	reminderHandler := handlers.NewReminderHandler(reminderSvc)

	// ─── STAGE 4.5: Start the background scheduler ───────
	sched := scheduler.NewScheduler(reminderRepo, 30*time.Second)
	go sched.Start()

	// ─── STAGE 5: Router setup ───────────────────────────
	gin.SetMode(config.App.GINMode)
	r := gin.Default()

	r.GET("/health", func(c *gin.Context) {
		c.JSON(200, gin.H{
			"status":  "ok",
			"service": "PingMate",
		})
	})

	v1 := r.Group("/api/v1")
	{
		// ─── Public routes (no auth required) ────────────
		v1.POST("/auth/register", authHandler.Register)
		v1.POST("/auth/login", authHandler.Login)

		// ─── Protected routes (JWT required) ─────────────
		protected := v1.Group("/")
		protected.Use(middleware.AuthMiddleware(authSvc))
		{
			protected.POST("/auth/logout", authHandler.Logout)

			// Rate limited reminder write routes (30 requests/minute)
			rateLimited := protected.Group("/")
			rateLimited.Use(middleware.RateLimitMiddleware(30, time.Minute))
			{
				rateLimited.POST("/reminders", reminderHandler.Create)
				rateLimited.PUT("/reminders/:id", reminderHandler.Update)
				rateLimited.DELETE("/reminders/:id", reminderHandler.Delete)
			}

			// Read routes — not rate limited (reads are cheap)
			protected.GET("/reminders", reminderHandler.GetAll)
			protected.GET("/reminders/:id", reminderHandler.GetByID)
		}
	}

	// ─── STAGE 6: Start server ───────────────────────────
	log.Printf("PingMate running on :%s", config.App.Port)
	if err := r.Run(":" + config.App.Port); err != nil {
		log.Fatalf("Server failed: %v", err)
	}
}

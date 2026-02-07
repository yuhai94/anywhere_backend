package main

import (
	"context"
	"flag"
	"fmt"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/gin-gonic/gin"
	_ "github.com/go-sql-driver/mysql"
	"github.com/jmoiron/sqlx"
	"github.com/yuhai94/anywhere_backend/internal/api/handlers"
	"github.com/yuhai94/anywhere_backend/internal/api/routes"
	"github.com/yuhai94/anywhere_backend/internal/aws"
	"github.com/yuhai94/anywhere_backend/internal/config"
	"github.com/yuhai94/anywhere_backend/internal/logging"
	"github.com/yuhai94/anywhere_backend/internal/repository"
	"github.com/yuhai94/anywhere_backend/internal/scheduler"
	"github.com/yuhai94/anywhere_backend/internal/service"
)

func main() {
	// Parse command line arguments
	configPath := flag.String("config", "conf/conf.yaml", "Path to configuration file")
	logDir := flag.String("log-dir", "./logs", "Path to log directory")
	flag.Parse()

	// Load configuration
	fmt.Println("Loading configuration...")
	fmt.Printf("Using config file: %s\n", *configPath)
	if err := config.LoadConfig(*configPath); err != nil {
		fmt.Printf("Failed to load config: %v\n", err)
		os.Exit(1)
	}
	fmt.Println("Configuration loaded successfully")

	// Initialize logging
	fmt.Printf("Initializing logging... log dir: %s\n", *logDir)
	if err := logging.Init(*logDir); err != nil {
		fmt.Printf("Failed to initialize logging: %v\n", err)
		os.Exit(1)
	}
	fmt.Println("Logging initialized successfully")

	ctx := context.Background()
	logging.Info(ctx, "Starting V2Ray backend service")

	// Connect to database
	fmt.Println("Connecting to database...")
	dsn := config.GetDSN()
	fmt.Printf("Database DSN: %s\n", dsn)
	db, err := sqlx.Connect("mysql", dsn)
	if err != nil {
		fmt.Printf("Failed to connect to database: %v\n", err)
		os.Exit(1)
	}
	defer db.Close()
	fmt.Println("Connected to database successfully")

	// Initialize repository
	repo := repository.New(db)

	// Create database schema
	if err := repo.InitSchema(ctx); err != nil {
		logging.Fatal(ctx, "Failed to initialize schema: %v", err)
	}

	// Initialize AWS EC2 client
	ec2Client, err := aws.NewEC2Client()
	if err != nil {
		logging.Fatal(ctx, "Failed to initialize EC2 client: %v", err)
	}

	// Initialize service
	v2rayService := service.NewV2RayService(repo, ec2Client)

	// Initialize scheduler and start AWS instance sync task
	s := scheduler.NewScheduler()
	awsSyncTask := scheduler.NewAWSInstanceSyncTask(ec2Client, repo)
	s.Register(awsSyncTask)

	// Start all tasks
	s.Start()

	// Initialize handler
	v2rayHandler := handlers.NewV2RayHandler(v2rayService)

	// Setup Gin router
	router := gin.Default()

	// Setup routes
	routes.SetupRoutes(router, v2rayHandler)

	// Create HTTP server
	var addr = fmt.Sprintf("%s:%d", config.AppConfig.Server.Host, config.AppConfig.Server.Port)
	srv := &http.Server{
		Addr:    addr,
		Handler: router,
	}

	// Start server in a goroutine
	go func() {
		logging.Info(ctx, "Server starting on addr %s", addr)
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			logging.Fatal(ctx, "Failed to start server: %v", err)
		}
	}()

	// Wait for interrupt signal to gracefully shutdown the server
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit
	logging.Info(ctx, "Shutting down server...")

	// Create a deadline for server shutdown
	ctx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()

	// Shutdown server
	if err := srv.Shutdown(ctx); err != nil {
		logging.Fatal(ctx, "Server forced to shutdown: %v", err)
	}

	// Stop scheduler
	s.Stop()

	// Wait for async operations to complete
	v2rayService.Wait()

	logging.Info(ctx, "Server exited")
}

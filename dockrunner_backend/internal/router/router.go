package router

import (
	"context"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/jackc/pgx/v5/pgxpool"
	"go.uber.org/zap"

	"dockrunner_backend/internal/config"
	"dockrunner_backend/internal/middleware"
	"dockrunner_backend/internal/modules/admin"
	"dockrunner_backend/internal/modules/agents"
	"dockrunner_backend/internal/modules/auth"
	"dockrunner_backend/internal/modules/balances"
	"dockrunner_backend/internal/modules/containers"
	"dockrunner_backend/internal/modules/databases"
	"dockrunner_backend/internal/modules/deploymentrequests"
	"dockrunner_backend/internal/modules/envconfigs"
	"dockrunner_backend/internal/modules/files"
	"dockrunner_backend/internal/modules/overview"
	"dockrunner_backend/internal/modules/pricing"
	"dockrunner_backend/internal/modules/quotas"
	"dockrunner_backend/internal/modules/servers"
	"dockrunner_backend/internal/modules/storage"
	"dockrunner_backend/internal/modules/users"
	"dockrunner_backend/internal/modules/volumes"
	"dockrunner_backend/internal/oplock"
	"dockrunner_backend/pkg/database/influxdb"
	"dockrunner_backend/pkg/queue"
	"dockrunner_backend/pkg/token"

	swaggerFiles "github.com/swaggo/files"
	ginSwagger "github.com/swaggo/gin-swagger"

	_ "dockrunner_backend/docs"
)

type Dependencies struct {
	Config   *config.Config
	Logger   *zap.Logger
	Postgres *pgxpool.Pool
	Influx   *influxdb.Client
	Queue    *queue.Client
	// Shared instances built in main and also used by the events consumer.
	// If nil, the router builds local ones (dev mode without task results).
	OpTracker              *oplock.Tracker
	AdminService           *admin.Service
	VolumesService         *volumes.Service
	ContainersService      *containers.Service
	DeploymentRequestsService *deploymentrequests.Service
	BalancesService        *balances.Service
	PricingService         *pricing.Service
}

func postgresReady(deps Dependencies, ctx context.Context) bool {
	if deps.Postgres == nil {
		return true // dev mode without DB = ready
	}
	return deps.Postgres.Ping(ctx) == nil
}

func influxReady(deps Dependencies, ctx context.Context) bool {
	if deps.Influx == nil {
		return true
	}
	return deps.Influx.Ping(ctx) == nil
}

func New(deps Dependencies) *gin.Engine {
	if deps.Config.IsProduction() {
		gin.SetMode(gin.ReleaseMode)
	}

	r := gin.New()
	r.Use(middleware.RequestID())
	r.Use(middleware.ZapLogger(deps.Logger))
	r.Use(middleware.Recovery(deps.Logger))
	r.Use(middleware.SecurityHeaders(deps.Config.IsProduction()))
	r.Use(middleware.CORS(middleware.CORSConfig{
		AllowedOrigins: deps.Config.CORSAllowedOrigins,
		AllowedMethods: deps.Config.CORSAllowedMethods,
		AllowedHeaders: deps.Config.CORSAllowedHeaders,
	}))

	// Rate limiter global (optional)
	limiter := middleware.NewRateLimiter(deps.Config.RateLimitRPS, deps.Config.RateLimitBurst)
	r.Use(limiter.Middleware())

	// Health & readiness
	r.GET("/health", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{
			"status":  "ok",
			"time":    time.Now().UTC(),
			"version": "0.1.0",
			"env":     deps.Config.Env,
		})
	})
	r.GET("/ready", func(c *gin.Context) {
		ctx := c.Request.Context()
		pgOK := postgresReady(deps, ctx)
		influxOK := influxReady(deps, ctx)
		ready := pgOK && influxOK

		status := http.StatusOK
		if !ready {
			status = http.StatusServiceUnavailable
		}
		c.JSON(status, gin.H{"ready": ready, "postgres": pgOK, "influxdb": influxOK})
	})

	api := r.Group("/api")
	v1 := api.Group("/v1")

	tokenMgr := token.NewManager(deps.Config.JWTSecret, deps.Config.JWTAccessTTL, deps.Config.JWTRefreshTTL)

	// --- Auth ---
	authRepo := auth.NewRepository(deps.Postgres, deps.Logger)
	refreshStore := auth.NewRefreshStore(deps.Config.JWTRefreshTTL)
	authService := auth.NewService(authRepo, tokenMgr, refreshStore, deps.Logger, deps.Config.BcryptCost)
	authHandler := auth.NewHandler(authService, deps.Logger)
	auth.RegisterRoutes(v1, authHandler, deps.Config.JWTSecret)

	// --- Users ---
	usersRepo := users.NewRepository(deps.Postgres, deps.Logger)
	usersService := users.NewService(usersRepo, deps.Logger)
	usersHandler := users.NewHandler(usersService, deps.Logger, deps.Config.BcryptCost)
	users.RegisterRoutes(v1, usersHandler, deps.Config.JWTSecret)

	// --- Admin ---
	adminRepo := admin.NewRepository(deps.Postgres, deps.Logger)
	adminService := deps.AdminService
	if adminService == nil {
		adminService = admin.NewService(adminRepo, deps.Queue, deps.Logger, deps.OpTracker)
	}
	adminHandler := admin.NewHandler(adminService, deps.Logger)
	admin.RegisterRoutes(v1, adminHandler, deps.Config.JWTSecret)

	// --- Servers ---
	serversRepo := servers.NewRepository(deps.Postgres, deps.Logger)
	serversService := servers.NewService(serversRepo, deps.Logger)
	serversHandler := servers.NewHandler(serversService, deps.Logger)
	servers.RegisterRoutes(v1, serversHandler, deps.Config.JWTSecret)

	// --- Agents (worker heartbeats) ---
	agentsRepo := agents.NewRepository(deps.Postgres, deps.Logger)
	agentsService := agents.NewService(agentsRepo, deps.Influx, deps.Logger)
	agentsHandler := agents.NewHandler(agentsService, deps.Logger)
	agents.RegisterRoutes(v1, agentsHandler, deps.Config.JWTSecret, deps.Config.AgentToken)

	// --- Overview (cluster stats) ---
	overviewHandler := overview.NewHandler(deps.Influx, deps.Logger)
	overview.RegisterRoutes(v1, overviewHandler, deps.Config.JWTSecret)

	// --- Volumes ---
	volumesRepo := volumes.NewRepository(deps.Postgres, deps.Logger)
	volumesService := deps.VolumesService
	if volumesService == nil {
		volumesService = volumes.NewService(volumesRepo, deps.Queue, deps.Logger)
	}
	volumesHandler := volumes.NewHandler(volumesService, deps.Logger)
	volumes.RegisterRoutes(v1, volumesHandler, deps.Config.JWTSecret)

	// --- Env configs ---
	envRepo := envconfigs.NewRepository(deps.Postgres, deps.Logger)
	envService := envconfigs.NewService(envRepo, deps.Logger)
	envHandler := envconfigs.NewHandler(envService, deps.Logger)
	envconfigs.RegisterRoutes(v1, envHandler, deps.Config.JWTSecret)

	// --- Containers (docker-style run) ---
	// NOTE: must reuse the consumer's instance — checks live in its memory.
	containersService := deps.ContainersService
	if containersService == nil {
		containersService = containers.NewService(adminRepo, agentsRepo, volumesService, deps.Config.AgentToken, deps.Queue, deps.Logger, deps.OpTracker, containers.PortsPolicy{Min: deps.Config.PortsMin, Max: deps.Config.PortsMax})
	}
	containersHandler := containers.NewHandler(containersService, deps.Logger, deps.Config.JWTSecret)
	containers.RegisterRoutes(v1, containersHandler, deps.Config.JWTSecret)

	// --- Files (user folders on agents) ---
	filesService := files.NewService(deps.Queue, deps.Logger)
	filesHandler := files.NewHandler(filesService, deps.Logger)
	files.RegisterRoutes(v1, filesHandler, deps.Config.JWTSecret)

	// --- Storage usage ---
	storageService := storage.NewService(agentsRepo, deps.Logger)
	storageHandler := storage.NewHandler(storageService, deps.Logger)
	storage.RegisterRoutes(v1, storageHandler, deps.Config.JWTSecret)

	// --- Quotas ---
	quotasRepo := quotas.NewRepository(deps.Postgres, deps.Logger)
	quotasService := quotas.NewService(quotasRepo, deps.Logger, deps.Config.PortsMin, deps.Config.PortsMax)
	quotasHandler := quotas.NewHandler(quotasService, deps.Logger)
	quotas.RegisterRoutes(v1, quotasHandler, deps.Config.JWTSecret)

	// --- Databases (managed postgres/mongo/redis) ---
	databasesRepo := databases.NewRepository(deps.Postgres, deps.Logger)
	databasesService := databases.NewService(databasesRepo, databases.DBConfig{
		PostgresHost:     deps.Config.DBPostgresHost,
		PostgresPort:     deps.Config.DBPostgresPort,
		PostgresName:     deps.Config.DBPostgresName,
		PostgresAdminURL: deps.Config.DBPostgresAdminURL,
		MongoHost:        deps.Config.DBMongoHost,
		MongoPort:        deps.Config.DBMongoPort,
		MongoName:        deps.Config.DBMongoName,
		MongoAdminURL:    deps.Config.DBMongoAdminURL,
		RedisHost:        deps.Config.DBRedisHost,
		RedisPort:        deps.Config.DBRedisPort,
		RedisPassword:    deps.Config.DBRedisPassword,
	}, deps.Logger)
	databasesHandler := databases.NewHandler(databasesService, deps.Logger)
	databases.RegisterRoutes(v1, databasesHandler, deps.Config.JWTSecret)

	// --- Balances ---
	balancesRepo := balances.NewRepository(deps.Postgres, deps.Logger)
	balancesService := deps.BalancesService
	if balancesService == nil {
		balancesService = balances.NewService(balancesRepo, deps.Logger)
	}
	balancesHandler := balances.NewHandler(balancesService, deps.Logger)
	balances.RegisterRoutes(v1, balancesHandler, deps.Config.JWTSecret)

	// --- Deployment Requests ---
	deploymentRequestsRepo := deploymentrequests.NewRepository(deps.Postgres, deps.Logger)
	pricingRepo := pricing.NewRepository(deps.Postgres, deps.Logger)
	pricingService := deps.PricingService
	if pricingService == nil {
		pricingService = pricing.NewService(pricingRepo, deps.Logger)
	}
	deploymentRequestsService := deps.DeploymentRequestsService
	if deploymentRequestsService == nil {
		deploymentRequestsService = deploymentrequests.NewService(deploymentRequestsRepo, pricingRepo, containersService, balancesService, deps.Logger)
	}
	deploymentRequestsHandler := deploymentrequests.NewHandler(deploymentRequestsService, deps.Logger)
	deploymentrequests.RegisterRoutes(v1, deploymentRequestsHandler, deps.Config.JWTSecret)

	// --- Pricing ---
	pricingHandler := pricing.NewHandler(pricingService, deps.Logger)
	pricing.RegisterRoutes(v1, pricingHandler, deps.Config.JWTSecret)

	// 404
	r.NoRoute(func(c *gin.Context) {
		c.JSON(http.StatusNotFound, gin.H{"error": "route not found"})
	})

	r.GET("/swagger/*any", ginSwagger.WrapHandler(swaggerFiles.Handler))
	r.GET("/swagger", func(c *gin.Context) {
		c.Redirect(http.StatusFound, "/swagger/index.html")
	})
	deps.Logger.Info("swagger ui enabled", zap.String("path", "/swagger/index.html"))

	return r
}

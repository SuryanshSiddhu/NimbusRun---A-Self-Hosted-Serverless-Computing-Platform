package http

import (
	"github.com/gin-gonic/gin"
	"github.com/nimbusrun/nimbusrun/internal/auth"
	"github.com/nimbusrun/nimbusrun/internal/config"
	"github.com/nimbusrun/nimbusrun/internal/db"
	"github.com/nimbusrun/nimbusrun/internal/http/handler"
	"github.com/nimbusrun/nimbusrun/internal/http/middleware"
	"github.com/nimbusrun/nimbusrun/internal/repository"
)

// Server is the main HTTP gateway server.
type Server struct {
	router *gin.Engine
	cfg    *config.Config
	db     *db.DB
	auth   *auth.Service
}

// NewServer creates a new HTTP server.
func NewServer(cfg *config.Config, database *db.DB, authService *auth.Service) *Server {
	gin.SetMode(gin.ReleaseMode)
	r := gin.New()
	r.Use(gin.Recovery())
	r.Use(requestLogger())
	return &Server{
		router: r,
		cfg:    cfg,
		db:     database,
		auth:   authService,
	}
}

// SetupRoutes configures all API routes.
func (s *Server) SetupRoutes() {
	fnRepo := repository.NewFunctionRepository(s.db)
	verRepo := repository.NewVersionRepository(s.db)
	invRepo := repository.NewInvocationRepository(s.db)

	authH := handler.NewAuthHandler(s.db, s.auth)
	fnH := handler.NewFunctionHandler(fnRepo, verRepo, invRepo)
	invH := handler.NewInvocationHandler(invRepo, fnRepo, verRepo)

	// Health check
	s.router.GET("/health", func(c *gin.Context) {
		c.JSON(200, gin.H{"status": "healthy"})
	})

	// Public auth routes
	s.router.POST("/auth/register", authH.Register)
	s.router.POST("/auth/login", authH.Login)

	// Protected routes (JWT required)
	protected := s.router.Group("/")
	protected.Use(middleware.AuthMiddleware(s.auth))
	{
		// Functions CRUD
		protected.GET("/functions", fnH.List)
		protected.POST("/functions", fnH.Create)
		protected.GET("/functions/:id", fnH.Get)
		protected.DELETE("/functions/:id", fnH.Delete)
		protected.POST("/functions/:id/deploy", fnH.Deploy)
		protected.GET("/functions/:id/versions", fnH.ListVersions)
		protected.POST("/functions/:id/rollback/:version", fnH.Rollback)
		protected.GET("/functions/:id/invocations", invH.ListByFunction)
	}

	// Public invocation endpoint
	s.router.POST("/f/:id", invH.Invoke)
	s.router.GET("/f/:id/invocations/:inv_id/logs", invH.GetLogs)
}

// Run starts the HTTP server.
func (s *Server) Run() error {
	addr := s.cfg.Server.Host + ":" + s.cfg.Server.Port
	return s.router.Run(addr)
}

func requestLogger() gin.HandlerFunc {
	return gin.LoggerWithFormatter(func(param gin.LogFormatterParams) string {
		return param.TimeStamp.Format("2006/01/02 - 15:04:05") + " | " +
			param.Method + " " + param.Path + " | " +
			param.StatusCodeColor() + " " + string(rune(param.StatusCode)) + param.ResetColor() + " | " +
			param.Latency.String() + "\n"
	})
}

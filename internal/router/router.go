package router

import (
	"lakukan-be/internal/config"
	"lakukan-be/internal/modules/core/auth"
	"lakukan-be/internal/modules/core/company"
	"lakukan-be/internal/modules/core/permission"
	"lakukan-be/internal/modules/core/permission_template"
	"lakukan-be/internal/modules/core/role"
	"lakukan-be/internal/modules/core/user"
	"lakukan-be/internal/modules/crm/auto_reply_rules"
	"lakukan-be/internal/modules/crm/chats"
	"lakukan-be/internal/modules/crm/company_settings"
	"lakukan-be/internal/modules/crm/lead_sources"
	"lakukan-be/internal/modules/crm/leads"
	"lakukan-be/internal/modules/crm/sales_persons"
	"lakukan-be/internal/modules/crm/webhooks"
	"time"

	// Finance modules - commented out until implemented
	// "lakukan-be/internal/modules/finance/banks"
	// "lakukan-be/internal/modules/finance/categories"
	// "lakukan-be/internal/modules/finance/coa"
	// "lakukan-be/internal/modules/finance/customers"
	// "lakukan-be/internal/modules/finance/expense"
	// "lakukan-be/internal/modules/finance/invoice"
	// "lakukan-be/internal/modules/finance/items"
	// "lakukan-be/internal/modules/finance/receipt"
	// "lakukan-be/internal/modules/finance/reports"
	// "lakukan-be/internal/modules/finance/tax_rates"
	// "lakukan-be/internal/modules/finance/vendors"
	"lakukan-be/pkg/logger"

	"github.com/gin-contrib/cors"
	ginzap "github.com/gin-contrib/zap"
	"github.com/gin-gonic/gin"
	"github.com/jackc/pgx/v5/pgxpool"
	swaggerFiles "github.com/swaggo/files"
	ginSwagger "github.com/swaggo/gin-swagger"
	"go.uber.org/zap"
)

func Setup(router *gin.Engine, db *pgxpool.Pool, cfg *config.Config) {
	// Get logger instance
	log := logger.GetLogger()

	// Ginzap middleware for logging HTTP requests
	router.Use(ginzap.Ginzap(log, time.RFC3339, true))

	// Recovery middleware with Zap (handles panics)
	router.Use(ginzap.RecoveryWithZap(log, true))

	// CORS middleware configuration
	router.Use(cors.New(cors.Config{
		AllowOrigins:     []string{"http://localhost:5173", "http://localhost:3000", "https://app.lakukan.id"},
		AllowMethods:     []string{"GET", "POST", "PUT", "PATCH", "DELETE", "OPTIONS"},
		AllowHeaders:     []string{"Origin", "Content-Type", "Accept", "Authorization"},
		ExposeHeaders:    []string{"Content-Length"},
		AllowCredentials: true,
		MaxAge:           12 * time.Hour,
	}))

	// Swagger documentation endpoints
	// Serve custom Swagger UI with persistent auth
	router.StaticFile("/swagger", "./web/swagger/index.html")
	router.StaticFile("/swagger/", "./web/swagger/index.html")

	// Serve swagger.json and other swagger assets
	router.GET("/swagger/swagger.json", func(c *gin.Context) {
		c.File("./docs/swagger/swagger.json")
	})
	router.GET("/swagger/doc.json", func(c *gin.Context) {
		c.File("./docs/swagger/swagger.json")
	})

	// Fallback to default swagger UI if needed
	router.GET("/swagger-ui/*any", ginSwagger.WrapHandler(swaggerFiles.Handler))

	// Health check endpoint
	router.GET("/health", func(c *gin.Context) {
		c.JSON(200, gin.H{
			"status":  "ok",
			"message": "Lakukan API is running",
		})
	})

	// API v1 routes
	v1 := router.Group("/core/v1")
	{
		// Core modules
		// Initialize and setup auth module
		authModule := auth.Initialize(db, cfg)
		authModule.SetupRoutes(v1)

		// Initialize and setup user module
		userModule := user.Initialize(db)
		userModule.SetupRoutes(v1)

		// Initialize and setup role module
		roleModule := role.Initialize(db)
		roleModule.SetupRoutes(v1)

		// Initialize and setup permission module
		permissionModule := permission.Initialize(db)
		permissionModule.SetupRoutes(v1)

		// Initialize and setup permission template module
		permissionTemplateModule := permission_template.Initialize(db)
		permissionTemplateModule.SetupRoutes(v1)

		// Initialize and setup company module
		companyModule := company.Initialize(db)
		companyModule.SetupRoutes(v1)
	}

	// CRM v1 routes
	crmV1 := router.Group("/crm/v1")
	{
		// Initialize and setup company settings module
		companySettingsModule := company_settings.Initialize(db)
		companySettingsModule.SetupRoutes(crmV1)

		// Initialize and setup lead sources module
		leadSourcesModule := lead_sources.Initialize(db)
		leadSourcesModule.SetupRoutes(crmV1)

		// Initialize and setup leads module
		leadsModule := leads.Initialize(db)
		leadsModule.SetupRoutes(crmV1)

		// Initialize and setup chats module
		chatsModule := chats.Initialize(db)
		chatsModule.SetupRoutes(crmV1)

		// Initialize and setup sales persons module
		salesPersonsModule := sales_persons.Initialize(db, cfg)
		salesPersonsModule.SetupRoutes(crmV1)

		// Initialize and setup auto reply rules module
		autoReplyRulesModule := auto_reply_rules.Initialize(db)
		autoReplyRulesModule.SetupRoutes(crmV1)

		// Initialize and setup webhooks module
		webhooksModule := webhooks.Initialize(db, cfg)
		webhooksModule.SetupRoutes(crmV1)
	}

	// Finance v1 routes (commented out until implemented)
	// financeV1 := router.Group("/finance/v1")
	// {
	// 	// Initialize and setup COA module
	// 	coaModule := coa.Initialize(db)
	// 	coaModule.SetupRoutes(financeV1)
	//
	// 	// Initialize and setup Tax Rates module
	// 	taxRateModule := tax_rates.Initialize(db)
	// 	taxRateModule.SetupRoutes(financeV1)
	//
	// 	// Initialize and setup Categories module
	// 	categoryModule := categories.Initialize(db)
	// 	categoryModule.SetupRoutes(financeV1)
	//
	// 	// Initialize and setup Items module
	// 	itemModule := items.Initialize(db)
	// 	itemModule.SetupRoutes(financeV1)
	//
	// 	// Initialize and setup Vendors module
	// 	vendorModule := vendors.Initialize(db)
	// 	vendorModule.SetupRoutes(financeV1)
	//
	// 	// Initialize and setup Customers module
	// 	customerModule := customers.Initialize(db)
	// 	customerModule.SetupRoutes(financeV1)
	//
	// 	// Initialize and setup Banks module
	// 	bankModule := banks.Initialize(db)
	// 	bankModule.SetupRoutes(financeV1)
	//
	// 	// Initialize and setup Invoice module
	// 	invoiceModule := invoice.Initialize(db)
	// 	invoiceModule.SetupRoutes(financeV1)
	//
	// 	// Initialize and setup Receipt module
	// 	receiptModule := receipt.Initialize(db)
	// 	receiptModule.SetupRoutes(financeV1)
	//
	// 	// Initialize and setup Expense module
	// 	expenseModule := expense.Initialize(db)
	// 	expenseModule.SetupRoutes(financeV1)
	//
	// 	// Initialize and setup Reports module
	// 	reportModule := reports.Initialize(db)
	// 	reportModule.SetupRoutes(financeV1)
	// }

	log.Info("Routes setup completed", zap.Int("routes", len(router.Routes())))
}

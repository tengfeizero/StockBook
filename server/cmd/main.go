package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"io/fs"
	"log"
	"net/http"
	"os"
	"os/signal"
	"path/filepath"
	"strings"
	"syscall"
	"time"

	"github.com/gin-gonic/gin"

	"jxc/server/internal/api"
	"jxc/server/internal/config"
	"jxc/server/internal/middleware"
	"jxc/server/internal/model"
	"jxc/server/internal/pkg/errcode"
	"jxc/server/internal/pkg/gormx"
	"jxc/server/internal/pkg/license"
	"jxc/server/internal/pkg/logfile"
	"jxc/server/internal/pkg/response"
	"jxc/server/internal/platform/winsvc"
	"jxc/server/internal/repo"
	"jxc/server/internal/service"
	"jxc/server/internal/webembed"
)

func main() {
	// 命令行工具类子命令：执行完即退出，不启动 HTTP 服务
	showCode := flag.Bool("machine-code", false, "打印本机机器码后退出")
	install := flag.Bool("install", false, "注册为 Windows 服务（开机自启、后台运行，需管理员权限）")
	uninstall := flag.Bool("uninstall", false, "卸载 Windows 服务（数据保留）")
	status := flag.Bool("status", false, "查看 Windows 服务状态")
	flag.Parse()

	switch {
	case *showCode:
		fmt.Println("本机机器码:", license.MachineCode())
		return
	case *install:
		report(winsvc.Install())
		return
	case *uninstall:
		report(winsvc.Uninstall())
		return
	case *status:
		report(winsvc.Status())
		return
	}

	// Windows 服务模式：由 SCM 拉起，没有控制台，工作目录是 System32，
	// 必须先切回程序目录并把日志落盘，否则 data/ 与日志都会跑丢
	if winsvc.IsService() {
		daily, err := setupServiceRuntime()
		if err != nil {
			log.Fatalf("服务初始化失败: %v", err)
		}
		defer func() { _ = daily.Close() }()
		if err := winsvc.Run(runApp); err != nil {
			log.Fatalf("服务运行失败: %v", err)
		}
		return
	}

	// 前台模式：Ctrl+C / 关闭窗口 / kill 时优雅退出（关库、停 HTTP）
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	if err := runApp(ctx); err != nil {
		log.Fatalf("服务运行失败: %v", err)
	}
}

// report 输出安装/卸载/状态命令的结果，失败时以退出码 1 结束（便于脚本判断）。
func report(msg string, err error) {
	if err != nil {
		fmt.Fprintln(os.Stderr, "失败: "+err.Error())
		os.Exit(1)
	}
	fmt.Println(msg)
}

// setupServiceRuntime 服务模式的三件套：切换工作目录、日志落盘、关掉 gin 的路由清单输出。
func setupServiceRuntime() (*logfile.Daily, error) {
	home := os.Getenv("JXC_HOME")
	if home == "" {
		exe, err := os.Executable()
		if err != nil {
			return nil, err
		}
		home = filepath.Dir(exe)
	}
	if err := os.Chdir(home); err != nil {
		return nil, fmt.Errorf("切换工作目录到 %s 失败: %w", home, err)
	}

	daily, err := logfile.NewDaily(filepath.Join("data", "logs"), 14)
	if err != nil {
		return nil, fmt.Errorf("初始化日志失败: %w", err)
	}

	gin.SetMode(gin.ReleaseMode)
	log.SetOutput(daily)
	gin.DefaultWriter = daily
	gin.DefaultErrorWriter = daily
	log.Printf("服务模式启动：工作目录 %s，日志 %s", home, daily.String())
	return daily, nil
}

// runApp 初始化数据库、路由并启动 HTTP 服务；ctx 取消时优雅关闭。
func runApp(ctx context.Context) error {
	cfg := config.Load()

	db, err := model.InitDB(cfg.DBPath)
	if err != nil {
		return fmt.Errorf("数据库初始化失败: %w", err)
	}
	if err := model.AutoMigrate(db); err != nil {
		return fmt.Errorf("建表失败: %w", err)
	}
	if err := repo.SeedAdmin(db); err != nil {
		return fmt.Errorf("初始化管理员失败: %w", err)
	}
	// 多租户改造：业务编码唯一约束下沉到租户维度、系统设置按租户存储（幂等）
	if err := model.MigrateTenantSchema(db); err != nil {
		return fmt.Errorf("多租户迁移失败: %w", err)
	}
	// 套餐（平台维护，首次启动写入）
	if err := model.SeedPlans(db); err != nil {
		return fmt.Errorf("初始化套餐失败: %w", err)
	}
	// 秤码规则内置预设（首次启动写入行业常见码制，已有规则时跳过）
	if err := service.SeedScaleRules(db); err != nil {
		return fmt.Errorf("初始化秤码规则失败: %w", err)
	}
	// 库位数量账：升级时把存量库存整体挂到「未分配」库位，保证 Σ库位数量 == 仓库总量
	if n, err := model.BackfillLocationStock(db); err != nil {
		return fmt.Errorf("回填库位库存失败: %w", err)
	} else if n > 0 {
		log.Printf("库位库存初始化：%d 条存量库存已登记为「未分配」，可用移库单上架", n)
	}
	// 多租户基础设施：租户隔离插件（context 无租户信息时零干预，单机版行为不变）
	if err := db.Use(gormx.TenantPlugin{}); err != nil {
		return fmt.Errorf("注册租户插件失败: %w", err)
	}

	// 授权管理（M5）：license.key 位于 data/，无文件则按首次启动时间计试用期
	lic := license.NewManager("data/license.key")
	if err := lic.Load(); err != nil {
		log.Printf("授权警告: %v", err)
	}

	authSvc := service.NewAuthService(db)
	categorySvc := service.NewCategoryService(db)
	productSvc := service.NewProductService(db)
	partnerSvc := service.NewPartnerService(db)
	sysSvc := service.NewSystemService(db, cfg.DBPath)
	sysSvc.AutoBackup(context.Background()) // 每天首次启动自动备份

	r := gin.Default()
	// 反向代理（Caddy/Nginx）部署：只信任代理下发的 X-Forwarded-* 头，
	// 操作日志与限流才能拿到真实客户端 IP，同时外部无法伪造来源 IP。
	if err := r.SetTrustedProxies(cfg.TrustedProxies); err != nil {
		log.Printf("信任代理配置无效（%v），已退回仅信任本机回环地址", err)
		_ = r.SetTrustedProxies([]string{"127.0.0.1", "::1"})
	}
	r.Use(middleware.SecurityHeaders())
	r.Use(middleware.CORS())

	v1 := r.Group("/api/v1")
	v1.POST("/auth/login", middleware.Audit(db), api.NewAuthHandler(authSvc, cfg.JWTSecret).Login)

	// 需登录的接口区（实时加载用户状态+角色权限；审计记录所有变更类请求；
	// 授权过期时拦截写操作；租户上下文为未来 SaaS 化预留）
	authed := v1.Group("")
	authed.Use(
		middleware.JWTAuth(cfg.JWTSecret, db),
		middleware.TenantCtx(),
		middleware.TenantGuard(db),
		middleware.LicenseGuard(lic, func() time.Time { return sysSvc.FirstRun(context.Background()) }),
		middleware.Audit(db),
	)
	authed.GET("/auth/me", api.NewAuthHandler(authSvc, cfg.JWTSecret).Me)

	// M8 SaaS 租户：注册开户（公开）+ 套餐/订阅/账单（登录后）
	tenantSvc := service.NewTenantService(db)
	tenantHandler := api.NewTenantHandler(tenantSvc, authSvc, cfg.JWTSecret)
	v1.POST("/tenants/register", tenantHandler.Register)
	v1.GET("/tenants/plans", tenantHandler.Plans)
	v1.GET("/tenants/lookup", tenantHandler.PublicInfo)
	authed.GET("/tenants/current", tenantHandler.Current)
	authed.GET("/tenants/bills", tenantHandler.Bills)
	authed.POST("/tenants/subscribe", middleware.RequirePerm("system"), tenantHandler.Subscribe)

	// M1 基础档案（权限：archive）
	catHandler := api.NewCategoryHandler(categorySvc)
	authed.GET("/categories", middleware.RequirePerm("archive"), catHandler.List)
	authed.POST("/categories", middleware.RequirePerm("archive"), catHandler.Create)
	authed.PUT("/categories/:id", middleware.RequirePerm("archive"), catHandler.Update)
	authed.DELETE("/categories/:id", middleware.RequirePerm("archive"), catHandler.Delete)

	prodHandler := api.NewProductHandler(productSvc)
	authed.GET("/products", middleware.RequirePerm("archive", "sales", "purchase", "stock"), prodHandler.List)
	authed.GET("/products/:id", middleware.RequirePerm("archive", "sales", "purchase", "stock"), prodHandler.Get)
	authed.POST("/products", middleware.RequirePerm("archive"), prodHandler.Create)
	authed.PUT("/products/:id", middleware.RequirePerm("archive"), prodHandler.Update)
	authed.DELETE("/products/:id", middleware.RequirePerm("archive"), prodHandler.Delete)

	partnerHandler := api.NewPartnerHandler(partnerSvc)
	authed.GET("/partners/:kind", middleware.RequirePerm("archive", "sales", "purchase", "finance"), partnerHandler.List)
	authed.POST("/partners/:kind", middleware.RequirePerm("archive"), partnerHandler.Create)
	authed.PUT("/partners/:kind/:id", middleware.RequirePerm("archive"), partnerHandler.Update)
	authed.DELETE("/partners/:kind/:id", middleware.RequirePerm("archive"), partnerHandler.Delete)

	// M6 库位档案（权限：archive；开单页需要列表，放行 sales/purchase/stock）
	locSvc := service.NewLocationService(db)
	locHandler := api.NewLocationHandler(locSvc)
	authed.GET("/locations", middleware.RequirePerm("archive", "sales", "purchase", "stock"), locHandler.List)
	authed.GET("/locations/stock", middleware.RequirePerm("archive", "sales", "purchase", "stock"), locHandler.Stock)
	authed.GET("/locations/alerts", middleware.RequirePerm("archive", "sales", "purchase", "stock"), locHandler.Alerts)
	authed.POST("/locations", middleware.RequirePerm("archive"), locHandler.Create)
	authed.PUT("/locations/:id", middleware.RequirePerm("archive"), locHandler.Update)
	authed.DELETE("/locations/:id", middleware.RequirePerm("archive"), locHandler.Delete)

	// v0.11 秤码解析（规则管理：archive/system；开单扫码解析：所有开单角色）
	scaleSvc := service.NewScaleService(db)
	scaleHandler := api.NewScaleHandler(scaleSvc)
	authed.GET("/scale-rules", middleware.RequirePerm("archive", "system", "sales", "purchase", "stock"), scaleHandler.List)
	authed.POST("/scale-rules", middleware.RequirePerm("archive", "system"), scaleHandler.Create)
	authed.PUT("/scale-rules/:id", middleware.RequirePerm("archive", "system"), scaleHandler.Update)
	authed.DELETE("/scale-rules/:id", middleware.RequirePerm("archive", "system"), scaleHandler.Delete)
	authed.POST("/scale/parse", middleware.RequirePerm("archive", "system", "sales", "purchase", "stock"), scaleHandler.Parse)

	// M1 期初库存 + 批量导入（权限：archive）
	openingSvc := service.NewOpeningService(db)
	importSvc := service.NewImportService(db)
	openingHandler := api.NewOpeningHandler(openingSvc)
	authed.GET("/opening/stocks", middleware.RequirePerm("archive", "stock"), openingHandler.Stocks)
	authed.POST("/opening", middleware.RequirePerm("archive"), openingHandler.Save)
	importHandler := api.NewImportHandler(importSvc, openingSvc)
	authed.POST("/import/products", middleware.RequirePerm("archive"), importHandler.Products)
	authed.POST("/import/partners/:kind", middleware.RequirePerm("archive"), importHandler.Partners)
	authed.GET("/import/template/:kind", middleware.RequirePerm("archive"), importHandler.Template)

	// M2 核心单据（采购/销售/库存作业共用 docs 接口，按模块放行）
	docSvc := service.NewDocumentService(db)
	docHandler := api.NewDocHandler(docSvc)
	authed.POST("/docs", middleware.RequirePerm("purchase", "sales", "stock"), docHandler.Create)
	authed.GET("/docs", middleware.RequirePerm("purchase", "sales", "stock", "finance"), docHandler.List)
	authed.GET("/docs/:id", middleware.RequirePerm("purchase", "sales", "stock", "finance"), docHandler.Get)
	authed.POST("/docs/:id/confirm", middleware.RequirePerm("purchase", "sales", "stock"), docHandler.Confirm)
	authed.POST("/docs/:id/void", middleware.RequirePerm("purchase", "sales", "stock"), docHandler.Void)
	// 盘点差异审批（v0.16）：仅 system 权限（主管）可审批
	authed.POST("/docs/:id/review", middleware.RequirePerm("system"), docHandler.Review)

	// M2 应收应付（权限：finance）
	finSvc := service.NewFinanceService(db)
	finHandler := api.NewFinanceHandler(finSvc)
	authed.GET("/finance/balances", middleware.RequirePerm("finance"), finHandler.Balances)
	authed.GET("/finance/payments", middleware.RequirePerm("finance"), finHandler.ListPayments)
	authed.POST("/finance/payments", middleware.RequirePerm("finance"), finHandler.CreatePayment)
	authed.POST("/finance/payments/:id/void", middleware.RequirePerm("finance"), finHandler.VoidPayment)

	// M3 库存查询 / 报表 / 看板
	reportSvc := service.NewReportService(db)
	stockSvc := service.NewStockService(db)
	reportHandler := api.NewReportHandler(reportSvc, stockSvc)
	authed.GET("/reports/summary", middleware.RequirePerm("reports"), reportHandler.Summary)
	authed.GET("/reports/profit", middleware.RequirePerm("reports"), reportHandler.Profit)
	authed.GET("/dashboard/stats", reportHandler.Dashboard)
	authed.GET("/inventory", middleware.RequirePerm("stock", "sales", "purchase"), reportHandler.Stocks)
	authed.GET("/inventory/:id/batches", middleware.RequirePerm("stock", "sales", "purchase"), reportHandler.Batches)

	// M4 系统管理（权限：system；改密/看板除外）
	sysHandler := api.NewSystemHandler(sysSvc, lic)
	authed.GET("/system/users", middleware.RequirePerm("system"), sysHandler.ListUsers)
	authed.POST("/system/users", middleware.RequirePerm("system"), sysHandler.CreateUser)
	authed.PUT("/system/users/:id", middleware.RequirePerm("system"), sysHandler.UpdateUser)
	authed.POST("/system/users/:id/password", middleware.RequirePerm("system"), sysHandler.ResetPassword)
	authed.GET("/system/roles", middleware.RequirePerm("system"), sysHandler.ListRoles)
	authed.POST("/system/roles", middleware.RequirePerm("system"), sysHandler.SaveRole)
	authed.POST("/system/roles/:id", middleware.RequirePerm("system"), sysHandler.SaveRole)
	authed.DELETE("/system/roles/:id", middleware.RequirePerm("system"), sysHandler.DeleteRole)
	authed.GET("/system/settings", sysHandler.GetSettings)
	authed.PUT("/system/settings", middleware.RequirePerm("system"), sysHandler.SaveSettings)
	authed.GET("/system/backups", middleware.RequirePerm("system"), sysHandler.ListBackups)
	authed.POST("/system/backups", middleware.RequirePerm("system"), sysHandler.CreateBackup)
	authed.GET("/system/backups/:name", middleware.RequirePerm("system"), sysHandler.DownloadBackup)
	authed.GET("/system/audit-logs", middleware.RequirePerm("system"), sysHandler.ListAuditLogs)
	authed.POST("/auth/password", sysHandler.ChangePassword)

	// M5 授权（读与导入不设权限门槛：试用期满的账号也能自助导入新授权）
	authed.GET("/system/license", sysHandler.GetLicense)
	authed.POST("/system/license", sysHandler.UploadLicense)

	// M7 演示数据一键初始化（多行业模板：五金店/便利店/生鲜/批发）
	// 公开入口：登录页「一键体验演示数据」；系统入口：系统管理页（需 system 权限）
	demoSvc := service.NewDemoService(db)
	demoHandler := api.NewDemoHandler(demoSvc)
	v1.GET("/demo/templates", demoHandler.Templates)
	v1.POST("/demo/init", demoHandler.PublicInit)
	authed.POST("/system/demo-data", middleware.RequirePerm("system"), demoHandler.ResetAuthed)

	// M5 静态托管：生产模式下前端由本二进制直接服务（单文件部署）
	sub, err := fs.Sub(webembed.FS, "webdist")
	if err != nil {
		return fmt.Errorf("内嵌前端资源损坏: %w", err)
	}
	if _, err := fs.Stat(sub, "index.html"); err != nil {
		// 源码直接运行时（未执行 build.sh / npm run build）只有占位文件：
		// 接口照常可用，根路径给出提示，避免误以为服务坏了
		log.Printf("提示: 未检测到前端构建产物，仅提供 API。开发请用 `cd web && npm run dev`，生产请执行 build.sh / build.bat")
		r.GET("/", func(c *gin.Context) {
			c.String(http.StatusOK, "JXC 服务已启动（前端未构建）。\n\n开发：cd web && npm install && npm run dev，访问 http://localhost:5173\n发布：项目根目录执行 build.sh（Linux/macOS）或 build.bat（Windows）\n")
		})
	} else {
		fileServer := http.StripPrefix("/", http.FileServer(http.FS(sub)))
		r.NoRoute(func(c *gin.Context) {
			if strings.HasPrefix(c.Request.URL.Path, "/api/") {
				response.Fail(c, errcode.New(1002, "接口不存在"))
				return
			}
			// 缓存策略：带 hash 的静态资源长缓存；index.html 不缓存（保证升级后浏览器立即拿到新版本）
			if strings.HasPrefix(c.Request.URL.Path, "/assets/") {
				c.Header("Cache-Control", "public, max-age=31536000, immutable")
			} else {
				c.Header("Cache-Control", "no-cache")
			}
			// SPA fallback：文件不存在时回 index.html（/print/:id 等前端路由）
			if _, err := fs.Stat(sub, strings.TrimPrefix(c.Request.URL.Path, "/")); err != nil {
				c.Request.URL.Path = "/"
			}
			fileServer.ServeHTTP(c.Writer, c.Request)
		})
	}

	if len(cfg.TrustedProxies) == 0 {
		log.Printf("反向代理：未信任任何代理（JXC_TRUSTED_PROXIES=none），客户端 IP 取 TCP 对端")
	} else {
		log.Printf("反向代理：信任来源 %v（Caddy/Nginx 同机部署用默认值即可）", cfg.TrustedProxies)
	}

	srv := &http.Server{Addr: ":" + cfg.Port, Handler: r}
	errCh := make(chan error, 1)
	go func() {
		if err := srv.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			errCh <- err
		}
	}()
	log.Printf("服务启动: http://localhost:%s（对外请经 Caddy 反向代理走 HTTPS）", cfg.Port)

	select {
	case err := <-errCh:
		return fmt.Errorf("监听端口 %s 失败: %w", cfg.Port, err)
	case <-ctx.Done():
	}
	log.Printf("收到退出信号，正在关闭服务…")
	shutCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	if err := srv.Shutdown(shutCtx); err != nil {
		return fmt.Errorf("优雅关闭超时: %w", err)
	}
	log.Printf("已安全退出")
	return nil
}

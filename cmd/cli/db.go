package cli

// import (
// 	"W-dev/internal/service"
// 	"context"
// 	"fmt"
// 	"log"
// )

// func RunDBProvision(cmd *cobra.Command, args []string) {
// 	engine := args[0] // postgres|mysql|mongodb|redis
// 	appSlug := args[1]

// 	req := service.DBProvisionRequest{
// 		AppSlug: appSlug,
// 		Engine:  service.DBEngine(engine),
// 	}

// 	result, err := dbSvc.Provision(context.Background(), req)
// 	if err != nil {
// 		log.Fatalf("gagal provision database: %v", err)
// 	}

// 	fmt.Printf("✅ Database %s siap!\n", engine)
// 	fmt.Printf("   Host:     %s:%d\n", result.Host, result.Port)
// 	fmt.Printf("   Database: %s\n", result.Database)
// 	fmt.Printf("   Username: %s\n", result.Username)
// 	fmt.Printf("   Password: %s\n", result.Password)
// 	fmt.Printf("   URI:      %s\n", result.ConnectionURI)
// }

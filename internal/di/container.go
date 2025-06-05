package di

import (
	usecase "github.com/HoBom-s/hobom-event-processor/internal/usecase"
	"go.uber.org/fx"
	"gorm.io/driver/mysql"
	"gorm.io/gorm"
)

var Module = fx.Options(
	fx.Provide(
		NewGormDB,
		usecase.NewOutboxProcessor,
	),
	fx.Invoke(usecase.RunOutboxProcessor),
)

func NewGormDB() (*gorm.DB, error) {
	dsn := "user:password@tcp(localhost:3306)/dbname?parseTime=true"
	return gorm.Open(mysql.Open(dsn), &gorm.Config{})
}
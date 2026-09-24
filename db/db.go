package db

import (
	"errors"
	"fmt"

	"LinkDesk/config"

	"gorm.io/driver/mysql"
	"gorm.io/gorm"
)

var Db *gorm.DB

func Connect() error {
	cfg := config.Config.Database
	// 地址、端口、库名、账号都从配置来,不写死在代码里
	dsn := fmt.Sprintf("%s:%s@tcp(%s:%s)/%s?charset=utf8mb4&parseTime=True&loc=Local",
		cfg.User, cfg.Password, cfg.Host, cfg.Port, cfg.Path)

	// TranslateError:把驱动的原生错误(如 MySQL 1062 重复键)翻译成 gorm.ErrDuplicatedKey
	db, err := gorm.Open(mysql.Open(dsn), &gorm.Config{TranslateError: true})
	if err != nil {
		panic("[DB] failed to connect database because: " + err.Error())
		return err
	}

	// 自动迁移:让数据库里的表结构,和 model 结构体保持一致(没有表就建表)。
	db.AutoMigrate(&User{})
	db.AutoMigrate(&Link{})
	db.AutoMigrate(&Session{})

	Db = db

	return nil
}

// Ping 检查数据库连接是否还可用,给 /health 用。
func Ping() error {
	if Db == nil {
		return errors.New("数据库尚未连接")
	}
	sqlDB, err := Db.DB()
	if err != nil {
		return err
	}
	return sqlDB.Ping()
}

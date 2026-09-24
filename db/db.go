package db

import (
	"LinkDesk/config"

	"gorm.io/driver/mysql"
	"gorm.io/gorm"
)

var Db *gorm.DB

func Connect() error {
	dsn := config.Config.Database.User + ":" + config.Config.Database.Password + "@tcp(127.0.0.1:3306)/" + config.Config.Database.Path + "?charset=utf8mb4&parseTime=True&loc=Local"
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

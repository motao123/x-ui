package database

import (
	"os"
	"path"
	"x-ui/config"
	"x-ui/database/model"
	"x-ui/logger"
	"x-ui/util/random"

	"github.com/glebarez/sqlite"
	"gorm.io/gorm"
	gormLogger "gorm.io/gorm/logger"
)

var db *gorm.DB

func initUser(dbPath string) error {
	err := db.AutoMigrate(&model.User{})
	if err != nil {
		return err
	}
	var count int64
	err = db.Model(&model.User{}).Count(&count).Error
	if err != nil {
		return err
	}
	if count == 0 {
		password := random.Seq(16)
		user := &model.User{
			Username: "admin",
			Password: password,
		}
		passwordFile := dbPath + ".initial-admin-password"
		if writeErr := os.WriteFile(passwordFile, []byte(password+"\n"), 0600); writeErr != nil {
			logger.Warning("no user found, created initial admin user with random password. username: admin. Failed to write initial password file: ", writeErr)
		} else {
			logger.Warning("no user found, created initial admin user with random password. username: admin. Initial password was written to ", passwordFile, " . Delete this file after changing the password.")
		}
		return db.Create(user).Error
	}
	return nil
}

func initInbound() error {
	return db.AutoMigrate(&model.Inbound{})
}

func initSetting() error {
	return db.AutoMigrate(&model.Setting{})
}

func InitDB(dbPath string) error {
	dir := path.Dir(dbPath)
	err := os.MkdirAll(dir, 0700)
	if err != nil {
		return err
	}

	var dbLogger gormLogger.Interface

	if config.IsDebug() {
		dbLogger = gormLogger.Default
	} else {
		dbLogger = gormLogger.Discard
	}

	c := &gorm.Config{
		Logger: dbLogger,
	}
	db, err = gorm.Open(sqlite.Open(dbPath), c)
	if err != nil {
		return err
	}

	err = initUser(dbPath)
	if err != nil {
		return err
	}
	err = initInbound()
	if err != nil {
		return err
	}
	err = initSetting()
	if err != nil {
		return err
	}

	return nil
}

func GetDB() *gorm.DB {
	return db
}

func IsNotFound(err error) bool {
	return err == gorm.ErrRecordNotFound
}

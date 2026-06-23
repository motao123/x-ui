package database

import (
	"os"
	"path"
	"time"
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
			logger.Warning("no user found, created initial admin user with random password. username: admin. Initial password was written to ", passwordFile, " . It will be removed automatically after first login or password change.")
		}
		return db.Create(user).Error
	}
	return nil
}

// RemoveInitialPasswordFile 删除首次安装时生成的随机口令文件。
// 在管理员首次登录或修改密码后调用，避免明文口令长期残留在磁盘上。
// 文件不存在时返回 nil（视为已清理）。
func RemoveInitialPasswordFile() error {
	passwordFile := config.GetDBPath() + ".initial-admin-password"
	err := os.Remove(passwordFile)
	if err == nil {
		logger.Info("removed initial admin password file:", passwordFile)
		return nil
	}
	if os.IsNotExist(err) {
		return nil
	}
	return err
}

func initInbound() error {
	return db.AutoMigrate(&model.Inbound{})
}

func initSetting() error {
	return db.AutoMigrate(&model.Setting{})
}

func initTrafficHistory() error {
	if err := db.AutoMigrate(&model.TrafficHistory{}); err != nil {
		return err
	}
	// 清理 30 天前的历史记录
	cutoff := time.Now().AddDate(0, 0, -30).Unix()
	return db.Where("record_at < ?", cutoff).Delete(&model.TrafficHistory{}).Error
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
	err = initTrafficHistory()
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

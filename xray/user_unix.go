//go:build !windows

package xray

import (
	"os"
	"os/exec"
	"os/user"
	"strconv"
	"syscall"

	"x-ui/logger"
)

const xrayRunUser = "xray"

// setProcessUser 尝试以低权限 xray 用户运行 xray 进程
func setProcessUser(cmd *exec.Cmd) {
	u, err := user.Lookup(xrayRunUser)
	if err != nil {
		logger.Debugf("xray user %q not found, running as current user", xrayRunUser)
		return
	}
	uid, err := strconv.Atoi(u.Uid)
	if err != nil {
		logger.Debugf("invalid xray uid %q: %v", u.Uid, err)
		return
	}
	gid, err := strconv.Atoi(u.Gid)
	if err != nil {
		logger.Debugf("invalid xray gid %q: %v", u.Gid, err)
		return
	}
	cmd.SysProcAttr = &syscall.SysProcAttr{
		Credential: &syscall.Credential{
			Uid: uint32(uid),
			Gid: uint32(gid),
		},
	}
	logger.Infof("xray process will run as user %q (uid=%d, gid=%d)", xrayRunUser, uid, gid)
}

// setConfigFileOwner 将配置文件属主改为 xray 用户，确保 xray 进程可读
func setConfigFileOwner(path string) {
	u, err := user.Lookup(xrayRunUser)
	if err != nil {
		logger.Debugf("xray user %q not found, keeping config file owner as-is", xrayRunUser)
		return
	}
	uid, err := strconv.Atoi(u.Uid)
	if err != nil {
		logger.Debugf("invalid xray uid %q: %v", u.Uid, err)
		return
	}
	gid, err := strconv.Atoi(u.Gid)
	if err != nil {
		logger.Debugf("invalid xray gid %q: %v", u.Gid, err)
		return
	}
	if err := os.Chown(path, uid, gid); err != nil {
		logger.Debugf("failed to chown config file to %q: %v", xrayRunUser, err)
		return
	}
	logger.Infof("config file owner changed to %q (uid=%d, gid=%d)", xrayRunUser, uid, gid)
}

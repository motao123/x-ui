//go:build !windows

package controller

import (
	"os"
	"os/user"
	"strconv"
)

func setAcmeCertificatePermissions(paths ...string) error {
	group, err := user.LookupGroup("xray")
	if err != nil {
		return nil
	}
	gid, err := strconv.Atoi(group.Gid)
	if err != nil {
		return err
	}
	for i, path := range paths {
		if err := os.Chown(path, 0, gid); err != nil {
			return err
		}
		perm := os.FileMode(0750)
		if i >= len(paths)-2 {
			perm = 0640
		}
		if err := os.Chmod(path, perm); err != nil {
			return err
		}
	}
	return nil
}

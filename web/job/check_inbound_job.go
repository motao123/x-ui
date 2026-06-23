package job

import (
	"x-ui/logger"
	"x-ui/util/common"
	"x-ui/web/service"
)

type CheckInboundJob struct {
	xrayService    service.XrayService
	inboundService service.InboundService
}

func NewCheckInboundJob() *CheckInboundJob {
	return new(CheckInboundJob)
}

func (j *CheckInboundJob) Run() {
	defer common.Recover("CheckInboundJob")
	count, err := j.inboundService.DisableInvalidInbounds()
	if err != nil {
		logger.Warning("disable invalid inbounds err:", err)
	} else if count > 0 {
		logger.Debugf("disabled %v inbounds", count)
		j.xrayService.SetToNeedRestart()
	}
}

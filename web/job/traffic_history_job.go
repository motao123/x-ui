package job

import (
	"time"

	"x-ui/database"
	"x-ui/database/model"
	"x-ui/logger"
	"x-ui/util/common"
	"x-ui/web/service"
)

type TrafficHistoryJob struct {
	inboundService service.InboundService
}

func NewTrafficHistoryJob() *TrafficHistoryJob {
	return new(TrafficHistoryJob)
}

func (j *TrafficHistoryJob) Run() {
	defer common.Recover("TrafficHistoryJob")
	inbounds, err := j.inboundService.GetAllInbounds()
	if err != nil {
		logger.Warning("traffic history job get inbounds failed:", err)
		return
	}
	var totalUp, totalDown int64
	for _, inbound := range inbounds {
		totalUp += inbound.Up
		totalDown += inbound.Down
	}
	record := &model.TrafficHistory{
		Up:       totalUp,
		Down:     totalDown,
		RecordAt: time.Now().Unix(),
	}
	if err := database.GetDB().Create(record).Error; err != nil {
		logger.Warning("traffic history job create failed:", err)
	}
}

package handlers

import (
	"saferoute/models"
	"saferoute/services"
)

var motorSyncSvc *services.MotorSyncService

func SetMotorSyncService(svc *services.MotorSyncService) {
	motorSyncSvc = svc
}

func syncReporteCreado(reporte models.ReporteResponse) {
	if motorSyncSvc == nil {
		return
	}
	go motorSyncSvc.SyncReporteCreado(reporte)
}

func syncReporteValidado(reporteID string, vigente bool) {
	if motorSyncSvc == nil {
		return
	}
	go motorSyncSvc.SyncReporteValidado(reporteID, vigente)
}

func syncInteraccionMotor(tipo, userID, rutaID string, data map[string]interface{}) {
	if motorSyncSvc == nil {
		return
	}
	go motorSyncSvc.SyncInteraccion(tipo, userID, rutaID, data)
}

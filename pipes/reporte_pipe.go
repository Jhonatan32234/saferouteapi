package pipes

import (
	"fmt"
	"strings"

	"saferoute/models"
)

// tipos de incidentes permitidos (sincronizado con models.TiposValidos)
var tiposPermitidosPipe = map[string]bool{
	"accidente":  true,
	"inundacion": true,
	"bache":      true,
	"derrumbe":   true,
	"sin_luz":    true,
	"niebla":     true,
	"bloqueo":    true,
	"otro":       true,
}

// ValidateReporte valida y sanitiza el DTO de creación de reporte.
// Aplica transformaciones en lugar de solo rechazar: rellena campos opcionales
// con valores por defecto seguros para reducir errores del cliente.
func ValidateReporte(req *models.ReporteRequest) error {
	// Normalizar tipo a minúsculasa
	req.Tipo = strings.ToLower(strings.TrimSpace(req.Tipo))

	if req.Tipo == "" {
		return fmt.Errorf("el campo 'tipo' es requerido")
	}
	if !tiposPermitidosPipe[req.Tipo] {
		tipos := make([]string, 0, len(tiposPermitidosPipe))
		for t := range tiposPermitidosPipe {
			tipos = append(tipos, t)
		}
		return fmt.Errorf("tipo inválido '%s'. Valores permitidos: %s", req.Tipo, strings.Join(tipos, ", "))
	}

	// Validar coordenadas con rangos geográficos básicos
	if req.Latitud == 0 && req.Longitud == 0 {
		return fmt.Errorf("los campos 'latitud' y 'longitud' son requeridos")
	}
	if req.Latitud < -90 || req.Latitud > 90 {
		return fmt.Errorf("la latitud debe estar entre -90 y 90")
	}
	if req.Longitud < -180 || req.Longitud > 180 {
		return fmt.Errorf("la longitud debe estar entre -180 y 180")
	}

	// Valor por defecto para ruta_id opcional
	if strings.TrimSpace(req.RutaID) == "" {
		req.RutaID = "sin-ruta"
	}

	// Sanitizar nota de voz: truncar y limpiar espacios
	req.NotaVoz = strings.TrimSpace(req.NotaVoz)
	if len(req.NotaVoz) > 300 {
		req.NotaVoz = req.NotaVoz[:297] + "..."
	}

	// Si no hay nota de voz, generar descripción automática por tipo
	if req.NotaVoz == "" {
		req.NotaVoz = generarDescripcion(req.Tipo)
	}

	return nil
}

// generarDescripcion produce una nota de voz automática para cada tipo de incidente.
func generarDescripcion(tipo string) string {
	descripciones := map[string]string{
		"accidente":  "Accidente vial reportado en la vía",
		"inundacion": "Inundación reportada, precaución al circular",
		"bache":      "Bache en la vía, reducir velocidad",
		"derrumbe":   "Derrumbe o deslizamiento reportado en la carretera",
		"sin_luz":    "Zona sin iluminación, precaución nocturna",
		"niebla":     "Banco de niebla densa, visibilidad reducida",
		"bloqueo":    "Bloqueo vial reportado",
		"otro":       "Incidente vial reportado",
	}
	if desc, ok := descripciones[tipo]; ok {
		return desc
	}
	return "Incidente vial reportado"
}

package reporte

import (
	"fmt"
	"strings"
)

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

func ValidateReporte(req *ReporteRequest) error {
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

	if req.Latitud == 0 && req.Longitud == 0 {
		return fmt.Errorf("los campos 'latitud' y 'longitud' son requeridos")
	}
	if req.Latitud < -90 || req.Latitud > 90 {
		return fmt.Errorf("la latitud debe estar entre -90 y 90")
	}
	if req.Longitud < -180 || req.Longitud > 180 {
		return fmt.Errorf("la longitud debe estar entre -180 y 180")
	}

	if strings.TrimSpace(req.RutaID) == "" {
		req.RutaID = "sin-ruta"
	}

	req.NotaVoz = strings.TrimSpace(req.NotaVoz)
	if len(req.NotaVoz) > 300 {
		req.NotaVoz = req.NotaVoz[:297] + "..."
	}

	if req.NotaVoz == "" {
		req.NotaVoz = generarDescripcion(req.Tipo)
	}

	return nil
}

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

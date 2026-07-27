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

// palabrasClavePorTipo mapea cada tipo de incidente con palabras clave esperadas en la nota de voz.
var palabrasClavePorTipo = map[string][]string{
	"accidente":  {"choque", "colisión", "accidente", "impacto", "volcadura", "choqué", "chocó", "golpe", "atropell", "estrell", "chocar"},
	"inundacion": {"inundación", "inundado", "inundada", "agua", "lluvia", "anegamiento", "charco", "crecida", "desbordamiento", "lloviendo", "anegado", "mojado", "inund"},
	"bache":      {"bache", "hoyo", "hueco", "hundimiento", "depresión", "baches", "hoyos"},
	"derrumbe":   {"derrumbe", "deslave", "deslizamiento", "tierra", "roca", "cerro", "derrumb", "derrumbado", "derrumbes", "piedra"},
	"sin_luz":    {"luz", "iluminación", "oscuro", "apagón", "farol", "alumbrado", "oscuridad", "apagado", "lámpara", "sin luz"},
	"niebla":     {"niebla", "neblina", "visibilidad", "bruma", "nube", "densa", "neblinoso"},
	"bloqueo":    {"bloqueo", "cerrado", "obstrucción", "manifestación", "cierre", "tráfico", "bloqueado", "corte", "cerrada", "obstruido"},
}

// palabrasContradictoriasPorTipo mapea cada tipo con palabras que indican una posible contradicción
// (el usuario seleccionó un tipo pero dictó algo completamente diferente).
var palabrasContradictoriasPorTipo = map[string][]string{
	"inundacion": {"incendio", "fuego", "quemando", "quemó", "quema", "humo", "ardiente", "quemar", "incendi"},
	"accidente":  {"inundación", "inundado", "inundada", "lluvia", "inund"},
	"bache":      {"incendio", "inundación", "niebla", "inund", "incendi"},
	"niebla":     {"inundación", "incendio", "bache", "inund", "incendi"},
	"sin_luz":    {"inundación", "incendio", "inund", "incendi"},
	"derrumbe":   {"incendio", "inundación", "inund", "incendi"},
	"bloqueo":    {"inundación", "incendio", "inund", "incendi"},
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

	// Si el usuario proporcionó una nota de voz, validar coherencia con el tipo seleccionado
	if req.NotaVoz != "" {
		if err := validarCoherenciaNotaVoz(req.Tipo, req.NotaVoz); err != nil {
			return err
		}
	}

	// Si no hay nota de voz, generar una descripción automática
	if req.NotaVoz == "" {
		req.NotaVoz = generarDescripcion(req.Tipo)
	}

	return nil
}

// validarCoherenciaNotaVoz verifica que el contenido de la nota de voz sea coherente
// con el tipo de incidente seleccionado por el usuario.
func validarCoherenciaNotaVoz(tipo, notaVoz string) error {
	notaLower := strings.ToLower(notaVoz)

	// 1. Verificar si hay palabras contradictorias (el usuario probablemente se equivocó de tipo)
	if contradictorias, ok := palabrasContradictoriasPorTipo[tipo]; ok {
		for _, palabra := range contradictorias {
			if strings.Contains(notaLower, palabra) {
				return fmt.Errorf(
					"la nota de voz menciona '%s', lo cual no coincide con el tipo '%s'. "+
						"Por favor verifica el tipo de incidente o corrige la descripción",
					palabra, tipo,
				)
			}
		}
	}

	// 2. Para tipos específicos (no "otro"), verificar que al menos una palabra clave coincida
	if tipo != "otro" {
		palabras, ok := palabrasClavePorTipo[tipo]
		if !ok {
			return nil // tipo sin palabras clave definidas, no validamos
		}

		tieneCoincidencia := false
		for _, palabra := range palabras {
			if strings.Contains(notaLower, palabra) {
				tieneCoincidencia = true
				break
			}
		}

		if !tieneCoincidencia {
			return fmt.Errorf(
				"la nota de voz no parece describir un '%s'. "+
					"Asegúrate de que la descripción coincida con el tipo de incidente seleccionado, "+
					"o selecciona el tipo 'otro' si no encuentras una categoría adecuada",
				tipo,
			)
		}
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
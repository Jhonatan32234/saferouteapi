package services

import (
	"math"
	"testing"
)

func TestDecodePolyline(t *testing.T) {
	// Polilínea codificada para los puntos:
	// (38.5, -120.2), (40.7, -120.95), (43.252, -126.453)
	// Algoritmo de Google Polyline estándar
	encoded := "_p~iF~ps|U_ulLnnqC_mqNvxq`@"
	
	expected := [][2]float64{
		{38.5, -120.2},
		{40.7, -120.95},
		{43.252, -126.453},
	}

	coords, err := DecodePolyline(encoded)
	if err != nil {
		t.Fatalf("Error inesperado al decodificar: %v", err)
	}

	if len(coords) != len(expected) {
		t.Fatalf("Longitud esperada %d, obtenida %d", len(expected), len(coords))
	}

	const margin = 0.0001
	for i, pt := range coords {
		exp := expected[i]
		if math.Abs(pt[0]-exp[0]) > margin || math.Abs(pt[1]-exp[1]) > margin {
			t.Errorf("Índice %d: se esperaba %+v, obtenido %+v", i, exp, pt)
		}
	}
}

func TestHaversineDistance(t *testing.T) {
	// Distancia entre Casa Blanca (38.8977, -77.0365) y Monumento a Washington (38.8895, -77.0353)
	// La distancia geodésica real es de aproximadamente 918 metros
	lat1, lon1 := 38.8977, -77.0365
	lat2, lon2 := 38.8895, -77.0353

	expectedMeters := 918.0
	calculated := HaversineDistance(lat1, lon1, lat2, lon2)

	// Margen de tolerancia de 10 metros para Haversine sobre la Tierra
	if math.Abs(calculated-expectedMeters) > 10.0 {
		t.Errorf("Distancia esperada ~%.1f metros, calculada %.1f metros", expectedMeters, calculated)
	}
}

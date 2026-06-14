package handlers

import (
	"bytes"
	"encoding/json"
	"net/http"

	"saferoute/models"
)

func GetAdminResumenHandler(motorLLMURL string) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		// TODO: Estas URLs deberían venir de tu archivo de configuración
		motorNLPURL := "http://localhost:8001"

		// 1. Obtener tópicos de LDA
		respNLP, err := http.Get(motorNLPURL + "/nlp/topicos")
		if err != nil {
			writeError(w, http.StatusServiceUnavailable, "servicio NLP no disponible")
			return
		}
		defer respNLP.Body.Close()

		var topicosResp struct {
			Topicos []models.TopicoInfo `json:"topicos"`
		}
		json.NewDecoder(respNLP.Body).Decode(&topicosResp)

		// 2. Generar resumen con LLM
		llmReq := map[string]interface{}{
			"topicos": topicosResp.Topicos,
		}
		llmBody, _ := json.Marshal(llmReq)

		respLLM, err := http.Post(
			motorLLMURL+"/llm/resumen",
			"application/json",
			bytes.NewBuffer(llmBody),
		)

		var resumenLLM string
		if err == nil {
			defer respLLM.Body.Close()
			var llmResp struct {
				Resumen string `json:"resumen"`
			}
			json.NewDecoder(respLLM.Body).Decode(&llmResp)
			resumenLLM = llmResp.Resumen
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(models.AdminResumenResponse{
			TotalReportes: 200,
			Topicos:       topicosResp.Topicos,
			ResumenLLM:    resumenLLM,
		})
	}
}

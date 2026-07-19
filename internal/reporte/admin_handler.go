package reporte

import (
	"bytes"
	"encoding/json"
	"io"
	"log"
	"net/http"

	"saferoute/internal/common"
)

type AdminHandler struct {
	motorNLPURL string
	motorLLMURL string
}

func NewAdminHandler(motorNLPURL, motorLLMURL string) *AdminHandler {
	return &AdminHandler{
		motorNLPURL: motorNLPURL,
		motorLLMURL: motorLLMURL,
	}
}

func (h *AdminHandler) GetAdminResumenHandler() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		respNLP, err := http.Get(h.motorNLPURL + "/nlp/topicos?n_topicos=5")
		if err != nil {
			log.Printf("ERROR motor NLP (topicos): %v", err)
			common.WriteError(w, http.StatusServiceUnavailable, "servicio NLP no disponible")
			return
		}
		defer respNLP.Body.Close()

		nlpBody, err := io.ReadAll(respNLP.Body)
		if err != nil {
			common.WriteError(w, http.StatusInternalServerError, "error leyendo respuesta NLP")
			return
		}

		var nlpResponse struct {
			Topicos         []map[string]interface{} `json:"topicos"`
			TotalReportes   int                      `json:"total_reportes"`
			TopicoDominante map[string]interface{}   `json:"topico_dominante"`
		}
		if err := json.Unmarshal(nlpBody, &nlpResponse); err != nil {
			log.Printf("ERROR parseando NLP: %v", err)
			common.WriteError(w, http.StatusInternalServerError, "error procesando datos NLP")
			return
		}

		llmReq := map[string]interface{}{
			"topicos":          nlpResponse.Topicos,
			"total_reportes":   nlpResponse.TotalReportes,
			"topico_dominante": nlpResponse.TopicoDominante,
		}
		llmBody, _ := json.Marshal(llmReq)

		respLLM, err := http.Post(
			h.motorLLMURL+"/llm/resumen",
			"application/json",
			bytes.NewBuffer(llmBody),
		)

		var resumenLLM string
		if err == nil {
			defer respLLM.Body.Close()
			llmRespBody, _ := io.ReadAll(respLLM.Body)
			var llmResponse struct {
				Resumen string `json:"resumen"`
			}
			if err := json.Unmarshal(llmRespBody, &llmResponse); err == nil {
				resumenLLM = llmResponse.Resumen
			}
		}

		if resumenLLM == "" {
			resumenLLM = "Resumen no disponible en este momento."
		}

		respuesta := map[string]interface{}{
			"topicos":          nlpResponse.Topicos,
			"total_reportes":   nlpResponse.TotalReportes,
			"topico_dominante": nlpResponse.TopicoDominante,
			"resumen_llm":      resumenLLM,
		}

		w.Header().Set("Content-Type", "application/json")
		w.Header().Set("X-Data-Source", "motor-nlp+llm")
		json.NewEncoder(w).Encode(respuesta)
	}
}

func (h *AdminHandler) BuscarReportesHandler() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		bodyBytes, err := io.ReadAll(r.Body)
		if err != nil {
			common.WriteError(w, http.StatusBadRequest, "error leyendo datos de entrada")
			return
		}
		defer r.Body.Close()

		var req map[string]interface{}
		if err := json.Unmarshal(bodyBytes, &req); err != nil {
			common.WriteError(w, http.StatusBadRequest, "datos de entrada inválidos")
			return
		}

		query, ok := req["query"].(string)
		if !ok || query == "" {
			common.WriteError(w, http.StatusBadRequest, "campo 'query' es requerido")
			return
		}

		log.Printf("Buscando reportes: '%s'", query)

		respNLP, err := http.Post(
			h.motorNLPURL+"/nlp/buscar",
			"application/json",
			bytes.NewBuffer(bodyBytes),
		)
		if err != nil {
			log.Printf("ERROR motor NLP (buscar): %v", err)
			common.WriteError(w, http.StatusServiceUnavailable, "servicio NLP no disponible")
			return
		}
		defer respNLP.Body.Close()

		respBody, err := io.ReadAll(respNLP.Body)
		if err != nil {
			common.WriteError(w, http.StatusInternalServerError, "error leyendo respuesta NLP")
			return
		}

		log.Printf("NLP búsqueda: %d bytes", len(respBody))

		w.Header().Set("Content-Type", "application/json")
		w.Header().Set("X-Data-Source", "motor-nlp")
		w.Write(respBody)
	}
}

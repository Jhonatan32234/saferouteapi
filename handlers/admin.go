package handlers

import (
	"bytes"
	"encoding/json"
	"io"
	"log"
	"net/http"

	"saferoute/models"
	"saferoute/pipes"
	"saferoute/services"
)


func RegistrarConductorHandler(authSvc *services.AuthService) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var req struct {
			Email    string `json:"email"`
			Password string `json:"password"`
			Nombre   string `json:"nombre"`
			Telefono string `json:"telefono"`
		}

		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			writeError(w, http.StatusBadRequest, "datos inválidos")
			return
		}

		validateReq := models.RegisterRequest{
			Email:    req.Email,
			Password: req.Password,
			Nombre:   req.Nombre,
			Tipo:     "conductor",
			Telefono: req.Telefono,
		}
		if err := pipes.ValidateRegister(&validateReq); err != nil {
			writeError(w, http.StatusBadRequest, err.Error())
			return
		}

		id, err := authSvc.RegisterConductor(validateReq.Email, validateReq.Password, validateReq.Nombre, validateReq.Telefono)
		if err != nil {
			writeError(w, http.StatusConflict, err.Error())
			return
		}

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusCreated)
		json.NewEncoder(w).Encode(map[string]string{
			"id":     id,
			"status": "conductor registrado",
			"email":  req.Email,
		})
	}
}

func GetAdminResumenHandler(motorNLPURL string, motorLLMURL string) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		respNLP, err := http.Get(motorNLPURL + "/nlp/topicos?n_topicos=5")
		if err != nil {
			log.Printf("ERROR motor NLP (topicos): %v", err)
			writeError(w, http.StatusServiceUnavailable, "servicio NLP no disponible")
			return
		}
		defer respNLP.Body.Close()

		nlpBody, err := io.ReadAll(respNLP.Body)
		if err != nil {
			writeError(w, http.StatusInternalServerError, "error leyendo respuesta NLP")
			return
		}

		var nlpResponse struct {
			Topicos         []map[string]interface{} `json:"topicos"`
			TotalReportes   int                      `json:"total_reportes"`
			TopicoDominante map[string]interface{}   `json:"topico_dominante"`
		}
		if err := json.Unmarshal(nlpBody, &nlpResponse); err != nil {
			log.Printf("ERROR parseando NLP: %v", err)
			writeError(w, http.StatusInternalServerError, "error procesando datos NLP")
			return
		}

		llmReq := map[string]interface{}{
			"topicos":          nlpResponse.Topicos,
			"total_reportes":   nlpResponse.TotalReportes,
			"topico_dominante": nlpResponse.TopicoDominante,
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

func BuscarReportesHandler(motorNLPURL string) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		bodyBytes, err := io.ReadAll(r.Body)
		if err != nil {
			writeError(w, http.StatusBadRequest, "error leyendo datos de entrada")
			return
		}
		defer r.Body.Close()

		var req map[string]interface{}
		if err := json.Unmarshal(bodyBytes, &req); err != nil {
			writeError(w, http.StatusBadRequest, "datos de entrada inválidos")
			return
		}

		query, ok := req["query"].(string)
		if !ok || query == "" {
			writeError(w, http.StatusBadRequest, "campo 'query' es requerido")
			return
		}

		log.Printf("Buscando reportes: '%s'", query)

		respNLP, err := http.Post(
			motorNLPURL+"/nlp/buscar",
			"application/json",
			bytes.NewBuffer(bodyBytes),
		)
		if err != nil {
			log.Printf("ERROR motor NLP (buscar): %v", err)
			writeError(w, http.StatusServiceUnavailable, "servicio NLP no disponible")
			return
		}
		defer respNLP.Body.Close()

		respBody, err := io.ReadAll(respNLP.Body)
		if err != nil {
			writeError(w, http.StatusInternalServerError, "error leyendo respuesta NLP")
			return
		}

		log.Printf("NLP búsqueda: %d bytes", len(respBody))

		w.Header().Set("Content-Type", "application/json")
		w.Header().Set("X-Data-Source", "motor-nlp")
		w.Write(respBody)
	}
}

package util

import (
	"encoding/json"
	"net/http"
)

type Resp struct {
	Data    interface{} `json:"data"`
	Message string      `json:"message"`
	Code    int         `json:"code"`
}

func ResponseSuccess(w http.ResponseWriter, data interface{}, message string) *Resp {
	if message == "" {
		message = "Success"
	}
	resp := &Resp{}
	resp.Data = data
	resp.Message = message
	resp.Code = 200

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(resp.Code)
	json.NewEncoder(w).Encode(resp)
	return resp
}

func ResponseError(w http.ResponseWriter, err string, code int) *Resp {
	if code == 0 {
		code = 400
	}
	resp := &Resp{}
	resp.Data = nil
	resp.Message = err
	resp.Code = code

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(resp.Code)
	json.NewEncoder(w).Encode(resp)
	return resp
}

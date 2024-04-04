package api

import (
	"database/sql"
	"encoding/json"
	"net/http"
	"time"

	"github.com/dennyaris/html-rotate/adapter/models"
	"github.com/dennyaris/html-rotate/util"
	"github.com/go-playground/validator"
	"github.com/gorilla/mux"
)

type Handler struct {
	DB *sql.DB
}

var validate *validator.Validate
var pageModel models.Page

func (h *Handler) CreatePage(w http.ResponseWriter, r *http.Request) {
	err := json.NewDecoder(r.Body).Decode(&pageModel)
	if err != nil {
		util.ResponseError(w, err.Error(), http.StatusBadRequest)
		return
	}

	validate = validator.New()
	if err := validate.Struct(pageModel); err != nil {
		util.ResponseError(w, err.Error(), http.StatusBadRequest)
		return
	}

	if err := pageModel.Create(h.DB); err != nil {
		util.ResponseError(w, err.Error(), http.StatusInternalServerError)
		return
	}

	pageModel.Created = time.Now().Format("2006-01-02 15:04:05")

	util.ResponseSuccess(w, pageModel, "Success created")
}

func (h *Handler) GetPage(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	pageID := vars["id"]

	if pageID == "" {
		util.ResponseError(w, "params is empty", http.StatusBadRequest)
		return
	}

	data, err := pageModel.Show(h.DB, pageID)
	if err != nil {
		util.ResponseError(w, err.Error(), http.StatusNotFound)
		return
	}

	util.ResponseSuccess(w, data, "")
}

func (h *Handler) Update(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	pageID := vars["id"]

	if pageID == "" {
		util.ResponseError(w, "params is empty", http.StatusBadRequest)
		return
	}

	data, err := pageModel.Show(h.DB, pageID)
	if err != nil {
		util.ResponseError(w, err.Error(), http.StatusNotFound)
		return
	}

	var page models.Page
	err = json.NewDecoder(r.Body).Decode(&page)
	if err != nil {
		util.ResponseError(w, err.Error(), http.StatusBadRequest)
		return
	}

	if page.PageKey != "" {
		data.PageKey = page.PageKey
	}
	if page.UrlKey != "" {
		data.UrlKey = page.UrlKey
	}
	if page.Url != "" {
		data.Url = page.Url
	}
	if page.IsRotator < 2 {
		data.IsRotator = page.IsRotator
	}
	if page.UserID > 0 {
		data.UserID = page.UserID
	}
	if page.SiteID > 0 {
		data.SiteID = page.SiteID
	}

	if err := pageModel.Update(h.DB, pageID, *data); err != nil {
		util.ResponseError(w, err.Error(), http.StatusInternalServerError)
		return
	}

	util.ResponseSuccess(w, nil, "Success update")
}

func (h *Handler) DeletePage(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	pageID := vars["id"]

	if pageID == "" {
		util.ResponseError(w, "params is empty", http.StatusBadRequest)
		return
	}

	_, err := pageModel.Show(h.DB, pageID)
	if err != nil {
		util.ResponseError(w, err.Error(), http.StatusNotFound)
		return
	}

	if err = pageModel.Delete(h.DB, pageID); err != nil {
		util.ResponseError(w, err.Error(), http.StatusInternalServerError)
		return
	}

	util.ResponseSuccess(w, nil, "success deleted")
}

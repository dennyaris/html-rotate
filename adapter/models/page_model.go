package models

import (
	"database/sql"
	"time"

	"github.com/dennyaris/html-rotate/util"
)

type Page struct {
	PageID    string `json:"page_id" validate:"required"`
	PageKey   string `json:"page_key" validate:"required"`
	UrlKey    string `json:"url_key" validate:"required"`
	Url       string `json:"url" validate:"required"`
	IsRotator int    `json:"is_rotator" validate:"required"`
	UserID    int    `json:"user_id" validate:"required"`
	SiteID    int    `json:"site_id" validate:"required"`
	Created   string `json:"created"`
}

func (p *Page) Create(db *sql.DB) error {
	q := "INSERT INTO page (page_id, page_key, url_key, url, is_rotator, user_id, site_id, created)" +
		"Values(?, UNHEX(?), UNHEX(?), ?, ?, ?, ?, ?)"

	p.PageKey = util.EncodeString(p.PageKey)
	p.UrlKey = util.EncodeString(p.UrlKey)

	_, err := db.Exec(q, p.PageID, p.PageKey, p.UrlKey, p.Url, p.IsRotator, p.UserID, p.SiteID, time.Now())
	if err != nil {
		return err
	}

	return nil
}

func (p *Page) Show(db *sql.DB, id string) (*Page, error) {
	var page Page
	q := "SELECT * FROM page where page_id = ?"
	err := db.QueryRow(q, id).Scan(&page.PageID, &page.PageKey, &page.UrlKey, &page.Url, &page.IsRotator, &page.UserID, &page.SiteID, &page.Created)
	if err != nil {
		return nil, err
	}

	return &page, nil
}

func (p *Page) Update(db *sql.DB, id string, data Page) error {
	q := "Update page set page_key=UNHEX(?), url_key=UNHEX(?), url=?, is_rotator=?, user_id=?, site_id=? " +
		"WHERE page_id = ?"

	data.PageKey = util.EncodeString(data.PageKey)
	data.UrlKey = util.EncodeString(data.UrlKey)

	_, err := db.Exec(q, data.PageKey, data.UrlKey, data.Url, data.IsRotator, data.UserID, data.SiteID, id)
	if err != nil {
		return err
	}

	return nil
}

func (p *Page) Delete(db *sql.DB, id string) error {
	q := "DELETE FROM page WHERE page_id = ?"

	_, err := db.Exec(q, id)
	if err != nil {
		return err
	}

	return nil
}

package server

import (
	"errors"
	"net/http"
	"strconv"
	"strings"

	"github.com/google/uuid"
	"github.com/martinsaul/lost/internal/qr"
	"github.com/martinsaul/lost/internal/store"
)

// tagDTO is the owner-facing JSON representation of a tag.
type tagDTO struct {
	GUID      string `json:"guid"`
	Name      string `json:"name"`
	ShowEmail bool   `json:"showEmail"`
	ShowPhone bool   `json:"showPhone"`
	Phone     string `json:"phone"`
	FoundURL  string `json:"foundUrl"`
	CreatedAt string `json:"createdAt"`
}

func (s *Server) tagToDTO(t *store.Tag) tagDTO {
	return tagDTO{
		GUID:      t.GUID,
		Name:      t.Name,
		ShowEmail: t.ShowEmail,
		ShowPhone: t.ShowPhone,
		Phone:     t.Phone,
		FoundURL:  s.foundURL(t.GUID),
		CreatedAt: t.CreatedAt.Format("2006-01-02T15:04:05Z07:00"),
	}
}

func (s *Server) foundURL(guid string) string {
	return s.cfg.BaseURL + "/found/" + guid
}

func (s *Server) handleListTags(w http.ResponseWriter, r *http.Request) {
	u := currentUser(r)
	tags, err := s.store.TagsByUser(u.ID)
	if err != nil {
		writeErr(w, http.StatusInternalServerError, "could not list tags")
		return
	}
	out := make([]tagDTO, len(tags))
	for i, t := range tags {
		out[i] = s.tagToDTO(t)
	}
	writeJSON(w, http.StatusOK, map[string]any{"tags": out})
}

type tagWriteBody struct {
	Name      string `json:"name"`
	ShowEmail bool   `json:"showEmail"`
	ShowPhone bool   `json:"showPhone"`
	Phone     string `json:"phone"`
}

func (s *Server) handleCreateTag(w http.ResponseWriter, r *http.Request) {
	u := currentUser(r)
	var body tagWriteBody
	if err := decodeJSON(r, &body); err != nil {
		writeErr(w, http.StatusBadRequest, "invalid request")
		return
	}
	name := strings.TrimSpace(body.Name)
	if len(name) > 200 {
		name = name[:200]
	}
	guid := uuid.NewString()
	t, err := s.store.CreateTag(u.ID, guid, name, body.ShowEmail, body.ShowPhone, strings.TrimSpace(body.Phone))
	if err != nil {
		writeErr(w, http.StatusInternalServerError, "could not create tag")
		return
	}
	writeJSON(w, http.StatusCreated, s.tagToDTO(t))
}

// ownedTag fetches a tag by guid and verifies the current user owns it.
func (s *Server) ownedTag(w http.ResponseWriter, r *http.Request) (*store.Tag, bool) {
	u := currentUser(r)
	guid := r.PathValue("guid")
	t, err := s.store.TagByGUID(guid)
	if err != nil {
		writeErr(w, http.StatusNotFound, "tag not found")
		return nil, false
	}
	if t.UserID != u.ID {
		// Don't reveal existence of other users' tags.
		writeErr(w, http.StatusNotFound, "tag not found")
		return nil, false
	}
	return t, true
}

func (s *Server) handleGetTag(w http.ResponseWriter, r *http.Request) {
	t, ok := s.ownedTag(w, r)
	if !ok {
		return
	}
	writeJSON(w, http.StatusOK, s.tagToDTO(t))
}

func (s *Server) handleUpdateTag(w http.ResponseWriter, r *http.Request) {
	u := currentUser(r)
	if _, ok := s.ownedTag(w, r); !ok {
		return
	}
	var body tagWriteBody
	if err := decodeJSON(r, &body); err != nil {
		writeErr(w, http.StatusBadRequest, "invalid request")
		return
	}
	name := strings.TrimSpace(body.Name)
	if len(name) > 200 {
		name = name[:200]
	}
	t, err := s.store.UpdateTag(r.PathValue("guid"), u.ID, name, body.ShowEmail, body.ShowPhone, strings.TrimSpace(body.Phone))
	if err != nil {
		writeErr(w, http.StatusInternalServerError, "could not update tag")
		return
	}
	writeJSON(w, http.StatusOK, s.tagToDTO(t))
}

func (s *Server) handleDeleteTag(w http.ResponseWriter, r *http.Request) {
	u := currentUser(r)
	if _, ok := s.ownedTag(w, r); !ok {
		return
	}
	if err := s.store.DeleteTag(r.PathValue("guid"), u.ID); err != nil {
		writeErr(w, http.StatusInternalServerError, "could not delete tag")
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

// handleTagQRSVG streams a scalable vector QR encoding the tag's found URL.
func (s *Server) handleTagQRSVG(w http.ResponseWriter, r *http.Request) {
	t, ok := s.ownedTag(w, r)
	if !ok {
		return
	}
	svg, err := qr.SVG(s.foundURL(t.GUID))
	if err != nil {
		writeErr(w, http.StatusInternalServerError, "could not render qr")
		return
	}
	w.Header().Set("Content-Type", "image/svg+xml")
	w.Header().Set("Content-Disposition", "attachment; filename=\"lost-"+t.GUID+".svg\"")
	_, _ = w.Write(svg)
}

// handleTagQRPNG streams a rasterized QR at ?size= pixels (default 1024).
func (s *Server) handleTagQRPNG(w http.ResponseWriter, r *http.Request) {
	t, ok := s.ownedTag(w, r)
	if !ok {
		return
	}
	size := 1024
	if v := r.URL.Query().Get("size"); v != "" {
		if n, err := strconv.Atoi(v); err == nil {
			size = n
		}
	}
	png, err := qr.PNG(s.foundURL(t.GUID), size)
	if err != nil {
		writeErr(w, http.StatusInternalServerError, "could not render qr")
		return
	}
	w.Header().Set("Content-Type", "image/png")
	w.Header().Set("Content-Disposition", "attachment; filename=\"lost-"+t.GUID+".png\"")
	_, _ = w.Write(png)
}

var _ = errors.Is

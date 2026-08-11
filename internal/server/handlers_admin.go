package server

import (
	"net/http"
	"time"
)

// adminUserDTO is one row of the admin user list.
type adminUserDTO struct {
	Email     string `json:"email"`
	CreatedAt string `json:"createdAt"`
	Tags      int    `json:"tags"`
	Reports   int    `json:"reports"`
}

func (s *Server) handleAdminUsers(w http.ResponseWriter, r *http.Request) {
	users, err := s.store.ListUsersWithStats()
	if err != nil {
		writeErr(w, http.StatusInternalServerError, "could not list users")
		return
	}
	out := make([]adminUserDTO, len(users))
	for i, u := range users {
		out[i] = adminUserDTO{
			Email:     u.Email,
			CreatedAt: u.CreatedAt.Format(time.RFC3339),
			Tags:      u.TagCount,
			Reports:   u.ReportCount,
		}
	}
	writeJSON(w, http.StatusOK, map[string]any{"users": out})
}

func (s *Server) handleAdminStats(w http.ResponseWriter, r *http.Request) {
	users, _ := s.store.CountUsers()
	scans, _ := s.store.CountScanEvents()
	reports, _ := s.store.CountFoundReports()
	writeJSON(w, http.StatusOK, map[string]any{
		"users":       users,
		"maxUsers":    s.cfg.MaxUsers,
		"scans":       scans,
		"reports":     reports,
		"geoProvider": s.cfg.GeoProvider,
	})
}

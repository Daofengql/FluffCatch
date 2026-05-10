package http

import (
	stdhttp "net/http"

	"fluffcatch/internal/uploads"
)

func (server *Server) createSubmissionLink(w stdhttp.ResponseWriter, r *stdhttp.Request) {
	eventID, ok := parseIDParam(w, r, "id")
	if !ok {
		return
	}
	var req uploads.CreateSubmissionLinkRequest
	if err := decodeJSON(r, &req); err != nil {
		writeError(w, stdhttp.StatusBadRequest, "invalid submission link payload")
		return
	}
	link, err := server.uploadService.CreateSubmissionLink(r.Context(), eventID, req)
	if err != nil {
		writeError(w, stdhttp.StatusBadRequest, err.Error())
		return
	}
	writeJSON(w, stdhttp.StatusCreated, map[string]any{"link": link})
}

func (server *Server) listSubmissionLinks(w stdhttp.ResponseWriter, r *stdhttp.Request) {
	eventID, ok := parseIDParam(w, r, "id")
	if !ok {
		return
	}
	links, err := server.uploadService.ListSubmissionLinks(r.Context(), eventID)
	if err != nil {
		writeError(w, stdhttp.StatusInternalServerError, "failed to list submission links")
		return
	}
	writeJSON(w, stdhttp.StatusOK, map[string]any{"links": links})
}

func (server *Server) revokeSubmissionLink(w stdhttp.ResponseWriter, r *stdhttp.Request) {
	eventID, ok := parseIDParam(w, r, "id")
	if !ok {
		return
	}
	linkID, ok := parseIDParam(w, r, "linkID")
	if !ok {
		return
	}
	revoked, err := server.uploadService.RevokeSubmissionLink(r.Context(), eventID, linkID)
	if err != nil {
		writeError(w, stdhttp.StatusInternalServerError, "failed to revoke submission link")
		return
	}
	if !revoked {
		writeError(w, stdhttp.StatusNotFound, "submission link not found")
		return
	}
	writeJSON(w, stdhttp.StatusOK, map[string]any{"message": "submission link revoked"})
}

func (server *Server) deleteRevokedSubmissionLink(w stdhttp.ResponseWriter, r *stdhttp.Request) {
	eventID, ok := parseIDParam(w, r, "id")
	if !ok {
		return
	}
	linkID, ok := parseIDParam(w, r, "linkID")
	if !ok {
		return
	}
	deleted, err := server.uploadService.DeleteRevokedSubmissionLink(r.Context(), eventID, linkID)
	if err != nil {
		writeError(w, stdhttp.StatusInternalServerError, "failed to delete submission link")
		return
	}
	if !deleted {
		writeError(w, stdhttp.StatusBadRequest, "submission link must be revoked before deletion")
		return
	}
	writeJSON(w, stdhttp.StatusOK, map[string]any{"message": "submission link deleted"})
}

func (server *Server) resolveSubmissionToken(w stdhttp.ResponseWriter, r *stdhttp.Request) {
	eventID, ok := parseIDParam(w, r, "id")
	if !ok {
		return
	}
	token := r.URL.Query().Get("token")
	link, valid, err := server.uploadService.ResolveSubmissionToken(r.Context(), eventID, token)
	if err != nil {
		writeError(w, stdhttp.StatusInternalServerError, "failed to resolve submission token")
		return
	}
	writeJSON(w, stdhttp.StatusOK, map[string]any{"valid": valid, "link": link})
}

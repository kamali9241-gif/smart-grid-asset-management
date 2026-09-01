package api

import (
	"errors"
	"net/http"
	"strconv"
	"strings"

	"github.com/go-chi/chi/v5"

	"github.com/sivakumarkam/smart-grid/backend/internal/domain"
	"github.com/sivakumarkam/smart-grid/backend/internal/importer"
	"github.com/sivakumarkam/smart-grid/backend/internal/repository"
)

type assetDetail struct {
	Asset            domain.Asset            `json:"asset"`
	Ancestors        []domain.Asset          `json:"ancestors"`
	ChildrenByType   []domain.ChildGroup     `json:"childrenByType"`
	ChildCount       int                     `json:"childCount"`
	DescendantCounts domain.DescendantCounts `json:"descendantCounts"`
}

func (s *Server) handleHealth(w http.ResponseWriter, r *http.Request) {
	if err := s.repo.Pool().Ping(r.Context()); err != nil {
		writeError(w, http.StatusServiceUnavailable, "database_unavailable", err.Error())
		return
	}
	count, _ := s.repo.CountAssets(r.Context())
	writeJSON(w, http.StatusOK, map[string]any{"status": "ok", "assetCount": count})
}

func (s *Server) handleMeta(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, map[string]any{
		"assetTypes":          domain.AssetTypes(),
		"operationalStatuses": domain.Statuses(),
		"importModes":         []string{string(importer.ModeAllOrNothing), string(importer.ModePartial)},
	})
}

func (s *Server) handleImport(w http.ResponseWriter, r *http.Request) {
	if err := r.ParseMultipartForm(s.maxUploadBytes); err != nil {
		writeError(w, http.StatusBadRequest, "invalid_upload",
			"expected a multipart/form-data upload with a 'file' field: "+err.Error())
		return
	}
	file, header, err := r.FormFile("file")
	if err != nil {
		writeError(w, http.StatusBadRequest, "missing_file", "no CSV file was supplied in the 'file' field")
		return
	}
	defer file.Close()

	if header.Size > s.maxUploadBytes {
		writeError(w, http.StatusRequestEntityTooLarge, "file_too_large",
			"the uploaded file exceeds the configured limit")
		return
	}
	if name := strings.ToLower(header.Filename); name != "" && !strings.HasSuffix(name, ".csv") {
		writeError(w, http.StatusUnsupportedMediaType, "unsupported_file_type", "only .csv files are accepted")
		return
	}

	mode, err := importer.ParseMode(r.FormValue("mode"))
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid_mode", err.Error())
		return
	}

	report, err := s.imports.Import(r.Context(), header.Filename, file, mode)
	if err != nil {
		var pe *importer.ParseError
		if errors.As(err, &pe) {
			writeError(w, http.StatusBadRequest, "invalid_csv", pe.Message)
			return
		}
		s.logger.Error("import failed", "error", err, "filename", header.Filename)
		writeError(w, http.StatusInternalServerError, "import_failed", "the import could not be completed")
		return
	}

	status := http.StatusCreated
	if !report.Committed {
		status = http.StatusUnprocessableEntity
	}
	writeJSON(w, status, report)
}

func (s *Server) handleRoots(w http.ResponseWriter, r *http.Request) {
	roots, err := s.repo.Roots(r.Context())
	if err != nil {
		s.serverError(w, "could not load root assets", err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"assets": roots})
}

func (s *Server) handleSearch(w http.ResponseWriter, r *http.Request) {
	q := strings.TrimSpace(r.URL.Query().Get("q"))
	if q == "" {
		writeError(w, http.StatusBadRequest, "missing_query", "query parameter 'q' is required")
		return
	}
	assetType := strings.ToUpper(strings.TrimSpace(r.URL.Query().Get("type")))
	if assetType != "" && !domain.IsValidAssetType(assetType) {
		writeError(w, http.StatusBadRequest, "invalid_type",
			"unsupported asset type filter: "+assetType)
		return
	}
	limit, _ := strconv.Atoi(r.URL.Query().Get("limit"))

	results, err := s.repo.Search(r.Context(), q, assetType, limit)
	if err != nil {
		s.serverError(w, "search failed", err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"query": q, "count": len(results), "assets": results})
}

func (s *Server) handleAsset(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "assetId")
	ctx := r.Context()

	asset, err := s.repo.GetByID(ctx, id)
	if err != nil {
		s.notFoundOrServerError(w, id, err)
		return
	}
	ancestors, err := s.repo.Ancestors(ctx, id)
	if err != nil {
		s.serverError(w, "could not load ancestors", err)
		return
	}
	children, err := s.repo.Children(ctx, id)
	if err != nil {
		s.serverError(w, "could not load children", err)
		return
	}
	counts, err := s.repo.DescendantCounts(ctx, id)
	if err != nil {
		s.serverError(w, "could not load descendant counts", err)
		return
	}

	writeJSON(w, http.StatusOK, assetDetail{
		Asset:            asset,
		Ancestors:        ancestors,
		ChildrenByType:   groupByType(children),
		ChildCount:       len(children),
		DescendantCounts: counts,
	})
}

func (s *Server) handleChildren(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "assetId")
	if ok, err := s.repo.Exists(r.Context(), id); err != nil {
		s.serverError(w, "could not load children", err)
		return
	} else if !ok {
		writeError(w, http.StatusNotFound, "asset_not_found", "no asset with id "+id)
		return
	}
	children, err := s.repo.Children(r.Context(), id)
	if err != nil {
		s.serverError(w, "could not load children", err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"assets": children})
}

func (s *Server) handleAncestors(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "assetId")
	if ok, err := s.repo.Exists(r.Context(), id); err != nil {
		s.serverError(w, "could not load ancestors", err)
		return
	} else if !ok {
		writeError(w, http.StatusNotFound, "asset_not_found", "no asset with id "+id)
		return
	}
	ancestors, err := s.repo.Ancestors(r.Context(), id)
	if err != nil {
		s.serverError(w, "could not load ancestors", err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"assets": ancestors})
}

// groupByType buckets immediate children so the UI can render "grouped by type"
// without doing the work itself.
func groupByType(children []domain.Asset) []domain.ChildGroup {
	order := domain.AssetTypes()
	byType := map[string][]domain.Asset{}
	for _, c := range children {
		byType[c.AssetType] = append(byType[c.AssetType], c)
	}
	groups := []domain.ChildGroup{}
	for _, t := range order {
		if list, ok := byType[t]; ok {
			groups = append(groups, domain.ChildGroup{AssetType: t, Count: len(list), Assets: list})
		}
	}
	return groups
}

func (s *Server) serverError(w http.ResponseWriter, msg string, err error) {
	s.logger.Error(msg, "error", err)
	writeError(w, http.StatusInternalServerError, "internal_error", msg)
}

func (s *Server) notFoundOrServerError(w http.ResponseWriter, id string, err error) {
	if errors.Is(err, repository.ErrNotFound) {
		writeError(w, http.StatusNotFound, "asset_not_found", "no asset with id "+id)
		return
	}
	s.serverError(w, "could not load asset", err)
}

package service

import (
	"context"
	"io"
	"log/slog"
	"time"

	"github.com/sivakumarkam/smart-grid/backend/internal/importer"
	"github.com/sivakumarkam/smart-grid/backend/internal/repository"
)

// ImportReport is the API-facing summary of one upload.
type ImportReport struct {
	ImportID     int64                `json:"importId"`
	Filename     string               `json:"filename"`
	Mode         string               `json:"mode"`
	TotalRows    int                  `json:"totalRows"`
	ImportedRows int                  `json:"importedRows"`
	RejectedRows int                  `json:"rejectedRows"`
	Committed    bool                 `json:"committed"`
	Message      string               `json:"message"`
	Rejections   []importer.Rejection `json:"rejections"`
}

type ImportService struct {
	repo   *repository.Repository
	logger *slog.Logger
}

func NewImportService(repo *repository.Repository, logger *slog.Logger) *ImportService {
	return &ImportService{repo: repo, logger: logger}
}

// Import parses, validates and (when the mode allows) persists a CSV upload.
//
// Transactional contract:
//   - all_or_nothing (default): a single rejected row means nothing is written.
//   - partial: the rows that validate are written in one transaction; rows that
//     depend on a rejected ancestor are rejected too, so the stored tree is
//     always complete from the substation root down.
func (s *ImportService) Import(ctx context.Context, filename string, body io.Reader, mode importer.Mode) (*ImportReport, error) {
	started := time.Now()

	rows, err := importer.Parse(body)
	if err != nil {
		return nil, err
	}

	existing, err := s.repo.ExistingByIDs(ctx, importer.ReferencedIDs(rows))
	if err != nil {
		return nil, err
	}

	result := importer.Validate(rows, existing, mode)

	report := &ImportReport{
		Filename:     filename,
		Mode:         string(mode),
		TotalRows:    result.TotalRows,
		RejectedRows: len(result.Rejections),
		Rejections:   result.Rejections,
	}
	if report.Rejections == nil {
		report.Rejections = []importer.Rejection{}
	}

	shouldCommit := len(result.Accepted) > 0 &&
		(mode == importer.ModePartial || len(result.Rejections) == 0)

	if shouldCommit {
		if err := s.repo.InsertAssets(ctx, result.Accepted); err != nil {
			return nil, err
		}
		report.ImportedRows = len(result.Accepted)
		report.Committed = true
	}

	switch {
	case report.Committed && report.RejectedRows == 0:
		report.Message = "All rows were valid and committed."
	case report.Committed:
		report.Message = "Valid rows were committed; rejected rows were not written."
	case mode == importer.ModeAllOrNothing && report.RejectedRows > 0:
		report.Message = "All-or-nothing import: the file contained rejections, so the transaction was rolled back and nothing was written."
	default:
		report.Message = "No rows were written."
	}

	importID, err := s.repo.RecordImport(ctx, filename, string(mode),
		report.TotalRows, report.ImportedRows, report.RejectedRows, report.Committed, result.Rejections)
	if err != nil {
		// The audit trail is best-effort; the data outcome is already decided.
		s.logger.Error("could not record import audit row", "error", err)
	}
	report.ImportID = importID

	s.logger.Info("import processed",
		"filename", filename,
		"mode", mode,
		"total", report.TotalRows,
		"imported", report.ImportedRows,
		"rejected", report.RejectedRows,
		"committed", report.Committed,
		"duration_ms", time.Since(started).Milliseconds())

	return report, nil
}

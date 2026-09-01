import { useRef, useState } from 'react'
import { useNavigate } from 'react-router-dom'
import { ApiError, uploadImport } from '../api/client'
import type { ImportMode, ImportReport } from '../types'

export function ImportPage() {
  const [file, setFile] = useState<File | null>(null)
  const [mode, setMode] = useState<ImportMode>('all_or_nothing')
  const [submitting, setSubmitting] = useState(false)
  const [report, setReport] = useState<ImportReport | null>(null)
  const [error, setError] = useState<string | null>(null)
  const fileInputRef = useRef<HTMLInputElement>(null)
  const navigate = useNavigate()

  async function handleSubmit(e: React.FormEvent) {
    e.preventDefault()
    if (!file) {
      setError('Choose a CSV file first.')
      return
    }
    setSubmitting(true)
    setError(null)
    setReport(null)
    try {
      const result = await uploadImport(file, mode)
      setReport(result)
    } catch (err) {
      setError(err instanceof ApiError ? err.message : 'The upload failed unexpectedly.')
    } finally {
      setSubmitting(false)
    }
  }

  function resetForm() {
    setFile(null)
    setReport(null)
    setError(null)
    if (fileInputRef.current) fileInputRef.current.value = ''
  }

  return (
    <div className="import-page">
      <h1>Import asset data</h1>
      <p className="import-intro">
        Upload the CSV extract of substations and equipment. The file is validated before anything is written to the
        database.
      </p>

      <form onSubmit={handleSubmit} className="import-form">
        <label className="import-file-label">
          <span>CSV file</span>
          <input
            ref={fileInputRef}
            type="file"
            accept=".csv,text/csv"
            onChange={(e) => setFile(e.target.files?.[0] ?? null)}
          />
        </label>

        <fieldset className="import-mode">
          <legend>Import mode</legend>
          <label>
            <input
              type="radio"
              name="mode"
              value="all_or_nothing"
              checked={mode === 'all_or_nothing'}
              onChange={() => setMode('all_or_nothing')}
            />
            All-or-nothing — commit only if every row is valid
          </label>
          <label>
            <input
              type="radio"
              name="mode"
              value="partial"
              checked={mode === 'partial'}
              onChange={() => setMode('partial')}
            />
            Partial — commit the valid rows, report the rest
          </label>
        </fieldset>

        <div className="import-actions">
          <button type="submit" disabled={submitting || !file}>
            {submitting ? 'Uploading…' : 'Upload and validate'}
          </button>
          <button type="button" onClick={resetForm} disabled={submitting}>
            Reset
          </button>
        </div>
      </form>

      {error && (
        <div className="state state-error" role="alert">
          {error}
        </div>
      )}

      {report && <ImportResultsView report={report} onExplore={() => navigate('/explorer')} />}
    </div>
  )
}

function ImportResultsView({ report, onExplore }: { report: ImportReport; onExplore: () => void }) {
  return (
    <section className="import-results" aria-label="Import results">
      <div className={`import-outcome ${report.committed ? 'outcome-committed' : 'outcome-rolled-back'}`}>
        <strong>{report.committed ? 'Data committed' : 'Nothing was written'}</strong>
        <p>{report.message}</p>
      </div>

      <dl className="import-summary">
        <div>
          <dt>Total rows</dt>
          <dd>{report.totalRows}</dd>
        </div>
        <div>
          <dt>Imported</dt>
          <dd>{report.importedRows}</dd>
        </div>
        <div>
          <dt>Rejected</dt>
          <dd>{report.rejectedRows}</dd>
        </div>
      </dl>

      {report.committed && (
        <button type="button" onClick={onExplore} className="link-button">
          Go to the asset explorer →
        </button>
      )}

      {report.rejections.length > 0 && (
        <table className="rejections-table">
          <caption>Rejected rows</caption>
          <thead>
            <tr>
              <th scope="col">Row</th>
              <th scope="col">Asset ID</th>
              <th scope="col">Field</th>
              <th scope="col">Reason</th>
              <th scope="col">Original CSV row</th>
            </tr>
          </thead>
          <tbody>
            {report.rejections.map((r, i) => (
              <tr key={`${r.rowNumber}-${i}`}>
                <td>{r.rowNumber}</td>
                <td>{r.assetId || '—'}</td>
                <td>{r.field || '—'}</td>
                <td>{r.message}</td>
                <td>
                  <code className="raw-row">{r.rawRow || '—'}</code>
                </td>
              </tr>
            ))}
          </tbody>
        </table>
      )}
    </section>
  )
}

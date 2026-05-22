package enc_charts_parser

import (
	"time"

	"github.com/google/uuid"
)

// CellAudit records a historical snapshot of a Cell row at the time it was updated or deleted.
type CellAudit struct {
	AuditID   uuid.UUID `db:"audit_id"  json:"audit_id"`
	Operation string    `db:"operation" json:"operation"` // UPDATE or DELETE
	AuditedAt time.Time `db:"audited_at" json:"audited_at"`
	Cell
}

// FeatureAudit records a historical snapshot of a Feature row at the time it was updated or deleted.
type FeatureAudit struct {
	AuditID   uuid.UUID `db:"audit_id"  json:"audit_id"`
	Operation string    `db:"operation" json:"operation"` // UPDATE or DELETE
	AuditedAt time.Time `db:"audited_at" json:"audited_at"`
	Feature
}

// Cell represents a parsed S-57 ENC cell with metadata extracted from the DSID and DSPM layers.
type Cell struct {
	ID               uuid.UUID  `db:"id" json:"id"`
	CellName         string     `db:"cell_name" json:"cell_name"`
	FileID           *uuid.UUID `db:"file_id" json:"file_id,omitempty"`
	Bbox             *string    `db:"bbox" json:"bbox,omitempty"` // EWKT — POLYGON or MULTIPOLYGON (SRID=4326)
	Lat              *float64   `db:"lat" json:"lat,omitempty"`
	Lon              *float64   `db:"lon" json:"lon,omitempty"`
	Edition          *int       `db:"edition" json:"edition,omitempty"`
	UpdateNumber     int        `db:"update_number" json:"update_number"`
	UpdateDate       *time.Time `db:"update_date" json:"update_date,omitempty"`
	DatasetName      *string    `db:"dataset_name" json:"dataset_name,omitempty"`
	IntendedUsage    *int       `db:"intended_usage" json:"intended_usage,omitempty"`
	IssueDate        *time.Time `db:"issue_date" json:"issue_date,omitempty"`
	ProducingAgency  *int       `db:"producing_agency" json:"producing_agency,omitempty"`
	Comment          *string    `db:"comment" json:"comment,omitempty"`
	CompilationScale *int       `db:"compilation_scale" json:"compilation_scale,omitempty"`
	HorizontalDatum  *int       `db:"horizontal_datum" json:"horizontal_datum,omitempty"`
	VerticalDatum    *int       `db:"vertical_datum" json:"vertical_datum,omitempty"`
	SoundingDatum    *int       `db:"sounding_datum" json:"sounding_datum,omitempty"`
	DepthUnits       *int       `db:"depth_units" json:"depth_units,omitempty"`
	HeightUnits      *int       `db:"height_units" json:"height_units,omitempty"`
	Status           string     `db:"status" json:"status"`
	IsCanonical      bool       `db:"is_canonical" json:"is_canonical"`
	CreatedAt        time.Time  `db:"created_at" json:"created_at"`
	UpdatedAt        time.Time  `db:"updated_at" json:"updated_at"`
}

// Feature represents a single S-57 geographic or cartographic object extracted from an ENC cell layer.
type Feature struct {
	ID          uuid.UUID `db:"id" json:"id"`
	CellID      uuid.UUID `db:"cell_id" json:"cell_id,omitempty"`
	S57Code     string    `db:"s_57_code" json:"s_57_code"`
	Lat         *float64  `db:"lat" json:"lat,omitempty"`
	Lon         *float64  `db:"lon" json:"lon,omitempty"`
	Depth       *float64  `db:"depth" json:"depth,omitempty"` // sounding depth (SOUNDG Z coordinate)
	Geom        *string   `db:"geom" json:"geom,omitempty"`   // EWKT
	Attrs       *string   `db:"attrs" json:"attrs,omitempty"` // JSON text
	IsCanonical bool      `db:"is_canonical" json:"is_canonical"`
	CreatedAt   time.Time `db:"created_at" json:"created_at"`
}

// File represents an ENC file record, including its raw data and processing status.
type File struct {
	ID           uuid.UUID `db:"id" json:"id"`
	Name         string    `db:"name" json:"name"`
	Ext          string    `db:"ext" json:"ext"`
	Data         []byte    `db:"data" json:"data,omitempty"`
	Hash         string    `db:"hash" json:"hash,omitempty"`
	SizeBytes    int64     `db:"size_bytes" json:"size_bytes,omitempty"`
	Status       string    `db:"status" json:"status"`
	ErrorMessage *string   `db:"error_message" json:"error_message,omitempty"`
	CreatedAt    time.Time `db:"created_at" json:"created_at"`
	UpdatedAt    time.Time `db:"updated_at" json:"updated_at"`
}

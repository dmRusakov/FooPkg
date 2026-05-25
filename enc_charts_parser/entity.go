package enc_charts_parser

import (
	"time"

	"github.com/google/uuid"
)

// CellAudit records a historical snapshot of a Cell row at the time it was updated or deleted.
type CellAudit struct {
	AuditID   uuid.UUID `db:"audit_id"  json:"audit_id"  pk:"true"                                  `
	Operation string    `db:"operation" json:"operation" pg:"VARCHAR(10)"                           `
	AuditedAt time.Time `db:"audited_at" json:"audited_at" default:"CURRENT_TIMESTAMP"              `
	Cell
}

// FeatureAudit records a historical snapshot of a Feature row at the time it was updated or deleted.
type FeatureAudit struct {
	AuditID   uuid.UUID `db:"audit_id"  json:"audit_id"  pk:"true"                                  `
	Operation string    `db:"operation" json:"operation" pg:"VARCHAR(10)"                           `
	AuditedAt time.Time `db:"audited_at" json:"audited_at" default:"CURRENT_TIMESTAMP"              `
	Feature
}

// Cell represents a parsed S-57 ENC cell with metadata extracted from the DSID and DSPM layers.
type Cell struct {
	ID               uuid.UUID  `db:"id"               json:"id"                                                                            `
	CellName         string     `db:"cell_name"        json:"cell_name"        index:"unique"                                               `
	FileID           *uuid.UUID `db:"file_id"          json:"file_id,omitempty" index:"true"                                                `
	Bbox             *string    `db:"bbox"             json:"bbox,omitempty"    pg:"GEOMETRY"   index:"gist"                                `
	Lat              *float64   `db:"lat"              json:"lat,omitempty"     pg:"NUMERIC(11,7)" index:"true"                             `
	Lon              *float64   `db:"lon"              json:"lon,omitempty"     pg:"NUMERIC(11,7)" index:"true"                             `
	Edition          *int       `db:"edition"          json:"edition,omitempty"                                                             `
	UpdateNumber     int        `db:"update_number"    json:"update_number"     default:"0"                                                 `
	UpdateDate       *time.Time `db:"update_date"      json:"update_date,omitempty" pg:"DATE"                                               `
	DatasetName      *string    `db:"dataset_name"     json:"dataset_name,omitempty"                                                        `
	IntendedUsage    *int       `db:"intended_usage"   json:"intended_usage,omitempty" index:"true"                                         `
	IssueDate        *time.Time `db:"issue_date"       json:"issue_date,omitempty" pg:"DATE"                                                `
	ProducingAgency  *int       `db:"producing_agency" json:"producing_agency,omitempty"                                                    `
	Comment          *string    `db:"comment"          json:"comment,omitempty"                                                             `
	CompilationScale *int       `db:"compilation_scale" json:"compilation_scale,omitempty" index:"true"                                     `
	HorizontalDatum  *int       `db:"horizontal_datum" json:"horizontal_datum,omitempty"                                                    `
	VerticalDatum    *int       `db:"vertical_datum"   json:"vertical_datum,omitempty"                                                      `
	SoundingDatum    *int       `db:"sounding_datum"   json:"sounding_datum,omitempty"                                                      `
	DepthUnits       *int       `db:"depth_units"      json:"depth_units,omitempty"                                                         `
	HeightUnits      *int       `db:"height_units"     json:"height_units,omitempty"                                                        `
	Status           string     `db:"status"           json:"status"           pg:"VARCHAR(1)" default:"'N'" index:"true"                   `
	IsCanonical      bool       `db:"is_canonical"     json:"is_canonical"      default:"TRUE"                                              `
	CreatedAt        time.Time  `db:"created_at"       json:"created_at"                                                                    `
	UpdatedAt        time.Time  `db:"updated_at"       json:"updated_at"                                                                    `
}

// Feature represents a single S-57 geographic or cartographic object extracted from an ENC cell layer.
type Feature struct {
	ID          uuid.UUID `db:"id"           json:"id"                                                                                 `
	CellID      uuid.UUID `db:"cell_id"      json:"cell_id,omitempty"  index:"true"  fk:"cells.id:cascade"                             `
	S57Code     string    `db:"s_57_code"    json:"s_57_code"          pg:"VARCHAR(10)" index:"true"                                   `
	Lat         *float64  `db:"lat"          json:"lat,omitempty"      pg:"NUMERIC(11,7)" index:"true"                                 `
	Lon         *float64  `db:"lon"          json:"lon,omitempty"      pg:"NUMERIC(11,7)" index:"true"                                 `
	Depth       *float64  `db:"depth"        json:"depth,omitempty"    pg:"NUMERIC(8,2)"                                               `
	Geom        *string   `db:"geom"         json:"geom,omitempty"     pg:"GEOMETRY"   index:"gist"                                    `
	Attrs       *string   `db:"attrs"        json:"attrs,omitempty"    pg:"JSONB"                                                      `
	IsCanonical bool      `db:"is_canonical" json:"is_canonical"       default:"TRUE"                                                  `
	CreatedAt   time.Time `db:"created_at"   json:"created_at"                                                                         `
}

// File represents an ENC file record, including its raw data and processing status.
type File struct {
	ID           uuid.UUID `db:"id"            json:"id"                                                                              `
	Name         string    `db:"name"          json:"name"            index:"true"                                                    `
	Ext          string    `db:"ext"           json:"ext"                                                                             `
	Data         []byte    `db:"data"          json:"data,omitempty"                                                                  `
	Hash         string    `db:"hash"          json:"hash,omitempty"  index:"true"                                                    `
	SizeBytes    int64     `db:"size_bytes"    json:"size_bytes,omitempty"                                                            `
	Status       string    `db:"status"        json:"status"          pg:"CHAR(1)"   default:"'N'"                                    `
	ErrorMessage *string   `db:"error_message" json:"error_message,omitempty"                                                         `
	CreatedAt    time.Time `db:"created_at"    json:"created_at"                                                                      `
	UpdatedAt    time.Time `db:"updated_at"    json:"updated_at"                                                                      `
}

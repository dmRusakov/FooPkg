package enc_charts_parser

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/airbusgeo/godal"
	"github.com/google/uuid"
)

// godalInitOnce ensures GDALAllRegister is called exactly once.
// GDALAllRegister modifies a global driver list and is not thread-safe;
// calling it concurrently from multiple goroutines corrupts GDAL's state.
var godalInitOnce sync.Once

func ParseFileData(files map[string]*File) (cells []*Cell, features []*Feature, err error) {
	baseFile, ok := files["000"]
	if !ok || len(baseFile.Data) == 0 {
		return nil, nil, fmt.Errorf("[pfd001] base file (.000) not found or empty")
	}
	cellName := baseFile.Name

	tmpDir, osErr := os.MkdirTemp("", "enc_parse_*")
	if osErr != nil {
		return nil, nil, fmt.Errorf("[pfd002] failed to create temp dir: %w", osErr)
	}
	defer os.RemoveAll(tmpDir)

	// write actual file bytes
	for ext, f := range files {
		if len(f.Data) == 0 {
			continue
		}
		fPath := filepath.Join(tmpDir, cellName+"."+ext)
		if writeErr := os.WriteFile(fPath, f.Data, 0644); writeErr != nil {
			return nil, nil, writeErr
		}
	}

	// GDAL's S-57 reader requires a sequentially complete update chain; fill
	// any gaps (e.g. NOAA sometimes withdraws interim updates) with minimal
	// valid ISO 8211 dummy files so all subsequent updates are still applied.
	if gapErr := fillUpdateGaps(tmpDir, cellName, files); gapErr != nil {
		return nil, nil, gapErr
	}

	godalInitOnce.Do(godal.RegisterAll)
	ds, dsErr := godal.Open(filepath.Join(tmpDir, cellName+".000"), godal.VectorOnly())
	if dsErr != nil {
		return nil, nil, dsErr
	}
	defer ds.Close()

	cell := &Cell{
		ID:       uuid.New(),
		CellName: cellName,
		FileID:   &baseFile.ID,
		Status:   "N",
	}

	layerByName := make(map[string]godal.Layer)
	for _, l := range ds.Layers() {
		layerByName[l.Name()] = l
	}

	// DSID — dataset identification
	if dsid, ok := layerByName["DSID"]; ok {
		if feat := dsid.NextFeature(); feat != nil {
			f := feat.Fields()
			cell.DatasetName = s57StrField(f, "DSNM")
			cell.Edition = s57IntField(f, "EDTN")
			cell.UpdateNumber = s57IntFieldVal(f, "UPDN")
			cell.UpdateDate = s57DateField(f, "UADT")
			cell.IssueDate = s57DateField(f, "ISDT")
			cell.IntendedUsage = s57IntField(f, "INTU")
			cell.ProducingAgency = s57IntField(f, "AGEN")
			cell.Comment = s57StrField(f, "COMT")
			feat.Close()
		}
	}

	// DSPM — dataset parameters (scale, datums)
	if dspm, ok := layerByName["DSPM"]; ok {
		if feat := dspm.NextFeature(); feat != nil {
			f := feat.Fields()
			cell.CompilationScale = s57IntField(f, "CSCL")
			cell.HorizontalDatum = s57IntField(f, "HDAT")
			cell.VerticalDatum = s57IntField(f, "VDAT")
			cell.SoundingDatum = s57IntField(f, "SDAT")
			cell.DepthUnits = s57IntField(f, "DUNI")
			cell.HeightUnits = s57IntField(f, "HUNI")
			feat.Close()
		}
	}

	// M_COVR — iterate ALL coverage and exclusion polygons (OpenCPN pattern).
	// CATCOV=1 → positive coverage (can be multiple disjoint polygons).
	// CATCOV=2 → no-coverage / exclusion zones.
	// Both types are stored as features; CATCOV=1 WKTs build the cell bbox.
	if mcovr, ok := layerByName["M_COVR"]; ok {
		var coverageWKTs []string
		for {
			feat := mcovr.NextFeature()
			if feat == nil {
				break
			}
			f := feat.Fields()
			catcov := s57IntFieldVal(f, "CATCOV")
			var wkt string
			if geom := feat.Geometry(); geom != nil {
				wkt, _ = geom.WKT()
			}
			feat.Close()
			if wkt == "" {
				continue
			}
			if catcov == 1 {
				coverageWKTs = append(coverageWKTs, wkt)
			}
			ewkt := "SRID=4326;" + wkt
			catcovJSON := fmt.Sprintf(`{"CATCOV":%d}`, catcov)
			mcovrFeat := &Feature{
				ID:      uuid.New(),
				CellID:  cell.ID,
				S57Code: "M_COVR",
				Geom:    &ewkt,
				Attrs:   &catcovJSON,
			}
			if lat, lon := s57GeomFirstPoint(wkt); lat != nil {
				mcovrFeat.Lat = lat
				mcovrFeat.Lon = lon
			}
			features = append(features, mcovrFeat)
		}
		cell.Bbox = buildMultiBbox(coverageWKTs)
		if len(coverageWKTs) > 0 {
			if lat, lon := s57GeomFirstPoint(coverageWKTs[0]); lat != nil {
				cell.Lat = lat
				cell.Lon = lon
			}
		}
	}

	cells = []*Cell{cell}

	skipLayer := map[string]bool{
		"DSID": true, "DSPM": true, "DSPR": true, "DSSI": true,
		"M_COVR": true, // handled above
	}

	for name, layer := range layerByName {
		if skipLayer[name] {
			continue
		}
		for {
			feat := layer.NextFeature()
			if feat == nil {
				break
			}
			rawFields := feat.Fields()
			var attrsPtr *string
			if len(rawFields) > 0 {
				if b, jsonErr := json.Marshal(s57FieldsToMap(rawFields)); jsonErr == nil {
					s := string(b)
					attrsPtr = &s
				}
			}

			if name == "SOUNDG" {
				// SOUNDG uses MULTIPOINT Z; decompose into one row per sounding
				// point so each depth value can be queried independently.
				features = append(features, decomposeSoundg(feat, cell.ID, attrsPtr)...)
			} else {
				feature := &Feature{
					ID:      uuid.New(),
					CellID:  cell.ID,
					S57Code: name,
					Attrs:   attrsPtr,
				}
				if geom := feat.Geometry(); geom != nil {
					if wkt, wktErr := geom.WKT(); wktErr == nil && wkt != "" {
						ewkt := "SRID=4326;" + wkt
						feature.Geom = &ewkt
						if lat, lon := s57GeomFirstPoint(wkt); lat != nil {
							feature.Lat = lat
							feature.Lon = lon
						}
					}
				}
				features = append(features, feature)
			}
			feat.Close()
		}
	}

	return cells, features, nil
}

// dummyISO8211 is a minimal valid ISO 8211 Data Descriptive Record with no
// field definitions. GDAL's S-57 reader opens it without error but finds no
// update operations to apply — used to fill gaps in the update chain.
var dummyISO8211 = []byte{
	'0', '0', '0', '2', '6', // record length: 26
	'3',      // interchange level
	'L',      // leader identifier
	'E',      // inline code extension indicator
	'1',      // version number
	'0',      // application indicator
	'0', '9', // field control length
	'0', '0', '0', '2', '5', // base address of field area: 25
	'!', '!', '!', // extended character set indicator (S-57 standard)
	'3',  // size of field length
	'4',  // size of field position
	'0',  // reserved
	'4',  // size of field tag
	0x1E, // field terminator — end of empty directory
	0x1D, // record terminator
}

// fillUpdateGaps creates dummy ISO 8211 files for every missing extension in
// the range [001..maxUpdate]. NOAA sometimes withdraws interim updates, leaving
// gaps (e.g. only .017–.019 present). GDAL stops applying updates at the first
// missing file, so we fill holes with empty-but-valid ISO 8211 content.
func fillUpdateGaps(tmpDir, cellName string, files map[string]*File) error {
	maxExt := 0
	for ext := range files {
		n, err := strconv.Atoi(ext)
		if err != nil || n == 0 {
			continue
		}
		if n > maxExt {
			maxExt = n
		}
	}
	if maxExt == 0 {
		return nil
	}
	for i := 1; i <= maxExt; i++ {
		fPath := filepath.Join(tmpDir, fmt.Sprintf("%s.%03d", cellName, i))
		if _, statErr := os.Stat(fPath); statErr == nil {
			continue // already written by the main loop
		}
		if writeErr := os.WriteFile(fPath, dummyISO8211, 0644); writeErr != nil {
			return fmt.Errorf("failed to create dummy .%03d: %w", i, writeErr)
		}
	}
	return nil
}

// buildMultiBbox returns an EWKT bbox from one or more POLYGON WKT strings.
// Multiple coverage polygons (disjoint cell areas) are combined into a
// MULTIPOLYGON, matching how OpenCPN stores multi-area ENCs.
func buildMultiBbox(wkts []string) *string {
	if len(wkts) == 0 {
		return nil
	}
	if len(wkts) == 1 {
		s := "SRID=4326;" + wkts[0]
		return &s
	}
	rings := make([]string, 0, len(wkts))
	for _, wkt := range wkts {
		if after, found := strings.CutPrefix(wkt, "POLYGON "); found {
			rings = append(rings, after)
		} else if after, found := strings.CutPrefix(wkt, "POLYGON("); found {
			rings = append(rings, "("+after)
		}
	}
	var s string
	if len(rings) == len(wkts) {
		s = "SRID=4326;MULTIPOLYGON (" + strings.Join(rings, ", ") + ")"
	} else {
		s = "SRID=4326;" + wkts[0] // fallback: not all were plain POLYGONs
	}
	return &s
}

// soundingPt holds a single decomposed sounding point.
type soundingPt struct {
	lat, lon, depth float64
	hasZ            bool
}

// decomposeSoundg splits a SOUNDG MULTIPOINT Z feature into one Feature row
// per sounding point so each depth value is independently queryable.
func decomposeSoundg(feat *godal.Feature, cellID uuid.UUID, attrs *string) []*Feature {
	geom := feat.Geometry()
	if geom == nil {
		return nil
	}
	wkt, err := geom.WKT()
	if err != nil || wkt == "" {
		return nil
	}
	pts := parseSoundgWKT(wkt)
	out := make([]*Feature, 0, len(pts))
	for _, pt := range pts {
		latF, lonF := pt.lat, pt.lon
		var geomStr string
		if pt.hasZ {
			geomStr = fmt.Sprintf("SRID=4326;POINT Z (%g %g %g)", pt.lon, pt.lat, pt.depth)
		} else {
			geomStr = fmt.Sprintf("SRID=4326;POINT (%g %g)", pt.lon, pt.lat)
		}
		f := &Feature{
			ID:      uuid.New(),
			CellID:  cellID,
			S57Code: "SOUNDG",
			Lat:     &latF,
			Lon:     &lonF,
			Geom:    &geomStr,
			Attrs:   attrs,
		}
		if pt.hasZ {
			depthF := pt.depth
			f.Depth = &depthF
		}
		out = append(out, f)
	}
	return out
}

// parseSoundgWKT extracts individual sounding points from a MULTIPOINT (Z) WKT.
// Handles both the parenthesised form "((lon lat z),...)" and the bare form
// "lon lat z, ...".
func parseSoundgWKT(wkt string) []soundingPt {
	hasZ := strings.Contains(wkt, " Z ") || strings.Contains(wkt, " Z(")

	outer := strings.Index(wkt, "(")
	lastClose := strings.LastIndex(wkt, ")")
	if outer < 0 || lastClose <= outer {
		return nil
	}
	body := strings.TrimSpace(wkt[outer+1 : lastClose])

	var coordSets [][]string
	if strings.HasPrefix(body, "(") {
		// "((lon lat z),(lon lat z),...)" — each point in its own parens
		parenDepth := 0
		var cur strings.Builder
		for _, ch := range body {
			switch ch {
			case '(':
				parenDepth++
			case ')':
				parenDepth--
				if parenDepth == 0 {
					coordSets = append(coordSets, strings.Fields(cur.String()))
					cur.Reset()
				}
			default:
				if parenDepth > 0 {
					cur.WriteRune(ch)
				}
			}
		}
	} else {
		// "lon lat z, lon lat z, ..." — bare coordinates
		for _, part := range strings.Split(body, ",") {
			if fields := strings.Fields(strings.TrimSpace(part)); len(fields) >= 2 {
				coordSets = append(coordSets, fields)
			}
		}
	}

	pts := make([]soundingPt, 0, len(coordSets))
	for _, c := range coordSets {
		if len(c) < 2 {
			continue
		}
		lon, e1 := strconv.ParseFloat(c[0], 64)
		lat, e2 := strconv.ParseFloat(c[1], 64)
		if e1 != nil || e2 != nil {
			continue
		}
		pt := soundingPt{lat: lat, lon: lon, hasZ: hasZ && len(c) >= 3}
		if pt.hasZ {
			if z, e := strconv.ParseFloat(c[2], 64); e == nil {
				pt.depth = z
			}
		}
		pts = append(pts, pt)
	}
	return pts
}

// s57IntField returns a pointer to the field value as int, or nil if unset.
func s57IntField(f map[string]godal.Field, key string) *int {
	fld, ok := f[key]
	if !ok || !fld.IsSet() {
		return nil
	}
	v := int(fld.Int())
	return &v
}

// s57IntFieldVal returns the field value as int, or 0 if unset.
func s57IntFieldVal(f map[string]godal.Field, key string) int {
	fld, ok := f[key]
	if !ok || !fld.IsSet() {
		return 0
	}
	return int(fld.Int())
}

// s57StrField returns a pointer to the field value as string, or nil if empty/unset.
func s57StrField(f map[string]godal.Field, key string) *string {
	fld, ok := f[key]
	if !ok || !fld.IsSet() {
		return nil
	}
	s := fld.String()
	if s == "" {
		return nil
	}
	return &s
}

// s57DateField parses an S-57 date field (FTString YYYYMMDD or FTDate).
func s57DateField(f map[string]godal.Field, key string) *time.Time {
	fld, ok := f[key]
	if !ok || !fld.IsSet() {
		return nil
	}
	if fld.Type() == godal.FTDate || fld.Type() == godal.FTDateTime {
		return fld.DateTime()
	}
	s := fld.String()
	if s == "" || s == "00000000" {
		return nil
	}
	t, err := time.Parse("20060102", s)
	if err != nil {
		return nil
	}
	return &t
}

// s57FieldsToMap converts a godal field map to a JSON-serialisable map.
func s57FieldsToMap(fields map[string]godal.Field) map[string]any {
	m := make(map[string]any, len(fields))
	for k, v := range fields {
		if !v.IsSet() {
			continue
		}
		switch v.Type() {
		case godal.FTInt, godal.FTInt64:
			m[k] = v.Int()
		case godal.FTReal:
			m[k] = v.Float()
		case godal.FTDate, godal.FTTime, godal.FTDateTime:
			m[k] = v.DateTime()
		case godal.FTBinary:
			m[k] = v.Bytes()
		case godal.FTIntList, godal.FTInt64List:
			m[k] = v.IntList()
		case godal.FTRealList:
			m[k] = v.FloatList()
		case godal.FTStringList:
			m[k] = v.StringList()
		default:
			m[k] = v.String()
		}
	}
	return m
}

// s57GeomFirstPoint extracts (lat, lon) from the first coordinate of any WKT geometry
// (POINT, LINESTRING, POLYGON, MULTIPOINT, MULTILINESTRING, MULTIPOLYGON, etc.).
func s57GeomFirstPoint(wkt string) (lat, lon *float64) {
	i := strings.Index(wkt, "(")
	if i < 0 {
		return nil, nil
	}
	// Advance past all opening '(' and spaces to reach the first coordinate token.
	for i < len(wkt) && (wkt[i] == '(' || wkt[i] == ' ') {
		i++
	}
	parts := strings.Fields(wkt[i:])
	if len(parts) < 2 {
		return nil, nil
	}
	latStr := parts[1]
	if idx := strings.IndexAny(latStr, ",)"); idx >= 0 {
		latStr = latStr[:idx]
	}
	lonVal, err1 := strconv.ParseFloat(parts[0], 64)
	latVal, err2 := strconv.ParseFloat(latStr, 64)
	if err1 != nil || err2 != nil {
		return nil, nil
	}
	return &latVal, &lonVal
}

package services

import (
	"encoding/json"
	"fmt"
	"time"
)

// DocumentDate accepts a calendar date at midnight in Peru or an RFC3339 instant.
// Values are always normalized to UTC, independently of the host timezone.
type DocumentDate struct{ time.Time }

func (d *DocumentDate) UnmarshalJSON(data []byte) error {
	var value string
	if err := json.Unmarshal(data, &value); err != nil {
		return err
	}
	var parsed time.Time
	var err error
	if len(value) == len("2006-01-02") {
		parsed, err = time.ParseInLocation("2006-01-02", value, time.FixedZone("America/Lima", -5*60*60))
	} else {
		parsed, err = time.Parse(time.RFC3339Nano, value)
	}
	if err != nil {
		return fmt.Errorf("document_date must use YYYY-MM-DD or RFC3339 with timezone")
	}
	d.Time = parsed.UTC()
	return nil
}

func (d DocumentDate) MarshalJSON() ([]byte, error) { return json.Marshal(d.UTC()) }

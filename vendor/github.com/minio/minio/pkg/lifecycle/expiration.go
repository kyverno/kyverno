/*
 * MinIO Cloud Storage, (C) 2019 MinIO, Inc.
 *
 * Licensed under the Apache License, Version 2.0 (the "License");
 * you may not use this file except in compliance with the License.
 * You may obtain a copy of the License at
 *
 *     http://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing, software
 * distributed under the License is distributed on an "AS IS" BASIS,
 * WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
 * See the License for the specific language governing permissions and
 * limitations under the License.
 */

package lifecycle

import (
	"encoding/xml"
	"errors"
	"time"
)

var (
	errLifecycleInvalidDate       = errors.New("Date must be provided in ISO 8601 format")
	errLifecycleInvalidDays       = errors.New("Days must be positive integer when used with Expiration")
	errLifecycleInvalidExpiration = errors.New("At least one of Days or Date should be present inside Expiration")
	errLifecycleDateNotMidnight   = errors.New(" 'Date' must be at midnight GMT")
)

// ExpirationDays is a type alias to unmarshal Days in Expiration
type ExpirationDays int

// UnmarshalXML parses number of days from Expiration and validates if
// greater than zero
func (eDays *ExpirationDays) UnmarshalXML(d *xml.Decoder, startElement xml.StartElement) error {
	var numDays int
	err := d.DecodeElement(&numDays, &startElement)
	if err != nil {
		return err
	}
	if numDays <= 0 {
		return errLifecycleInvalidDays
	}
	*eDays = ExpirationDays(numDays)
	return nil
}

// MarshalXML encodes number of days to expire if it is non-zero and
// encodes empty string otherwise
func (eDays *ExpirationDays) MarshalXML(e *xml.Encoder, startElement xml.StartElement) error {
	if *eDays == ExpirationDays(0) {
		return nil
	}
	return e.EncodeElement(int(*eDays), startElement)
}

// ExpirationDate is a embedded type containing time.Time to unmarshal
// Date in Expiration
type ExpirationDate struct {
	time.Time
}

// UnmarshalXML parses date from Expiration and validates date format
func (eDate *ExpirationDate) UnmarshalXML(d *xml.Decoder, startElement xml.StartElement) error {
	var dateStr string
	err := d.DecodeElement(&dateStr, &startElement)
	if err != nil {
		return err
	}
	// While AWS documentation mentions that the date specified
	// must be present in ISO 8601 format, in reality they allow
	// users to provide RFC 3339 compliant dates.
	expDate, err := time.Parse(time.RFC3339, dateStr)
	if err != nil {
		return errLifecycleInvalidDate
	}
	// Allow only date timestamp specifying midnight GMT
	hr, min, sec := expDate.Clock()
	nsec := expDate.Nanosecond()
	loc := expDate.Location()
	if !(hr == 0 && min == 0 && sec == 0 && nsec == 0 && loc.String() == time.UTC.String()) {
		return errLifecycleDateNotMidnight
	}

	*eDate = ExpirationDate{expDate}
	return nil
}

// MarshalXML encodes expiration date if it is non-zero and encodes
// empty string otherwise
func (eDate *ExpirationDate) MarshalXML(e *xml.Encoder, startElement xml.StartElement) error {
	if *eDate == (ExpirationDate{time.Time{}}) {
		return nil
	}
	return e.EncodeElement(eDate.Format(time.RFC3339), startElement)
}

// Expiration - expiration actions for a rule in lifecycle configuration.
type Expiration struct {
	XMLName xml.Name       `xml:"Expiration"`
	Days    ExpirationDays `xml:"Days,omitempty"`
	Date    ExpirationDate `xml:"Date,omitempty"`
}

// Validate - validates the "Expiration" element
func (e Expiration) Validate() error {
	// Neither expiration days or date is specified
	if e.IsDaysNull() && e.IsDateNull() {
		return errLifecycleInvalidExpiration
	}

	// Both expiration days and date are specified
	if !e.IsDaysNull() && !e.IsDateNull() {
		return errLifecycleInvalidExpiration
	}
	return nil
}

// IsDaysNull returns true if days field is null
func (e Expiration) IsDaysNull() bool {
	return e.Days == ExpirationDays(0)

}

// IsDateNull returns true if date field is null
func (e Expiration) IsDateNull() bool {
	return e.Date == ExpirationDate{time.Time{}}
}

// IsNull returns true if both date and days fields are null
func (e Expiration) IsNull() bool {
	return e.IsDaysNull() && e.IsDateNull()
}

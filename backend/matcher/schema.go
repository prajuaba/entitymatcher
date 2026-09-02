package matcher

import (
	"fmt"
	"math"
	"strings"
)

type SecondaryFieldMapping struct {
	Name        string  `json:"name"`         // e.g. "Tax ID", "Phone Number", "Category"
	FieldSrc    string  `json:"field_src"`    // Source column name
	FieldDest   string  `json:"field_dest"`   // Destination column name
	MatchType   string  `json:"match_type"`   // "EXACT" | "FUZZY" | "NUMERIC_DELTA"
	Weight      float64 `json:"weight"`       // Weight fraction e.g. 0.10
	IsMandatory bool    `json:"is_mandatory"` // If true, mismatch drops overall score
}

type ColumnMapping struct {
	NameFieldsSrc   []string                `json:"name_fields_src"`  // Single or multiple source name columns
	NameFieldsDest  []string                `json:"name_fields_dest"` // Single or multiple dest name columns
	RefIDSrc        string                  `json:"ref_id_src"`       // Source reference ID column
	RefIDDest       string                  `json:"ref_id_dest"`      // Destination customer ID column
	DateFieldSrc    string                  `json:"date_field_src"`   // Source date column
	DateFieldDest   string                  `json:"date_field_dest"`  // Destination date column
	SecondaryFields []SecondaryFieldMapping `json:"secondary_fields"` // Additional pairing columns
}

func DefaultColumnMapping() ColumnMapping {
	return ColumnMapping{
		NameFieldsSrc:   []string{"customer_name", "name", "raw_name", "company_name"},
		NameFieldsDest:  []string{"customer_name", "name", "raw_name", "company_name"},
		RefIDSrc:        "reference_id",
		RefIDDest:       "customer_id",
		DateFieldSrc:    "transaction_date",
		DateFieldDest:   "transaction_date",
		SecondaryFields: []SecondaryFieldMapping{},
	}
}

// ExtractCompositeName combines multiple configured name fields into a single raw entity string.
func ExtractCompositeName(attributes map[string]interface{}, fields []string) string {
	if len(attributes) == 0 {
		return ""
	}

	var parts []string
	for _, f := range fields {
		if val, exists := attributes[f]; exists && val != nil {
			strVal := fmt.Sprintf("%v", val)
			strVal = strings.TrimSpace(strVal)
			if strVal != "" && strVal != "<nil>" {
				parts = append(parts, strVal)
			}
		}
	}

	// Fallback to searching known default keys if no matching fields found
	if len(parts) == 0 {
		defaultKeys := []string{"customer_name", "name", "raw_name", "company_name", "customer_name_raw", "first_name", "last_name"}
		for _, k := range defaultKeys {
			if val, exists := attributes[k]; exists && val != nil {
				strVal := strings.TrimSpace(fmt.Sprintf("%v", val))
				if strVal != "" && strVal != "<nil>" {
					parts = append(parts, strVal)
				}
			}
		}
	}

	return strings.Join(parts, " ")
}

// ExtractFieldValue retrieves string value of attribute key.
func ExtractFieldValue(attributes map[string]interface{}, fieldName string) string {
	if len(attributes) == 0 || fieldName == "" {
		return ""
	}
	if val, exists := attributes[fieldName]; exists && val != nil {
		strVal := strings.TrimSpace(fmt.Sprintf("%v", val))
		if strVal != "<nil>" {
			return strVal
		}
	}
	return ""
}

// EvaluateSecondaryFields computes composite score and reasons for secondary pairing columns.
func EvaluateSecondaryFields(
	srcAttrs, destAttrs map[string]interface{},
	mappings []SecondaryFieldMapping,
) (float64, []string, bool) {
	if len(mappings) == 0 {
		return 1.0, nil, true
	}

	var totalWeightedScore float64
	var totalWeight float64
	var reasons []string
	mandatoryFailed := false

	for _, sec := range mappings {
		srcVal := ExtractFieldValue(srcAttrs, sec.FieldSrc)
		destVal := ExtractFieldValue(destAttrs, sec.FieldDest)

		if srcVal == "" || destVal == "" {
			continue
		}

		weight := sec.Weight
		if weight <= 0 {
			weight = 0.10
		}
		totalWeight += weight

		var fieldScore float64
		switch sec.MatchType {
		case "EXACT":
			if strings.EqualFold(srcVal, destVal) {
				fieldScore = 1.0
				reasons = append(reasons, fmt.Sprintf("Exact match on %s (%s == %s)", sec.Name, srcVal, destVal))
			} else {
				fieldScore = 0.0
				if sec.IsMandatory {
					mandatoryFailed = true
					reasons = append(reasons, fmt.Sprintf("Mandatory mismatch on %s (%s != %s)", sec.Name, srcVal, destVal))
				}
			}
		case "FUZZY":
			fieldScore = JaroWinkler(strings.ToLower(srcVal), strings.ToLower(destVal))
			if fieldScore >= 0.85 {
				reasons = append(reasons, fmt.Sprintf("High similarity on %s (%.0f%%)", sec.Name, fieldScore*100))
			}
		case "NUMERIC_DELTA":
			var numSrc, numDest float64
			_, err1 := fmt.Sscanf(srcVal, "%f", &numSrc)
			_, err2 := fmt.Sscanf(destVal, "%f", &numDest)
			if err1 == nil && err2 == nil {
				diff := math.Abs(numSrc - numDest)
				if diff == 0 {
					fieldScore = 1.0
					reasons = append(reasons, fmt.Sprintf("Exact numeric match on %s", sec.Name))
				} else if diff <= 0.01*math.Max(numSrc, numDest) {
					fieldScore = 0.90
					reasons = append(reasons, fmt.Sprintf("Close numeric value on %s", sec.Name))
				} else {
					fieldScore = 0.0
				}
			}
		default:
			if strings.EqualFold(srcVal, destVal) {
				fieldScore = 1.0
			}
		}

		totalWeightedScore += fieldScore * weight
	}

	if totalWeight == 0 {
		return 1.0, nil, true
	}

	finalScore := totalWeightedScore / totalWeight
	return finalScore, reasons, !mandatoryFailed
}

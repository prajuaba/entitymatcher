package matcher

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"time"
)

type LLMRequestCandidate struct {
	DestinationCustomerID string `json:"destination_customer_id"`
	CustomerName          string `json:"customer_name"`
	TransactionDate       string `json:"transaction_date"`
}

type LLMRequestPayload struct {
	SourceRefID       string                `json:"source_reference_id"`
	SourceCustomerName string               `json:"source_customer_name"`
	SourceTxDate      string                `json:"source_transaction_date"`
	SourceTxType      string                `json:"source_transaction_type"`
	Candidates        []LLMRequestCandidate `json:"candidates"`
}

type LLMMatchDetail struct {
	DestinationCustomerID string   `json:"destination_customer_id"`
	ConfidenceScore       float64  `json:"confidence_score"`
	NameSimilarityScore   float64  `json:"name_similarity_score"`
	DateMatchStatus       string   `json:"date_match_status"` // EXACT | CLOSE | MISMATCH
	MatchedNameType       string   `json:"matched_name_type"` // INDIVIDUAL | CORPORATE
	MatchReasons          []string `json:"match_reasons"`
}

type LLMResponse struct {
	SourceReferenceID string           `json:"source_reference_id"`
	Matches           []LLMMatchDetail `json:"matches"`
}

type LLMResolver struct {
	APIKey  string
	Model   string
	BaseURL string
}

func NewLLMResolver() *LLMResolver {
	apiKey := os.Getenv("GEMINI_API_KEY")
	if apiKey == "" {
		apiKey = os.Getenv("LLM_API_KEY")
	}
	return &LLMResolver{
		APIKey:  apiKey,
		Model:   "gemini-1.5-flash",
		BaseURL: "https://generativelanguage.googleapis.com/v1beta/models",
	}
}

// EvaluateEdgeCases sends source and candidate records to LLM or executes local fallback rule analyzer.
func (r *LLMResolver) EvaluateEdgeCases(ctx context.Context, req LLMRequestPayload) (*LLMResponse, error) {
	if r.APIKey == "" {
		// Fall back to rule-based edge case analyzer with detailed JSON response
		return r.fallbackEvaluate(req), nil
	}

	prompt := buildPrompt(req)

	// Gemini REST API call payload
	geminiReqBody := map[string]interface{}{
		"contents": []map[string]interface{}{
			{
				"parts": []map[string]string{
					{"text": prompt},
				},
			},
		},
		"generationConfig": map[string]interface{}{
			"response_mime_type": "application/json",
			"temperature":        0.1,
		},
	}

	jsonBytes, err := json.Marshal(geminiReqBody)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal Gemini request: %w", err)
	}

	url := fmt.Sprintf("%s/%s:generateContent?key=%s", r.BaseURL, r.Model, r.APIKey)
	httpReq, err := http.NewRequestWithContext(ctx, "POST", url, bytes.NewBuffer(jsonBytes))
	if err != nil {
		return nil, fmt.Errorf("failed to create http request: %w", err)
	}
	httpReq.Header.Set("Content-Type", "application/json")

	client := &http.Client{Timeout: 30 * time.Second}
	resp, err := client.Do(httpReq)
	if err != nil {
		return r.fallbackEvaluate(req), nil
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return r.fallbackEvaluate(req), nil
	}

	bodyBytes, err := io.ReadAll(resp.Body)
	if err != nil {
		return r.fallbackEvaluate(req), nil
	}

	var geminiResp struct {
		Candidates []struct {
			Content struct {
				Parts []struct {
					Text string `json:"text"`
				} `json:"parts"`
			} `json:"content"`
		} `json:"candidates"`
	}

	if err := json.Unmarshal(bodyBytes, &geminiResp); err != nil || len(geminiResp.Candidates) == 0 || len(geminiResp.Candidates[0].Content.Parts) == 0 {
		return r.fallbackEvaluate(req), nil
	}

	rawText := geminiResp.Candidates[0].Content.Parts[0].Text

	var llmResult LLMResponse
	if err := json.Unmarshal([]byte(rawText), &llmResult); err != nil {
		return r.fallbackEvaluate(req), nil
	}

	return &llmResult, nil
}

func buildPrompt(req LLMRequestPayload) string {
	candsJSON, _ := json.MarshalIndent(req.Candidates, "", "  ")

	return fmt.Sprintf(`You are an expert bilingual entity-resolution engine specializing in Thai and English names (individuals and legal entities).

Task:
Compare SOURCE_RECORD against a list of CANDIDATE_RECORDS and calculate a matching confidence score (0.00 to 1.00) for each candidate.

Normalization Rules to Apply:
1. Remove honorifics and titles:
   - Thai individuals: นาย, นาง, นางสาว, ด.ช., ด.หญิง, นพ., พญ., etc.
   - English individuals: Mr., Mrs., Ms., Miss, Dr., Prof., etc.
   - Thai corporate: บริษัท, บจก, บมจ, ห้างหุ้นส่วนจำกัด, หจก, สาขา, จำกัด (มหาชน)
   - English corporate: Co., Ltd., Company Limited, Corp., Inc., LLC, PLC, Branch
2. Handle transliteration phonetics (e.g., "สมชาย" == "Somchai", "เจริญ" == "Charoen").
3. Strip all non-alphanumeric characters, normalize multiple whitespaces, ignore case and Thai tone marks/vowel inconsistencies.
4. Compare secondary attributes: Transaction Date proximity (exact match = boost; ±3 days = minor penalty; >30 days = heavy penalty).

Input Data:
SOURCE_RECORD:
{
  "reference_id": "%s",
  "customer_name": "%s",
  "transaction_date": "%s",
  "transaction_type": "%s"
}

CANDIDATE_RECORDS:
%s

Output Format (Strict JSON only):
{
  "source_reference_id": "%s",
  "matches": [
    {
      "destination_customer_id": "string",
      "confidence_score": 0.85,
      "name_similarity_score": 0.88,
      "date_match_status": "EXACT",
      "matched_name_type": "INDIVIDUAL",
      "match_reasons": ["string"]
    }
  ]
}`, req.SourceRefID, req.SourceCustomerName, req.SourceTxDate, req.SourceTxType, string(candsJSON), req.SourceRefID)
}

func (r *LLMResolver) fallbackEvaluate(req LLMRequestPayload) *LLMResponse {
	srcNorm := Normalize(req.SourceCustomerName)
	srcDate, _ := time.Parse("2006-01-02", req.SourceTxDate)

	matches := make([]LLMMatchDetail, 0, len(req.Candidates))

	for _, cand := range req.Candidates {
		candNorm := Normalize(cand.CustomerName)
		candDate, _ := time.Parse("2006-01-02", cand.TransactionDate)

		scoreRes := CalculateCompositeScoreWithCorpus(
			srcNorm,
			candNorm,
			srcDate,
			candDate,
			DefaultWeights,
			DefaultAlgorithms,
			30,
			nil, // corpus: no IDF weighting in ad-hoc LLM evaluation mode
		)

		dateStatus := "MISMATCH"
		if scoreRes.DateScore == 1.0 {
			dateStatus = "EXACT"
		} else if scoreRes.DateScore >= 0.70 {
			dateStatus = "CLOSE"
		}

		nameType := "INDIVIDUAL"
		if reThaiCorporate.MatchString(req.SourceCustomerName) || reEngCorporate.MatchString(req.SourceCustomerName) ||
			reThaiCorporate.MatchString(cand.CustomerName) || reEngCorporate.MatchString(cand.CustomerName) {
			nameType = "CORPORATE"
		}

		reasons := scoreRes.MatchReasons
		if nameType == "CORPORATE" {
			reasons = append(reasons, "Identified as corporate legal entity name")
		}

		matches = append(matches, LLMMatchDetail{
			DestinationCustomerID: cand.DestinationCustomerID,
			ConfidenceScore:       scoreRes.TotalScore,
			NameSimilarityScore:   scoreRes.NameScore,
			DateMatchStatus:       dateStatus,
			MatchedNameType:       nameType,
			MatchReasons:          reasons,
		})
	}

	return &LLMResponse{
		SourceReferenceID: req.SourceRefID,
		Matches:           matches,
	}
}

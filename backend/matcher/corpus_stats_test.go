package matcher

import (
	"math"
	"testing"
)

func TestBuildCorpusStats_DFCountsTokenOncePerRecord(t *testing.T) {
	// Create records where tokens repeat within the same name
	src1 := SourceRecord{
		NormalizedName: Normalize("สยาม สยาม ธรรมชาติ"),
	}
	src2 := SourceRecord{
		NormalizedName: Normalize("สยาม การค้า"),
	}

	corpus := BuildCorpusStats([]SourceRecord{src1, src2}, nil)
	if corpus == nil {
		t.Fatal("expected non-nil corpus stats")
	}

	// 'สยาม' appears in both records -> df should be 2, not 3 (counted once per record)
	if df := corpus.GetDocumentFrequency("สยาม"); df != 2 {
		t.Errorf("expected df['สยาม'] == 2 (once per record), got %d", df)
	}

	// 'ธรรมชาติ' appears in one record -> df should be 1
	if df := corpus.GetDocumentFrequency("ธรรมชาติ"); df != 1 {
		t.Errorf("expected df['ธรรมชาติ'] == 1, got %d", df)
	}

	// 'การค้า' appears in one record -> df should be 1
	if df := corpus.GetDocumentFrequency("การค้า"); df != 1 {
		t.Errorf("expected df['การค้า'] == 1, got %d", df)
	}
}

func TestCorpusStats_FrequentTokenHasLowWeight(t *testing.T) {
	// Build corpus: 'ขนส่ง' appears in 70% of records, 'สยาม' appears in 2 records
	var sources []SourceRecord
	var dests []DestinationRecord

	// 70 source records contain 'ขนส่ง'
	for i := 0; i < 70; i++ {
		sources = append(sources, SourceRecord{
			ID: "src" + string(rune(i)),
			NormalizedName: Normalize("บริษัทขนส่ง จำกัด"),
		})
	}
	// 30 source records without 'ขนส่ง'
	for i := 70; i < 100; i++ {
		sources = append(sources, SourceRecord{
			ID: "src" + string(rune(i)),
			NormalizedName: Normalize("บริษัท logistics"),
		})
	}

	// 10 dest records contain 'ขนส่ง'
	for i := 0; i < 10; i++ {
		dests = append(dests, DestinationRecord{
			ID: "dst" + string(rune(i)),
			NormalizedName: Normalize("ขนส่ง กรุงเทพ"),
		})
	}
	// 10 dest records without 'ขนส่ง'
	for i := 10; i < 20; i++ {
		dests = append(dests, DestinationRecord{
			ID: "dst" + string(rune(i)),
			NormalizedName: Normalize("logistics thailand"),
		})
	}

	corpus := BuildCorpusStats(sources, dests)
	if corpus == nil {
		t.Fatal("expected non-nil corpus stats")
	}

	// 'ขนส่ง' appears in 70+10=80 out of 120 documents (66.7%) -> should have low weight
	weightFrequent := corpus.Weight("ขนส่ง")
	if weightFrequent >= 0.35 {
		t.Errorf("expected frequent token 'ขนส่ง' (df=80/120) to have weight < 0.35, got %f", weightFrequent)
	}

	// 'สยาม' does not appear in this corpus (unseen) -> should have weight 1.0
	weightUnseen := corpus.Weight("สยาม")
	if weightUnseen != 1.0 {
		t.Errorf("expected unseen token 'สยาม' to have weight 1.0, got %f", weightUnseen)
	}

	// Verify weights are in [0,1]
	if weightFrequent < 0 || weightFrequent > 1 {
		t.Errorf("frequent token weight %f out of [0,1] range", weightFrequent)
	}
	if weightUnseen < 0 || weightUnseen > 1 {
		t.Errorf("unseen token weight %f out of [0,1] range", weightUnseen)
	}
}

func TestDistinctiveTokenScore_FrequentVsRareToken(t *testing.T) {
	// Build corpus: 'ขนส่ง' frequent (70%), 'สยาม' rare (2%)
	var sources []SourceRecord
	var dests []DestinationRecord

	// 70 records with 'ขนส่ง'
	for i := 0; i < 70; i++ {
		sources = append(sources, SourceRecord{
			ID: "src" + string(rune(i)),
			NormalizedName: Normalize("บริษัทขนส่ง"),
		})
		dests = append(dests, DestinationRecord{
			ID: "dst" + string(rune(i)),
			NormalizedName: Normalize("บริษัทขนส่ง"),
		})
	}

	// 2 records with 'สยาม'
	for i := 70; i < 72; i++ {
		sources = append(sources, SourceRecord{
			ID: "src" + string(rune(i)),
			NormalizedName: Normalize("สยาม"),
		})
		dests = append(dests, DestinationRecord{
			ID: "dst" + string(rune(i)),
			NormalizedName: Normalize("สยาม"),
		})
	}

	corpus := BuildCorpusStats(sources, dests)
	if corpus == nil {
		t.Fatal("expected non-nil corpus stats")
	}

	// Key assertion: rare token should have higher weight than frequent token
	weightFrequent := corpus.Weight("ขนส่ง")  // df=70, should be low
	weightRare := corpus.Weight("สยาม")      // df=2, should be high

	if weightRare <= weightFrequent {
		t.Errorf("rare token weight (%f) should be > frequent token weight (%f)", weightRare, weightFrequent)
	}

	// Simulate DistinctiveTokenScore behavior:
	// If two names match ONLY on 'ขนส่ง', the weighted match is low
	// If two names match ONLY on 'สยาม', the weighted match is high
	// This is the core improvement: IDF weighting makes rare matches more valuable
	frequentMatchScore := 1.0 * weightFrequent / 1.0  // 1 match on 'ขนส่ง'
	rareMatchScore := 1.0 * weightRare / 1.0          // 1 match on 'สยาม'

	if rareMatchScore <= frequentMatchScore {
		t.Errorf("match on rare token (%f) should score higher than match on frequent token (%f)",
			rareMatchScore, frequentMatchScore)
	}
}

func TestCorpusStats_NilHandling(t *testing.T) {
	// Call BuildCorpusStats([], []) -> should return nil
	corpus := BuildCorpusStats([]SourceRecord{}, []DestinationRecord{})
	if corpus != nil {
		t.Errorf("expected nil corpus stats for empty input, got non-nil")
	}

	// Call nil.Weight('token') -> should return 1.0 (no crash, fallback works)
	var nilStats *CorpusStats
	weight := nilStats.Weight("test-token")
	if weight != 1.0 {
		t.Errorf("expected nil corpus.Weight() == 1.0, got %f", weight)
	}

	// Call nil.GetDocumentCount() -> should return 0
	if count := nilStats.GetDocumentCount(); count != 0 {
		t.Errorf("expected nil corpus.GetDocumentCount() == 0, got %d", count)
	}

	// Call nil.GetDocumentFrequency() -> should return 0
	if df := nilStats.GetDocumentFrequency("token"); df != 0 {
		t.Errorf("expected nil corpus.GetDocumentFrequency() == 0, got %d", df)
	}
}

func TestCorpusStats_UnseenTokenWeight(t *testing.T) {
	// Build corpus with known tokens
	src := SourceRecord{
		NormalizedName: Normalize("บริษัท สยาม"),
	}
	corpus := BuildCorpusStats([]SourceRecord{src}, []DestinationRecord{})

	if corpus == nil {
		t.Fatal("expected non-nil corpus stats")
	}

	// Unseen token gets weight 1.0 (maximally distinctive)
	unseenWeight := corpus.Weight("ไม่เคยเห็น")
	if unseenWeight != 1.0 {
		t.Errorf("expected unseen token weight = 1.0, got %f", unseenWeight)
	}

	// Seen token in small corpus gets lower weight
	siamWeight := corpus.Weight("สยาม")
	if siamWeight >= 1.0 {
		t.Errorf("expected seen token 'สยาม' to have weight < 1.0 in this small corpus, got %f", siamWeight)
	}
	if siamWeight <= 0.0 {
		t.Errorf("expected seen token 'สยาม' to have positive weight, got %f", siamWeight)
	}
}

func TestCorpusStats_IDFFormula(t *testing.T) {
	// Build small corpus: 10 records
	// Token 'rare' appears in 1 record
	// Token 'medium' appears in 5 records
	var sources []SourceRecord

	// 1 record with 'rare'
	sources = append(sources, SourceRecord{
		NormalizedName: Normalize("rare token"),
	})

	// 4 records with 'medium'
	for i := 0; i < 4; i++ {
		sources = append(sources, SourceRecord{
			NormalizedName: Normalize("medium token"),
		})
	}

	// 5 records with neither
	for i := 0; i < 5; i++ {
		sources = append(sources, SourceRecord{
			NormalizedName: Normalize("other token"),
		})
	}

	corpus := BuildCorpusStats(sources, nil)
	if corpus == nil {
		t.Fatal("expected non-nil corpus stats")
	}

	if n := corpus.GetDocumentCount(); n != 10 {
		t.Errorf("expected n=10, got %d", n)
	}

	// Verify df values
	dfRare := corpus.GetDocumentFrequency("rare")
	if dfRare != 1 {
		t.Errorf("expected df['rare']=1, got %d", dfRare)
	}

	dfMedium := corpus.GetDocumentFrequency("medium")
	if dfMedium != 4 {
		t.Errorf("expected df['medium']=4, got %d", dfMedium)
	}

	// Compute expected weights using formula: log(1 + N/(1+df)) / log(1+N)
	N := 10.0
	idfRareExpected := math.Log(1.0+N/(1.0+1.0)) / math.Log(1.0+N)
	idfMediumExpected := math.Log(1.0+N/(1.0+4.0)) / math.Log(1.0+N)

	idfRareCalculated := corpus.Weight("rare")
	idfMediumCalculated := corpus.Weight("medium")

	// Verify formula correctness
	epsilon := 1e-9
	if math.Abs(idfRareCalculated-idfRareExpected) > epsilon {
		t.Errorf("expected weight['rare'] = %f, got %f", idfRareExpected, idfRareCalculated)
	}
	if math.Abs(idfMediumCalculated-idfMediumExpected) > epsilon {
		t.Errorf("expected weight['medium'] = %f, got %f", idfMediumExpected, idfMediumCalculated)
	}

	// Verify rarity ordering: rare > medium
	if idfRareCalculated <= idfMediumCalculated {
		t.Errorf("rare token weight (%f) should be > medium token weight (%f)",
			idfRareCalculated, idfMediumCalculated)
	}
}

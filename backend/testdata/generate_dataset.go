package testdata

import (
	"fmt"
	"math/rand"
	"time"

	"entitymatcher/matcher"
)

type LabeledPair struct {
	Source      matcher.SourceRecord
	Destination matcher.DestinationRecord
	IsMatch     bool
	Category    string
}

// GenerateBigMockDataset generates N paired records with ground-truth labels for evaluation.
func GenerateBigMockDataset(count int) ([]matcher.SourceRecord, []matcher.DestinationRecord, map[string]bool, []LabeledPair) {
	rand.Seed(42) // Fixed seed for reproducible benchmarks

	thaiFirstNames := []string{"สมชาย", "อารียา", "วีระชัย", "กนกวรรณ", "ณัฐพงษ์", "ภัทรพล", "ชลธิชา", "ธนกร", "สิรินธร", "กิตติพงษ์"}
	thaiLastNames := []string{"เข็มกลัด", "สุขสันต์", "พงษ์สวัสดิ์", "สิริโชติ", "รัตนโชติ", "วงศ์สุวรรณ", "ชัยประเสริฐ", "อินทร์แก้ว", "จันทร์หอม", "บุญมี"}
	thaiTitles := []string{"นาย", "นางสาว", "นาง", "คุณ", "ดร.", "นพ.", "พญ.", "ศาสตราจารย์"}

	engFirstNames := []string{"John", "Michael", "Sarah", "David", "Emma", "Robert", "James", "Emily", "William", "Sophia"}
	engLastNames := []string{"Smith", "Johnson", "Williams", "Brown", "Jones", "Miller", "Davis", "Wilson", "Anderson", "Taylor"}
	engTitles := []string{"Mr.", "Mrs.", "Ms.", "Dr.", "Prof."}

	corpPrefixes := []string{"สยาม", "กรุงเทพ", "เจริญโภคภัณฑ์", "กสิกรไทย", "เอไอเอส", "ปตท", "ไทยเบฟ", "เซ็นทรัล", "บิ๊กซี", "โลตัส"}
	corpSuffixesThai := []string{"จำกัด", "บจก.", "บมจ.", "จำกัด (มหาชน)", "ห้างหุ้นส่วนจำกัด"}
	corpSuffixesEng := []string{"Co., Ltd.", "Company Limited", "Corp.", "Inc.", "LLC", "PLC"}

	var sources []matcher.SourceRecord
	var dests []matcher.DestinationRecord
	groundTruthMatches := make(map[string]bool)
	var pairs []LabeledPair

	baseDate := time.Date(2026, 8, 15, 0, 0, 0, 0, time.UTC)
	batchID := "big-mock-batch-5000"

	pairID := 1
	for i := 0; i < count/2; i++ {
		// Category 1: Thai Personal Transposition & Title Removal
		fn := fmt.Sprintf("%s%d", thaiFirstNames[i%len(thaiFirstNames)], i+1)
		ln := thaiLastNames[i%len(thaiLastNames)]
		srcTitle := thaiTitles[i%len(thaiTitles)]

		srcRaw := fmt.Sprintf("%s %s %s", srcTitle, fn, ln)
		destRaw := fmt.Sprintf("%s %s", ln, fn) // Transposed

		srcID := fmt.Sprintf("src-%05d", pairID)
		destID := fmt.Sprintf("dest-%05d", pairID)

		dateDelta := rand.Intn(3) // 0-2 days delta
		txDateSrc := baseDate.AddDate(0, 0, -dateDelta)
		txDateDest := baseDate

		srcRec := matcher.SourceRecord{
			ID:              srcID,
			BatchID:         batchID,
			ReferenceID:     fmt.Sprintf("REF-TH-PERS-%05d", pairID),
			CustomerNameRaw: srcRaw,
			NormalizedName:  matcher.Normalize(srcRaw),
			TransactionDate: txDateSrc,
			TransactionType: "PAYMENT",
		}

		destRec := matcher.DestinationRecord{
			ID:              destID,
			BatchID:         batchID,
			CustomerID:      fmt.Sprintf("CUST-TH-PERS-%05d", pairID),
			CustomerNameRaw: destRaw,
			NormalizedName:  matcher.Normalize(destRaw),
			TransactionDate: txDateDest,
		}

		sources = append(sources, srcRec)
		dests = append(dests, destRec)

		matchKey := fmt.Sprintf("%s_%s", srcID, destID)
		groundTruthMatches[matchKey] = true

		pairs = append(pairs, LabeledPair{
			Source:      srcRec,
			Destination: destRec,
			IsMatch:     true,
			Category:    "THAI_PERSONAL_TRANSPOSITION",
		})
		pairID++

		// Category 2: Thai Corporate Suffix & Branch Number Match
		corpName := fmt.Sprintf("%s%d", corpPrefixes[i%len(corpPrefixes)], i+1)
		sufSrc := corpSuffixesThai[i%len(corpSuffixesThai)]
		sufDest := corpSuffixesThai[(i+1)%len(corpSuffixesThai)]

		srcCorpRaw := fmt.Sprintf("บริษัท %s เทคโนโลยี %s สาขาที่ 1", corpName, sufSrc)
		destCorpRaw := fmt.Sprintf("%s เทคโนโลยี %s สาขาที่ 1", corpName, sufDest)

		srcID = fmt.Sprintf("src-%05d", pairID)
		destID = fmt.Sprintf("dest-%05d", pairID)

		srcCorpRec := matcher.SourceRecord{
			ID:              srcID,
			BatchID:         batchID,
			ReferenceID:     fmt.Sprintf("REF-TH-CORP-%05d", pairID),
			CustomerNameRaw: srcCorpRaw,
			NormalizedName:  matcher.Normalize(srcCorpRaw),
			TransactionDate: baseDate,
			TransactionType: "TRANSFER",
		}

		destCorpRec := matcher.DestinationRecord{
			ID:              destID,
			BatchID:         batchID,
			CustomerID:      fmt.Sprintf("CUST-TH-CORP-%05d", pairID),
			CustomerNameRaw: destCorpRaw,
			NormalizedName:  matcher.Normalize(destCorpRaw),
			TransactionDate: baseDate,
		}

		sources = append(sources, srcCorpRec)
		dests = append(dests, destCorpRec)

		matchKey = fmt.Sprintf("%s_%s", srcID, destID)
		groundTruthMatches[matchKey] = true

		pairs = append(pairs, LabeledPair{
			Source:      srcCorpRec,
			Destination: destCorpRec,
			IsMatch:     true,
			Category:    "THAI_CORPORATE_SUFFIX_MATCH",
		})
		pairID++

		// Category 3: English Personal & Corporate Variations
		efn := fmt.Sprintf("%s%d", engFirstNames[i%len(engFirstNames)], i+1)
		eln := engLastNames[i%len(engLastNames)]
		eTitle := engTitles[i%len(engTitles)]
		sufEngSrc := corpSuffixesEng[i%len(corpSuffixesEng)]
		sufEngDest := corpSuffixesEng[(i+1)%len(corpSuffixesEng)]

		srcEngRaw := fmt.Sprintf("%s %s %s", eTitle, efn, eln)
		destEngRaw := fmt.Sprintf("%s, %s", eln, efn)
		if i%2 == 0 {
			srcEngRaw = fmt.Sprintf("Global %s %s", efn, sufEngSrc)
			destEngRaw = fmt.Sprintf("Global %s %s", efn, sufEngDest)
		}

		srcID = fmt.Sprintf("src-%05d", pairID)
		destID = fmt.Sprintf("dest-%05d", pairID)

		srcEngRec := matcher.SourceRecord{
			ID:              srcID,
			BatchID:         batchID,
			ReferenceID:     fmt.Sprintf("REF-EN-PERS-%05d", pairID),
			CustomerNameRaw: srcEngRaw,
			NormalizedName:  matcher.Normalize(srcEngRaw),
			TransactionDate: baseDate,
			TransactionType: "PAYMENT",
		}

		destEngRec := matcher.DestinationRecord{
			ID:              destID,
			BatchID:         batchID,
			CustomerID:      fmt.Sprintf("CUST-EN-PERS-%05d", pairID),
			CustomerNameRaw: destEngRaw,
			NormalizedName:  matcher.Normalize(destEngRaw),
			TransactionDate: baseDate,
		}

		sources = append(sources, srcEngRec)
		dests = append(dests, destEngRec)

		matchKey = fmt.Sprintf("%s_%s", srcID, destID)
		groundTruthMatches[matchKey] = true

		pairs = append(pairs, LabeledPair{
			Source:      srcEngRec,
			Destination: destEngRec,
			IsMatch:     true,
			Category:    "ENGLISH_PERSONAL_VARIATION",
		})
		pairID++
	}

	// Category 4: Negative Controls (Non-Matches / Branch Mismatches / Generic Overlaps)
	for i := 0; i < count/2; i++ {
		srcID := fmt.Sprintf("src-neg-%05d", i+1)
		destID := fmt.Sprintf("dest-neg-%05d", i+1)

		// Non-match variant A: Branch Mismatch (Same company, different branch numbers)
		corpName := fmt.Sprintf("%s%d", corpPrefixes[i%len(corpPrefixes)], i+1)
		srcCorpRaw := fmt.Sprintf("บริษัท %s นวัตกรรม จำกัด (สาขาที่ 1)", corpName)
		destCorpRaw := fmt.Sprintf("%s นวัตกรรม บจก. (สาขาที่ 99)", corpName)

		srcRec := matcher.SourceRecord{
			ID:              srcID,
			BatchID:         batchID,
			ReferenceID:     fmt.Sprintf("REF-NEG-%05d", i+1),
			CustomerNameRaw: srcCorpRaw,
			NormalizedName:  matcher.Normalize(srcCorpRaw),
			TransactionDate: baseDate,
			TransactionType: "DEPOSIT",
		}

		destRec := matcher.DestinationRecord{
			ID:              destID,
			BatchID:         batchID,
			CustomerID:      fmt.Sprintf("CUST-NEG-%05d", i+1),
			CustomerNameRaw: destCorpRaw,
			NormalizedName:  matcher.Normalize(destCorpRaw),
			TransactionDate: baseDate,
		}

		sources = append(sources, srcRec)
		dests = append(dests, destRec)

		// GROUND TRUTH: IsMatch = false due to branch mismatch (Branch 1 vs Branch 99)
		pairs = append(pairs, LabeledPair{
			Source:      srcRec,
			Destination: destRec,
			IsMatch:     false,
			Category:    "NON_MATCH_BRANCH_MISMATCH",
		})
	}

	return sources, dests, groundTruthMatches, pairs
}

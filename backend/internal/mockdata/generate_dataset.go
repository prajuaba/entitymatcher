package mockdata

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

// Thai realistic homophone consonant classes (commonly-occurring substitutions)
var thaiInitialHomophones = map[rune][]rune{
	'ศ': {'ส'},           // s class
	'ส': {'ศ'},           // s class
	'ท': {'ธ'},           // th class
	'ธ': {'ท'},           // th class
	'พ': {'ภ'},           // ph class
	'ภ': {'พ'},           // ph class
	'ณ': {'น'},           // n class
	'น': {'ณ'},           // n class
}

// applyConsonantVariant replaces the first Thai consonant with a homophone
func applyConsonantVariant(text string, rng *rand.Rand) string {
	runes := []rune(text)
	for i, r := range runes {
		if homophones, exists := thaiInitialHomophones[r]; exists && len(homophones) > 0 {
			runes[i] = homophones[rng.Intn(len(homophones))]
			break
		}
	}
	return string(runes)
}

// GenerateBigMockDataset generates N paired records with ground-truth labels for evaluation.
// It includes positive matches and hard negative categories.
func GenerateBigMockDataset(count int) ([]matcher.SourceRecord, []matcher.DestinationRecord, map[string]bool, []LabeledPair) {
	rng := rand.New(rand.NewSource(42)) // Deterministic with new Source API

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
	negCount := 0

	// POSITIVE MATCHES: ~60% of dataset
	positiveCount := (count * 3) / 5
	for i := 0; i < positiveCount; i++ {
		// Category 1: Thai Personal Transposition & Title Removal
		fn := fmt.Sprintf("%s%d", thaiFirstNames[i%len(thaiFirstNames)], i+1)
		ln := thaiLastNames[i%len(thaiLastNames)]
		srcTitle := thaiTitles[i%len(thaiTitles)]

		srcRaw := fmt.Sprintf("%s %s %s", srcTitle, fn, ln)
		destRaw := fmt.Sprintf("%s %s", ln, fn) // Transposed

		srcID := fmt.Sprintf("src-%05d", pairID)
		destID := fmt.Sprintf("dest-%05d", pairID)

		dateDelta := rng.Intn(3) // 0-2 days delta
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

	// NEW POSITIVE CATEGORIES: Thai Spelling Variants (4 categories, unpadded and curated)
	variantCount := (count * 1) / 20 // 5% of dataset for indexed categories

	// Category 4: THAI_VARIANT_SHORT_GIVEN - Homophone consonant substitution in short names
	shortGivenPairs := []struct {
		base, variant string
	}{
		{"ศิริ", "สิริ"},     {"ณัฐ", "นัฐ"},       {"ธนา", "ทนา"},       {"ภัทร", "พัทร"},
		{"ศักดิ์", "สักดิ์"},  {"สุข", "สุก"},       {"ชัย", "ไชย"},       {"เอก", "เอ็ก"},
		{"โชค", "ชค"},       {"ไพศาล", "พิศาล"},   {"ธีร์", "ทีร์"},     {"วุฒิ", "วุฐิ"},
		{"จรัส", "จรัซ"},     {"ปิติ", "ปีติ"},     {"สุวรรณ", "สุวัน"},
	}
	for _, pair := range shortGivenPairs {
		srcID := fmt.Sprintf("src-%05d", pairID)
		destID := fmt.Sprintf("dest-%05d", pairID)
		srcRec := matcher.SourceRecord{
			ID:              srcID,
			BatchID:         batchID,
			ReferenceID:     fmt.Sprintf("REF-THAI-VAR-SHORT-%05d", pairID),
			CustomerNameRaw: pair.base,
			NormalizedName:  matcher.Normalize(pair.base),
			TransactionDate: baseDate,
			TransactionType: "PAYMENT",
		}
		destRec := matcher.DestinationRecord{
			ID:              destID,
			BatchID:         batchID,
			CustomerID:      fmt.Sprintf("CUST-THAI-VAR-SHORT-%05d", pairID),
			CustomerNameRaw: pair.variant,
			NormalizedName:  matcher.Normalize(pair.variant),
			TransactionDate: baseDate,
		}
		sources = append(sources, srcRec)
		dests = append(dests, destRec)
		matchKey := fmt.Sprintf("%s_%s", srcID, destID)
		groundTruthMatches[matchKey] = true
		pairs = append(pairs, LabeledPair{
			Source:      srcRec,
			Destination: destRec,
			IsMatch:     true,
			Category:    "THAI_VARIANT_SHORT_GIVEN",
		})
		pairID++
	}

	// Category 5: THAI_VARIANT_FULL_NAME - Homophone substitution in full names (indexed to avoid collision)
	for i := 0; i < variantCount; i++ {
		baseFN := fmt.Sprintf("%s%d", thaiFirstNames[i%len(thaiFirstNames)], i+500)
		baseLN := fmt.Sprintf("%s%d", thaiLastNames[i%len(thaiLastNames)], i+500)
		fullName := fmt.Sprintf("%s %s", baseFN, baseLN)
		if i%2 == 0 {
			baseFN = applyConsonantVariant(baseFN, rng)
		} else {
			baseLN = applyConsonantVariant(baseLN, rng)
		}
		variant := fmt.Sprintf("%s %s", baseFN, baseLN)
		srcID := fmt.Sprintf("src-%05d", pairID)
		destID := fmt.Sprintf("dest-%05d", pairID)
		srcRec := matcher.SourceRecord{
			ID:              srcID,
			BatchID:         batchID,
			ReferenceID:     fmt.Sprintf("REF-THAI-VAR-FULL-%05d", pairID),
			CustomerNameRaw: fullName,
			NormalizedName:  matcher.Normalize(fullName),
			TransactionDate: baseDate,
			TransactionType: "PAYMENT",
		}
		destRec := matcher.DestinationRecord{
			ID:              destID,
			BatchID:         batchID,
			CustomerID:      fmt.Sprintf("CUST-THAI-VAR-FULL-%05d", pairID),
			CustomerNameRaw: variant,
			NormalizedName:  matcher.Normalize(variant),
			TransactionDate: baseDate,
		}
		sources = append(sources, srcRec)
		dests = append(dests, destRec)
		matchKey := fmt.Sprintf("%s_%s", srcID, destID)
		groundTruthMatches[matchKey] = true
		pairs = append(pairs, LabeledPair{
			Source:      srcRec,
			Destination: destRec,
			IsMatch:     true,
			Category:    "THAI_VARIANT_FULL_NAME",
		})
		pairID++
	}

	// Category 6: THAI_VARIANT_LEADING_VOWEL - Vowel position and ใ/ไ confusion (real closed-class variants)
	// Sub-class A: ใ/ไ confusion (mai muan vs mai malai - THE classic Thai spelling error)
	maiMuanVariants := []struct {
		base, variant string
	}{
		{"ใจ", "ไจ"}, {"ใจดี", "ไจดี"}, {"ใจงาม", "ไจงาม"}, {"ใจเย็น", "ไจเย็น"},
		{"ใหม่", "ไหม่"}, {"ใหญ่", "ไหญ่"}, {"ใส", "ไส"}, {"ใสสอาด", "ไสสอาด"},
		{"ให้", "ไห้"}, {"ใช้", "ไช้"}, {"ใช่", "ไช่"}, {"น้ำใจ", "น้ำไจ"},
		{"สุใจ", "สุไจ"},
	}
	// Sub-class B: Leading vowel reordering (written before consonant, pronounced after - phonetically identical)
	leadingVowelReorderPairs := []struct {
		base, variant string
	}{
		{"ชัย", "ไชย"}, {"ชัยวัฒน์", "ไชยวัฒน์"}, {"ชัยยา", "ไชยยา"},
	}
	allLeadingVowelPairs := append(maiMuanVariants, leadingVowelReorderPairs...)
	for _, pair := range allLeadingVowelPairs {
		srcID := fmt.Sprintf("src-%05d", pairID)
		destID := fmt.Sprintf("dest-%05d", pairID)
		srcRec := matcher.SourceRecord{
			ID:              srcID,
			BatchID:         batchID,
			ReferenceID:     fmt.Sprintf("REF-THAI-VAR-VOWEL-%05d", pairID),
			CustomerNameRaw: pair.base,
			NormalizedName:  matcher.Normalize(pair.base),
			TransactionDate: baseDate,
			TransactionType: "PAYMENT",
		}
		destRec := matcher.DestinationRecord{
			ID:              destID,
			BatchID:         batchID,
			CustomerID:      fmt.Sprintf("CUST-THAI-VAR-VOWEL-%05d", pairID),
			CustomerNameRaw: pair.variant,
			NormalizedName:  matcher.Normalize(pair.variant),
			TransactionDate: baseDate,
		}
		sources = append(sources, srcRec)
		dests = append(dests, destRec)
		matchKey := fmt.Sprintf("%s_%s", srcID, destID)
		groundTruthMatches[matchKey] = true
		pairs = append(pairs, LabeledPair{
			Source:      srcRec,
			Destination: destRec,
			IsMatch:     true,
			Category:    "THAI_VARIANT_LEADING_VOWEL",
		})
		pairID++
	}

	// Category 7: THAI_VARIANT_CORPORATE - Corporate names with homophone variant (indexed)
	for i := 0; i < variantCount; i++ {
		baseName := fmt.Sprintf("%s%d", corpPrefixes[i%len(corpPrefixes)], i+1500)
		variant := applyConsonantVariant(baseName, rng)
		srcCorpRaw := fmt.Sprintf("บริษัท %s เทคโนโลยี จำกัด", baseName)
		destCorpRaw := fmt.Sprintf("บริษัท %s เทคโนโลยี จำกัด", variant)
		srcID := fmt.Sprintf("src-%05d", pairID)
		destID := fmt.Sprintf("dest-%05d", pairID)
		srcRec := matcher.SourceRecord{
			ID:              srcID,
			BatchID:         batchID,
			ReferenceID:     fmt.Sprintf("REF-THAI-VAR-CORP-%05d", pairID),
			CustomerNameRaw: srcCorpRaw,
			NormalizedName:  matcher.Normalize(srcCorpRaw),
			TransactionDate: baseDate,
			TransactionType: "TRANSFER",
		}
		destRec := matcher.DestinationRecord{
			ID:              destID,
			BatchID:         batchID,
			CustomerID:      fmt.Sprintf("CUST-THAI-VAR-CORP-%05d", pairID),
			CustomerNameRaw: destCorpRaw,
			NormalizedName:  matcher.Normalize(destCorpRaw),
			TransactionDate: baseDate,
		}
		sources = append(sources, srcRec)
		dests = append(dests, destRec)
		matchKey := fmt.Sprintf("%s_%s", srcID, destID)
		groundTruthMatches[matchKey] = true
		pairs = append(pairs, LabeledPair{
			Source:      srcRec,
			Destination: destRec,
			IsMatch:     true,
			Category:    "THAI_VARIANT_CORPORATE",
		})
		pairID++
	}

	// NEGATIVE MATCHES: ~40% of dataset (8 per iteration to distribute across 8 categories)
	negativeCount := (count * 2) / 5
	for i := 0; i < negativeCount; i++ {
		negCount = i + 1
		srcID := fmt.Sprintf("src-neg-%05d", negCount)
		destID := fmt.Sprintf("dest-neg-%05d", negCount)

		// Distribute among 8 negative categories evenly
		category := i % 8

		switch category {
		case 0:
			// Category A: Branch Mismatch (existing, easy)
			corpName := fmt.Sprintf("%s%d", corpPrefixes[i%len(corpPrefixes)], i+1)
			srcCorpRaw := fmt.Sprintf("บริษัท %s นวัตกรรม จำกัด (สาขาที่ 1)", corpName)
			destCorpRaw := fmt.Sprintf("%s นวัตกรรม บจก. (สาขาที่ 99)", corpName)

			srcRec := matcher.SourceRecord{
				ID:              srcID,
				BatchID:         batchID,
				ReferenceID:     fmt.Sprintf("REF-NEG-A-%05d", negCount),
				CustomerNameRaw: srcCorpRaw,
				NormalizedName:  matcher.Normalize(srcCorpRaw),
				TransactionDate: baseDate,
				TransactionType: "DEPOSIT",
			}

			destRec := matcher.DestinationRecord{
				ID:              destID,
				BatchID:         batchID,
				CustomerID:      fmt.Sprintf("CUST-NEG-A-%05d", negCount),
				CustomerNameRaw: destCorpRaw,
				NormalizedName:  matcher.Normalize(destCorpRaw),
				TransactionDate: baseDate,
			}

			sources = append(sources, srcRec)
			dests = append(dests, destRec)

			pairs = append(pairs, LabeledPair{
				Source:      srcRec,
				Destination: destRec,
				IsMatch:     false,
				Category:    "NON_MATCH_BRANCH_MISMATCH",
			})

		case 1:
			// Category B: Same Surname, Different Person (Thai)
			ln := thaiLastNames[i%len(thaiLastNames)]
			fn1 := thaiFirstNames[i%len(thaiFirstNames)]
			fn2 := thaiFirstNames[(i+1)%len(thaiFirstNames)]

			srcRaw := fmt.Sprintf("นาย %s %s", fn1, ln)
			destRaw := fmt.Sprintf("นางสาว %s %s", fn2, ln)

			srcRec := matcher.SourceRecord{
				ID:              srcID,
				BatchID:         batchID,
				ReferenceID:     fmt.Sprintf("REF-NEG-B-%05d", negCount),
				CustomerNameRaw: srcRaw,
				NormalizedName:  matcher.Normalize(srcRaw),
				TransactionDate: baseDate,
				TransactionType: "PAYMENT",
			}

			destRec := matcher.DestinationRecord{
				ID:              destID,
				BatchID:         batchID,
				CustomerID:      fmt.Sprintf("CUST-NEG-B-%05d", negCount),
				CustomerNameRaw: destRaw,
				NormalizedName:  matcher.Normalize(destRaw),
				TransactionDate: baseDate,
			}

			sources = append(sources, srcRec)
			dests = append(dests, destRec)

			pairs = append(pairs, LabeledPair{
				Source:      srcRec,
				Destination: destRec,
				IsMatch:     false,
				Category:    "NEG_SAME_SURNAME_DIFF_PERSON",
			})

		case 2:
			// Category C: Same Surname, Different Person (English)
			ln := engLastNames[i%len(engLastNames)]
			fn1 := engFirstNames[i%len(engFirstNames)]
			fn2 := engFirstNames[(i+1)%len(engFirstNames)]

			srcRaw := fmt.Sprintf("Mr. %s %s", fn1, ln)
			destRaw := fmt.Sprintf("Ms. %s %s", fn2, ln)

			srcRec := matcher.SourceRecord{
				ID:              srcID,
				BatchID:         batchID,
				ReferenceID:     fmt.Sprintf("REF-NEG-C-%05d", negCount),
				CustomerNameRaw: srcRaw,
				NormalizedName:  matcher.Normalize(srcRaw),
				TransactionDate: baseDate,
				TransactionType: "PAYMENT",
			}

			destRec := matcher.DestinationRecord{
				ID:              destID,
				BatchID:         batchID,
				CustomerID:      fmt.Sprintf("CUST-NEG-C-%05d", negCount),
				CustomerNameRaw: destRaw,
				NormalizedName:  matcher.Normalize(destRaw),
				TransactionDate: baseDate,
			}

			sources = append(sources, srcRec)
			dests = append(dests, destRec)

			pairs = append(pairs, LabeledPair{
				Source:      srcRec,
				Destination: destRec,
				IsMatch:     false,
				Category:    "NEG_SAME_SURNAME_DIFF_PERSON",
			})

		case 3:
			// Category D: Same Corporate Prefix, Different Entity (Thai)
			prefix := corpPrefixes[i%len(corpPrefixes)]

			srcRaw := fmt.Sprintf("บริษัท %s เทคโนโลยี จำกัด", prefix)
			destRaw := fmt.Sprintf("บริษัท %s ประกันภัย จำกัด", prefix)

			srcRec := matcher.SourceRecord{
				ID:              srcID,
				BatchID:         batchID,
				ReferenceID:     fmt.Sprintf("REF-NEG-D-%05d", negCount),
				CustomerNameRaw: srcRaw,
				NormalizedName:  matcher.Normalize(srcRaw),
				TransactionDate: baseDate,
				TransactionType: "TRANSFER",
			}

			destRec := matcher.DestinationRecord{
				ID:              destID,
				BatchID:         batchID,
				CustomerID:      fmt.Sprintf("CUST-NEG-D-%05d", negCount),
				CustomerNameRaw: destRaw,
				NormalizedName:  matcher.Normalize(destRaw),
				TransactionDate: baseDate,
			}

			sources = append(sources, srcRec)
			dests = append(dests, destRec)

			pairs = append(pairs, LabeledPair{
				Source:      srcRec,
				Destination: destRec,
				IsMatch:     false,
				Category:    "NEG_SAME_PREFIX_DIFF_ENTITY",
			})

		case 4:
			// Category E: Generic Token Overlap (Thai)
			// Two companies sharing only generic words but different distinctive tokens
			srcRaw := fmt.Sprintf("บริษัท เทคโนโลยี โซลูชั่น จำกัด")
			destRaw := fmt.Sprintf("บริษัท โซลูชั่น เทคโนโลยี จำกัด")

			srcRec := matcher.SourceRecord{
				ID:              srcID,
				BatchID:         batchID,
				ReferenceID:     fmt.Sprintf("REF-NEG-E-%05d", negCount),
				CustomerNameRaw: srcRaw,
				NormalizedName:  matcher.Normalize(srcRaw),
				TransactionDate: baseDate,
				TransactionType: "TRANSFER",
			}

			destRec := matcher.DestinationRecord{
				ID:              destID,
				BatchID:         batchID,
				CustomerID:      fmt.Sprintf("CUST-NEG-E-%05d", negCount),
				CustomerNameRaw: destRaw,
				NormalizedName:  matcher.Normalize(destRaw),
				TransactionDate: baseDate,
			}

			sources = append(sources, srcRec)
			dests = append(dests, destRec)

			pairs = append(pairs, LabeledPair{
				Source:      srcRec,
				Destination: destRec,
				IsMatch:     false,
				Category:    "NEG_GENERIC_TOKEN_OVERLAP",
			})

		case 5:
			// Category F: Initials Only (English)
			// "J. Smith" vs "John Smith" where they are different customers
			fn := engFirstNames[i%len(engFirstNames)]
			initial := string(fn[0])
			ln := engLastNames[i%len(engLastNames)]

			srcRaw := fmt.Sprintf("%s. %s", initial, ln)
			destRaw := fmt.Sprintf("Jane %s", engLastNames[(i+1)%len(engLastNames)])

			srcRec := matcher.SourceRecord{
				ID:              srcID,
				BatchID:         batchID,
				ReferenceID:     fmt.Sprintf("REF-NEG-F-%05d", negCount),
				CustomerNameRaw: srcRaw,
				NormalizedName:  matcher.Normalize(srcRaw),
				TransactionDate: baseDate,
				TransactionType: "PAYMENT",
			}

			destRec := matcher.DestinationRecord{
				ID:              destID,
				BatchID:         batchID,
				CustomerID:      fmt.Sprintf("CUST-NEG-F-%05d", negCount),
				CustomerNameRaw: destRaw,
				NormalizedName:  matcher.Normalize(destRaw),
				TransactionDate: baseDate,
			}

			sources = append(sources, srcRec)
			dests = append(dests, destRec)

			pairs = append(pairs, LabeledPair{
				Source:      srcRec,
				Destination: destRec,
				IsMatch:     false,
				Category:    "NEG_INITIALS_ONLY",
			})

		case 6:
			// Category G: Transposed Names (English) — Different People
			fn1 := engFirstNames[i%len(engFirstNames)]
			ln1 := engLastNames[i%len(engLastNames)]

			srcRaw := fmt.Sprintf("%s %s", fn1, ln1)
			destRaw := fmt.Sprintf("%s %s", ln1, fn1) // Different people with swapped names

			srcRec := matcher.SourceRecord{
				ID:              srcID,
				BatchID:         batchID,
				ReferenceID:     fmt.Sprintf("REF-NEG-G-%05d", negCount),
				CustomerNameRaw: srcRaw,
				NormalizedName:  matcher.Normalize(srcRaw),
				TransactionDate: baseDate,
				TransactionType: "PAYMENT",
			}

			destRec := matcher.DestinationRecord{
				ID:              destID,
				BatchID:         batchID,
				CustomerID:      fmt.Sprintf("CUST-NEG-G-%05d", negCount),
				CustomerNameRaw: destRaw,
				NormalizedName:  matcher.Normalize(destRaw),
				TransactionDate: baseDate,
			}

			sources = append(sources, srcRec)
			dests = append(dests, destRec)

			pairs = append(pairs, LabeledPair{
				Source:      srcRec,
				Destination: destRec,
				IsMatch:     false,
				Category:    "NEG_TRANSPOSED_DIFFERENT_PEOPLE",
			})

		case 7:
			// Category H: Transposed Names (Thai) — Different People
			fn1 := thaiFirstNames[i%len(thaiFirstNames)]
			ln1 := thaiLastNames[i%len(thaiLastNames)]

			srcRaw := fmt.Sprintf("นาย %s %s", fn1, ln1)
			destRaw := fmt.Sprintf("นาย %s %s", ln1, fn1) // Different people with swapped names

			srcRec := matcher.SourceRecord{
				ID:              srcID,
				BatchID:         batchID,
				ReferenceID:     fmt.Sprintf("REF-NEG-H-%05d", negCount),
				CustomerNameRaw: srcRaw,
				NormalizedName:  matcher.Normalize(srcRaw),
				TransactionDate: baseDate,
				TransactionType: "PAYMENT",
			}

			destRec := matcher.DestinationRecord{
				ID:              destID,
				BatchID:         batchID,
				CustomerID:      fmt.Sprintf("CUST-NEG-H-%05d", negCount),
				CustomerNameRaw: destRaw,
				NormalizedName:  matcher.Normalize(destRaw),
				TransactionDate: baseDate,
			}

			sources = append(sources, srcRec)
			dests = append(dests, destRec)

			pairs = append(pairs, LabeledPair{
				Source:      srcRec,
				Destination: destRec,
				IsMatch:     false,
				Category:    "NEG_TRANSPOSED_DIFFERENT_PEOPLE",
			})
		}
	}

	return sources, dests, groundTruthMatches, pairs
}

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

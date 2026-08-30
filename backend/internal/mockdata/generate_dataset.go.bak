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
		{"ศักดิ์", "สักดิ์"},  {"สุข", "สุก"},       {"เอก", "เอ็ก"},
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
		{"ให้", "ไห้"}, {"ใช้", "ไช้"}, {"น้ำใจ", "น้ำไจ"},
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

	// POSITIVE BILINGUAL CATEGORIES: Thai-Latin Cross-Script Matching

	// Category 8: BILINGUAL_IN_DICTIONARY - Uses entities that ARE in the hardcoded map (memorization)
	// These 9 pairs are the ones hardcoded in matcher/normalizer.go
	// Make Thai side unique with full surnames to avoid collision, Latin side unindexed
	bilingualInDictPairs := []struct {
		thaiUnique, latin string
	}{
		{"ชื่อหนึ่งบัญชาการสยาม", "siam"},
		{"ท่าขมสูงกรุงเทพมหานคร", "bangkok"},
		{"คณะบัญชีกสิกรไทย", "kasikornbank"},
		{"เครือข่ายเอไอเอสนานาชาติ", "ais"},
		{"โครงการจัดการเจริญโภคภัณฑ์", "charoen pokphand"},
		{"ตัวแทนอย่างสมชายผู้บริหาร", "somchai"},
		{"ดีกรีศศาสตร์อารียา", "areeya"},
		{"องค์กรวีระชัยวิจัยพัฒนา", "weerachai"},
		{"ระบบจัดการพงษ์สวัสดิ์ต่างประเทศ", "pongsawat"},
	}
	for idx, pair := range bilingualInDictPairs {
		srcID := fmt.Sprintf("src-%05d", pairID)
		destID := fmt.Sprintf("dest-%05d", pairID)

		// Alternate direction: even indices Thai->Latin, odd indices Latin->Thai
		var srcRaw, destRaw string
		if idx%2 == 0 {
			srcRaw = pair.thaiUnique  // Thai with unique surname
			destRaw = pair.latin       // Latin unindexed
		} else {
			srcRaw = pair.latin        // Latin unindexed
			destRaw = pair.thaiUnique  // Thai with unique surname
		}

		srcRec := matcher.SourceRecord{
			ID:              srcID,
			BatchID:         batchID,
			ReferenceID:     fmt.Sprintf("REF-BILINGUAL-DICT-%05d", pairID),
			CustomerNameRaw: srcRaw,
			NormalizedName:  matcher.Normalize(srcRaw),
			TransactionDate: baseDate,
			TransactionType: "PAYMENT",
		}

		destRec := matcher.DestinationRecord{
			ID:              destID,
			BatchID:         batchID,
			CustomerID:      fmt.Sprintf("CUST-BILINGUAL-DICT-%05d", pairID),
			CustomerNameRaw: destRaw,
			NormalizedName:  matcher.Normalize(destRaw),
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
			Category:    "BILINGUAL_IN_DICTIONARY",
		})
		pairID++
	}

	// Category 9: BILINGUAL_OUT_OF_DICT - Real Thai names with RTGS romanizations NOT in the hardcoded map (generalization)
	// These pairs use completely unique Thai names not in the corpus, with RTGS romanization.
	// Use place names, less common personal names, and corporate names to ensure uniqueness.
	bilingualOutOfDictPairs := []struct {
		thaiUnique, latinUnique string
	}{
		{"ท่อมระนัง สัตยาพร", "Suchat Pracharoen"},
		{"ท่าแพร่ศิลป์ ประเทศ", "Suchart Sangpradit"},       // RTGS variant
		{"บ้านศรีสะพัน มงคลนิเวศ", "Prasert Chantarangsul"},
		{"บ้านแหลม วิชัยวัฒนา", "Wichai Sirinimit"},
		{"คลองแสนแสบ เวิ้งวังวิจิตร", "Vichai Namprasit"},      // variant
		{"แม่ประมาณสุข ธรรมรักษ์", "Thanakorn Phromarak"},
		{"ไร่เหมืองชัยภูมิ สัตชัย", "Napha Khunchai"},
		{"หนองวัวซอรจนา กมลาวรรณ", "Kamon Sukhumvit"},
		{"วังสะพัน ศักดิ์สยามสิทธิ์", "Sakchai Sitthiprom"},
		{"บ่อวิน เบญจพลอรุณ", "Bunmi Ratchapruk"},
		{"โครงการตั้งนครดุษฎี", "Boonmee Settakul"},          // variant
		{"บ้านปงน้ำเย็นชัย", "Chan Namdamrong"},
		{"โครงการสวนป่าแสนทรัพย์", "Jan Sitsuktham"},               // variant
		{"ศรีสมุทรเจดีย์ศิลป์", "Saengthong Suthisak"},
		{"บ้านกว่างไผ่ประสิทธิ์", "Rungroj Phakarom"},
		{"หนองผึ้งส่วนราชการเรื่อง", "Phimchai Thairat"},
		{"ไม้โครงสร้างชลธารา", "Chonthicha Nilthai"},
		{"เขตภูเก็ตบ้านนรา", "Narong Prachachuen"},
		{"เขตนวมพระนครสิทธิ์", "Narongse Sothawech"},        // variant
		{"เขตพยุหะอรุณธารา", "Arun Rungsawang"},
		{"ท่อมสิงห์เมฆาศรี", "Metha Kananek"},
		{"โครงการน้ำชลสิทธิ์ไทยปูนอินดัสเทรี", "Thai Cement International Holdings"},
		{"บริษัทชุมชนสมเด็จพระนเรศวรอนุสรณ์", "Eastern Siam Concrete Works"},
		{"บริษัทการขนส่งเอกชนรัฐการจัดการ", "Asian Airways Logistics"},
		{"เขตธัญบุรีท่องเที่ยวพัฒนา", "Phetchabun Tourism Services"},
		{"เขตข่วงท่าแพสินค้า", "Udon Thani International"},
		{"เขตชั้นเลิศท่าบ่อจังหวัร", "Sakon Nakhon Regional"},
		{"บริษัทพนันน้ำหลวงเขต", "Maha Sarakham Cooperative"},
		{"บ้านศรีฟ้าครบถ้วนศิลป์", "Loei Provincial Authority"},
		{"โครงการชั้นราษฎรสิทธิ์", "Nakhon Phanom Federation"},
	}
	for idx, pair := range bilingualOutOfDictPairs {
		srcID := fmt.Sprintf("src-%05d", pairID)
		destID := fmt.Sprintf("dest-%05d", pairID)

		// Vary direction: every 3rd is Thai->Latin, others are Latin->Thai
		var srcRaw, destRaw string
		if idx%3 == 0 {
			srcRaw = pair.thaiUnique    // Thai with unique name
			destRaw = pair.latinUnique  // Latin with unique name
		} else {
			srcRaw = pair.latinUnique   // Latin with unique name
			destRaw = pair.thaiUnique   // Thai with unique name
		}

		srcRec := matcher.SourceRecord{
			ID:              srcID,
			BatchID:         batchID,
			ReferenceID:     fmt.Sprintf("REF-BILINGUAL-OUTDICT-%05d", pairID),
			CustomerNameRaw: srcRaw,
			NormalizedName:  matcher.Normalize(srcRaw),
			TransactionDate: baseDate,
			TransactionType: "PAYMENT",
		}

		destRec := matcher.DestinationRecord{
			ID:              destID,
			BatchID:         batchID,
			CustomerID:      fmt.Sprintf("CUST-BILINGUAL-OUTDICT-%05d", pairID),
			CustomerNameRaw: destRaw,
			NormalizedName:  matcher.Normalize(destRaw),
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
			Category:    "BILINGUAL_OUT_OF_DICT",
		})
		pairID++
	}

	// Category 10: NEG_BILINGUAL_FALSE_FRIEND - Completely unique Thai and English names that should NOT match each other
	// Using completely new names not in the corpus to ensure they remain TN (true negatives).
	bilingualFalseFriendPairs := []struct {
		thai, latin string
	}{
		{"เชิดชัย ตันไทย", "Somchit Johnson"},
		{"วรสิทธิ์ สุขวิสัย", "Somsak Lee"},
		{"วิพัฒน์ คงสมพงษ์", "Wichian Brown"},
		{"วิเศษ ศิลภา", "Wichit Davis"},
		{"สิรินธร นิลนอก", "Siri Smith"},
		{"สิริสาร ลิ่มค้อ", "Siril Williams"},
		{"ชัยพร ชิณะวรรณ", "Chai Miller"},
		{"ชาญชัย นวลสวรรค์", "Chay Wilson"},
		{"ธรรมชาตินรงค์ สหระ", "Tam Anderson"},
		{"ธรรมวิทย์ ศรีหา", "Thum Taylor"},
		{"นรเศร ภูมิศรี", "Nara Thomas"},
		{"นรพิพัฒน์ กลิ่นมะลิ", "Naran Jackson"},
		{"ปรีชา อินทร์ศิลป์", "Priya White"},
		{"ปรีชากร โครงสร้าง", "Preeya Harris"},
		{"สิชน ธรรมชั้น", "Sichon Martin"},
		{"สิชล ทองมนต์", "Sichol Garcia"},
		{"วรรณวัฒน์ คำหา", "Wanna Rodriguez"},
		{"วรรณพร ศรีบูชา", "Wanit Martinez"},
		{"กฤษณพล ลี้แล", "Krish Robinson"},
		{"กฤษณสิลป์ จันทศรี", "Krishna Clark"},
		{"นิรามล ทรัพย์เศรษฐ", "Neera Lewis"},
		{"นิรทา พุ่มพวง", "Neran Lee"},
		{"สมรสาตรี ธรรมรักษ์", "Somrasa Walker"},
		{"สมรสา เจริญสิน", "Samrasa Hall"},
		{"ศิลปกร แก้วกำแพง", "Sil Young"},
		{"ศิลปสิน อังคาร", "Silp Hernandez"},
		{"นันทศรี โกศล", "Nan King"},
		{"นันทีย์ มูนเนือง", "Nant Wright"},
		{"โยธินวร นิ่มนวล", "Yothin Lopez"},
		{"โยธิน สระดี", "Yodin Hill"},
		{"ธรรมชาติสิลป์ ลิ้มปิยะ", "Thamachai Scott"},
		{"ธรรมชาตินอก ผลิเสรี", "Thammachai Green"},
		{"กมลา วัฒนาพร", "Kamala Adams"},
		{"กมลาพร ชื่นสวัสดิ์", "Kamela Nelson"},
		{"ขจรศักดิ์ กิจสวัสดิ์", "Khachon Carter"},
		{"ขจรศักดี้ อินทร์สิทธิ์", "Khachorn Mitchell"},
		{"บัญชา คุณรัตน์", "Bancha Perez"},
		{"บัญชาพร ไทรทอง", "Buncha Roberts"},
		{"นิพัฒน์สิน ศรีบัณฑิต", "Niphat Phillips"},
		{"นิพัฒนา นิยมนีย์", "Niphon Campbell"},
		{"ศรัณยา ชุมชอบ", "Saran Parker"},
		{"ศรัณย์ นะภักดี", "Sarin Evans"},
		{"ธวัชชัยกร ศิริวงษ์", "Thawachai Edwards"},
		{"ธวัชชัย สารศิลป์", "Thavachai Collins"},
		{"วารีนันท์ พลธีร", "Wari Reyes"},
		{"วารี เหลี่ยมกา", "Waree Morris"},
		{"เอกรัฐสิทธิ์ ทีระวัฒน์", "Ekarat Murphy"},
		{"เอกรัฐ คำศิริ", "Ekarath Rogers"},
		{"อัจฉริยะวัฒน์ เกื้อกูล", "Achariya Morgan"},
		{"อัจฉริยะพร นิลประสาท", "Achariyah Peterson"},
		{"บรรจงชัย ลิ่มรักษ์", "Banjong Gray"},
		{"บรรจง สุรศิลป์", "Banjon Ramirez"},
	}
	for negIdx, pair := range bilingualFalseFriendPairs {
		srcID := fmt.Sprintf("src-neg-bilingual-%05d", negIdx+1)
		destID := fmt.Sprintf("dest-neg-bilingual-%05d", negIdx+1)

		// Vary direction to avoid patterns
		var srcRaw, destRaw string
		if negIdx%2 == 0 {
			srcRaw = pair.thai
			destRaw = pair.latin
		} else {
			srcRaw = pair.latin
			destRaw = pair.thai
		}

		srcRec := matcher.SourceRecord{
			ID:              srcID,
			BatchID:         batchID,
			ReferenceID:     fmt.Sprintf("REF-NEG-BILINGUAL-FRIEND-%05d", negIdx+1),
			CustomerNameRaw: srcRaw,
			NormalizedName:  matcher.Normalize(srcRaw),
			TransactionDate: baseDate,
			TransactionType: "PAYMENT",
		}

		destRec := matcher.DestinationRecord{
			ID:              destID,
			BatchID:         batchID,
			CustomerID:      fmt.Sprintf("CUST-NEG-BILINGUAL-FRIEND-%05d", negIdx+1),
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
			Category:    "NEG_BILINGUAL_FALSE_FRIEND",
		})
		negCount++
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

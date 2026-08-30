package matcher

import "testing"

func TestThaiPhoneticForm_Homophones(t *testing.T) {
	homophonePairs := [][2]string{
		{"ศิริ", "สิริ"},
		{"ณัฐ", "นัฐ"},
		{"ธนา", "ทนา"},
		{"ภัทร", "พัทร"},
		{"ชัย", "ไชย"},
		{"พงษ์", "พงศ์"},
		{"พงษ์", "พงส์"},
		{"พงศ์", "พงส์"},
		{"สุวรรณ", "สุวัณณ์"},
		{"ใจดี", "ไจดี"},
	}

	for _, pair := range homophonePairs {
		output1 := ThaiPhoneticForm(pair[0])
		output2 := ThaiPhoneticForm(pair[1])
		if output1 != output2 {
			t.Errorf("homophones '%s' and '%s' should produce identical phonetic forms, got '%s' and '%s'", pair[0], pair[1], output1, output2)
		}
	}
}

func TestThaiPhoneticForm_DifferentNames(t *testing.T) {
	differentPairs := [][2]string{
		{"ราม", "ลาม"},
		{"สมชาย", "สมหญิง"},
		{"ชัย", "ชาย"},
	}

	for _, pair := range differentPairs {
		output1 := ThaiPhoneticForm(pair[0])
		output2 := ThaiPhoneticForm(pair[1])
		if output1 == output2 {
			t.Errorf("different names '%s' and '%s' should produce different phonetic forms, both got '%s'", pair[0], pair[1], output1)
		}
	}
}

package main

var wrightstoneCN = map[string]string{
	"Dread Wrightstone":         "恐惧祝福",
	"Vitality Wrightstone":      "生命祝福",
	"Fortification Wrightstone": "坚守祝福",
	"Sequestration Wrightstone": "隔绝祝福",
}

func cnWrightstone(en string) string {
	if !useChinese() {
		return en
	}
	if v, ok := wrightstoneCN[en]; ok {
		return v
	}
	return en
}

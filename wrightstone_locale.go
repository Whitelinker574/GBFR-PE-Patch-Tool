package main

var wrightstoneCN = map[string]string{
	"Dread Wrightstone":         "畏惧之祝福",
	"Vitality Wrightstone":      "生机之祝福",
	"Fortification Wrightstone": "镇守之祝福",
	"Sequestration Wrightstone": "隔绝之祝福",
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

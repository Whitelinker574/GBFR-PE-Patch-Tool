package main

import "testing"

func TestSigilMemoryOptionsUseSharedSecondaryRules(t *testing.T) {
	options, err := (&App{}).SigilMemoryGetOptions()
	if err != nil {
		t.Fatal(err)
	}

	const pursuitVPlus = uint32(0x035A4DDD)
	const preciseWrath = uint32(0x7EDD69D0)
	for _, sigil := range options.Sigils {
		if sigil.Hash != pursuitVPlus {
			continue
		}
		for _, hash := range sigil.AllowedSecondaryTraitHashes {
			if hash == preciseWrath {
				return
			}
		}
		t.Fatalf("追击 V+ 合规列表缺少怒发冲冠 (0x%08X)", preciseWrath)
	}
	t.Fatalf("因子选项中未找到追击 V+ (0x%08X)", pursuitVPlus)
}

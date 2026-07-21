package backend

import (
	"strings"
	"testing"
)

func TestSharedRuntimePatchClassifiesEveryOwnedState(t *testing.T) {
	original := append([]byte(nil), sharedInventoryMaterialOriginal...)
	material := append([]byte(nil), original...)
	copy(material, materialConsumePatch)
	inventory := append([]byte(nil), original...)
	copy(inventory, []byte{0xE9, 0x11, 0x22, 0x33, 0x44})

	tests := []struct {
		name string
		data []byte
		want sharedRuntimePatchOwner
	}{
		{name: "original", data: original, want: sharedRuntimePatchOwnerNone},
		{name: "material consume nop", data: material, want: sharedRuntimePatchOwnerMaterialConsume},
		{name: "inventory quantity hook", data: inventory, want: sharedRuntimePatchOwnerInventoryQuantity},
		{name: "foreign jump bytes", data: []byte{0xE9, 0x11, 0x22, 0x33, 0x44, 0x90, 0x90}, want: sharedRuntimePatchOwnerUnknown},
		{name: "short read", data: original[:4], want: sharedRuntimePatchOwnerUnknown},
		{name: "foreign bytes", data: []byte{0x90, 0x90, 0x90, 0x90, 0x90, 0x90, 0x90}, want: sharedRuntimePatchOwnerUnknown},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := classifySharedRuntimePatch(tt.data); got != tt.want {
				t.Fatalf("classifySharedRuntimePatch(% X) = %q, want %q", tt.data, got, tt.want)
			}
		})
	}
}

func TestSharedRuntimePatchRejectsBothCrossFeatureOrders(t *testing.T) {
	original := append([]byte(nil), sharedInventoryMaterialOriginal...)
	material := append([]byte(nil), original...)
	copy(material, materialConsumePatch)
	inventory := append([]byte(nil), original...)
	copy(inventory, []byte{0xE9, 0x11, 0x22, 0x33, 0x44})

	tests := []struct {
		name    string
		current []byte
		actor   sharedRuntimePatchOwner
		enable  bool
		want    string
	}{
		{
			name:    "material consume cannot overwrite inventory hook",
			current: inventory, actor: sharedRuntimePatchOwnerMaterialConsume, enable: true,
			want: "小钳蟹",
		},
		{
			name:    "material restore cannot overwrite inventory hook",
			current: inventory, actor: sharedRuntimePatchOwnerMaterialConsume, enable: false,
			want: "小钳蟹",
		},
		{
			name:    "inventory hook cannot overwrite material nop",
			current: material, actor: sharedRuntimePatchOwnerInventoryQuantity, enable: true,
			want: "素材不消耗",
		},
		{
			name:    "inventory restore cannot overwrite material nop",
			current: material, actor: sharedRuntimePatchOwnerInventoryQuantity, enable: false,
			want: "素材不消耗",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := validateSharedRuntimePatchTransition(tt.current, tt.actor, tt.enable)
			if err == nil {
				t.Fatal("expected the overlapping patch transition to fail closed")
			}
			if !strings.Contains(err.Error(), tt.want) {
				t.Fatalf("error %q does not identify current owner %q", err, tt.want)
			}
		})
	}
}

func TestSharedRuntimePatchAllowsOnlyItsOwnLifecycle(t *testing.T) {
	original := append([]byte(nil), sharedInventoryMaterialOriginal...)
	material := append([]byte(nil), original...)
	copy(material, materialConsumePatch)
	inventory := append([]byte(nil), original...)
	copy(inventory, []byte{0xE9, 0x11, 0x22, 0x33, 0x44})

	allowed := []struct {
		name    string
		current []byte
		actor   sharedRuntimePatchOwner
		enable  bool
	}{
		{name: "enable material from original", current: original, actor: sharedRuntimePatchOwnerMaterialConsume, enable: true},
		{name: "refresh enabled material", current: material, actor: sharedRuntimePatchOwnerMaterialConsume, enable: true},
		{name: "restore material", current: material, actor: sharedRuntimePatchOwnerMaterialConsume, enable: false},
		{name: "enable inventory from original", current: original, actor: sharedRuntimePatchOwnerInventoryQuantity, enable: true},
		{name: "refresh enabled inventory", current: inventory, actor: sharedRuntimePatchOwnerInventoryQuantity, enable: true},
		{name: "restore inventory", current: inventory, actor: sharedRuntimePatchOwnerInventoryQuantity, enable: false},
	}

	for _, tt := range allowed {
		t.Run(tt.name, func(t *testing.T) {
			if err := validateSharedRuntimePatchTransition(tt.current, tt.actor, tt.enable); err != nil {
				t.Fatalf("expected transition to be allowed, got %v", err)
			}
		})
	}
}

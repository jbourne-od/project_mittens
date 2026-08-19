package model_test

import (
	"testing"

	"github.com/optimaldynamics/project-mittens/internal/domain/model"
)

func TestEquipment_Compatibility(t *testing.T) {
	reefer := model.Equipment{
		Type:         model.EquipReefer,
		Endorsements: []model.Endorsement{model.EndorsementHazmat},
	}

	dryVan := model.Equipment{
		Type:         model.EquipDryVan,
		Endorsements: []model.Endorsement{},
	}

	// 1. Reefer can haul dry van load
	if !reefer.CanHandle(model.EquipDryVan, nil) {
		t.Fatalf("reefer should be compatible with dry van freight")
	}

	// 2. Reefer can haul reefer load
	if !reefer.CanHandle(model.EquipReefer, nil) {
		t.Fatalf("reefer should be compatible with reefer freight")
	}

	// 3. Dry van CANNOT haul reefer load
	if dryVan.CanHandle(model.EquipReefer, nil) {
		t.Fatalf("dry van must NOT be compatible with reefer freight")
	}

	// 4. Hazmat endorsement check
	if !reefer.CanHandle(model.EquipDryVan, []model.Endorsement{model.EndorsementHazmat}) {
		t.Fatalf("hazmat certified reefer should handle hazmat load")
	}
	if dryVan.CanHandle(model.EquipDryVan, []model.Endorsement{model.EndorsementHazmat}) {
		t.Fatalf("dry van without hazmat endorsement must NOT handle hazmat load")
	}
}

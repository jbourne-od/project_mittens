package model

import (
	"strings"
)

// EquipmentType represents the physical tractor-trailer equipment specification.
type EquipmentType string

const (
	// EquipDryVan represents standard 53ft dry van trailer.
	EquipDryVan EquipmentType = "VAN_53"
	// EquipReefer represents refrigerated temperature-controlled trailer.
	EquipReefer EquipmentType = "REEFER_53"
	// EquipFlatbed represents flatbed open-deck trailer.
	EquipFlatbed EquipmentType = "FLATBED_53"
	// EquipTanker represents liquid bulk tanker trailer.
	EquipTanker EquipmentType = "TANKER"
	// EquipStepDeck represents drop-deck specialized trailer.
	EquipStepDeck EquipmentType = "STEP_DECK"
	// EquipAny represents universal compatibility.
	EquipAny EquipmentType = "ANY"
)

// Endorsement represents special regulatory certifications or operating capabilities required for a load.
type Endorsement string

const (
	// EndorsementHazmat indicates hazardous materials placard certification.
	EndorsementHazmat Endorsement = "HAZMAT"
	// EndorsementTanker indicates liquid bulk endorsement.
	EndorsementTanker Endorsement = "TANKER"
	// EndorsementDoublesTriples indicates multi-trailer commercial combination certification.
	EndorsementDoublesTriples Endorsement = "DOUBLES_TRIPLES"
	// EndorsementTWIC indicates Transportation Worker Identification Credential for maritime ports.
	EndorsementTWIC Endorsement = "TWIC"
	// EndorsementLiftgate indicates hydraulic liftgate delivery capability.
	EndorsementLiftgate Endorsement = "LIFTGATE"
	// EndorsementTeam indicates 2-driver team expedited transit capability.
	EndorsementTeam Endorsement = "TEAM"
)

// Equipment represents a power unit / trailer combination asset.
type Equipment struct {
	Type         EquipmentType
	Endorsements []Endorsement
}

// HasEndorsement returns true if the equipment possesses the requested endorsement.
func (e Equipment) HasEndorsement(target Endorsement) bool {
	for _, end := range e.Endorsements {
		if strings.EqualFold(string(end), string(target)) {
			return true
		}
	}
	return false
}

// CanHandle verifies whether this equipment can legally and physically service a load's equipment requirements.
func (e Equipment) CanHandle(requiredType EquipmentType, requiredEndorsements []Endorsement) bool {
	// 1. Equipment type compatibility
	if requiredType != "" && requiredType != EquipAny {
		// Reefer can haul dry freight (dry van loads), but dry van cannot haul temperature-controlled freight
		if requiredType == EquipDryVan {
			if e.Type != EquipDryVan && e.Type != EquipReefer {
				return false
			}
		} else if e.Type != requiredType {
			return false
		}
	}

	// 2. Required endorsements verification
	for _, req := range requiredEndorsements {
		if !e.HasEndorsement(req) {
			return false
		}
	}

	return true
}

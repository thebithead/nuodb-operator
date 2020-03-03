package v2alpha1

// ######################################################
// Start of Nuodb Health & Status section
// ######################################################
// NuodbHealth is the health of the NuoDB Domain as returned by the health API.
type NuodbHealth string

// Possible "traffic light" states NuoDB health can have.
const (
	NuodbUnknownHealth   NuodbHealth = "Unknown"
	NuodbRedHealth       NuodbHealth = "Red"
	NuodbYellowHealth    NuodbHealth = "Yellow"
	NuodbGreenHealth     NuodbHealth = "Green"
)

var nuodbHealthOrder = map[NuodbHealth]int{
	NuodbUnknownHealth:  0,
	NuodbRedHealth:      1,
	NuodbYellowHealth:   2,
	NuodbGreenHealth:    3,
}

// Less for NuodbHealth means green > yellow > red > unknown
func (h NuodbHealth) Less(other NuodbHealth) bool {
	l := nuodbHealthOrder[h]
	r := nuodbHealthOrder[other]
	// 0 is not found/unknown and less is not defined for that
	return l != 0 && r != 0 && l < r
}

// NuodbOrchestrationPhase is the phase NuoDB Domain is in from the controller point of view.
type NuodbOrchestrationPhase string

// NuoDB OrchestrationPhases
//noinspection GoUnusedConst
const (
	// NuodbOperationalPhase is operating at the desired spec.
	NuodbOperationalPhase NuodbOrchestrationPhase = "Operational"
	// NuodbPendingPhase controller is working towards a desired state, NuoDB Domain may be unavailable.
	NuodbPendingPhase NuodbOrchestrationPhase = "Pending"
	// NuodbMigratingDataPhase Elasticsearch is currently migrating data to another node.
	NuodbMigratingDataPhase NuodbOrchestrationPhase = "MigratingData"
	// NuodbResourceInvalid is marking a resource as invalid
	NuodbResourceInvalid NuodbOrchestrationPhase = "Invalid"
)

package backend

import (
	"encoding/json"
	"time"
)

// FateUncheckedArchive lists associations without a verified save-vector index.
// These archives remain explicitly
// unchecked. Do not infer their unlock state from Fate completion alone.
type FateUncheckedArchive struct {
	ArchiveID        string `json:"archiveId"`
	CharacterCode    string `json:"characterCode"`
	EpisodeKey       string `json:"episodeKey"`
	MissionCode      string `json:"missionCode"`
	RequiredQuestID  string `json:"requiredQuestId"`
	EpisodeCompleted bool   `json:"episodeCompleted"`
}

func fateUncheckedArchives(_ *fateEpisodeLayout) []FateUncheckedArchive {
	return []FateUncheckedArchive{}
}

type FateArchiveVectorObservation struct {
	IDType    uint32 `json:"idType"`
	Values    []int  `json:"values,omitempty"`
	Available bool   `json:"available"`
}

func fateArchiveVectorObservations(save *SaveData) []FateArchiveVectorObservation {
	definitions := []uint32{fateArchiveHashID, fateArchiveFlagsID}
	rows := make([]FateArchiveVectorObservation, 0, len(definitions))
	for _, definition := range definitions {
		row := FateArchiveVectorObservation{IDType: definition}
		entries, err := findUnitsByTypeFast(save, definition, fateArchiveRecordCount)
		if err == nil {
			row.Available = true
			row.Values = make([]int, fateArchiveRecordCount)
			for _, entry := range entries {
				if entry.UnitID >= fateArchiveRecordCount || entry.ValueCnt != 1 {
					row.Available = false
					row.Values = nil
					break
				}
				row.Values[entry.UnitID] = int(entry.Uint32())
			}
		}
		rows = append(rows, row)
	}
	return rows
}

func fateArchiveEvidenceFromPath(path string, at time.Time) ([]byte, error) {
	absolute, raw, err := readFateEpisodeRaw(path)
	if err != nil {
		return nil, err
	}
	save, err := newSaveData(absolute, raw)
	if err != nil {
		return nil, err
	}
	layout, err := inspectFateEpisodeLayout(save)
	if err != nil {
		return nil, err
	}
	data, err := fateEpisodeEvidenceJSON(&layout.status, at)
	if err != nil {
		return nil, err
	}
	var payload FateEpisodeEvidenceExport
	if err := json.Unmarshal(data, &payload); err != nil {
		return nil, err
	}
	payload.UncheckedArchives = fateUncheckedArchives(layout)
	payload.ArchiveObservations = fateArchiveVectorObservations(save)
	if vector, vectorErr := readFateArchiveRecords(save); vectorErr == nil {
		payload.StoryArchives, err = inspectFateStoryArchives(layout, vector)
		if err != nil {
			return nil, err
		}
	}
	return json.MarshalIndent(payload, "", "  ")
}

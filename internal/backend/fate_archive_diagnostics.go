package backend

import (
	"encoding/json"
	"fmt"
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

func fateUncheckedArchives(layout *fateEpisodeLayout) []FateUncheckedArchive {
	rows := []FateUncheckedArchive{
		{ArchiveID: "ARC_OTHER_061", CharacterCode: "PL1600"},
		{ArchiveID: "ARC_OTHER_062", CharacterCode: "PL1700"},
		{ArchiveID: "ARC_OTHER_063", CharacterCode: "PL2200"},
		{ArchiveID: "ARC_OTHER_064", CharacterCode: "PL2300"},
		{ArchiveID: "ARC_OTHER_065", CharacterCode: "PL2100"},
		{ArchiveID: "ARC_OTHER_066", CharacterCode: "PL2600"},
		{ArchiveID: "ARC_OTHER_067", CharacterCode: "PL2700"},
		{ArchiveID: "ARC_OTHER_068", CharacterCode: "PL2800", MissionCode: "00301028", RequiredQuestID: "0040A301"},
		{ArchiveID: "ARC_OTHER_069", CharacterCode: "PL2900", MissionCode: "00301029", RequiredQuestID: "0040A306"},
		{ArchiveID: "ARC_OTHER_070", CharacterCode: "PL2400"},
		{ArchiveID: "ARC_OTHER_071", CharacterCode: "PL2500"},
	}
	for index := range rows {
		row := &rows[index]
		row.EpisodeKey = fmt.Sprintf("FATE_%s_10", row.CharacterCode)
		if entry := layout.stateByHash[gbfrHash32(row.EpisodeKey)]; entry != nil {
			row.EpisodeCompleted = entry.Uint32() == fateCompletedState
		}
	}
	return rows
}

type FateArchiveVectorObservation struct {
	IDType    uint32 `json:"idType"`
	Values    []int  `json:"values,omitempty"`
	Available bool   `json:"available"`
}

func fateArchiveVectorObservations(save *SaveData) []FateArchiveVectorObservation {
	// Bounded, read-only candidate journal/progression vectors for paired-save
	// comparison. No semantic claim or writer is derived from these samples.
	definitions := []struct {
		id    uint32
		count int
	}{
		{2554, 200}, {2555, 200}, {2575, 512}, {2576, 512}, {2577, 1536}, {2578, 512},
	}
	rows := make([]FateArchiveVectorObservation, 0, len(definitions))
	for _, definition := range definitions {
		row := FateArchiveVectorObservation{IDType: definition.id}
		entry, err := findVectorUnitFast(save, definition.id, 0, definition.count)
		if err == nil && len(entry.Bytes()) == definition.count {
			row.Available = true
			row.Values = make([]int, definition.count)
			for index, value := range entry.Bytes() {
				row.Values[index] = int(value)
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
	if vector, vectorErr := requireFateStoryArchiveVector(save); vectorErr == nil {
		payload.StoryArchives, err = inspectFateStoryArchives(layout, vector)
		if err != nil {
			return nil, err
		}
	}
	return json.MarshalIndent(payload, "", "  ")
}

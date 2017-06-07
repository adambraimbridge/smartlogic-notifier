package smartlogic

type Graph struct {
	Changesets []Changeset `json:"@graph"`
}

type Changeset struct {
	Concepts []ChangedConcept `json:"sem:about"`
}

type ChangedConcept struct {
	URI string `json:"@id"`
}

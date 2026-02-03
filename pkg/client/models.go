package client

type Group struct {
	ID             *string              `json:"id,omitempty"`
	Name           *string              `json:"name,omitempty"`
	Path           *string              `json:"path,omitempty"`
	SubGroups      *[]Group             `json:"subGroups,omitempty"`
	SubGroupsCount *int                 `json:"subGroupCount,omitempty"`
	Attributes     *map[string][]string `json:"attributes,omitempty"`
	Access         *map[string]bool     `json:"access,omitempty"`
	ClientRoles    *map[string][]string `json:"clientRoles,omitempty"`
	RealmRoles     *[]string            `json:"realmRoles,omitempty"`
}

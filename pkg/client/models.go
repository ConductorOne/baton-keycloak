package client

// Group represents a Keycloak group with correct JSON field mappings.
type Group struct {
	ID            *string              `json:"id,omitempty"`
	Name          *string              `json:"name,omitempty"`
	Description   *string              `json:"description,omitempty"`
	Path          *string              `json:"path,omitempty"`
	SubGroups     *[]Group             `json:"subGroups,omitempty"`
	SubGroupCount *int                 `json:"subGroupCount,omitempty"` // This is available from Keycloak 23 or higher.
	Attributes    *map[string][]string `json:"attributes,omitempty"`
	Access        *map[string]bool     `json:"access,omitempty"`
	ClientRoles   *map[string][]string `json:"clientRoles,omitempty"`
	RealmRoles    *[]string            `json:"realmRoles,omitempty"`
}

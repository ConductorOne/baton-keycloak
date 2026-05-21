package connector

import (
	v2 "github.com/conductorone/baton-sdk/pb/c1/connector/v2"
	"github.com/conductorone/baton-sdk/pkg/annotations"
)

var userResourceType = &v2.ResourceType{
	Id:          "user",
	DisplayName: "User",
	Traits:      []v2.ResourceType_Trait{v2.ResourceType_TRAIT_USER},
	Annotations: annotations.New(&v2.SkipEntitlementsAndGrants{}),
}

var groupResourceType = &v2.ResourceType{
	Id:          "group",
	DisplayName: "Group",
	Traits:      []v2.ResourceType_Trait{v2.ResourceType_TRAIT_GROUP},
}

var realmRoleResourceType = &v2.ResourceType{
	Id:          "realm_role",
	DisplayName: "Realm Role",
	Traits:      []v2.ResourceType_Trait{v2.ResourceType_TRAIT_ROLE},
}

var clientResourceType = &v2.ResourceType{
	Id:          "client",
	DisplayName: "Client",
	Traits:      []v2.ResourceType_Trait{v2.ResourceType_TRAIT_APP},
	Annotations: annotations.New(&v2.SkipEntitlementsAndGrants{}),
}

var clientRoleResourceType = &v2.ResourceType{
	Id:          "client_role",
	DisplayName: "Client Role",
	Traits:      []v2.ResourceType_Trait{v2.ResourceType_TRAIT_ROLE},
}

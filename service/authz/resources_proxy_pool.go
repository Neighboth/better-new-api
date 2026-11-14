package authz

const ResourceProxyPool = "proxy_pool"

var (
	ProxyPoolRead  = Permission{Resource: ResourceProxyPool, Action: ActionRead}
	ProxyPoolWrite = Permission{Resource: ResourceProxyPool, Action: ActionWrite}
)

func init() {
	RegisterResource(ResourceDefinition{
		Resource: ResourceProxyPool,
		LabelKey: "Proxy Pool",
		Actions: []ActionDefinition{
			{
				Action:         ActionRead,
				LabelKey:       "Read proxy pool",
				DescriptionKey: "View registered proxies and their channel key assignments.",
				DefaultRoles:   []string{BuiltInRoleAdmin},
			},
			{
				Action:         ActionWrite,
				LabelKey:       "Edit proxy pool",
				DescriptionKey: "Create, update, delete or test proxies, and assign them to channel keys.",
				DefaultRoles:   []string{BuiltInRoleAdmin},
			},
		},
	})
}

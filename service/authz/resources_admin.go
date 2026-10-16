package authz

const (
	ResourceModel         = "model"
	ResourceUser          = "user"
	ResourceBlog          = "blog"
	ResourceRedemption    = "redemption"
	ResourceSubscription  = "subscription"
	ResourceSystemSetting = "system_setting"
	ResourceTicket        = "ticket"

	ActionDelete = "delete"
)

var (
	ModelRead  = Permission{Resource: ResourceModel, Action: ActionRead}
	ModelWrite = Permission{Resource: ResourceModel, Action: ActionWrite}

	UserRead   = Permission{Resource: ResourceUser, Action: ActionRead}
	UserWrite  = Permission{Resource: ResourceUser, Action: ActionWrite}
	UserDelete = Permission{Resource: ResourceUser, Action: ActionDelete}

	BlogRead  = Permission{Resource: ResourceBlog, Action: ActionRead}
	BlogWrite = Permission{Resource: ResourceBlog, Action: ActionWrite}

	RedemptionRead  = Permission{Resource: ResourceRedemption, Action: ActionRead}
	RedemptionWrite = Permission{Resource: ResourceRedemption, Action: ActionWrite}

	SubscriptionRead  = Permission{Resource: ResourceSubscription, Action: ActionRead}
	SubscriptionWrite = Permission{Resource: ResourceSubscription, Action: ActionWrite}

	SystemSettingRead  = Permission{Resource: ResourceSystemSetting, Action: ActionRead}
	SystemSettingWrite = Permission{Resource: ResourceSystemSetting, Action: ActionWrite}

	TicketRead   = Permission{Resource: ResourceTicket, Action: ActionRead}
	TicketWrite  = Permission{Resource: ResourceTicket, Action: ActionWrite}
	TicketDelete = Permission{Resource: ResourceTicket, Action: ActionDelete}
)

func init() {
	RegisterResource(ResourceDefinition{
		Resource: ResourceModel,
		LabelKey: "Model Management",
		Actions: []ActionDefinition{
			{
				Action:         ActionRead,
				LabelKey:       "Read models",
				DescriptionKey: "View models list, details, and pricing configurations.",
				DefaultRoles:   []string{BuiltInRoleAdmin},
			},
			{
				Action:         ActionWrite,
				LabelKey:       "Manage models",
				DescriptionKey: "Create, edit, sync, and delete models, configure pricing and ratios.",
				DefaultRoles:   []string{BuiltInRoleAdmin},
			},
		},
	})

	RegisterResource(ResourceDefinition{
		Resource: ResourceUser,
		LabelKey: "User Management",
		Actions: []ActionDefinition{
			{
				Action:         ActionRead,
				LabelKey:       "Read users",
				DescriptionKey: "View user lists and user profiles.",
				DefaultRoles:   []string{BuiltInRoleAdmin},
			},
			{
				Action:         ActionWrite,
				LabelKey:       "Edit users",
				DescriptionKey: "Create and update user accounts, modify status and quota.",
				DefaultRoles:   []string{BuiltInRoleAdmin},
			},
			{
				Action:         ActionDelete,
				LabelKey:       "Delete users",
				DescriptionKey: "Delete user accounts from the system.",
				DefaultRoles:   []string{BuiltInRoleAdmin},
			},
		},
	})

	RegisterResource(ResourceDefinition{
		Resource: ResourceBlog,
		LabelKey: "Blog Management",
		Actions: []ActionDefinition{
			{
				Action:         ActionRead,
				LabelKey:       "Read blog posts",
				DescriptionKey: "View blog drafts and management list.",
				DefaultRoles:   []string{BuiltInRoleAdmin},
			},
			{
				Action:         ActionWrite,
				LabelKey:       "Manage blog posts",
				DescriptionKey: "Create, generate with AI, edit, publish, and delete blog posts.",
				DefaultRoles:   []string{BuiltInRoleAdmin},
			},
		},
	})

	RegisterResource(ResourceDefinition{
		Resource: ResourceRedemption,
		LabelKey: "Redemption Code Management",
		Actions: []ActionDefinition{
			{
				Action:         ActionRead,
				LabelKey:       "Read redemption codes",
				DescriptionKey: "View redemption codes and statuses.",
				DefaultRoles:   []string{BuiltInRoleAdmin},
			},
			{
				Action:         ActionWrite,
				LabelKey:       "Manage redemption codes",
				DescriptionKey: "Create, enable, disable, and delete redemption codes.",
				DefaultRoles:   []string{BuiltInRoleAdmin},
			},
		},
	})

	RegisterResource(ResourceDefinition{
		Resource: ResourceSubscription,
		LabelKey: "Subscription Management",
		Actions: []ActionDefinition{
			{
				Action:         ActionRead,
				LabelKey:       "Read subscriptions",
				DescriptionKey: "View subscription plans, orders, and records.",
				DefaultRoles:   []string{BuiltInRoleAdmin},
			},
			{
				Action:         ActionWrite,
				LabelKey:       "Manage subscriptions",
				DescriptionKey: "Create and update subscription plans, manage user subscriptions.",
				DefaultRoles:   []string{BuiltInRoleAdmin},
			},
		},
	})

	RegisterResource(ResourceDefinition{
		Resource: ResourceSystemSetting,
		LabelKey: "System Settings",
		Actions: []ActionDefinition{
			{
				Action:         ActionRead,
				LabelKey:       "Read system settings",
				DescriptionKey: "View system configurations and options.",
				DefaultRoles:   []string{BuiltInRoleAdmin},
			},
			{
				Action:         ActionWrite,
				LabelKey:       "Manage system settings",
				DescriptionKey: "Update system options, branding, and platform settings.",
				DefaultRoles:   []string{BuiltInRoleAdmin},
			},
		},
	})

	RegisterResource(ResourceDefinition{
		Resource: ResourceTicket,
		LabelKey: "Ticket Management",
		Actions: []ActionDefinition{
			{
				Action:         ActionRead,
				LabelKey:       "Read tickets",
				DescriptionKey: "View support tickets and messages.",
				DefaultRoles:   []string{BuiltInRoleAdmin},
			},
			{
				Action:         ActionWrite,
				LabelKey:       "Manage tickets",
				DescriptionKey: "Reply to tickets and update ticket status.",
				DefaultRoles:   []string{BuiltInRoleAdmin},
			},
			{
				Action:         ActionDelete,
				LabelKey:       "Delete tickets",
				DescriptionKey: "Permanently delete closed tickets and transcripts.",
				DefaultRoles:   []string{BuiltInRoleRoot},
			},
		},
	})
}


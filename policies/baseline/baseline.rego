package normgate.baseline

# Every configured layer must publish a default and a complete decision array.
default decisions := []
decisions := [entry | some entry in denied] if count(denied) > 0
decisions := [{"rule_id": "baseline_allow", "outcome": "allow", "reason_codes": ["ng.policy.baseline_allow"], "obligations": []}] if count(denied) == 0

denied contains {"rule_id": "identity_unknown", "outcome": "deny", "reason_codes": ["ng.policy.identity_unknown"], "obligations": []} if {
 not identity_registered
}
identity_registered if {
 some identity in data.config.identities
 identity.id == input.principal.id
 identity.tenant_id == input.principal.tenant_id
 identity.kind == input.principal.kind
}
denied contains {"rule_id": "tenant_mismatch", "outcome": "deny", "reason_codes": ["ng.policy.tenant_mismatch"], "obligations": []} if {
 some resource in input.operation.resources
 resource.tenant_id != input.principal.tenant_id
}
denied contains {"rule_id": "operation_unknown", "outcome": "deny", "reason_codes": ["ng.policy.operation_unknown"], "obligations": []} if {
 not operation_registered
}
operation_registered if {
 input.operation.kind in data.config.operation_kinds
 input.operation.name in data.config.operation_names
}
denied contains {"rule_id": "destination_unknown", "outcome": "deny", "reason_codes": ["ng.policy.destination_unknown"], "obligations": []} if {
 not destination_registered
}
destination_registered if {
 some destination in data.config.destinations
 destination.id == input.operation.destination.id
 destination.uri == input.operation.destination.uri
 destination.kind == input.operation.destination.kind
}
denied contains {"rule_id": "purpose_undeclared", "outcome": "deny", "reason_codes": ["ng.policy.purpose_undeclared"], "obligations": []} if {
 not input.operation.purpose in data.config.purposes
}
denied contains {"rule_id": "context_incomplete", "outcome": "deny", "reason_codes": ["ng.policy.context_incomplete"], "obligations": []} if {
 not context_registered
}
context_registered if {
 some application in data.config.applications
 application.id == input.context.application_id
 input.context.environment in application.environments
}

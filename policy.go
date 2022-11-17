package mapi

func GetPolicy(policy *Policy, key string) interface{} {
	policies := *policy.Policies
	p, ok := policies[key]
	if !ok {
		return nil
	}

	return p
}

func GetPolicyInt(policy *Policy, key string) int {
	policyValue := GetPolicy(policy, key)
	if policyValue == nil {
		return 0
	}

	switch p := policyValue.(type) {
	case int:
		return p
	default:
		return 0
	}
}

func GetPolicyString(policy *Policy, key string) string {
	policyValue := GetPolicy(policy, key)
	if policyValue == nil {
		return ""
	}

	switch p := policyValue.(type) {
	case string:
		return p
	default:
		return ""
	}
}

func GetPolicySlice(policy *Policy, key string) []string {
	policyValue := GetPolicy(policy, key)
	if policyValue == nil {
		return []string{}
	}

	switch p := policyValue.(type) {
	case []string:
		return p
	default:
		return []string{}
	}
}

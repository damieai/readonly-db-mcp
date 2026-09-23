package elasticsearch

import (
	"encoding/json"
	"strings"
	"unicode"

	"github.com/your-org/readonly-db-mcp/internal/config"
)

// Remote grants may coexist with a local read identity. They are proved here
// solely as read-only grants: no remote expression is admitted by the query
// source resolver until a separate remote key, topology and scope profile is
// attested. Do not reuse this result alone to authorize cross-cluster search.
type remoteIndexPrivilege struct {
	Clusters        oneOrManyStrings `json:"clusters"`
	Names           oneOrManyStrings `json:"names"`
	Privileges      []string         `json:"privileges"`
	FieldSecurity   json.RawMessage  `json:"field_security,omitempty"`
	Query           json.RawMessage  `json:"query,omitempty"`
	AllowRestricted *bool            `json:"allow_restricted_indices,omitempty"`
}

type remoteClusterPrivilege struct {
	Clusters   oneOrManyStrings `json:"clusters"`
	Privileges []string         `json:"privileges"`
}

type oneOrManyStrings []string

func (names *oneOrManyStrings) UnmarshalJSON(raw []byte) error {
	var one string
	if err := json.Unmarshal(raw, &one); err == nil {
		*names = []string{one}
		return nil
	}
	var many []string
	if err := json.Unmarshal(raw, &many); err != nil {
		return err
	}
	*names = many
	return nil
}

func validateRemotePrivileges(indices, clusters []json.RawMessage) error {
	if len(indices)+len(clusters) > 128 {
		return failure("authority_unproven", "remote privilege entry count exceeds proof limit")
	}
	for _, raw := range indices {
		var grant remoteIndexPrivilege
		if err := decodeProof(raw, &grant); err != nil {
			return err
		}
		if !validRemoteAliases(grant.Clusters) || len(grant.Names) == 0 || len(grant.Names) > 64 || len(grant.Privileges) == 0 || grant.AllowRestricted != nil && *grant.AllowRestricted {
			return failure("authority_unproven", "remote index grant is incomplete or includes restricted indices")
		}
		for _, name := range grant.Names {
			if !config.ValidElasticsearchPattern(name) {
				return failure("authority_unproven", "remote index grant has an unsupported selector")
			}
		}
		for _, privilege := range grant.Privileges {
			switch privilege {
			case "read", "read_cross_cluster", "view_index_metadata", "none":
			default:
				return failure("authority_unproven", "remote index grant exceeds read-only privileges")
			}
		}
	}
	for _, raw := range clusters {
		var grant remoteClusterPrivilege
		if err := decodeProof(raw, &grant); err != nil {
			return err
		}
		if !validRemoteAliases(grant.Clusters) || len(grant.Privileges) == 0 {
			return failure("authority_unproven", "remote cluster grant is incomplete")
		}
		for _, privilege := range grant.Privileges {
			switch privilege {
			case "monitor_enrich", "monitor_stats":
			default:
				return failure("authority_unproven", "remote cluster grant exceeds read-only privileges")
			}
		}
	}
	return nil
}

func validRemoteAliases(aliases []string) bool {
	if len(aliases) == 0 || len(aliases) > 64 {
		return false
	}
	for _, alias := range aliases {
		if !validRemoteAliasPattern(alias) {
			return false
		}
	}
	return true
}

func validRemoteAliasPattern(alias string) bool {
	if alias == "" || len(alias) > 255 || alias == "." || alias == ".." || strings.HasPrefix(alias, "-") {
		return false
	}
	for _, r := range alias {
		if unicode.IsSpace(r) || unicode.IsControl(r) || strings.ContainsRune(`/\,:#%&[]=<>"|`, r) {
			return false
		}
	}
	return true
}

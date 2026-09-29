// Package share defines the versioned browser sharing wire format. Browser
// adapters own storage and platform behavior; CTX uses these types to bridge
// compatible resources across adapters.
package share

import "encoding/json"

// Version is the browser share request and bundle wire version.
const Version = 2

type Cookie struct {
	ID                int64  `json:"id,omitempty"`
	Ref               string `json:"ref,omitempty"`
	Name              string `json:"name"`
	Value             string `json:"value"`
	Domain            string `json:"domain"`
	Path              string `json:"path"`
	Expiry            int64  `json:"expiry"`
	Secure            bool   `json:"secure"`
	HTTPOnly          bool   `json:"http_only"`
	SameSitePolicy    string `json:"same_site_policy,omitempty"`
	PartitionKey      string `json:"partition_key,omitempty"`
	CrossSiteAncestor bool   `json:"has_cross_site_ancestor,omitempty"`
	// Attributes contains optional adapter-owned fields with namespaced keys.
	// Importers reject fields they cannot preserve.
	Attributes map[string]string `json:"attributes,omitempty"`
}

type CookieBundle struct {
	Version int    `json:"version"`
	Source  string `json:"source"`
	Site    string `json:"site"`
	Cookie  Cookie `json:"cookie"`
}

type CookieRequest struct {
	Version int           `json:"version"`
	Site    string        `json:"site,omitempty"`
	Cookie  Cookie        `json:"cookie,omitempty"`
	Bundle  *CookieBundle `json:"bundle,omitempty"`
	Replace bool          `json:"replace,omitempty"`
}

type ResourceBundle struct {
	Version  int             `json:"version"`
	Resource string          `json:"resource"`
	Source   string          `json:"source"`
	Payload  json.RawMessage `json:"payload"`
}

type ResourceRequest struct {
	Version int             `json:"version"`
	Args    []string        `json:"args,omitempty"`
	Bundle  *ResourceBundle `json:"bundle,omitempty"`
	Replace bool            `json:"replace,omitempty"`
}

type PolicyEntry struct {
	Location string `json:"location"`
	Level    string `json:"level"`
	Format   string `json:"format"`
	Content  string `json:"content"`
}

type PolicyRequest struct {
	Version int `json:"version"`
}

type PolicyBundle struct {
	Version int           `json:"version"`
	Source  string        `json:"source"`
	Entries []PolicyEntry `json:"entries"`
}

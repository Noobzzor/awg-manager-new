package connections

// Connection represents a single conntrack entry with resolved tunnel and client info.
type Connection struct {
	Protocol   string    `json:"protocol"`
	Src        string    `json:"src"`
	Dst        string    `json:"dst"`
	SrcPort    int       `json:"srcPort"`
	DstPort    int       `json:"dstPort"`
	State      string    `json:"state"`
	Packets    int64     `json:"packets"`
	Bytes      int64     `json:"bytes"`
	BytesIn    int64     `json:"bytesIn"`
	BytesOut   int64     `json:"bytesOut"`
	TTL        int64     `json:"ttl"`
	RouteClass string    `json:"routeClass"`
	Interface  string    `json:"interface"`
	TunnelID   string    `json:"tunnelId"`
	TunnelName string    `json:"tunnelName"`
	ClientMAC  string    `json:"clientMac"`
	ClientName string    `json:"clientName"`
	Rules      []RuleHit `json:"rules,omitempty"`
}

// ConnectionStats holds aggregate counts over all connections (pre-filter).
type ConnectionStats struct {
	Total     int           `json:"total"`
	Direct    int           `json:"direct"`
	Tunneled  int           `json:"tunneled"`
	Singbox   int           `json:"singbox"`
	Local     int           `json:"local"`
	Protocols ProtocolStats `json:"protocols"`
}

// ProtocolStats breaks down connection count by protocol.
type ProtocolStats struct {
	TCP  int `json:"tcp"`
	UDP  int `json:"udp"`
	ICMP int `json:"icmp"`
}

// Bucket is one row of a breakdown aggregate for the donut panels.
type Bucket struct {
	Key      string `json:"key"`
	Label    string `json:"label,omitempty"`
	Count    int    `json:"count"`
	BytesIn  int64  `json:"bytesIn"`
	BytesOut int64  `json:"bytesOut"`
}

// TunnelConnectionInfo describes a tunnel's connection count for the summary.
type TunnelConnectionInfo struct {
	Name      string `json:"name"`
	Interface string `json:"interface"`
	Count     int    `json:"count"`
}

// ListParams holds filtering, sorting, and pagination parameters.
type ListParams struct {
	Tunnel   string // "all", "direct", or tunnel ID
	Protocol string // "all", "tcp", "udp", "icmp"
	State    string // "all"/"" = любое; иначе точное значение (ESTABLISHED, ...)
	Search   string // substring match on src, dst, clientName
	Offset   int
	Limit    int    // default 50, max 500
	SortBy   string // "" | "proto" | "src" | "dst" | "iface" | "state" | "bytes"
	SortDir  string // "asc" | "desc" — defaults to "asc" if SortBy is set
}

// ListResponse is returned by Service.List().
type ListResponse struct {
	Stats       ConnectionStats                 `json:"stats"`
	Tunnels     map[string]TunnelConnectionInfo `json:"tunnels"`
	ByTunnel    []Bucket                        `json:"byTunnel"`
	ByClient    []Bucket                        `json:"byClient"`
	ByDst       []Bucket                        `json:"byDst"`
	Connections []Connection                    `json:"connections"`
	Pagination  PaginationInfo                  `json:"pagination"`
	FetchedAt   string                          `json:"fetchedAt"`
}

// PaginationInfo describes the current page of results.
type PaginationInfo struct {
	Total    int `json:"total"`
	Offset   int `json:"offset"`
	Limit    int `json:"limit"`
	Returned int `json:"returned"`
}

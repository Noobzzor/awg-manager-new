package connections

import "sort"

const bucketTopN = 10

// computeBuckets aggregates conns by the keyFn extractor, sorts by count
// desc and rolls everything past top-N into a single "@other" bucket.
// Считается по всем соединениям до фильтров — донаты честны при пагинации.
func computeBuckets(conns []Connection, keyFn func(Connection) (key, label string)) []Bucket {
	idx := make(map[string]int)
	var out []Bucket
	for _, c := range conns {
		key, label := keyFn(c)
		if key == "" {
			continue
		}
		i, ok := idx[key]
		if !ok {
			i = len(out)
			idx[key] = i
			out = append(out, Bucket{Key: key, Label: label})
		}
		out[i].Count++
		out[i].BytesIn += c.BytesIn
		out[i].BytesOut += c.BytesOut
	}
	sort.SliceStable(out, func(i, j int) bool { return out[i].Count > out[j].Count })
	if len(out) > bucketTopN {
		other := Bucket{Key: "@other", Label: "Прочее"}
		for _, b := range out[bucketTopN:] {
			other.Count += b.Count
			other.BytesIn += b.BytesIn
			other.BytesOut += b.BytesOut
		}
		out = append(out[:bucketTopN], other)
	}
	if out == nil {
		out = []Bucket{}
	}
	return out
}

func tunnelBucketKey(c Connection) (string, string) {
	switch c.RouteClass {
	case "tunnel":
		return c.TunnelID, c.TunnelName
	case "singbox":
		return "@singbox", "sing-box"
	case "local":
		return "@local", "Локально"
	default:
		return "@direct", "Напрямую"
	}
}

func clientBucketKey(c Connection) (string, string) {
	if c.ClientName != "" {
		return c.ClientName, c.ClientName
	}
	return c.Src, ""
}

func dstBucketKey(c Connection) (string, string) {
	label := ""
	if len(c.Rules) > 0 {
		label = c.Rules[0].FQDN
	}
	return c.Dst, label
}
